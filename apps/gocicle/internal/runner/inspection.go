package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	containerruntime "github.com/toozej/monogo/apps/gocicle/internal/runtime"
)

func (r *Runner) inspect(parent context.Context, a protocol.Assignment) error {
	ctx, cancel := context.WithDeadline(parent, a.Deadline)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(protocol.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if r.Client.Call(ctx, "POST", "/api/v1/runner/inspections/"+a.InspectionID+"/heartbeat", a.LeaseToken, nil, nil) != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	var text string
	inspectionErr := func() error {
		workspace, err := Prepare(ctx, r.StateDir, a)
		if err != nil {
			return err
		}
		defer func() { _ = workspace.Close() }()
		path, err := containerruntime.Resolve(workspace.Path, ".gocicle.yaml")
		if err != nil {
			return err
		}
		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		data, err := io.ReadAll(io.LimitReader(file, jobs.MaxDocument+1))
		if err != nil {
			return err
		}
		if _, err := jobs.Parse(data); err != nil {
			return err
		}
		text = string(data)
		return nil
	}()
	message := ""
	if inspectionErr != nil {
		message = "repository inspection failed"
	}
	return r.Client.Call(ctx, "POST", "/api/v1/runner/inspections/"+a.InspectionID+"/report", a.LeaseToken, map[string]string{"yaml": text, "error": message}, nil)
}
