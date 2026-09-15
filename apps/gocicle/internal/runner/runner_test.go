package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
)

func TestLocalSourceExcludesRunnerState(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	source := filepath.Join(root, "source")
	for _, dir := range []string{state, source} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	a := protocol.Assignment{Source: jobs.Source{Local: &jobs.Local{HostPath: root}}, ApprovedRoots: []string{root}}
	if _, err := Prepare(context.Background(), state, a); err == nil {
		t.Fatal("local mount included private runner state")
	}
	a.Source.Local.HostPath = source
	w, err := Prepare(context.Background(), state, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("cleanup removed the local source")
	}
}

func TestLocalSourceResolvesApprovedRoot(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	alias := filepath.Join(t.TempDir(), "approved")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	a := protocol.Assignment{Source: jobs.Source{Local: &jobs.Local{HostPath: filepath.Join(alias, "source")}}, ApprovedRoots: []string{alias}}
	w, err := Prepare(context.Background(), t.TempDir(), a)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	a.Source.Local.HostPath = filepath.Join(alias, "escape")
	if _, err := Prepare(context.Background(), t.TempDir(), a); err == nil {
		t.Fatal("local source escaped the resolved approved root")
	}
}

func TestSpoolRetriesAndRecovers(t *testing.T) {
	var attempts, accepted int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(503)
			return
		}
		var e protocol.Event
		if json.NewDecoder(r.Body).Decode(&e) != nil || e.Sequence != 1 {
			t.Error("invalid ordered event")
		}
		accepted++
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()
	client := &Client{URL: server.URL, Token: "runner", HTTP: server.Client()}
	dir := t.TempDir()
	s, err := NewSpool(dir, client, "run", "lease")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add("log", "hello"); err != nil {
		t.Fatal(err)
	}
	if s.Flush(context.Background()) == nil {
		t.Fatal("temporary server error was ignored")
	}
	if err := RecoverSpools(context.Background(), dir, client); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if accepted != 1 || len(entries) != 0 {
		t.Fatal("spool recovery did not finish")
	}
}
func TestRedactionAcrossFrames(t *testing.T) {
	s, err := NewSpool(t.TempDir(), nil, "run", "lease")
	if err != nil {
		t.Fatal(err)
	}
	writer := &logWriter{spool: s, secrets: []string{"very-secret-token"}}
	for _, part := range []string{"before very-", "secret-to", "ken after\n"} {
		if _, err := writer.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for _, event := range s.events {
		var content string
		_ = json.Unmarshal(event.Data, &content)
		output.WriteString(content)
	}
	if strings.Contains(output.String(), "very-secret-token") || !strings.Contains(output.String(), "[REDACTED]") {
		t.Fatalf("output=%s", output.String())
	}
}
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}
func TestGitFreshClonesAndPushes(t *testing.T) {
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("host Git is unavailable")
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	git(t, root, "init", "--bare", "--initial-branch=main", origin)
	git(t, origin, "config", "http.receivepack", "true")
	seed := filepath.Join(root, "seed")
	git(t, root, "clone", origin, seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("initial\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "initial")
	git(t, seed, "push", "origin", "main")
	server := httptest.NewServer(&cgi.Handler{Path: binary, Args: []string{"http-backend"}, Env: []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"}})
	defer server.Close()
	clone := func(branch string) *Workspace {
		t.Helper()
		dir := t.TempDir()
		w := &Workspace{Root: dir, Path: filepath.Join(dir, "source"), GitDir: filepath.Join(dir, "git"), URL: server.URL + "/origin.git", Branch: branch, env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
		if err := w.clone(context.Background()); err != nil {
			t.Fatal(err)
		}
		return w
	}
	first := clone("main")
	if _, err := os.Stat(filepath.Join(first.Path, ".git")); !os.IsNotExist(err) {
		t.Fatal("Git metadata remains in the mounted tree")
	}
	push, err := first.Push(context.Background(), "gocicle", "gocicle@example.com")
	if err != nil || push != "unchanged" {
		t.Fatalf("unchanged push=%s error=%v", push, err)
	}
	stale := clone("main")
	if err := os.WriteFile(filepath.Join(first.Path, "README"), []byte("updated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	push, err = first.Push(context.Background(), "gocicle", "gocicle@example.com")
	if err != nil || push != "pushed" {
		t.Fatalf("push=%s error=%v", push, err)
	}
	if err := os.WriteFile(filepath.Join(stale.Path, "README"), []byte("conflict\n"), 0644); err != nil {
		t.Fatal(err)
	}
	push, err = stale.Push(context.Background(), "gocicle", "gocicle@example.com")
	if err == nil || push != "rejected" {
		t.Fatal("non-fast-forward push was accepted")
	}
	latest := clone("main")
	data, err := os.ReadFile(filepath.Join(latest.Path, "README"))
	if err != nil || string(data) != "updated\n" {
		t.Fatal("fresh clone did not use the latest branch")
	}
	missing := &Workspace{Root: t.TempDir(), URL: server.URL + "/origin.git", Branch: "missing", env: []string{"PATH=" + os.Getenv("PATH")}}
	missing.Path = filepath.Join(missing.Root, "source")
	missing.GitDir = filepath.Join(missing.Root, "git")
	if missing.clone(context.Background()) == nil {
		t.Fatal("missing branch was accepted")
	}
}
