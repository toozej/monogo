package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gphotos-lister/internal/config"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
)

const albumURL = "https://photos.google.com/share/album"

type readerFunc func(context.Context, string) ([]string, error)

func (f readerFunc) LikedFilenames(ctx context.Context, album string) ([]string, error) {
	return f(ctx, album)
}
func server() *Server {
	return &Server{Config: config.Config{Timeout: time.Minute}, Reader: readerFunc(func(context.Context, string) ([]string, error) { return []string{"IMG.JPG", "旅行.heic"}, nil })}
}
func token(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal(cookies)
	}
	return cookies[0], cookies[0].Value
}
func submit(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	cookie, csrf := token(t, h)
	r := httptest.NewRequest(http.MethodPost, "http://localhost/submit", strings.NewReader(body))
	r.AddCookie(cookie)
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestFormAndAssets(t *testing.T) {
	s := server()
	s.Config.Prefix = `"><script>alert(1)</script>`
	h := s.Handler()
	for _, tc := range []struct {
		path, content string
		code          int
	}{{"/", "text/html", 200}, {"/style.css", "text/css", 200}, {"/script.js", "application/javascript", 200}, {"/missing", "text/plain", 404}, {"/assets/index.html", "text/plain", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost"+tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Header().Get("Content-Type"), tc.content) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Header())
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Header())
		}
		if tc.path == "/" && (strings.Contains(w.Body.String(), s.Config.Prefix) || !strings.Contains(w.Body.String(), "&#34;&gt;&lt;script&gt;")) {
			t.Fatal("template did not escape defaults")
		}
	}
	cookie, csrf := token(t, h)
	if len(csrf) != 64 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal(cookie)
	}
}

func TestSubmitFormatsAndCopies(t *testing.T) {
	s := server()
	source, destination := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "IMG.JPG"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(lister.Options{Album: albumURL, Prefix: "pre/", Suffix: ".suffix", Copy: true, Source: source, Destination: destination})
	w := submit(t, s.Handler(), string(body))
	var response struct {
		Result lister.Result `json:"result"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if response.Result.Text != "pre/IMG.JPG.suffix\npre/旅行.heic.suffix\n" || len(response.Result.Copied) != 1 || len(response.Result.Missing) != 1 {
		t.Fatalf("%+v", response)
	}
	data, err := os.ReadFile(filepath.Join(destination, "IMG.JPG"))
	if err != nil || string(data) != "original" {
		t.Fatal(err)
	}
}

func TestEmptyAndFailedAlbums(t *testing.T) {
	for _, failed := range []bool{false, true} {
		s := server()
		s.Reader = readerFunc(func(context.Context, string) ([]string, error) {
			if failed {
				return nil, errors.New("session expired")
			}
			return []string{}, nil
		})
		w := submit(t, s.Handler(), `{"album":"`+albumURL+`"}`)
		if failed {
			if w.Code != 422 || !strings.Contains(w.Body.String(), "session expired") {
				t.Fatal(w.Code, w.Body.String())
			}
		} else if w.Code != 200 || !strings.Contains(w.Body.String(), `"text":""`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestSubmitValidation(t *testing.T) {
	s := server()
	s.Reader = readerFunc(func(context.Context, string) ([]string, error) {
		t.Fatal("invalid request reached the reader")
		return nil, nil
	})
	for _, body := range []string{`{`, `{}`, `{"album":"https://evil.test/album/a"}`, `{"album":"` + albumURL + `","copy":true}`, `{"album":"` + albumURL + `","suffix":"\n"}`, `{"album":"` + albumURL + `","unknown":1}`, `{"album":"` + albumURL + `"} {}`, strings.Repeat(" ", 65<<10)} {
		w := submit(t, s.Handler(), body)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/submit", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestCSRF(t *testing.T) {
	h := server().Handler()
	for _, kind := range []string{"missing", "wrong-token", "origin", "scheme", "fetch-site"} {
		cookie, csrf := token(t, h)
		r := httptest.NewRequest(http.MethodPost, "http://localhost/submit", strings.NewReader(`{"album":"`+albumURL+`"}`))
		if kind != "missing" {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		switch kind {
		case "wrong-token":
			r.Header.Set("X-CSRF-Token", strings.Repeat("x", 64))
		case "origin":
			r.Header.Set("Origin", "http://evil.test")
		case "scheme":
			r.Header.Set("Origin", "https://localhost")
		case "fetch-site":
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s: %d", kind, w.Code)
		}
	}
}

func TestAuthentication(t *testing.T) {
	s := server()
	s.Config.WebUsername = "user"
	s.Config.WebPassword = "password"
	for _, valid := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		if valid {
			r.SetBasicAuth("user", "password")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if valid && w.Code != 200 || !valid && w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
}

func TestValidateBind(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		if err := ValidateBind(host, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{"0.0.0.0", "::", "192.168.1.2", ""} {
		if err := ValidateBind(host, "", ""); err == nil {
			t.Fatal("accepted remote host")
		}
		if err := ValidateBind(host, "user", "password"); err != nil {
			t.Fatal(err)
		}
	}
	if ValidateBind("localhost", "user", "") == nil || ValidateBind("localhost", "", "password") == nil {
		t.Fatal("accepted incomplete credentials")
	}
}

func TestServeValidation(t *testing.T) {
	for _, tc := range []config.Config{{WebHost: "0.0.0.0", WebPort: 8080, Timeout: time.Minute}, {WebHost: "127.0.0.1", WebPort: 0, Timeout: time.Minute}, {WebHost: "127.0.0.1", WebPort: 65536, Timeout: time.Minute}, {WebHost: "127.0.0.1", WebPort: 8080}} {
		s := server()
		s.Config = tc
		if err := s.Serve(context.Background()); err == nil {
			t.Fatal("accepted invalid server config")
		}
	}
}

func TestServeLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := server()
	completed := make(chan error, 1)
	go func() { completed <- server.serveHTTP(ctx, listener) }()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
	cancel()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
