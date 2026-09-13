package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	containerruntime "github.com/toozej/monogo/apps/gocicle/internal/runtime"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
)

type Runner struct {
	Client       *Client
	Runtime      containerruntime.Runtime
	StateDir, ID string
}

func (r *Runner) Run(ctx context.Context) error {
	if goruntime.GOOS != "linux" {
		return errors.New("runner requires Linux")
	}
	if !filepath.IsAbs(r.StateDir) {
		return errors.New("runner state directory must be absolute")
	}
	if err := os.MkdirAll(r.StateDir, 0700); err != nil {
		return err
	}
	release, err := lockState(r.StateDir)
	if err != nil {
		return err
	}
	defer release()
	if err := r.reconcile(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		_ = RecoverSpools(ctx, r.StateDir, r.Client)
		var assignment *protocol.Assignment
		if err := r.Client.Call(ctx, "POST", "/api/v1/runner/poll", "", nil, &assignment); err == nil && assignment != nil {
			if assignment.InspectionID != "" {
				_ = r.inspect(ctx, *assignment)
			} else {
				_ = r.execute(ctx, *assignment)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (r *Runner) reconcile(ctx context.Context) error {
	containers, err := r.Runtime.List(ctx, r.ID)
	if err != nil {
		return err
	}
	for _, container := range containers {
		_ = r.Runtime.Stop(ctx, container.ID)
		if err := r.Runtime.Remove(ctx, container.ID); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(r.StateDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "workspace-") {
			if err := os.RemoveAll(filepath.Join(r.StateDir, e.Name())); err != nil {
				return err
			}
		}
		if strings.HasPrefix(e.Name(), "image-") {
			b, err := os.ReadFile(filepath.Join(r.StateDir, e.Name()))
			if err != nil {
				return err
			}
			if err := r.Runtime.RemoveImage(ctx, string(b)); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(r.StateDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Runner) execute(parent context.Context, a protocol.Assignment) error {
	if err := a.Spec.Validate(a.Source); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(parent, a.Deadline)
	defer cancel()
	spool, err := NewSpool(r.StateDir, r.Client, a.RunID, a.LeaseToken)
	if err != nil {
		return err
	}
	defer func() {
		if spool.Empty() {
			_ = spool.Close()
		}
	}()
	started := time.Now()
	result := protocol.Completion{Status: "failure", ExitCode: -1, PushResult: "disabled"}
	var secrets []string
	for _, c := range a.Credentials {
		secrets = append(secrets, c.Value, c.Password, c.PrivateKey)
	}
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		ticker := time.NewTicker(protocol.HeartbeatInterval)
		defer ticker.Stop()
		expiry := a.ExpiresAt
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var reply struct {
					ExpiresAt time.Time `json:"expiresAt"`
					Cancel    bool      `json:"cancel"`
				}
				requestCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				err := r.Client.Call(requestCtx, "POST", "/api/v1/runner/runs/"+a.RunID+"/heartbeat", a.LeaseToken, nil, &reply)
				stop()
				if err == nil {
					if reply.Cancel {
						cancel()
						return
					}
					expiry = reply.ExpiresAt
				} else if time.Until(expiry) <= protocol.HeartbeatInterval {
					cancel()
					return
				}
				flushCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				_ = spool.Flush(flushCtx)
				stop()
			}
		}
	}()
	defer func() { cancel(); heartbeat.Wait() }()
	executionErr := func() error {
		workspace, err := Prepare(ctx, r.StateDir, a)
		if err != nil {
			return workspaceError("checkout", err)
		}
		defer func() { _ = workspace.Close() }()
		result.SourceCommit = workspace.Commit
		var image string
		if a.Spec.Container.Image != "" {
			image, err = r.Runtime.Pull(ctx, a.Spec.Container.Image)
		} else {
			tag := "gocicle-run-" + r.ID + "-" + a.RunID
			journal := filepath.Join(r.StateDir, "image-"+a.RunID)
			if err = os.WriteFile(journal, []byte(tag), 0600); err != nil {
				return err
			}
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
				defer stop()
				if r.Runtime.RemoveImage(cleanup, tag) == nil {
					_ = os.Remove(journal)
				}
			}()
			image, err = r.Runtime.Build(ctx, workspace.Path, a.Spec.Container.BuildContext, a.Spec.Container.Dockerfile, tag)
		}
		if err != nil {
			return workspaceError("image", err)
		}
		result.ImageIdentity = image
		environment := make([]string, 0, len(a.Spec.Environment)+len(a.Spec.Secrets))
		for k, v := range a.Spec.Environment {
			environment = append(environment, k+"="+v)
		}
		for k, id := range a.Spec.Secrets {
			c, ok := a.Credentials[id]
			if !ok {
				return errors.New("environment credential is missing")
			}
			environment = append(environment, k+"="+c.Value)
		}
		sort.Strings(environment)
		mount := "/workspace"
		readonly := false
		if a.Source.Local != nil {
			mount = a.Source.Local.ContainerPath
			readonly = a.Source.Local.ReadOnly
		}
		id, err := r.Runtime.Create(ctx, containerruntime.CreateRequest{RunID: a.RunID, RunnerID: r.ID, Image: image, Workspace: workspace.Path, MountPath: mount, ReadOnly: readonly, Spec: a.Spec, Environment: environment})
		if err != nil {
			return workspaceError("container creation", err)
		}
		defer func() {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			_ = r.Runtime.Stop(cleanup, id)
			_ = r.Runtime.Remove(cleanup, id)
		}()
		if err := r.Runtime.Start(ctx, id); err != nil {
			return workspaceError("container start", err)
		}
		metrics, metricErr := r.Runtime.Stats(ctx, id)
		if metricErr != nil {
			metrics = protocol.Metrics{At: time.Now(), Available: false}
		}
		if err := spool.Add("metrics", metrics); err != nil {
			return err
		}
		writer := &logWriter{spool: spool, secrets: secrets}
		logsDone := make(chan error, 1)
		logsCtx, stopLogs := context.WithCancel(ctx)
		logsFinished := false
		defer func() {
			stopLogs()
			if !logsFinished {
				<-logsDone
			}
		}()
		go func() { logsDone <- r.Runtime.Logs(logsCtx, id, writer) }()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		samples := 0
		for {
			select {
			case <-ctx.Done():
				stopLogs()
				<-logsDone
				logsFinished = true
				return ctx.Err()
			case <-ticker.C:
				container, err := r.Runtime.Inspect(ctx, id)
				if err != nil {
					return err
				}
				samples++
				if samples%3 == 0 {
					metrics, err := r.Runtime.Stats(ctx, id)
					if err != nil {
						metrics = protocol.Metrics{At: time.Now(), Available: false}
					}
					if err := spool.Add("metrics", metrics); err != nil {
						return err
					}
				}
				if !container.Running {
					select {
					case err := <-logsDone:
						logsFinished = true
						if err != nil {
							return err
						}
					case <-ctx.Done():
						return ctx.Err()
					}
					if err := writer.Close(); err != nil {
						return err
					}
					result.ExitCode = container.ExitCode
					if container.ExitCode != 0 {
						return errors.New("job command failed")
					}
					result.Status = "success"
					if a.Spec.PushChanges {
						// Renew the lease and recheck revoked grants immediately before a push.
						var reply struct {
							Cancel bool `json:"cancel"`
						}
						if err := r.Client.Call(ctx, "POST", "/api/v1/runner/runs/"+a.RunID+"/heartbeat", a.LeaseToken, nil, &reply); err != nil || reply.Cancel {
							return errors.New("push authorization is unavailable")
						}
						result.PushResult, err = workspace.Push(ctx, *a.Preferences.CommitName, *a.Preferences.CommitEmail)
						if err != nil {
							result.Status = "push_failure"
							return err
						}
					}
					return nil
				}
			}
		}
	}()
	if executionErr != nil {
		result.Error = security.Redact(executionErr.Error(), secrets)
		if result.Status == "success" {
			result.Status = "failure"
		}
	}
	if ctx.Err() != nil {
		result.Status = "cancelled"
		if time.Now().After(a.Deadline) {
			result.Status = "timeout"
		}
	}
	result.Duration = time.Since(started).Seconds()
	if err := spool.Add("complete", result); err != nil {
		return err
	}
	reportCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	return spool.Flush(reportCtx)
}

// Retain a suffix so a credential split across runtime frames cannot escape redaction.
type logWriter struct {
	spool     *Spool
	secrets   []string
	pending   string
	total     int
	truncated bool
}

func (w *logWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.truncated {
		return n, nil
	}
	w.pending += string(p)
	hold := 1
	for _, secret := range w.secrets {
		if len(secret) > hold {
			hold = len(secret)
		}
	}
	if len(w.pending) > hold {
		cut := len(w.pending) - hold
		for {
			previous := cut
			for _, secret := range w.secrets {
				if secret == "" {
					continue
				}
				start := 0
				for start < len(w.pending) {
					relative := strings.Index(w.pending[start:], secret)
					if relative < 0 {
						break
					}
					at := start + relative
					if at >= cut {
						break
					}
					if at+len(secret) > cut {
						cut = at
					}
					start = at + 1
				}
			}
			if cut == previous {
				break
			}
		}
		if cut > 0 {
			err := w.emit(w.pending[:cut])
			w.pending = w.pending[cut:]
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}
func (w *logWriter) emit(value string) error {
	value = security.Redact(value, w.secrets)
	if w.total+len(value) > protocol.MaxLogBytes {
		value = value[:protocol.MaxLogBytes-w.total]
		w.truncated = true
		if err := w.spool.Add("truncated", true); err != nil {
			return err
		}
	}
	w.total += len(value)
	for len(value) > 0 {
		n := len(value)
		if n > 8192 {
			n = 8192
		}
		if err := w.spool.Add("log", value[:n]); err != nil {
			return err
		}
		value = value[n:]
	}
	return nil
}
func (w *logWriter) Close() error { return w.emit(w.pending) }

// SaveEnrollment stores the revocable runner credential with private permissions.
func SaveEnrollment(path, id, token string) error {
	b, err := json.Marshal(map[string]string{"id": id, "token": token})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}
