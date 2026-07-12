package trailscompletionist

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
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
