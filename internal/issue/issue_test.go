package issue

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-github/v85/github"
)

func TestIssueCreator_CreateArchivedActionIssue(t *testing.T) {
	var createdIssue *github.IssueRequest

	creator := &IssueCreator{
		token:  "test-token",
		isTest: true,
		testImpl: func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
			tempCreator := &IssueCreator{}
			createdIssue = &github.IssueRequest{
				Title:  github.Ptr("Replace archived GitHub Actions"),
				Body:   github.Ptr(tempCreator.buildIssueBody(archivedActions)),
				Labels: &[]string{"maintenance", "github-actions", "security"},
			}
			return nil
		},
	}

	actions := []ArchivedActionInfo{
		{
			Repo:     "actions/checkout",
			Workflow: "ci.yml",
			Uses:     "actions/checkout@v3",
		},
		{
			Repo:     "actions/setup-go",
			Workflow: "ci.yml",
			Uses:     "actions/setup-go@v4",
		},
	}

	ctx := context.Background()
	err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", actions)

	if err != nil {
		t.Errorf("CreateArchivedActionIssue failed: %v", err)
	}

	if createdIssue == nil {
		t.Fatal("Expected issue to be created")
	}

	if *createdIssue.Title != "Replace archived GitHub Actions" {
		t.Errorf("Expected title 'Replace archived GitHub Actions', got '%s'", *createdIssue.Title)
	}

	body := *createdIssue.Body
	if !strings.Contains(body, "actions/checkout@v3") {
		t.Error("Expected issue body to contain actions/checkout@v3")
	}

	if !strings.Contains(body, "actions/setup-go@v4") {
		t.Error("Expected issue body to contain actions/setup-go@v4")
	}

	labels := *createdIssue.Labels
	expectedLabels := []string{"maintenance", "github-actions", "security"}
	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}
}

func TestIssueCreator_buildIssueBody(t *testing.T) {
	creator := &IssueCreator{}

	actions := []ArchivedActionInfo{
		{
			Repo:     "actions/checkout",
			Workflow: "ci.yml",
			Uses:     "actions/checkout@v3",
		},
		{
			Repo:     "actions/setup-go",
			Workflow: "test.yml",
			Uses:     "actions/setup-go@v4",
		},
	}

	body := creator.buildIssueBody(actions)

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

func TestIssueCreator_CreateArchivedActionIssue_EmptyActions(t *testing.T) {
	creator := &IssueCreator{
		token:  "test-token",
		isTest: true,
	}

	ctx := context.Background()
	err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", []ArchivedActionInfo{})

	if err != nil {
		t.Errorf("Expected nil error for empty actions, got %v", err)
	}
}

func TestIssueCreator_CreateArchivedActionIssue_422Status(t *testing.T) {
	creator := &IssueCreator{
		token:  "test-token",
		isTest: true,
		testImpl: func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
			return nil
		},
	}

	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	ctx := context.Background()
	err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", actions)

	if err != nil {
		t.Errorf("Expected nil error for 422 status, got %v", err)
	}
}

func TestIssueCreator_CreateArchivedActionIssue_Error(t *testing.T) {
	creator := &IssueCreator{
		token:  "test-token",
		isTest: true,
		testImpl: func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
			return fmt.Errorf("API error")
		},
	}

	actions := []ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	ctx := context.Background()
	err := creator.CreateArchivedActionIssue(ctx, "owner", "repo", actions)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("Expected error to contain 'API error', got %v", err)
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
