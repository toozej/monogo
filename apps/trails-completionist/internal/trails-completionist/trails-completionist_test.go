package trailscompletionist

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
