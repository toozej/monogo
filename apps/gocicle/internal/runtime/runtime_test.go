package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
)

func TestPodmanFinishesLogsAfterMissedExitEvent(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	var inspections, snapshots atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			_ = json.NewEncoder(w).Encode(map[string]any{"State": map[string]bool{"Running": inspections.Add(1) == 1}})
			return
		}
		frame := func(data string) {
			header := make([]byte, 8)
			header[0] = 1
			binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
			_, _ = w.Write(append(header, data...))
		}
		frame("first\n")
		if r.URL.Query().Get("follow") == "true" {
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		snapshots.Add(1)
		frame("last\n")
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	runtime, err := NewPodman(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var logs bytes.Buffer
	if err := runtime.Logs(ctx, "run", &logs); err != nil {
		t.Fatal(err)
	}
	if logs.String() != "first\nlast\n" || snapshots.Load() != 1 {
		t.Fatalf("logs=%q snapshots=%d", logs.String(), snapshots.Load())
	}
}

func TestAdapters(t *testing.T) {
	for _, kind := range []string{"docker", "podman"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			socket := filepath.Join(dir, "s")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			var created map[string]any
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasPrefix(r.URL.Path, "/images/create"):
					_, _ = w.Write([]byte(`{"status":"done"}`))
				case strings.HasPrefix(r.URL.Path, "/images/"):
					_, _ = w.Write([]byte(`{"Id":"sha256:image"}`))
				case r.URL.Path == "/containers/create":
					if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
						t.Error(err)
					}
					_, _ = w.Write([]byte(`{"Id":"container"}`))
				case strings.HasSuffix(r.URL.Path, "/logs"):
					payload := []byte("hello\n")
					header := make([]byte, 8)
					header[0] = 1
					binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
					_, _ = w.Write(append(header, payload...))
				case strings.HasSuffix(r.URL.Path, "/stats"):
					_, _ = w.Write([]byte(`{"cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":400,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"memory_stats":{"usage":123},"networks":{"eth0":{"rx_bytes":12,"tx_bytes":34}}}`))
				case strings.HasSuffix(r.URL.Path, "/json"):
					_, _ = w.Write([]byte(`{"Id":"container","Image":"sha256:image","State":{"Running":false,"ExitCode":0}}`))
				default:
					w.WriteHeader(204)
				}
			})}
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close() })
			runtime, err := New(kind, socket)
			if err != nil {
				t.Fatal(err)
			}
			image, err := runtime.Pull(context.Background(), "alpine:3.22")
			if err != nil || image != "sha256:image" {
				t.Fatal(err)
			}
			spec := jobs.Spec{Command: jobs.Command{Path: "/bin/echo", Args: []string{"hello"}}}
			spec.Defaults()
			if _, err := runtime.Create(context.Background(), CreateRequest{RunID: "unsafe", RunnerID: "host", Image: image, Workspace: dir, MountPath: "/workspace", Spec: spec}); err == nil {
				t.Fatal("runtime socket was allowed inside the mount")
			}
			id, err := runtime.Create(context.Background(), CreateRequest{RunID: "run", RunnerID: "host", Image: image, Workspace: t.TempDir(), MountPath: "/workspace", Spec: spec})
			if err != nil {
				t.Fatal(err)
			}
			host := created["HostConfig"].(map[string]any)
			if host["Memory"] != float64(2<<30) || host["NanoCpus"] != float64(2e9) || host["PidsLimit"] != float64(256) {
				t.Fatalf("limits=%+v", host)
			}
			if kind == "podman" && host["UsernsMode"] != "keep-id" {
				t.Fatal("Podman UID mapping is missing")
			}
			if err := runtime.Start(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			if err := runtime.Logs(context.Background(), id, &logs); err != nil || logs.String() != "hello\n" {
				t.Fatalf("logs=%q error=%v", logs.String(), err)
			}
			metrics, err := runtime.Stats(context.Background(), id)
			if err != nil || !metrics.Available || *metrics.Memory != 123 || *metrics.CPU != 100 || *metrics.NetworkRX != 12 {
				t.Fatalf("metrics=%+v err=%v", metrics, err)
			}
			if err := runtime.Remove(context.Background(), id); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestResolveAndArchiveRejectEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, "escape"); err == nil {
		t.Fatal("symlink escaped root")
	}
	if _, err := Resolve(root, "../outside"); err == nil {
		t.Fatal("traversal escaped root")
	}
	var b bytes.Buffer
	if archive(root, &b) == nil {
		t.Fatal("build context retained a symlink")
	}
}
func TestRuntimeIntegration(t *testing.T) {
	socket := os.Getenv("GOCICLE_TEST_RUNTIME_SOCKET")
	if socket == "" {
		t.Skip("set GOCICLE_TEST_RUNTIME_SOCKET for a live runtime")
	}
	kind := os.Getenv("GOCICLE_TEST_RUNTIME")
	if kind == "" {
		kind = "docker"
	}
	runtime, err := New(kind, socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	workspace := t.TempDir()
	spec := jobs.Spec{Command: jobs.Command{Path: "/bin/sh", Args: []string{"-c", "echo runtime-ok; echo changed > result"}}}
	spec.Defaults()
	image, err := runtime.Pull(ctx, "alpine:3.22")
	if err != nil {
		t.Fatal(err)
	}
	execute := func(image string) {
		id, err := runtime.Create(ctx, CreateRequest{RunID: "integration-" + strconv.FormatInt(time.Now().UnixNano(), 10), RunnerID: "gocicle-test", Image: image, Workspace: workspace, MountPath: "/workspace", Spec: spec})
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := runtime.Remove(context.Background(), id); err != nil {
				t.Error(err)
			}
		}()
		if err := runtime.Start(ctx, id); err != nil {
			t.Fatal(err)
		}
		var logs bytes.Buffer
		if err := runtime.Logs(ctx, id, &logs); err != nil {
			t.Fatal(err)
		}
		container, err := runtime.Inspect(ctx, id)
		if err != nil || container.ExitCode != 0 || !strings.Contains(logs.String(), "runtime-ok") {
			t.Fatalf("container=%+v logs=%s error=%v", container, logs.String(), err)
		}
		if _, err := os.Stat(filepath.Join(workspace, "result")); err != nil {
			t.Fatal("local mount did not persist writes")
		}
	}
	execute(image)
	if err := os.WriteFile(filepath.Join(workspace, "Dockerfile"), []byte("FROM alpine:3.22\nRUN echo built > /built\n"), 0644); err != nil {
		t.Fatal(err)
	}
	built, err := runtime.Build(ctx, workspace, ".", "Dockerfile", "gocicle-run-test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.RemoveImage(context.Background(), built); err != nil {
			t.Error(err)
		}
	}()
	execute(built)
}
