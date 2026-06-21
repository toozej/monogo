package workflow

import (
	"context"
	"fmt"
	"testing"
)

type mockRemoteContentFetcher struct {
	contents map[string]string
	err      error
}

func (m *mockRemoteContentFetcher) GetRemoteWorkflowContents(_ context.Context, _, _ string) (map[string]string, error) {
	return m.contents, m.err
}

func TestWorkflowParser_ParseWorkflowContent(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name      string
		path      string
		content   string
		wantUses  []string
		wantError bool
	}{
		{
			name:     "valid workflow with uses",
			path:     ".github/workflows/ci.yml",
			content:  "name: CI\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v3\n      - uses: actions/setup-go@v4\n",
			wantUses: []string{"actions/checkout", "actions/setup-go"},
		},
		{
			name:      "invalid yaml",
			path:      ".github/workflows/bad.yml",
			content:   "{{invalid",
			wantError: true,
		},
		{
			name:     "empty content - no uses",
			path:     ".github/workflows/empty.yml",
			content:  "name: Empty\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hello\n",
			wantUses: []string{},
		},
		{
			name:     "workflow with subpath actions",
			path:     ".github/workflows/codeql.yml",
			content:  "name: CodeQL\non: push\njobs:\n  analyze:\n    steps:\n      - uses: github/codeql-action/init@v3\n      - uses: github/codeql-action/analyze@v3\n",
			wantUses: []string{"github/codeql-action"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseWorkflowContent(tt.path, tt.content)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantError && result != nil {
				if result.Path != tt.path {
					t.Errorf("Expected path %s, got %s", tt.path, result.Path)
				}
				if len(tt.wantUses) == 0 && len(result.Uses) == 0 {
					return
				}
				if len(result.Uses) != len(tt.wantUses) {
					t.Errorf("Expected %d uses, got %d", len(tt.wantUses), len(result.Uses))
				}
				for i, use := range result.Uses {
					if use != tt.wantUses[i] {
						t.Errorf("Expected use[%d] = %s, got %s", i, tt.wantUses[i], use)
					}
				}
			}
		})
	}
}

func TestWorkflowParser_ParseWorkflowContent_SetsUsesWithVersions(t *testing.T) {
	parser := NewParser()

	content := "name: CI\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v3\n"
	result, err := parser.ParseWorkflowContent(".github/workflows/ci.yml", content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.UsesWithVersions) != 1 {
		t.Fatalf("Expected 1 action ref, got %d", len(result.UsesWithVersions))
	}

	ref := result.UsesWithVersions[0]
	if ref.OwnerRepo != "actions/checkout" {
		t.Errorf("Expected OwnerRepo actions/checkout, got %s", ref.OwnerRepo)
	}
	if ref.Version != "v3" {
		t.Errorf("Expected Version v3, got %s", ref.Version)
	}
}

func TestWorkflowParser_GetRemoteUsesFromRepo_Success(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	fetcher := &mockRemoteContentFetcher{
		contents: map[string]string{
			".github/workflows/ci.yml":      "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n",
			".github/workflows/release.yml": "name: Release\non: push\njobs:\n  build:\n    steps:\n      - uses: actions/setup-go@v4\n      - uses: actions/checkout@v3\n",
		},
	}

	actionRefs, workflows, err := parser.GetRemoteUsesFromRepo(ctx, fetcher, "owner/repo", "main")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(workflows) != 2 {
		t.Errorf("Expected 2 workflows, got %d", len(workflows))
	}

	if len(actionRefs) != 2 {
		t.Errorf("Expected 2 unique action refs, got %d", len(actionRefs))
	}

	refNames := make(map[string]bool)
	for _, ref := range actionRefs {
		refNames[ref.OwnerRepo] = true
	}
	if !refNames["actions/checkout"] {
		t.Error("Expected actions/checkout in action refs")
	}
	if !refNames["actions/setup-go"] {
		t.Error("Expected actions/setup-go in action refs")
	}
}

func TestWorkflowParser_GetRemoteUsesFromRepo_404NoWorkflows(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	fetcher := &mockRemoteContentFetcher{
		contents: map[string]string{},
	}

	actionRefs, workflows, err := parser.GetRemoteUsesFromRepo(ctx, fetcher, "owner/repo", "main")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(actionRefs) != 0 {
		t.Errorf("Expected 0 action refs, got %d", len(actionRefs))
	}
	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows, got %d", len(workflows))
	}
}

func TestWorkflowParser_GetRemoteUsesFromRepo_Deduplicates(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	fetcher := &mockRemoteContentFetcher{
		contents: map[string]string{
			".github/workflows/ci.yml":    "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n",
			".github/workflows/build.yml": "name: Build\non: push\njobs:\n  build:\n    steps:\n      - uses: actions/checkout@v3\n",
		},
	}

	actionRefs, _, err := parser.GetRemoteUsesFromRepo(ctx, fetcher, "owner/repo", "main")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(actionRefs) != 1 {
		t.Errorf("Expected 1 deduplicated action ref, got %d", len(actionRefs))
	}
	if len(actionRefs) > 0 && actionRefs[0].OwnerRepo != "actions/checkout" {
		t.Errorf("Expected actions/checkout, got %s", actionRefs[0].OwnerRepo)
	}
}

func TestWorkflowParser_GetRemoteUsesFromRepo_FetcherError(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	fetcher := &mockRemoteContentFetcher{
		err: fmt.Errorf("fetch failed"),
	}

	_, _, err := parser.GetRemoteUsesFromRepo(ctx, fetcher, "owner/repo", "main")
	if err == nil {
		t.Error("Expected error when fetcher returns error")
	}
}
