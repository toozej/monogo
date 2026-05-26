package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkflowParser_FindWorkflowFiles(t *testing.T) {
	parser := NewParser()

	// Create temporary directory structure
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	err := os.MkdirAll(workflowsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workflows directory: %v", err)
	}

	// Create test workflow files
	testFiles := []string{
		"ci.yml",
		"release.yaml",
		"test.yml",
		"not-a-workflow.txt", // Should be ignored
	}

	for _, file := range testFiles {
		path := filepath.Join(workflowsDir, file)
		content := "name: test\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n"
		if file == "not-a-workflow.txt" {
			content = "not yaml"
		}
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}

	files, err := parser.FindWorkflowFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindWorkflowFiles failed: %v", err)
	}

	expectedFiles := 3 // Only .yml and .yaml files
	if len(files) != expectedFiles {
		t.Errorf("Expected %d workflow files, got %d", expectedFiles, len(files))
	}

	// Check that all expected files are found
	expectedPaths := map[string]bool{
		filepath.Join(workflowsDir, "ci.yml"):       true,
		filepath.Join(workflowsDir, "release.yaml"): true,
		filepath.Join(workflowsDir, "test.yml"):     true,
	}

	for _, file := range files {
		if !expectedPaths[file] {
			t.Errorf("Unexpected file found: %s", file)
		}
	}
}

func TestWorkflowParser_ParseWorkflowFile(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		content  string
		expected []string
		hasError bool
	}{
		{
			name: "workflow with subpath uses",
			content: `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: github/codeql-action/init@v4.35.2
      - uses: github/codeql-action/autobuild@v4.35.4
      - uses: github/codeql-action/analyze@v4.35.4`,
			expected: []string{"github/codeql-action"},
			hasError: false,
		},
		{
			name: "valid workflow with uses",
			content: `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test ./...
      - uses: golangci/golangci-lint-action@v3`,
			expected: []string{"actions/checkout", "actions/setup-go", "golangci/golangci-lint-action"},
			hasError: false,
		},
		{
			name: "workflow without uses",
			content: `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo "hello"`,
			expected: []string{},
			hasError: false,
		},
		{
			name: "invalid yaml",
			content: `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: invalid
        uses: actions/setup-go@v4
        invalid_field: [unclosed bracket
  another:
    runs-on: ubuntu-latest`,
			expected: []string{}, // Should fail to parse, so no uses extracted
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "workflow.yml")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			result, err := parser.ParseWorkflowFile(filePath)

			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result != nil {
				if result.Uses == nil && len(tt.expected) == 0 {
					// Both are effectively empty
				} else if !reflect.DeepEqual(result.Uses, tt.expected) {
					t.Errorf("Expected uses %v (len=%d), got %v (len=%d)", tt.expected, len(tt.expected), result.Uses, len(result.Uses))
				}
				if result.Path != filePath {
					t.Errorf("Expected path %s, got %s", filePath, result.Path)
				}
			}
		})
	}
}

func TestWorkflowParser_extractUsesFromYAML(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "simple uses",
			yaml: `jobs:
  test:
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4`,
			expected: []string{"actions/checkout", "actions/setup-go"},
		},
		{
			name: "complex workflow structure",
			yaml: `jobs:
  test:
    steps:
      - uses: actions/checkout@v3
  build:
    steps:
      - uses: docker/build-push-action@v4
  matrix:
    strategy:
      matrix:
        go-version: [1.19, 1.20]
    steps:
      - uses: actions/setup-go@v4`,
			expected: []string{"actions/checkout", "actions/setup-go", "docker/build-push-action"},
		},
		{
			name: "duplicates removed",
			yaml: `jobs:
  test:
    steps:
      - uses: actions/checkout@v3
      - uses: actions/checkout@v3`,
			expected: []string{"actions/checkout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uses, err := parser.extractUsesFromYAML([]byte(tt.yaml))
			if err != nil {
				t.Errorf("extractUsesFromYAML failed: %v", err)
			}

			if !reflect.DeepEqual(uses, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, uses)
			}
		})
	}
}

