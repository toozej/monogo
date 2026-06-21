package issue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v85/github"
)

func TestIssueCreator_CreateArchivedActionIssue(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantErr     bool
		errContains string
	}{
		{
			name:       "201 success",
			statusCode: 201,
			wantErr:    false,
		},
		{
			name:       "422 already exists",
			statusCode: 422,
			wantErr:    false,
		},
		{
			name:        "500 server error",
			statusCode:  500,
			wantErr:     true,
			errContains: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := []ArchivedActionInfo{
				{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
			}
			var createdIssue *github.IssueRequest

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/owner/repo/issues" {
					switch tt.statusCode {
					case 201:
						createdIssue = &github.IssueRequest{
							Title:  github.Ptr("Replace archived GitHub Actions"),
							Body:   github.Ptr(BuildIssueBody(actions)),
							Labels: &[]string{"maintenance", "github-actions", "security"},
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(201)
						_, _ = fmt.Fprintln(w, `{"number":42,"title":"Replace archived GitHub Actions"}`)
					case 422:
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(422)
						_, _ = fmt.Fprintln(w, `{"message":"Validation Failed"}`)
					default:
						w.WriteHeader(tt.statusCode)
					}
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()

			client, _ := github.NewClient(nil).WithEnterpriseURLs(server.URL+"/", server.URL+"/")

			creator := &IssueCreator{
				token:  "test-token",
				client: client,
			}

			ctx := context.Background()
			err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", actions)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain '%s', got '%v'", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("Expected nil error, got %v", err)
			}

			if !tt.wantErr && tt.statusCode == 201 && createdIssue == nil {
				t.Error("Expected issue to be created")
			}
		})
	}
}

func TestIssueCreator_CreateArchivedActionIssue_EmptyActions(t *testing.T) {
	creator := &IssueCreator{
		token:  "test-token",
		client: nil,
	}

	ctx := context.Background()
	err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", []ArchivedActionInfo{})

	if err != nil {
		t.Errorf("Expected nil error for empty actions, got %v", err)
	}
}

func TestIssueCreator_CreateArchivedActionIssue_TestImpl(t *testing.T) {
	tests := []struct {
		name        string
		fn          TestIssueFunc
		wantErr     bool
		errContains string
	}{
		{
			name: "success via test impl",
			fn: func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "error via test impl",
			fn: func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
				return fmt.Errorf("API error")
			},
			wantErr:     true,
			errContains: "API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewTestIssueCreator(tt.fn)

			actions := []ArchivedActionInfo{
				{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
			}

			ctx := context.Background()
			err := ic.CreateArchivedActionIssue(ctx, "owner", "repo", actions)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain '%s', got %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("Expected nil error, got %v", err)
			}
		})
	}
}

func TestNewIssueCreator(t *testing.T) {
	token := "test-token"
	creator := NewIssueCreator(token)

	if creator.token != token {
		t.Errorf("Expected token %s, got %s", token, creator.token)
	}

	if creator.client == nil {
		t.Error("Expected client to be set")
	}
}

func TestNewIssueCreator_Token(t *testing.T) {
	token := "ghp_abcdef123456"
	creator := NewIssueCreator(token)

	if creator.token != token {
		t.Errorf("Expected token '%s', got '%s'", token, creator.token)
	}
	if creator.client == nil {
		t.Error("Expected client to not be nil")
	}
}

func TestIssueCreatorIface_TestImpl(t *testing.T) {
	var called bool
	var capturedOwner, capturedRepo string
	var capturedActions []ArchivedActionInfo

	ic := NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
		called = true
		capturedOwner = owner
		capturedRepo = repo
		capturedActions = archivedActions
		return nil
	})

	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	ctx := context.Background()
	err := ic.CreateArchivedActionIssue(ctx, "testowner", "testrepo", actions)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	if !called {
		t.Error("Expected test function to be called")
	}
	if capturedOwner != "testowner" {
		t.Errorf("Expected owner 'testowner', got '%s'", capturedOwner)
	}
	if capturedRepo != "testrepo" {
		t.Errorf("Expected repo 'testrepo', got '%s'", capturedRepo)
	}
	if len(capturedActions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(capturedActions))
	}
}

func TestBuildIssueBody(t *testing.T) {
	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
		{Repo: "actions/setup-go", Workflow: "test.yml", Uses: "actions/setup-go@v4"},
	}

	body := BuildIssueBody(actions)

	expectedContent := []string{
		"## Archived GitHub Actions Detected",
		"actions/checkout@v3",
		"actions/setup-go@v4",
		"## What does this mean?",
		"## Recommended Actions",
		"## Resources",
		"go-sort-out-gh-actions",
	}

	for _, content := range expectedContent {
		if !strings.Contains(body, content) {
			t.Errorf("Expected issue body to contain '%s'", content)
		}
	}
}

func TestBuildIssueBody_SingleAction(t *testing.T) {
	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	body := BuildIssueBody(actions)

	expectedSections := []string{
		"## Archived GitHub Actions Detected",
		"## What does this mean?",
		"## Recommended Actions",
		"## Resources",
		"go-sort-out-gh-actions",
	}
	for _, section := range expectedSections {
		if !strings.Contains(body, section) {
			t.Errorf("Expected body to contain '%s'", section)
		}
	}

	if !strings.Contains(body, "`actions/checkout@v3`") {
		t.Error("Expected body to contain the action uses in backticks")
	}
	if !strings.Contains(body, "`ci.yml`") {
		t.Error("Expected body to contain the workflow name in backticks")
	}
}

func TestBuildIssueBody_MultipleActions(t *testing.T) {
	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
		{Repo: "actions/setup-go", Workflow: "test.yml", Uses: "actions/setup-go@v4"},
		{Repo: "actions/cache", Workflow: "build.yml", Uses: "actions/cache@v3"},
	}

	body := BuildIssueBody(actions)

	for _, action := range actions {
		entry := fmt.Sprintf("`%s` (used in `%s`)", action.Uses, action.Workflow)
		if !strings.Contains(body, entry) {
			t.Errorf("Expected body to contain '%s'", entry)
		}
	}
}

func TestArchivedActionInfo_Fields(t *testing.T) {
	info := ArchivedActionInfo{
		Repo:     "actions/checkout",
		Workflow: "ci.yml",
		Uses:     "actions/checkout@v3",
	}

	if info.Repo != "actions/checkout" {
		t.Errorf("Expected Repo 'actions/checkout', got '%s'", info.Repo)
	}
	if info.Workflow != "ci.yml" {
		t.Errorf("Expected Workflow 'ci.yml', got '%s'", info.Workflow)
	}
	if info.Uses != "actions/checkout@v3" {
		t.Errorf("Expected Uses 'actions/checkout@v3', got '%s'", info.Uses)
	}
}
