package trailscompletionist

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
	"github.com/toozej/monogo/apps/trails-completionist/internal/parser"
)

func TestHTMLHandlerDoesNotExposeSiblingFiles(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"trails.html": "<html>trails</html>",
		"app.js":      "// app",
		"styles.css":  "/* styles */",
		".env":        "SECRET=do-not-serve",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := HTMLHandler(filepath.Join(dir, "trails.html"))

	for path, want := range map[string]int{"/": http.StatusOK, "/app.js": http.StatusOK, "/styles.css": http.StatusOK, "/.env": http.StatusNotFound, "/other.txt": http.StatusNotFound} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if recorder.Code != want {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, want)
		}
	}
}

func TestRunTrailsCompletionistSupportsRawOnlyGeneration(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "raw.txt")
	checklistPath := filepath.Join(dir, "checklist.md")
	htmlPath := filepath.Join(dir, "trails.html")
	if err := os.WriteFile(rawPath, []byte("Wildwood Trail\nTrail 3.2 miles\nRegion > Forest Park\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RunTrailsCompletionist(config.Config{
		InputFile:     rawPath,
		ChecklistFile: checklistPath,
		HTMLFile:      htmlPath,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{checklistPath, htmlPath, filepath.Join(dir, "app.js"), filepath.Join(dir, "styles.css")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("raw-only workflow did not generate %s: %v", filepath.Base(path), err)
		}
	}
	checklist, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(checklist), "Wildwood Trail") {
		t.Fatalf("generated checklist omitted raw trail: %s", checklist)
	}
}

func TestRunTrailsCompletionistRequiresOSMForTracks(t *testing.T) {
	err := RunTrailsCompletionist(config.Config{TrackFiles: t.TempDir()}, false)
	if err == nil || !strings.Contains(err.Error(), "OSM region data is required") {
		t.Fatalf("RunTrailsCompletionist() error = %v, want missing OSM error", err)
	}
}

func TestHTMLHandlerRejectsNonReadMethods(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trails.html"), []byte("<html>trails</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := HTMLHandler(filepath.Join(dir, "trails.html"))

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/", http.NoBody))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s / status = %d, want %d", method, recorder.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHTMLHandlerRejectsDirectoriesAndSymlinkedAssets(t *testing.T) {
	dir := t.TempDir()
	directoryHandler := HTMLHandler(dir)
	recorder := httptest.NewRecorder()
	directoryHandler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("directory HTML target status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	htmlPath := filepath.Join(dir, "trails.html")
	secretPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(htmlPath, []byte("<html>trails</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretPath, []byte("SECRET=do-not-serve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretPath, filepath.Join(dir, "app.js")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	handler := HTMLHandler(htmlPath)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app.js", http.NoBody))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("symlinked asset status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestFullRetainsTextAndInventoryMetadata(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.txt")
	inventory := filepath.Join(dir, "inventory.geojson")
	if err := os.WriteFile(raw, []byte("Original Trail\nTrail 1.0 miles\nRegion > Park\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"id":"1","name":"New & Existing Trail","park":"Another Park","status":"closed","url":"https://example.com/?a=1&b=2"},"geometry":{"type":"LineString","coordinates":[[-122,45],[-121.99,45]]}}]}`
	if err := os.WriteFile(inventory, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{InputFile: raw, InventoryFiles: []string{inventory}, ChecklistFile: filepath.Join(dir, "checklist.md"), HTMLFile: filepath.Join(dir, "trails.html")}
	if err := RunTrailsCompletionist(cfg, false); err != nil {
		t.Fatal(err)
	}
	trails, err := parser.ParseTrailsFromChecklist(cfg.ChecklistFile)
	if err != nil || len(trails) != 2 {
		t.Fatalf("additive input failed: %#v, %v", trails, err)
	}
	for _, trail := range trails {
		if trail.Name == "New & Existing Trail" {
			if trail.ID == "" || trail.Status != "closed" || trail.URL != "https://example.com/?a=1&b=2" {
				t.Fatalf("metadata did not survive the checklist: %#v", trail)
			}
			return
		}
	}
	t.Fatal("inventory name changed during checklist generation")
}

func TestHTMLServerAcceptsRequestsOnALANInterface(t *testing.T) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var lanIP net.IP
	for _, address := range addresses {
		if network, ok := address.(*net.IPNet); ok && network.IP.To4() != nil && network.IP.IsPrivate() {
			lanIP = network.IP.To4()
			break
		}
	}
	if lanIP == nil {
		t.Skip("No private IPv4 interface is available.")
	}
	dir := t.TempDir()
	htmlFile := filepath.Join(dir, "trails.html")
	if err := os.WriteFile(htmlFile, []byte("LAN trail page"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("private data"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newHTMLServer(htmlFile, "0.0.0.0:0")
	listener, err := net.Listen("tcp4", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		if err := <-result; err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	})
	port := listener.Addr().(*net.TCPAddr).Port
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	for _, host := range []string{"127.0.0.1", lanIP.String()} {
		for _, test := range []struct {
			path   string
			status int
		}{{"/", http.StatusOK}, {"/.env", http.StatusNotFound}} {
			response, err := client.Get("http://" + net.JoinHostPort(host, fmt.Sprint(port)) + test.path)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("GET %s%s = %d, want %d", host, test.path, response.StatusCode, test.status)
			}
		}
	}
}
