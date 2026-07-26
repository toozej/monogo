package readme

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
)

func TestHumanize(t *testing.T) {
	now := time.Now()
	cases := []struct {
		in   time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-2 * time.Minute), "2 minutes ago"},
		{now.Add(-1 * time.Minute), "1 minute ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-24 * time.Hour), "1 day ago"},
		{now.Add(-48 * time.Hour), "2 days ago"},
		{now.Add(-14 * 24 * time.Hour), "14 days ago"},
		{now.Add(-60 * 24 * time.Hour), "8 weeks ago"},
		{now.Add(-400 * 24 * time.Hour), "1 year ago"},
	}
	for _, c := range cases {
		got := humanize(c.in)
		if got != c.want {
			t.Errorf("humanize(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanizeZero(t *testing.T) {
	if got := humanize(time.Time{}); got != "" {
		t.Errorf("humanize(zero) = %q, want empty", got)
	}
}

func TestRender(t *testing.T) {
	dir := t.TempDir()
	tplPath := filepath.Join(dir, "tpl")
	outPath := filepath.Join(dir, "out")
	if err := os.WriteFile(tplPath, []byte("Hello {{.Username}}!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Render(tplPath, outPath, &Data{Username: "toozej"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Hello toozej!") {
		t.Errorf("output = %q, want it to contain %q", got, "Hello toozej!")
	}
}

func TestRenderEscapesHTML(t *testing.T) {
	dir := t.TempDir()
	tplPath := filepath.Join(dir, "tpl")
	outPath := filepath.Join(dir, "out")
	if err := os.WriteFile(tplPath, []byte("Hello {{.Username}}!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Render(tplPath, outPath, &Data{Username: "<script>alert('xss')</script>"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "<script>") {
		t.Errorf("output contains unescaped HTML: %q", got)
	}
	if !strings.Contains(string(got), "&lt;script&gt;") {
		t.Errorf("output = %q, want escaped HTML", got)
	}
}

func TestRenderGeneratedTemplate(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "README.md")
	templatePath := filepath.Join("..", "..", "templates", "README.md.tpl")
	data := &Data{
		Username:           "alice",
		RecentCreatedRepos: []Repo{{Name: "created", URL: "https://github.com/alice/created", Description: "created repo"}},
		RecentContributions: []Contribution{{
			Repo:       Repo{Name: "alice/work", URL: "https://github.com/alice/work", Description: "work repo"},
			OccurredAt: time.Now().Add(-time.Hour),
		}},
		RecentStarredRepos: []Repo{{Name: "owner/starred", URL: "https://github.com/owner/starred", Description: "starred repo"}},
	}

	if err := Render(templatePath, outPath, data); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(got)
	for _, want := range []string{
		"#### 👨💻 Repositories I created recently",
		"[created](https://github.com/alice/created)",
		"#### ⛏️ What I've been working on",
		"[alice/work](https://github.com/alice/work)",
		"#### ⭐ Recently starred repositories",
		"[owner/starred](https://github.com/owner/starred)",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("generated README is missing %q", want)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(readme), "- [owner/starred](https://github.com/owner/starred) - starred repo") {
		t.Error("starred repositories section should be the final README section")
	}
}

func TestFetchPopulatesSectionsAndExcludesStarredRepos(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice/starred":
			if r.URL.Query().Get("sort") != "created" || r.URL.Query().Get("direction") != "desc" || r.URL.Query().Get("per_page") != "100" {
				http.Error(w, "unexpected starred query", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("page") == "2" {
				if _, err := fmt.Fprint(w, `[{"starred_at":"2026-01-01T00:00:00Z","repo":{"full_name":"owner/older-star","html_url":"https://github.com/owner/older-star","description":"older"}}]`); err != nil {
					t.Errorf("write starred page 2 response: %v", err)
				}
				return
			}
			w.Header().Set("Link", `<https://api.github.com/users/alice/starred?sort=created&direction=desc&per_page=100&page=2>; rel="next"`)
			if _, err := fmt.Fprint(w, `[{"starred_at":"2026-07-02T00:00:00Z","repo":{"full_name":"owner/new-star","html_url":"https://github.com/owner/new-star","description":"new"}}]`); err != nil {
				t.Errorf("write starred response: %v", err)
			}
		case "/users/alice/repos":
			if r.URL.Query().Get("type") != "public" || r.URL.Query().Get("sort") != "created" {
				http.Error(w, "unexpected repos query", http.StatusBadRequest)
				return
			}
			if _, err := fmt.Fprint(w, `[{"name":"new-star","full_name":"owner/new-star","html_url":"https://github.com/owner/new-star"},{"name":"created","full_name":"alice/created","html_url":"https://github.com/alice/created","description":"created"}]`); err != nil {
				t.Errorf("write repos response: %v", err)
			}
		case "/users/alice/events":
			if _, err := fmt.Fprint(w, `[{"type":"PushEvent","repo":{"name":"owner/older-star","html_url":"https://github.com/owner/older-star"},"created_at":"2026-07-03T00:00:00Z"},{"type":"WatchEvent","repo":{"name":"owner/watch-event","html_url":"https://github.com/owner/watch-event"},"created_at":"2026-07-02T00:00:00Z"},{"type":"PushEvent","repo":{"name":"alice/work","html_url":"https://github.com/alice/work","description":"work"},"created_at":"2026-07-01T00:00:00Z"}]`); err != nil {
				t.Errorf("write events response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))

	data, err := client.Fetch(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got, want := repoNames(data.RecentStarredRepos), []string{"owner/new-star", "owner/older-star"}; !sameStrings(got, want) {
		t.Errorf("RecentStarredRepos = %v, want %v", got, want)
	}
	if got, want := repoNames(data.RecentCreatedRepos), []string{"created"}; !sameStrings(got, want) {
		t.Errorf("RecentCreatedRepos = %v, want %v", got, want)
	}
	if got, want := names(data.RecentContributions), []string{"alice/work"}; !sameStrings(got, want) {
		t.Errorf("RecentContributions = %v, want %v", got, want)
	}
}

func TestFetchReturnsGitHubErrors(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "GitHub unavailable", http.StatusServiceUnavailable)
	}))

	_, err := client.Fetch(context.Background(), "alice")
	if err == nil || !strings.Contains(err.Error(), "list starred repos") {
		t.Fatalf("Fetch error = %v, want starred-repos error", err)
	}
}

func TestNewClient(t *testing.T) {
	if client := NewClient(context.Background(), ""); client == nil || client.gh == nil {
		t.Fatal("NewClient returned an uninitialized client")
	}
	if client := NewClient(context.Background(), "test-token"); client == nil || client.gh == nil {
		t.Fatal("NewClient with token returned an uninitialized client")
	}
}

func TestMax(t *testing.T) {
	if got := max(2, 5); got != 5 {
		t.Errorf("max(2, 5) = %d, want 5", got)
	}
	if got := max(5, 2); got != 5 {
		t.Errorf("max(5, 2) = %d, want 5", got)
	}
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	gh, err := github.NewClient(github.WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		response := recorder.Result()
		response.Request = req
		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	return &Client{gh: gh}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func repoNames(repos []Repo) []string {
	names := make([]string, len(repos))
	for i, repo := range repos {
		names[i] = repo.Name
	}
	return names
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// strPtr is a small helper to take the address of a string literal.
func strPtr(s string) *string { return &s }

func newEvent(eventType, repoName, htmlURL, description string, at time.Time) *github.Event {
	return &github.Event{
		Type: strPtr(eventType),
		Repo: &github.Repository{
			Name:        strPtr(repoName),
			HTMLURL:     &htmlURL,
			Description: &description,
		},
		CreatedAt: &github.Timestamp{Time: at},
	}
}

func TestFilterAndDedupeEvents_ExcludesWatchEvents(t *testing.T) {
	now := time.Now()
	events := []*github.Event{
		newEvent("PushEvent", "toozej/toozej", "https://github.com/toozej/toozej", "self", now.Add(-1*time.Hour)),
		newEvent("WatchEvent", "stupside/castor", "https://github.com/stupside/castor", "starred", now.Add(-30*time.Minute)),
		newEvent("PullRequestEvent", "toozej/tools", "https://github.com/toozej/tools", "tools", now.Add(-2*time.Hour)),
		newEvent("WatchEvent", "perplexityai/bumblebee", "https://github.com/perplexityai/bumblebee", "starred", now.Add(-10*time.Minute)),
	}

	got := filterAndDedupeEvents(events)

	if len(got) != 2 {
		t.Fatalf("expected 2 contributions after filtering WatchEvents, got %d: %+v", len(got), names(got))
	}
	for _, c := range got {
		if c.Repo.Name == "stupside/castor" || c.Repo.Name == "perplexityai/bumblebee" {
			t.Errorf("starred repo %q should be excluded, got %+v", c.Repo.Name, c)
		}
	}
}

func TestFilterAndDedupeEventsExcluding_ExcludesStarredRepos(t *testing.T) {
	now := time.Now()
	events := []*github.Event{
		newEvent("PushEvent", "toozej/toozej", "https://github.com/toozej/toozej", "profile", now),
		newEvent("PushEvent", "stupside/castor", "https://github.com/stupside/castor", "starred", now.Add(-time.Hour)),
	}

	got := filterAndDedupeEventsExcluding(events, map[string]struct{}{"stupside/castor": {}})

	if len(got) != 1 || got[0].Repo.Name != "toozej/toozej" {
		t.Errorf("starred repository was not excluded: %+v", names(got))
	}
}

func TestFilterAndDedupeEvents_DedupesByRepo(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-2 * time.Hour)
	events := []*github.Event{
		newEvent("PushEvent", "toozej/toozej", "https://github.com/toozej/toozej", "self", now),
		newEvent("PullRequestEvent", "toozej/toozej", "https://github.com/toozej/toozej", "self", earlier),
		newEvent("PushEvent", "toozej/tools", "https://github.com/toozej/tools", "tools", now.Add(-1*time.Hour)),
	}

	got := filterAndDedupeEvents(events)

	if len(got) != 2 {
		t.Fatalf("expected 2 unique repos, got %d: %+v", len(got), names(got))
	}
	// First entry should be the most recent push on toozej/toozej (now).
	if got[0].Repo.Name != "toozej/toozej" || !got[0].OccurredAt.Equal(now) {
		t.Errorf("first entry = %+v, want toozej/toozej at %v", got[0], now)
	}
}

func TestFilterAndDedupeEvents_SortedDescendingByOccurredAt(t *testing.T) {
	now := time.Now()
	events := []*github.Event{
		newEvent("PushEvent", "toozej/a", "https://github.com/toozej/a", "a", now.Add(-3*time.Hour)),
		newEvent("PushEvent", "toozej/b", "https://github.com/toozej/b", "b", now.Add(-1*time.Hour)),
		newEvent("PushEvent", "toozej/c", "https://github.com/toozej/c", "c", now.Add(-2*time.Hour)),
	}

	got := filterAndDedupeEvents(events)

	if len(got) != 3 {
		t.Fatalf("expected 3 contributions, got %d", len(got))
	}
	want := []string{"toozej/b", "toozej/c", "toozej/a"}
	for i, w := range want {
		if got[i].Repo.Name != w {
			t.Errorf("got[%d].Repo.Name = %q, want %q", i, got[i].Repo.Name, w)
		}
		if i > 0 && got[i].OccurredAt.After(got[i-1].OccurredAt) {
			t.Errorf("result not sorted descending at index %d", i)
		}
	}
}

func TestFilterAndDedupeEvents_SkipsNilAndMissingRepo(t *testing.T) {
	now := time.Now()
	events := []*github.Event{
		nil,
		{Type: strPtr("PushEvent")}, // no Repo
		newEvent("PushEvent", "toozej/toozej", "https://github.com/toozej/toozej", "self", now),
	}

	got := filterAndDedupeEvents(events)

	if len(got) != 1 {
		t.Fatalf("expected 1 contribution, got %d: %+v", len(got), names(got))
	}
	if got[0].Repo.Name != "toozej/toozej" {
		t.Errorf("got %q, want toozej/toozej", got[0].Repo.Name)
	}
}

func TestFilterAndDedupeEvents_Empty(t *testing.T) {
	if got := filterAndDedupeEvents(nil); len(got) != 0 {
		t.Errorf("expected empty result for nil input, got %+v", got)
	}
	if got := filterAndDedupeEvents([]*github.Event{}); len(got) != 0 {
		t.Errorf("expected empty result for empty input, got %+v", got)
	}
}

func names(cs []Contribution) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Repo.Name
	}
	return out
}
