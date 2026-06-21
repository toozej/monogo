package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_ListOrgRepos(t *testing.T) {
	t.Parallel()

	repoPage1 := []RepoEntry{
		{FullName: "org/repo1", Name: "repo1", Archived: false, Private: false, Fork: false},
		{FullName: "org/repo2", Name: "repo2", Archived: true, Private: true, Fork: false},
	}
	repoPage2 := []RepoEntry{
		{FullName: "org/repo3", Name: "repo3", Archived: false, Private: false, Fork: true},
	}

	tests := []struct {
		name       string
		org        string
		opts       *ListOrgReposOptions
		handler    func(w http.ResponseWriter, r *http.Request)
		wantCount  int
		wantError  bool
		wantErrSub string
	}{
		{
			name: "success with pagination",
			org:  "myorg",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/orgs/myorg/repos") {
					w.WriteHeader(404)
					return
				}
				if r.URL.Query().Get("per_page") != "100" {
					t.Errorf("expected per_page=100, got %s", r.URL.Query().Get("per_page"))
				}
				if r.URL.Query().Get("page") == "2" {
					w.WriteHeader(200)
					body, _ := json.Marshal(repoPage2)
					_, _ = w.Write(body)
					return
				}
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
		{
			name: "success single page",
			org:  "myorg",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
		{
			name: "empty org",
			org:  "emptyorg",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte("[]"))
			},
			wantCount: 0,
			wantError: false,
		},
		{
			name: "404 org",
			org:  "notfound",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			},
			wantError:  true,
			wantErrSub: "not found",
		},
		{
			name: "rate limited",
			org:  "ratelimited",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(403)
			},
			wantError:  true,
			wantErrSub: "rate limited",
		},
		{
			name: "OnlyActive filters archived",
			org:  "myorg",
			opts: &ListOrgReposOptions{OnlyActive: true},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 1,
			wantError: false,
		},
		{
			name: "IncludeForks includes forks",
			org:  "myorg",
			opts: &ListOrgReposOptions{IncludeForks: true},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(append(repoPage1, repoPage2...))
				_, _ = w.Write(body)
			},
			wantCount: 3,
			wantError: false,
		},
		{
			name: "exclude forks by default",
			org:  "myorg",
			opts: &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(append(repoPage1, repoPage2...))
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()

			repos, err := client.ListOrgRepos(ctx, tt.org, tt.opts)

			if tt.wantError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErrSub, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(repos) != tt.wantCount {
				t.Errorf("Expected %d repos, got %d", tt.wantCount, len(repos))
			}
		})
	}
}

func TestClient_ListOrgRepos_Pagination(t *testing.T) {
	t.Parallel()

	page1 := []RepoEntry{
		{FullName: "org/repo1", Name: "repo1", Archived: false, Private: false, Fork: false},
		{FullName: "org/repo2", Name: "repo2", Archived: false, Private: false, Fork: false},
	}
	page2 := []RepoEntry{
		{FullName: "org/repo3", Name: "repo3", Archived: false, Private: false, Fork: false},
	}
	page3 := []RepoEntry{
		{FullName: "org/repo4", Name: "repo4", Archived: false, Private: false, Fork: false},
	}

	callCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			w.Header().Set("Link", fmt.Sprintf(`<%s/orgs/testorg/repos?per_page=100&page=2>; rel="next"`, server.URL))
			body, _ := json.Marshal(page1)
			w.WriteHeader(200)
			_, _ = w.Write(body)
		case 2:
			w.Header().Set("Link", fmt.Sprintf(`<%s/orgs/testorg/repos?per_page=100&page=3>; rel="next"`, server.URL))
			body, _ := json.Marshal(page2)
			w.WriteHeader(200)
			_, _ = w.Write(body)
		default:
			body, _ := json.Marshal(page3)
			w.WriteHeader(200)
			_, _ = w.Write(body)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	repos, err := client.ListOrgRepos(ctx, "testorg", &ListOrgReposOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 4 {
		t.Errorf("Expected 4 repos across 3 pages, got %d", len(repos))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 API calls for pagination, got %d", callCount)
	}
}

func TestClient_ListUserRepos(t *testing.T) {
	t.Parallel()

	repoPage1 := []RepoEntry{
		{FullName: "user/repo1", Name: "repo1", Archived: false, Private: false, Fork: false},
		{FullName: "user/repo2", Name: "repo2", Archived: true, Private: true, Fork: false},
	}
	repoPage2 := []RepoEntry{
		{FullName: "user/repo3", Name: "repo3", Archived: false, Private: false, Fork: true},
	}

	tests := []struct {
		name       string
		username   string
		opts       *ListOrgReposOptions
		handler    func(w http.ResponseWriter, r *http.Request)
		wantCount  int
		wantError  bool
		wantErrSub string
	}{
		{
			name:     "success with pagination",
			username: "myuser",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/users/myuser/repos") {
					w.WriteHeader(404)
					return
				}
				if r.URL.Query().Get("page") == "2" {
					w.WriteHeader(200)
					body, _ := json.Marshal(repoPage2)
					_, _ = w.Write(body)
					return
				}
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
		{
			name:     "success single page",
			username: "myuser",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
		{
			name:     "empty user",
			username: "emptyuser",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte("[]"))
			},
			wantCount: 0,
			wantError: false,
		},
		{
			name:     "404 user",
			username: "notfound",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			},
			wantError:  true,
			wantErrSub: "not found",
		},
		{
			name:     "rate limited",
			username: "ratelimited",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(403)
			},
			wantError:  true,
			wantErrSub: "rate limited",
		},
		{
			name:     "OnlyActive filters archived",
			username: "myuser",
			opts:     &ListOrgReposOptions{OnlyActive: true},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(repoPage1)
				_, _ = w.Write(body)
			},
			wantCount: 1,
			wantError: false,
		},
		{
			name:     "IncludeForks includes forks",
			username: "myuser",
			opts:     &ListOrgReposOptions{IncludeForks: true},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(append(repoPage1, repoPage2...))
				_, _ = w.Write(body)
			},
			wantCount: 3,
			wantError: false,
		},
		{
			name:     "exclude forks by default",
			username: "myuser",
			opts:     &ListOrgReposOptions{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body, _ := json.Marshal(append(repoPage1, repoPage2...))
				_, _ = w.Write(body)
			},
			wantCount: 2,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.handler != nil {
					tt.handler(w, r)
					return
				}
				w.WriteHeader(200)
				_, _ = w.Write([]byte("[]"))
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()

			repos, err := client.ListUserRepos(ctx, tt.username, tt.opts)

			if tt.wantError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErrSub, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(repos) != tt.wantCount {
				t.Errorf("Expected %d repos, got %d", tt.wantCount, len(repos))
			}
		})
	}
}

