package photos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := f(r)
	if response != nil && response.Request == nil {
		response.Request = r
	}
	return response, err
}
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

const publicPage = `<html><script>window.WIZ_global_data = {"Im6cmf":"/_/PhotosUi","FdrFJe":"anonymous-page-session","cfb2h":"build"};</script></html>`

func TestAnonymousClientEndToEnd(t *testing.T) {
	var calls []string
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := &Client{Timeout: time.Minute, Logger: logger, transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("client sent account credentials")
		}
		calls = append(calls, r.Method+" "+r.URL.Host+r.URL.Path)
		if r.URL.Host == "photos.app.goo.gl" {
			reply := response(302, "")
			reply.Header.Set("Location", testAlbum)
			return reply, nil
		}
		if r.Method == http.MethodGet {
			reply := response(200, publicPage)
			reply.Header.Set("Set-Cookie", "account-cookie=must-not-be-stored")
			return reply, nil
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Has("at") || len(r.PostForm) != 1 {
			t.Fatalf("unexpected account token or form fields: %v", r.PostForm)
		}
		if r.URL.Query().Get("f.sid") != "anonymous-page-session" || r.Header.Get("Origin") != "https://photos.google.com" {
			t.Fatal("invalid public bootstrap")
		}
		var envelope [][][]any
		if err := json.Unmarshal([]byte(r.PostForm.Get("f.req")), &envelope); err != nil {
			t.Fatal(err)
		}
		rpc := envelope[0][0][0].(string)
		var payload []any
		if err := json.Unmarshal([]byte(envelope[0][0][1].(string)), &payload); err != nil {
			t.Fatal(err)
		}
		var data []any
		switch rpc {
		case "snAcKc":
			if payload[3] != "secret" {
				t.Fatal("sharing key was lost")
			}
			data = albumPage([]any{[]any{"hearted"}, []any{"plain"}}, nil)
		case "wQ6iqd":
			if payload[2] != "secret" {
				t.Fatal("sharing key was lost")
			}
			data = activity([]any{heartEvent("hearted", 1), heartEvent("album", 2), []any{float64(2), "comment"}}, nil)
		case "fDcn4b":
			if payload[1] != nil || payload[2] != "secret" || at(payload, 5, 0) != float64(2) {
				t.Fatal(payload)
			}
			data = []any{[]any{"hearted", nil, "IMG_1234.HEIC"}}
		default:
			t.Fatal(rpc)
		}
		return response(200, rpcFrame(rpc, data)), nil
	})}
	for range 2 {
		names, err := client.LikedFilenames(context.Background(), "https://photos.app.goo.gl/short")
		if err != nil || !reflect.DeepEqual(names, []string{"IMG_1234.HEIC"}) {
			t.Fatalf("%v %v", names, err)
		}
	}
	if len(calls) != 10 {
		t.Fatal(calls)
	}
	for _, expected := range []string{"Open public album", "status=200", "rpc=snAcKc", "rpc=wQ6iqd", "rpc=fDcn4b", "photos_total=2", "album_hearts=1", "matched_photos=1", "Read original filename"} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("debug log does not contain %q: %s", expected, logs.String())
		}
	}
	for _, secret := range []string{"key=secret", "IMG_1234.HEIC", "account-cookie", "anonymous-page-session", "https://photos.google.com/share/album"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("debug log contains private data: %q", secret)
		}
	}
}

func TestAnonymousClientValidation(t *testing.T) {
	for _, client := range []*Client{{Timeout: 0}, {Timeout: -time.Second}} {
		if _, err := client.LikedFilenames(context.Background(), testAlbum); err == nil {
			t.Fatal("accepted invalid timeout")
		}
	}
	if _, err := (&Client{Timeout: time.Second}).LikedFilenames(context.Background(), "https://evil.test"); err == nil {
		t.Fatal("accepted external host")
	}
	expected := errors.New("network failed")
	client := &Client{Timeout: time.Second, transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, expected })}
	if _, err := client.LikedFilenames(context.Background(), testAlbum); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

func TestPublicSessionBootstrap(t *testing.T) {
	var session publicSession
	if err := session.bootstrap([]byte(publicPage)); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"<html>Sign in</html>", `window.WIZ_global_data = invalid`, `window.WIZ_global_data = {}`, `window.WIZ_global_data = {"Im6cmf":"//evil.test","FdrFJe":"sid","cfb2h":"build"}`, `window.WIZ_global_data = {"Im6cmf":"/_/PhotosUi?bad","FdrFJe":"sid","cfb2h":"build"}`, `window.WIZ_global_data = {"Im6cmf":"/_/PhotosUi","FdrFJe":"","cfb2h":"build"}`} {
		if err := session.bootstrap([]byte(body)); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	if _, err := (&publicSession{}).RPC(context.Background(), "x", []any{make(chan int)}); err == nil {
		t.Fatal("accepted invalid payload")
	}
	if _, err := (&publicSession{}).RPC(context.Background(), "x", nil); err == nil {
		t.Fatal("accepted missing bootstrap")
	}
}

func TestRedirectPolicy(t *testing.T) {
	for _, raw := range []string{"https://photos.google.com/share/album?key=key", "https://photos.app.goo.gl/short"} {
		u, _ := url.Parse(raw)
		if err := allowedURL(u); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"http://photos.google.com/share/a", "https://accounts.google.com/ServiceLogin", "https://evil.test/", "https://user@photos.google.com/", "https://photos.google.com:443/"} {
		u, _ := url.Parse(raw)
		if err := checkRedirect(&http.Request{URL: u}, nil); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	u, _ := url.Parse(testAlbum)
	if err := checkRedirect(&http.Request{URL: u}, make([]*http.Request, 10)); err == nil {
		t.Fatal("accepted a redirect loop")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func TestResponseErrors(t *testing.T) {
	for _, reply := range []*http.Response{response(403, "denied"), response(429, "rate limited"), response(200, strings.Repeat("x", maxResponseBytes+1)), {StatusCode: 200, Body: io.NopCloser(errorReader{})}} {
		if _, err := readResponse(reply); err == nil {
			t.Fatal("accepted invalid response")
		}
	}
	session := &publicSession{client: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(500, "failed"), nil })}}
	if _, err := session.Navigate(context.Background(), testAlbum); err == nil {
		t.Fatal("accepted HTTP error")
	}
	if err := session.bootstrap([]byte(publicPage)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.RPC(context.Background(), "x", nil); err == nil {
		t.Fatal("accepted RPC HTTP error")
	}
}