func TestWorkflowParser_deduplicateAndClean(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "subpath actions extract repo only",
			input:    []string{"github/codeql-action/init@v4.35.2", "github/codeql-action/autobuild@v4.35.4"},
			expected: []string{"github/codeql-action"},
		},
		{
			name:     "no duplicates",
			input:    []string{"actions/checkout@v3", "actions/setup-go@v4"},
			expected: []string{"actions/checkout", "actions/setup-go"},
		},
		{
			name:     "with duplicates",
			input:    []string{"actions/checkout@v3", "actions/checkout@v3", "actions/setup-go@v4"},
			expected: []string{"actions/checkout", "actions/setup-go"},
		},
		{
			name:     "empty and invalid",
			input:    []string{"", "actions/checkout@v3", "invalid"},
			expected: []string{"actions/checkout"},
		},
		{
			name:     "sorted output",
			input:    []string{"docker/build-push-action@v4", "actions/checkout@v3", "actions/setup-go@v4"},
			expected: []string{"actions/checkout", "actions/setup-go", "docker/build-push-action"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.deduplicateAndClean(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWorkflowParser_deduplicateAndCleanWithVersions(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    []string
		expected []ActionRef
	}{
		{
			name:  "subpath actions",
			input: []string{"github/codeql-action/init@v4.35.2", "github/codeql-action/autobuild@v4.35.4"},
			expected: []ActionRef{
				{OwnerRepo: "github/codeql-action", Subpath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
				{OwnerRepo: "github/codeql-action", Subpath: "autobuild", Version: "v4.35.4", FullRef: "github/codeql-action/autobuild@v4.35.4"},
			},
		},
		{
			name:  "subpath and plain same repo - both kept",
			input: []string{"owner/repo@v1", "owner/repo/sub@v2"},
			expected: []ActionRef{
				{OwnerRepo: "owner/repo", Subpath: "", Version: "v1", FullRef: "owner/repo@v1"},
				{OwnerRepo: "owner/repo", Subpath: "sub", Version: "v2", FullRef: "owner/repo/sub@v2"},
			},
		},
		{
			name:  "no duplicates",
			input: []string{"actions/checkout@v3", "actions/setup-go@v4"},
			expected: []ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
			},
		},
		{
			name:  "with duplicates - keeps first",
			input: []string{"actions/checkout@v3", "actions/checkout@v2", "actions/setup-go@v4"},
			expected: []ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
			},
		},
		{
			name:  "branch ref",
			input: []string{"actions/checkout@main", "actions/setup-go@master"},
			expected: []ActionRef{
				{OwnerRepo: "actions/checkout", Version: "main", FullRef: "actions/checkout@main"},
				{OwnerRepo: "actions/setup-go", Version: "master", FullRef: "actions/setup-go@master"},
			},
		},
		{
			name:  "commit sha",
			input: []string{"actions/checkout@abc123def456"},
			expected: []ActionRef{
				{OwnerRepo: "actions/checkout", Version: "abc123def456", FullRef: "actions/checkout@abc123def456"},
			},
		},
		{
			name:     "empty and invalid",
			input:    []string{"", "actions/checkout@v3", "invalid"},
			expected: []ActionRef{{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"}},
		},
		{
			name:  "sorted output",
			input: []string{"docker/build-push-action@v4", "actions/checkout@v3", "actions/setup-go@v4"},
			expected: []ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
				{OwnerRepo: "docker/build-push-action", Version: "v4", FullRef: "docker/build-push-action@v4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.deduplicateAndCleanWithVersions(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWorkflowParser_ParseWorkflowFiles(t *testing.T) {
	parser := NewParser()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.yml")
	file2 := filepath.Join(tmpDir, "file2.yml")

	if err := os.WriteFile(file1, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/setup-go@v4"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	paths := []string{file1, file2}
	results, err := parser.ParseWorkflowFiles(paths)

	if err != nil {
		t.Fatalf("ParseWorkflowFiles failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 workflow files, got %d", len(results))
	}
}

func TestWorkflowParser_GetAllUsesFromRepo(t *testing.T) {
	parser := NewParser()

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows directory: %v", err)
	}

	file1 := filepath.Join(workflowsDir, "ci.yml")
	file2 := filepath.Join(workflowsDir, "release.yml")

	if err := os.WriteFile(file1, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("jobs:\n  build:\n    steps:\n      - uses: actions/checkout@v3\n      - uses: actions/setup-go@v4"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	uses, files, err := parser.GetAllUsesFromRepo(tmpDir)

	if err != nil {
		t.Fatalf("GetAllUsesFromRepo failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	expectedUses := []string{"actions/checkout", "actions/setup-go"}
	if !reflect.DeepEqual(uses, expectedUses) {
		t.Errorf("Expected uses %v, got %v", expectedUses, uses)
	}

	// Test error scenarios
	workflows, _ := parser.ParseWorkflowFiles([]string{"nonexistent.yml"})
	if len(workflows) != 1 || workflows[0].Error == nil {
		t.Error("Expected error inside WorkflowFile for nonexistent file")
	}

	uses_all, files, err := parser.GetAllUsesFromRepo("/path/that/does/not/exist/surely")
	if err != nil {
		t.Errorf("Expected nil error from GetAllUsesFromRepo for nonexistent directory, got %v", err)
	}
	if len(uses_all) != 0 || len(files) != 0 {
		t.Errorf("Expected 0 uses and files, got %d and %d", len(uses_all), len(files))
	}
}

func TestWorkflowParser_FindWorkflowFilesInDir(t *testing.T) {
	parser := NewParser()

	tmpDir := t.TempDir()

	files, err := parser.FindWorkflowFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("FindWorkflowFilesInDir failed for empty dir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 files from nonexistent directory, got %d", len(files))
	}

	testFiles := []string{"ci.yml", "release.yaml", "test.yml", "not-a-workflow.txt"}
	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		content := "name: test\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n"
		if file == "not-a-workflow.txt" {
			content = "not yaml"
		}
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}

	files, err = parser.FindWorkflowFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("FindWorkflowFilesInDir failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 workflow files, got %d", len(files))
	}
}

func TestWorkflowParser_FindReposWithWorkflows(t *testing.T) {
	parser := NewParser()

	tmpDir := t.TempDir()

	repos, err := parser.FindReposWithWorkflows(tmpDir)
	if err != nil {
		t.Fatalf("FindReposWithWorkflows failed: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("Expected 0 repos from empty directory, got %d", len(repos))
	}

	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")
	repo3 := filepath.Join(tmpDir, "repo3-no-workflows")

	for _, repo := range []string{repo1, repo2, repo3} {
		if err := os.MkdirAll(repo, 0755); err != nil {
			t.Fatalf("Failed to create repo directory %s: %v", repo, err)
		}
	}

	workflowsDir1 := filepath.Join(repo1, ".github", "workflows")
	workflowsDir2 := filepath.Join(repo2, ".github", "workflows")

	for _, dir := range []string{workflowsDir1, workflowsDir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create workflows directory %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(workflowsDir1, "ci.yml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir2, "release.yml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	repos, err = parser.FindReposWithWorkflows(tmpDir)
	if err != nil {
		t.Fatalf("FindReposWithWorkflows failed: %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("Expected 2 repos with workflows, got %d", len(repos))
	}

	repoPaths := make(map[string]bool)
	for _, repo := range repos {
		repoPaths[repo] = true
	}
	if !repoPaths[repo1] || !repoPaths[repo2] {
		t.Errorf("Expected repo1 and repo2 to be found, got %v", repos)
	}
	if repoPaths[repo3] {
		t.Errorf("repo3 should not be found as it has no workflows")
	}
}

func TestWorkflowParser_GetAllUsesFromRepoWithVersions(t *testing.T) {
	parser := NewParser()

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows directory: %v", err)
	}

	file1 := filepath.Join(workflowsDir, "ci.yml")
	file2 := filepath.Join(workflowsDir, "security.yml")

	if err := os.WriteFile(file1, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n      - uses: github/codeql-action/init@v4.35.2\n"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("jobs:\n  analyze:\n    steps:\n      - uses: github/codeql-action/autobuild@v4.35.4\n      - uses: github/codeql-action/analyze@v4.35.4\n"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	actionRefs, files, err := parser.GetAllUsesFromRepoWithVersions(tmpDir)

	if err != nil {
		t.Fatalf("GetAllUsesFromRepoWithVersions failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	if len(actionRefs) != 4 {
		t.Fatalf("Expected 4 unique action refs, got %d: %v", len(actionRefs), actionRefs)
	}

	expected := map[string]ActionRef{
		"actions/checkout":               {OwnerRepo: "actions/checkout", Subpath: "", Version: "v3", FullRef: "actions/checkout@v3"},
		"github/codeql-action/init":      {OwnerRepo: "github/codeql-action", Subpath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
		"github/codeql-action/autobuild": {OwnerRepo: "github/codeql-action", Subpath: "autobuild", Version: "v4.35.4", FullRef: "github/codeql-action/autobuild@v4.35.4"},
		"github/codeql-action/analyze":   {OwnerRepo: "github/codeql-action", Subpath: "analyze", Version: "v4.35.4", FullRef: "github/codeql-action/analyze@v4.35.4"},
	}

	for _, ref := range actionRefs {
		key := ref.OwnerRepo
		if ref.Subpath != "" {
			key = ref.OwnerRepo + "/" + ref.Subpath
		}
		exp, ok := expected[key]
		if !ok {
			t.Errorf("Unexpected action ref: %v", ref)
			continue
		}
		if ref.OwnerRepo != exp.OwnerRepo || ref.Subpath != exp.Subpath || ref.Version != exp.Version || ref.FullRef != exp.FullRef {
			t.Errorf("Expected ref %v, got %v", exp, ref)
		}
	}
}