func TestClient_ListUserRepos_Pagination(t *testing.T) {
	t.Parallel()

	page1 := []RepoEntry{
		{FullName: "user/repo1", Name: "repo1", Archived: false, Private: false, Fork: false},
	}
	page2 := []RepoEntry{
		{FullName: "user/repo2", Name: "repo2", Archived: false, Private: false, Fork: false},
	}

	callCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Link", fmt.Sprintf(`<%s/users/testuser/repos?per_page=100&page=2>; rel="next"`, server.URL))
			body, _ := json.Marshal(page1)
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		body, _ := json.Marshal(page2)
		w.WriteHeader(200)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	repos, err := client.ListUserRepos(ctx, "testuser", &ListOrgReposOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("Expected 2 repos across 2 pages, got %d", len(repos))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestParseNextLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		allowedHost string
		expected    string
	}{
		{
			name:        "valid next link matching host",
			input:       `<https://api.github.com/orgs/myorg/repos?page=2>; rel="next"`,
			allowedHost: "api.github.com",
			expected:    "https://api.github.com/orgs/myorg/repos?page=2",
		},
		{
			name:        "multiple links with next matching host",
			input:       `<https://api.github.com/orgs/myorg/repos?page=2>; rel="next", <https://api.github.com/orgs/myorg/repos?page=5>; rel="last"`,
			allowedHost: "api.github.com",
			expected:    "https://api.github.com/orgs/myorg/repos?page=2",
		},
		{
			name:        "next link with wrong host rejected",
			input:       `<https://evil.example.com/orgs/myorg/repos?page=2>; rel="next"`,
			allowedHost: "api.github.com",
			expected:    "",
		},
		{
			name:        "no next link",
			input:       `<https://api.github.com/orgs/myorg/repos?page=1>; rel="first"`,
			allowedHost: "api.github.com",
			expected:    "",
		},
		{
			name:        "empty header",
			input:       "",
			allowedHost: "api.github.com",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNextLink(tt.input, tt.allowedHost)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFilterRepos(t *testing.T) {
	t.Parallel()

	repos := []RepoEntry{
		{FullName: "org/active", Name: "active", Archived: false, Fork: false},
		{FullName: "org/archived", Name: "archived", Archived: true, Fork: false},
		{FullName: "org/forked", Name: "forked", Archived: false, Fork: true},
		{FullName: "org/archived-fork", Name: "archived-fork", Archived: true, Fork: true},
	}

	tests := []struct {
		name      string
		opts      *ListOrgReposOptions
		wantCount int
		wantNames []string
	}{
		{
			name:      "no filtering except default fork exclusion",
			opts:      &ListOrgReposOptions{},
			wantCount: 2,
			wantNames: []string{"org/active", "org/archived"},
		},
		{
			name:      "only active",
			opts:      &ListOrgReposOptions{OnlyActive: true},
			wantCount: 1,
			wantNames: []string{"org/active"},
		},
		{
			name:      "include forks",
			opts:      &ListOrgReposOptions{IncludeForks: true},
			wantCount: 4,
			wantNames: []string{"org/active", "org/archived", "org/forked", "org/archived-fork"},
		},
		{
			name:      "only active with forks",
			opts:      &ListOrgReposOptions{OnlyActive: true, IncludeForks: true},
			wantCount: 2,
			wantNames: []string{"org/active", "org/forked"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterRepos(repos, tt.opts)
			if len(result) != tt.wantCount {
				t.Errorf("Expected %d repos, got %d", tt.wantCount, len(result))
			}
			if len(result) == len(tt.wantNames) {
				for i, name := range tt.wantNames {
					if result[i].FullName != name {
						t.Errorf("Expected repo %s at index %d, got %s", name, i, result[i].FullName)
					}
				}
			}
		})
	}
}
