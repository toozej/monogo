package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/toozej/monogo/apps/podgrab/db"
	"gorm.io/gorm"
)

func TestRunReturnsErrorWhenDatabaseInitFails(t *testing.T) {
	oldDB := db.DB
	oldInitDB := initDB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = oldDB
		initDB = oldInitDB
	})
	initDB = db.Init

	configPath := filepath.Join(t.TempDir(), "missing", "config")
	t.Setenv("CONFIG", configPath)

	exitCode := run()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when db init fails, got %d", exitCode)
	}
}

func TestSettingsTemplateUsesCurrentFieldNames(t *testing.T) {
	funcs := template.FuncMap{
		"intRange":            func(...any) []int { return nil },
		"removeStartingSlash": func(...any) string { return "" },
		"isDateNull":          func(...any) bool { return false },
		"formatDate":          func(...any) string { return "" },
		"naturalDate":         func(...any) string { return "" },
		"latestEpisodeDate":   func(...any) string { return "" },
		"downloadedEpisodes":  func(...any) int { return 0 },
		"downloadingEpisodes": func(...any) int { return 0 },
		"formatFileSize":      func(...any) string { return "" },
		"formatDuration":      func(...any) string { return "" },
	}
	tmpl, err := template.New("settings").Funcs(funcs).ParseFS(clientEmbed, "client/*")
	if err != nil {
		t.Fatal(err)
	}
	setting := &db.Setting{BaseURL: "https://podgrab.example", PassthroughPodcastGUID: true, MaxDownloadConcurrency: 5}
	var output bytes.Buffer
	err = tmpl.ExecuteTemplate(&output, "settings.html", map[string]any{
		"setting": setting,
		"diskStats": map[string]int64{
			"Downloaded":      0,
			"PendingDownload": 0,
		},
	})
	if err != nil {
		t.Fatalf("execute settings template: %v", err)
	}
	if !strings.Contains(output.String(), "podgrab.example") ||
		!strings.Contains(output.String(), "passthroughPodcastGuid: true") {
		t.Fatalf("settings fields were not rendered: %s", output.String())
	}
}

func TestWebsocketRouteUsesApplicationAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerWebsocketRoute(applicationRouter(r, "secret"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	r.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated websocket status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSwaggerRouteUsesApplicationAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerSwaggerRoute(applicationRouter(r, "secret"))

	unauthenticated := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/swagger/index.html", http.NoBody)
	r.ServeHTTP(unauthenticated, request)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Swagger status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	authenticated := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/swagger/index.html", http.NoBody)
	request.SetBasicAuth("podgrab", "secret")
	r.ServeHTTP(authenticated, request)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated Swagger status = %d, want %d", authenticated.Code, http.StatusOK)
	}
	if contentType := authenticated.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("authenticated Swagger content type = %q, want HTML", contentType)
	}

	document := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/swagger/doc.json", http.NoBody)
	request.SetBasicAuth("podgrab", "secret")
	r.ServeHTTP(document, request)
	if document.Code != http.StatusOK {
		t.Fatalf("authenticated Swagger document status = %d, want %d: %s", document.Code, http.StatusOK, document.Body.String())
	}
	var spec struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths               map[string]json.RawMessage `json:"paths"`
		SecurityDefinitions map[string]struct {
			Type string `json:"type"`
		} `json:"securityDefinitions"`
	}
	if err := json.Unmarshal(document.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode Swagger document: %v", err)
	}
	if spec.Info.Title != "Podgrab API" {
		t.Errorf("Swagger title = %q, want %q", spec.Info.Title, "Podgrab API")
	}
	if len(spec.Paths) != 30 {
		t.Errorf("Swagger path count = %d, want 30", len(spec.Paths))
	}
	for _, path := range []string{"/podcasts", "/podcastitems", "/tags", "/settings", "/version"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("Swagger document is missing %s", path)
		}
	}
	if definition, ok := spec.SecurityDefinitions["BasicAuth"]; !ok {
		t.Error("Swagger document is missing the BasicAuth security definition")
	} else if definition.Type != "basic" {
		t.Errorf("BasicAuth type = %q, want %q", definition.Type, "basic")
	}
}

func TestRunReturnsErrorWhenSQLiteInitFails(t *testing.T) {
	oldDB := db.DB
	oldInitDB := initDB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = oldDB
		initDB = oldInitDB
	})

	initDB = func() (*gorm.DB, error) {
		return nil, errors.New("failed to initialize sqlite driver")
	}

	exitCode := run()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for CGO-disabled sqlite init, got %d", exitCode)
	}
}
