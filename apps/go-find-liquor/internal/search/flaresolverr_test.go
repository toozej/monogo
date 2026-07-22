package search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFlareSolverrTransportForwardsRequestAndCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1" {
			t.Errorf("path = %q, want /v1", r.URL.Path)
		}
		var got flareSolverrRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Cmd != "request.post" || got.PostData != "item=whiskey" {
			t.Errorf("request = %+v, want POST form", got)
		}
		if got.Headers["User-Agent"] != "test-agent" {
			t.Errorf("user agent = %q", got.Headers["User-Agent"])
		}
		if len(got.Cookies) != 1 || got.Cookies[0].Name != "session" {
			t.Errorf("cookies = %+v", got.Cookies)
		}
		_, _ = io.WriteString(w, `{"status":"ok","solution":{"status":200,"headers":{"Content-Type":"text/html"},"response":"<html>ok</html>","cookies":[{"name":"cf_clearance","value":"cleared","domain":"www.oregonliquorsearch.com","path":"/"}]}}`)
	}))
	defer server.Close()

	transport, err := NewFlareSolverrTransport(server.URL + "/v1")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL, strings.NewReader("item=whiskey"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "test-agent")
	req.AddCookie(&http.Cookie{Name: "session", Value: "existing", HttpOnly: true, Secure: true})
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if body, _ := io.ReadAll(resp.Body); string(body) != "<html>ok</html>" {
		t.Errorf("body = %q", body)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "cf_clearance=cleared") {
		t.Errorf("Set-Cookie = %q", resp.Header.Get("Set-Cookie"))
	}
}
