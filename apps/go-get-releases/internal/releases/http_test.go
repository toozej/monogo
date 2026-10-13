package releases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testForgeHTTP(t *testing.T, handler http.HandlerFunc) *forgeHTTP {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := newForgeHTTP(server.URL, "Authorization", "token test", 2)
	client.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	return client
}

func TestForgeHTTPFailures(t *testing.T) {
	for _, status := range []int{200, 201, 204, 400, 401, 403, 404, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := testForgeHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, `{}`)
			})
			var data any
			_, err := client.json(context.Background(), client.base, &data)
			if (err == nil) != (status == 200) {
				t.Fatal(err)
			}
			want := 1
			if status == 429 || status >= 500 {
				want = 3
			}
			if calls != want {
				t.Fatalf("got %d calls, want %d", calls, want)
			}
		})
	}
	for _, input := range []string{``, `{`, `{"x":`, `{} {}`, `[] true`, `{"x":"` + strings.Repeat("a", 16*1024*1024) + `"}`} {
		client := testForgeHTTP(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, input) })
		var data any
		if _, err := client.json(context.Background(), client.base, &data); err == nil {
			t.Fatal("accepted invalid metadata")
		}
	}
	client := testForgeHTTP(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(429) })
	var waited time.Duration
	client.wait = func(_ context.Context, d time.Duration) error { waited = d; return context.Canceled }
	if _, err := client.request(context.Background(), client.base); !errors.Is(err, context.Canceled) || waited < time.Second {
		t.Fatal(err, waited)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.request(ctx, client.base); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	client.client.Transport = failingTransport{}
	client.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	if _, err := client.request(context.Background(), client.base); err == nil {
		t.Fatal("accepted a transport failure")
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("transport failed")
}

func TestForgePagination(t *testing.T) {
	for _, tc := range []struct {
		header        http.Header
		current, want int
		fail          bool
	}{
		{http.Header{}, 1, 0, false},
		{http.Header{"X-Next-Page": {"2"}}, 1, 2, false},
		{http.Header{"Link": {`<https://evil.test/?page=3>; rel="next", <https://forge.test/?page=1>; rel="prev"`}}, 1, 3, false},
		{http.Header{"Link": {`<https://forge.test/?page=2>; type="json"; rel="next"`}}, 1, 2, false},
		{http.Header{"Link": {`<https://forge.test/>; rel="last"`}}, 1, 0, false},
		{http.Header{"Link": {`invalid`}}, 1, 0, false},
		{http.Header{"Link": {`<https://forge.test/>; rel="next"`}}, 1, 0, true},
		{http.Header{"Link": {`<https://forge.test/%zz>; rel="next"`}}, 1, 0, true},
		{http.Header{"X-Next-Page": {"1"}}, 1, 0, true},
		{http.Header{"X-Next-Page": {"0"}}, 1, 0, true},
		{http.Header{"X-Next-Page": {"-1"}}, 1, 0, true},
		{http.Header{"X-Next-Page": {"bad"}}, 1, 0, true},
	} {
		got, err := nextPage(tc.header, tc.current)
		if (err != nil) != tc.fail || got != tc.want {
			t.Fatalf("%v: %d, %v", tc.header, got, err)
		}
	}
}

func TestForgeURLValidation(t *testing.T) {
	client := newForgeHTTP("https://forge.test", "Authorization", "token", 1)
	for _, raw := range []string{"https://forge.test/file", "https://downloads.test/file?signature=public"} {
		if err := client.validateURL(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"http://forge.test/file", "file:///etc/passwd", "//forge.test/file", "https://user:pass@forge.test/file", "https://forge.test/file#secret", "https://", "https://forge.test/%zz", ""} {
		if _, err := client.request(context.Background(), raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestForgeCredentialsAndRedirects(t *testing.T) {
	for _, header := range []string{"Authorization", "PRIVATE-TOKEN", "JOB-TOKEN"} {
		t.Run(header, func(t *testing.T) {
			assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for _, key := range []string{"Authorization", "Private-Token", "Job-Token"} {
					if r.Header.Get(key) != "" {
						t.Errorf("asset request leaked %s", key)
					}
				}
				_, _ = fmt.Fprint(w, "bytes")
			}))
			defer assetServer.Close()
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/file" {
					if r.Header.Get(header) != "" {
						t.Error("same-origin redirect retained credentials")
					}
					http.Redirect(w, r, assetServer.URL, http.StatusFound)
					return
				}
				if r.Header.Get(header) != "secret" {
					t.Error("API request omitted credentials")
				}
				http.Redirect(w, r, "/file", http.StatusFound)
			}))
			defer apiServer.Close()
			client := newForgeHTTP(apiServer.URL, header, "secret", 0)
			for _, raw := range []string{apiServer.URL, assetServer.URL} {
				response, err := client.request(context.Background(), raw)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil || string(data) != "bytes" {
					t.Fatal(string(data), err)
				}
			}
		})
	}
	client := testForgeHTTP(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	client.retries = 0
	if _, err := client.request(context.Background(), client.base); err == nil {
		t.Fatal("accepted a redirect loop")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://forge.test/file", nil)
	production := newForgeHTTP("https://forge.test", "Authorization", "token", 0)
	if err := production.client.CheckRedirect(req, []*http.Request{}); err == nil {
		t.Fatal("accepted an HTTP redirect")
	}
}

func TestForgeEnvironmentTokens(t *testing.T) {
	tests := []struct {
		keys          []string
		header, token string
	}{
		{[]string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN"}, "PRIVATE-TOKEN", "first"},
		{[]string{"GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN"}, "PRIVATE-TOKEN", "first"},
		{[]string{"OAUTH_TOKEN"}, "Authorization", "Bearer first"},
		{nil, "PRIVATE-TOKEN", ""},
	}
	for _, tc := range tests {
		env := map[string]string{}
		for _, key := range tc.keys {
			env[key] = " second "
		}
		if len(tc.keys) > 0 {
			env[tc.keys[0]] = " first "
		}
		c := Defaults()
		c.Repositories = []Repository{{Name: "gitlab.com/group/repo"}, {Name: "gitlab.com/group/other"}}
		sources, err := NewSources(context.Background(), c, func(k string) string { return env[k] }, func(context.Context) string { t.Fatal("GitHub credentials queried"); return "" })
		if err != nil || len(sources) != 1 {
			t.Fatal(sources, err)
		}
		client := sources["gitlab.com"].(*restSource).http
		if client.header != tc.header || client.token != tc.token {
			t.Fatal(client.header, client.token)
		}
	}
	for _, key := range []string{"FORGEJO_TOKEN", "CODEBERG_TOKEN"} {
		c := Defaults()
		c.Repositories = []Repository{{Name: "codeberg.org/owner/repo"}}
		sources, err := NewSources(context.Background(), c, func(k string) string {
			if k == key {
				return " value "
			}
			return ""
		}, nil)
		if err != nil || sources["codeberg.org"].(*restSource).http.token != "token value" {
			t.Fatal(sources, err)
		}
	}
	c := Defaults()
	c.Repositories = []Repository{{Name: "codeberg.org/owner/repo"}, {Name: "tangled.org/user.tngl.sh/repo"}, {Name: "owner/repo"}}
	calls := 0
	sources, err := NewSources(context.Background(), c, func(string) string { return "value" }, func(context.Context) string { calls++; return "github-token" })
	if err != nil || len(sources) != 3 || calls != 1 {
		t.Fatal(sources, calls, err)
	}
	if sources["tangled.sh"].(*tangledSource).http.token != "" {
		t.Fatal("Tangled public requests must omit credentials")
	}
	if got := sources["codeberg.org"].(*restSource).http.token; got != "token value" {
		t.Fatal(got)
	}
	c.Repositories = []Repository{{Name: "evil.test/a/b"}}
	if _, err := NewSources(context.Background(), c, nil, nil); err == nil {
		t.Fatal("accepted unsupported forge")
	}
	if token := environmentToken(func(k string) string {
		if k == "empty" {
			return "  "
		}
		return " value "
	}, "empty", "valid"); token != "value" {
		t.Fatal(token)
	}
}
