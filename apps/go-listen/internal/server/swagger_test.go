package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/pkg/swaggertest"
)

func newSwaggerTestServer(t *testing.T, username, password string) *Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080},
		Spotify: config.SpotifyConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURL:  "http://127.0.0.1:8080/callback",
			TokenFile:    filepath.Join(t.TempDir(), "token.json"),
		},
		Security: config.SecurityConfig{
			Username: username,
			Password: password,
			RateLimit: config.RateLimitConfig{
				RequestsPerSecond: 100,
				Burst:             100,
			},
		},
	}

	server := NewServer(cfg)
	server.setupRoutes()
	return server
}

func TestServer_ServesSwaggerUIAndDocument(t *testing.T) {
	server := newSwaggerTestServer(t, "", "")

	index := httptest.NewRecorder()
	server.router.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/swagger/index.html", http.NoBody))
	if index.Code != http.StatusOK {
		t.Fatalf("Swagger UI status = %d, want %d", index.Code, http.StatusOK)
	}
	if contentType := index.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Swagger UI Content-Type = %q, want HTML", contentType)
	}
	if body := index.Body.String(); !strings.Contains(body, "/swagger/doc.json") &&
		!strings.Contains(body, `\/swagger\/doc.json`) {
		t.Fatalf("Swagger UI does not reference the generated document: %q", body)
	} else if strings.Contains(body, "cdn.jsdelivr.net") {
		t.Fatalf("Swagger UI unexpectedly depends on CDN assets")
	}

	stylesheet := httptest.NewRecorder()
	server.router.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/swagger/swagger-ui.css", http.NoBody))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("Swagger stylesheet status = %d, want %d", stylesheet.Code, http.StatusOK)
	}
	if contentType := stylesheet.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("Swagger stylesheet Content-Type = %q, want CSS", contentType)
	}

	document := httptest.NewRecorder()
	server.router.ServeHTTP(document, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", http.NoBody))
	if document.Code != http.StatusOK {
		t.Fatalf("Swagger document status = %d, want %d: %s", document.Code, http.StatusOK, document.Body.String())
	}

	swaggertest.AssertDocument(t, document.Body.Bytes(), "go-listen API",
		"/api/add-artist",
		"/api/auth-status",
		"/api/csrf-token",
		"/api/playlists",
		"/api/scrape-artists",
	)
}

func TestServer_SwaggerUsesConfiguredBasicAuth(t *testing.T) {
	server := newSwaggerTestServer(t, "docs-user", "docs-password")

	unauthorized := httptest.NewRecorder()
	server.router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/swagger/index.html", http.NoBody))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Swagger UI status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}
	if challenge := unauthorized.Header().Get("WWW-Authenticate"); !strings.Contains(challenge, "Basic") {
		t.Fatalf("WWW-Authenticate = %q, want a Basic challenge", challenge)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/swagger/index.html", http.NoBody)
	request.SetBasicAuth("docs-user", "docs-password")
	server.router.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authenticated Swagger UI status = %d, want %d", authorized.Code, http.StatusOK)
	}
}
