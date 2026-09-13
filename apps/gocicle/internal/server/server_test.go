package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/runner"
	containerruntime "github.com/toozej/monogo/apps/gocicle/internal/runtime"

	"github.com/toozej/monogo/apps/gocicle/internal/config"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/apps/gocicle/internal/testutil"
)

func TestServerShutdownCancelsStreamingRequests(t *testing.T) {
	db := testutil.Database(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	control := New(config.Config{Listen: address, Retention: 30 * 24 * time.Hour}, service.New(db, security.Keyring{}))
	control.HTTP.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() { _ = control.HTTP.Close() }()
	done := make(chan error, 1)
	go func() { done <- control.Run(ctx) }()
	client := &http.Client{Timeout: 3 * time.Second}
	var response *http.Response
	for attempt := 0; attempt < 100; attempt++ {
		response, err = client.Get("http://" + address)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not cancel the active stream")
	}
}

func TestAPIAuthenticationCSRFAndWASM(t *testing.T) {
	db := testutil.Database(t)
	s := service.New(db, security.Keyring{Active: "test", Keys: map[string]string{"test": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}})
	owner := storage.User{ID: storage.ID(), Name: "owner", Enabled: true, Administrator: true, Version: 1, Preferences: storage.Encode(map[string]any{})}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	actor := service.Actor{User: owner, Scopes: []string{"admin"}}
	token, err := s.CreateToken(actor, []string{"admin"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	session, csrf := security.Token(), security.Token()
	if err := db.Exec("INSERT INTO sessions VALUES (?,?,?,?)", security.Hash(session), owner.ID, security.Hash(csrf), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	server := New(config.Config{PublicURL: "https://gocicle.example", Retention: 30 * 24 * time.Hour}, s)
	handler := server.Handler()
	request := func(method, path, body, auth, csrfHeader, origin string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if auth == "session" {
			r.AddCookie(&http.Cookie{Name: "__Host-gocicle", Value: session})
		} else if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		r.Header.Set("X-CSRF-Token", csrfHeader)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"name":"project","source":{"git":{"url":"https://example.com/repo.git","branch":"main"}}}`
	if w := request("GET", "/api/v1/projects", "", "", "", ""); w.Code != 403 {
		t.Fatalf("anonymous status=%d", w.Code)
	}
	if w := request("POST", "/api/v1/projects", body, "session", "", "https://gocicle.example"); w.Code != 403 {
		t.Fatal("missing CSRF token was accepted")
	}
	if w := request("POST", "/api/v1/projects", body, "session", csrf, "https://attacker.example"); w.Code != 403 {
		t.Fatal("foreign origin was accepted")
	}
	if w := request("POST", "/api/v1/projects", body, "session", csrf, "https://gocicle.example"); w.Code != 200 {
		t.Fatalf("session status=%d body=%s", w.Code, w.Body.String())
	}
	if w := request("GET", "/api/v1/projects", "", token, "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "project") {
		t.Fatalf("token status=%d body=%s", w.Code, w.Body.String())
	}
	if w := request("GET", "/", "", "", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "gocicle") {
		t.Fatalf("UI status=%d", w.Code)
	}
	if w := request("GET", "/web/app.wasm", "", "", "", ""); w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte{0, 'a', 's', 'm'}) {
		t.Fatalf("WASM status=%d", w.Code)
	}
	if _, err := s.Authenticate(context.Background(), token, false); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/api/v1/jobs/validate", `{"yaml":"invalid", "unknown":true}`, token, "", ""); w.Code != 400 {
		t.Fatal("unknown API fields were accepted")
	}
	var response map[string]any
	w := request("GET", "/api/v1/me", "", token, "", "")
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response["id"] != owner.ID {
		t.Fatal("user response is invalid")
	}
}
func TestTrustedProxyHeaders(t *testing.T) {
	s := Server{Config: config.Config{TrustedProxies: []string{"127.0.0.1/32"}}}
	r := httptest.NewRequest("GET", "http://gocicle.example/", nil)
	r.RemoteAddr = "192.0.2.1:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Forwarded-Proto", "https")
	s.proxyRequest(r)
	if r.Header.Get("X-Forwarded-For") != "" || r.URL.Scheme == "https" {
		t.Fatal("untrusted proxy headers were accepted")
	}
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Forwarded-Proto", "https")
	s.proxyRequest(r)
	if r.RemoteAddr != "203.0.113.9:0" || r.URL.Scheme != "https" {
		t.Fatal("trusted proxy metadata was not applied")
	}
}

func TestTwoDistributedRunners(t *testing.T) {
	socket := os.Getenv("GOCICLE_TEST_RUNTIME_SOCKET")
	if socket == "" {
		t.Skip("set GOCICLE_TEST_RUNTIME_SOCKET for distributed runner acceptance")
	}
	db := testutil.Database(t)
	s := service.New(db, security.Keyring{Active: "test", Keys: map[string]string{"test": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}})
	owner := storage.User{ID: storage.ID(), Name: "owner", Enabled: true, Administrator: true, Version: 1, Preferences: storage.Encode(map[string]any{})}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	actor := service.Actor{User: owner, Scopes: []string{"admin"}}
	httpServer := httptest.NewTLSServer(New(config.Config{PublicURL: "https://gocicle.example"}, s).Handler())
	defer httpServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	var runs []storage.Run
	for _, name := range []string{"first", "second"} {
		sourceDir := t.TempDir()
		token, err := s.EnrollToken(actor, name, []string{"linux"}, []string{sourceDir})
		if err != nil {
			t.Fatal(err)
		}
		registered, credential, err := s.Enroll(token)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RunnerGrant(actor, registered.ID, owner.ID, true); err != nil {
			t.Fatal(err)
		}
		source := jobs.Source{Local: &jobs.Local{RunnerID: registered.ID, HostPath: sourceDir, ContainerPath: "/workspace"}}
		project, err := s.CreateProject(actor, storage.Project{Name: name, Source: storage.Encode(source)})
		if err != nil {
			t.Fatal(err)
		}
		spec := jobs.Spec{Container: jobs.Container{Image: "alpine:3.22"}, Command: jobs.Command{Path: "/bin/sh", Args: []string{"-c", "echo distributed-runner; echo saved > result"}}}
		spec.Defaults()
		imported, err := s.Import(actor, project.ID, "import-"+name, service.Import{Document: jobs.Document{APIVersion: jobs.APIVersion, Project: jobs.Project{Name: name, Source: source}, Jobs: map[string]jobs.Spec{"test": spec}}})
		if err != nil {
			t.Fatal(err)
		}
		job, err := s.UpdateJob(actor, imported[0].ID, imported[0].Version, spec, true)
		if err != nil {
			t.Fatal(err)
		}
		run, err := s.Start(actor, job.ID, "run-"+name)
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
		runtime, err := containerruntime.New("docker", socket)
		if err != nil {
			t.Fatal(err)
		}
		worker := &runner.Runner{ID: registered.ID, StateDir: t.TempDir(), Client: &runner.Client{URL: httpServer.URL, Token: credential, HTTP: httpServer.Client()}, Runtime: runtime}
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := worker.Run(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		finished := 0
		for _, run := range runs {
			var stored storage.Run
			if err := db.First(&stored, "id=?", run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status == "success" {
				finished++
			} else if stored.Status != "running" && stored.Status != "queued" {
				cancel()
				t.Fatalf("run failed: %s %s", stored.Status, stored.Result)
			}
		}
		if finished == 2 {
			cancel()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	t.Fatal("distributed runners did not complete")
}
