package releases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
)

func TestToken(t *testing.T) {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_API_TOKEN"} {
		token := Token(context.Background(), func(name string) string {
			if name == key {
				return " env-token "
			}
			return ""
		}, func(context.Context) ([]byte, error) { t.Fatal("CLI called with environment token"); return nil, nil })
		if token != "env-token" {
			t.Fatal(token)
		}
	}
	token := Token(context.Background(), func(name string) string { return name }, nil)
	if token != "GH_TOKEN" {
		t.Fatal("incorrect priority")
	}
	for _, tc := range []struct {
		data string
		err  error
		want string
	}{{" cli-token\n", nil, "cli-token"}, {"", errors.New("not installed"), ""}, {"", nil, ""}} {
		got := Token(context.Background(), func(string) string { return "" }, func(context.Context) ([]byte, error) { return []byte(tc.data), tc.err })
		if got != tc.want {
			t.Fatal(got)
		}
	}
}

func apiClient(t *testing.T, handler http.HandlerFunc) *GitHub {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := github.NewClient(github.WithURLs(github.Ptr(server.URL+"/"), nil))
	if err != nil {
		t.Fatal(err)
	}
	return &GitHub{client: c, downloads: server.Client(), retries: 2, wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
}

func TestGitHubPagination(t *testing.T) {
	seen := map[string]int{}
	g := apiClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/owner/repo":
			_, _ = fmt.Fprint(w, `{"full_name":"owner/repo"}`)
		case "/repos/owner/repo/releases", "/repos/owner/repo/releases/1/assets":
			page := r.URL.Query().Get("page")
			if r.URL.Query().Get("per_page") != "100" {
				t.Error("incorrect page size")
			}
			if page == "1" {
				w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=2>; rel="next"`, r.Host, r.URL.Path))
			}
			_, _ = fmt.Fprintf(w, `[{"id":%s}]`, page)
		default:
			t.Error(r.URL.Path)
			http.NotFound(w, r)
		}
	})
	if err := g.VerifyRepository(context.Background(), "owner/repo"); err != nil {
		t.Fatal(err)
	}
	releases, err := g.Releases(context.Background(), "owner/repo")
	if err != nil || len(releases) != 2 || releases[1].GetID() != 2 {
		t.Fatal(releases, err)
	}
	assets, err := g.Assets(context.Background(), "owner/repo", 1)
	if err != nil || len(assets) != 2 || assets[1].GetID() != 2 {
		t.Fatal(assets, err)
	}
	if seen["/repos/owner/repo/releases"] != 2 {
		t.Fatal(seen)
	}
}

func TestGitHubErrorsAndRetries(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusInternalServerError, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			calls := 0
			g := apiClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, `{"message":"failed"}`)
			})
			if err := g.VerifyRepository(context.Background(), "owner/repo"); err == nil {
				t.Fatal("expected error")
			}
			want := 1
			if status >= 500 || status == 429 {
				want = 3
			}
			if calls != want {
				t.Fatalf("got %d requests, want %d", calls, want)
			}
			if _, err := g.Releases(context.Background(), "owner/repo"); err == nil {
				t.Fatal("expected list error")
			}
			if _, err := g.Assets(context.Background(), "owner/repo", 1); err == nil {
				t.Fatal("expected asset error")
			}
			if _, err := g.Download(context.Background(), "owner/repo", 1); err == nil {
				t.Fatal("expected download error")
			}
		})
	}
}

func TestRateLimitWaits(t *testing.T) {
	response := &http.Response{StatusCode: 403, Header: http.Header{}, Request: &http.Request{Method: "GET"}}
	delay := 7 * time.Second
	for _, tc := range []struct {
		name    string
		err     error
		minimum time.Duration
	}{
		{"primary", &github.RateLimitError{Rate: github.Rate{Reset: github.Timestamp{Time: time.Now().Add(10 * time.Second)}}, Response: response}, 10 * time.Second},
		{"secondary", &github.AbuseRateLimitError{RetryAfter: &delay, Response: response}, 7 * time.Second},
		{"secondary no header", &github.AbuseRateLimitError{Response: response}, time.Minute},
		{"retry after", &github.ErrorResponse{Response: &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"8"}}}}, 8 * time.Second},
		{"retry after date", &github.ErrorResponse{Response: &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{time.Now().Add(15 * time.Second).UTC().Format(http.TimeFormat)}}}}, 13 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryDelay(tc.err, 0); got < tc.minimum {
				t.Fatal(got)
			}
		})
	}
	g := &GitHub{retries: 1, wait: func(context.Context, time.Duration) error { return nil }}
	calls := 0
	if err := g.call(context.Background(), func() (*github.Response, error) {
		calls++
		if calls == 1 {
			return nil, &github.AbuseRateLimitError{RetryAfter: &delay, Response: response}
		}
		return nil, nil
	}); err != nil || calls != 2 {
		t.Fatal(calls, err)
	}
	var waited time.Duration
	g.wait = func(_ context.Context, d time.Duration) error { waited = d; return nil }
	g.next = time.Now().Add(20 * time.Second)
	if err := g.call(context.Background(), func() (*github.Response, error) { return nil, nil }); err != nil || waited < 19*time.Second {
		t.Fatal(waited, err)
	}
	g.next = time.Time{}
	reset := time.Now().Add(time.Hour)
	if err := g.call(context.Background(), func() (*github.Response, error) {
		return &github.Response{Rate: github.Rate{Limit: 60, Remaining: 0, Reset: github.Timestamp{Time: reset}}}, nil
	}); err != nil || g.next.Before(reset) {
		t.Fatal(g.next, err)
	}
	g.wait = func(context.Context, time.Duration) error { return context.Canceled }
	if err := g.call(context.Background(), func() (*github.Response, error) { t.Fatal("called after wait failed"); return nil, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDownloadRedirectHasNoToken(t *testing.T) {
	assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("asset request leaked a token")
		}
		_, _ = fmt.Fprint(w, "package bytes")
	}))
	defer assetServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("API request did not contain token")
		}
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Error("incorrect Accept header")
		}
		http.Redirect(w, r, assetServer.URL, http.StatusFound)
	}))
	defer apiServer.Close()
	client, err := github.NewClient(github.WithURLs(github.Ptr(apiServer.URL+"/"), nil), github.WithAuthToken("test-token"))
	if err != nil {
		t.Fatal(err)
	}
	g := &GitHub{client: client, downloads: assetServer.Client(), retries: 1, wait: waitContext}
	body, err := g.Download(context.Background(), "owner/repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "package bytes" {
		t.Fatal(string(data), err)
	}
}

func TestDownloadDirectAndRedirectFailure(t *testing.T) {
	for _, status := range []int{200, 500} {
		g := apiClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/1") {
				http.Redirect(w, r, "http://"+r.Host+"/content", http.StatusFound)
				return
			}
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, "bytes")
		})
		body, err := g.Download(context.Background(), "owner/repo", 1)
		if status == 200 {
			if err != nil {
				t.Fatal(err)
			}
			_ = body.Close()
		} else if err == nil {
			t.Fatal("accepted failed redirect download")
		}
	}
	g := apiClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "bytes") })
	body, err := g.Download(context.Background(), "owner/repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
}

func TestWaitContext(t *testing.T) {
	if err := waitContext(context.Background(), time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := waitContext(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if retryable(context.Canceled) {
		t.Fatal("context errors are retryable")
	}
	if _, err := NewGitHub("", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGitHub("test-token", 1); err != nil {
		t.Fatal(err)
	}
}
