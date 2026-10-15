package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDependabotPagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Error("missing API Accept header")
		}
		if r.URL.Query().Get("page") != "2" {
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?per_page=100&page=2&base=main&state=open>; rel="next"`, server.URL, r.URL.Path))
		}
		switch r.URL.Path {
		case "/repos/test/repo/releases":
			_ = json.NewEncoder(w).Encode([]ReleaseInfo{{TagName: "v" + r.URL.Query().Get("page")}})
		case "/repos/test/repo/pulls":
			_ = json.NewEncoder(w).Encode([]PullRequest{{Number: 1, State: "open", User: Owner{Login: "dependabot[bot]"}, Base: PullRequestRef{Ref: "main"}}})
		case "/repos/test/repo/pulls/1/files":
			_ = json.NewEncoder(w).Encode([]PullRequestFile{{Filename: "workflow" + r.URL.Query().Get("page")}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := NewClientWithHTTP(server.URL, server.Client(), WithCache(false, false, 0))
	releases, err := c.ListReleases(context.Background(), "test/repo")
	if err != nil || len(releases) != 2 {
		t.Fatalf("releases = %+v, error = %v", releases, err)
	}
	prs, err := c.ListDependabotPullRequests(context.Background(), "test/repo", "main")
	if err != nil || len(prs) != 2 {
		t.Fatalf("PRs = %+v, error = %v", prs, err)
	}
	files, err := c.ListPullRequestFiles(context.Background(), "test/repo", 1)
	if err != nil || len(files) != 2 {
		t.Fatalf("files = %+v, error = %v", files, err)
	}
}

func TestDependabotDefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/repo":
			_, _ = fmt.Fprint(w, `{"default_branch":"release/next"}`)
		case "/repos/test/repo/pulls":
			if r.URL.Query().Get("base") != "release/next" {
				t.Errorf("base = %q", r.URL.Query().Get("base"))
			}
			_, _ = fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := NewClientWithHTTP(server.URL, server.Client(), WithCache(false, false, 0))
	if _, err := c.ListDependabotPullRequests(context.Background(), "test/repo", ""); err != nil {
		t.Fatal(err)
	}
}

func TestListPagesRejectsRepeatedLink(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?per_page=100>; rel="next"`, server.URL, r.URL.Path))
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer server.Close()
	c := NewClientWithHTTP(server.URL, server.Client(), WithCache(false, false, 0))
	if _, err := c.ListReleases(context.Background(), "test/repo"); err == nil {
		t.Fatal("expected pagination error")
	}
}
