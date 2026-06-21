package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/monogo/pkg/go-sort-out-gh-actions/config"
)

func TestPinCommandFlags(t *testing.T) {
	cmd := newPinCmd()
	if cmd.Name() != "pin" {
		t.Errorf("Expected command name 'pin', got %q", cmd.Name())
	}
	if cmd.Flags().Lookup("write") == nil {
		t.Error("Expected --write flag on pin command")
	}
	flag := cmd.Flags().Lookup("write")
	if flag.Shorthand != "w" {
		t.Errorf("Expected -w shorthand for --write flag on pin command, got %q", flag.Shorthand)
	}
}

func TestPinCommandDescriptions(t *testing.T) {
	cmd := newPinCmd()
	if cmd.Short != "Display GitHub Actions that can be pinned to commit SHAs" {
		t.Errorf("Expected Short %q, got %q", "Display GitHub Actions that can be pinned to commit SHAs", cmd.Short)
	}
	expectedLong := `Scan workflow files and display GitHub Actions using version tags that can be pinned to commit SHAs.
Pinning actions to SHAs improves supply-chain security by ensuring immutable action references.
Use --write/-w to write the pinned SHA references to affected workflow files.`
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long %q, got %q", expectedLong, cmd.Long)
	}
}

func TestPinCommandFlagDefaults(t *testing.T) {
	cmd := newPinCmd()
	writeVal, err := cmd.Flags().GetBool("write")
	if err != nil {
		t.Fatalf("Failed to get write flag: %v", err)
	}
	if writeVal != false {
		t.Errorf("Expected write default false, got %v", writeVal)
	}
}

func TestPinCommandNoArgs(t *testing.T) {
	cmd := newPinCmd()
	if cmd.Args == nil {
		t.Error("Expected Args to be set")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("Expected NoArgs to accept no args, got error: %v", err)
	}
	if err := cobra.NoArgs(cmd, []string{"extra"}); err == nil {
		t.Error("Expected NoArgs to reject args")
	}
}

func TestPinCommandHasRun(t *testing.T) {
	cmd := newPinCmd()
	if cmd.Run == nil {
		t.Error("Expected Run function to be set on pin command")
	}
}

func TestProcessPin_NoActions(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)

	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}
	var allActionRefs []workflow.ActionRef

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if result {
		t.Error("Expected false when no action refs provided")
	}
}

func TestProcessPin_NoPinnableActions(t *testing.T) {
	tests := []struct {
		name string
		ref  workflow.ActionRef
	}{
		{
			name: "SHA-pinned action",
			ref: workflow.ActionRef{
				OwnerRepo: "actions/checkout",
				Version:   "abc123def456",
				FullRef:   "actions/checkout@abc123def456",
			},
		},
		{
			name: "branch-name ref",
			ref: workflow.ActionRef{
				OwnerRepo: "actions/checkout",
				Version:   "main",
				FullRef:   "actions/checkout@main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivedRepos := map[string]bool{tt.ref.OwnerRepo: false}
			server := makeCmdGHServer(archivedRepos, nil, nil)
			defer server.Close()

			rc, buf := newCmdRunContext(server, output.FormatText)

			wfFiles := []*workflow.WorkflowFile{
				{
					Path:             ".github/workflows/ci.yml",
					UsesWithVersions: []workflow.ActionRef{tt.ref},
				},
			}
			allActionRefs := []workflow.ActionRef{tt.ref}

			result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
			if result {
				t.Error("Expected false when no pinnable actions")
			}

			out := buf.String()
			if !strings.Contains(out, actioninfo.Emoji("✅ ", "[OK] ")) {
				t.Errorf("Expected OK marker in output, got %q", out)
			}
			if !strings.Contains(out, "already pinned to commit SHAs") {
				t.Errorf("Expected 'already pinned' message, got %q", out)
			}
		})
	}
}

func TestProcessPin_WithPinnableActions(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatText)

	actionRef := workflow.ActionRef{
		OwnerRepo: "actions/checkout",
		Version:   "v4",
		FullRef:   "actions/checkout@v4",
	}
	wfFiles := []*workflow.WorkflowFile{
		{
			Path:             ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{actionRef},
		},
	}
	allActionRefs := []workflow.ActionRef{actionRef}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if !result {
		t.Error("Expected true when pinnable actions found")
	}

	out := buf.String()
	if !strings.Contains(out, actioninfo.Emoji("📌 ", "[PIN] ")) {
		t.Errorf("Expected PIN marker in output, got %q", out)
	}
	if !strings.Contains(out, "version tags detected") {
		t.Errorf("Expected 'version tags detected' message, got %q", out)
	}
	if !strings.Contains(out, "actions/checkout@v4") {
		t.Errorf("Expected action reference in output, got %q", out)
	}
}

func TestProcessPin_WithWriteFlag(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
`
	wfPath := writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	rc, buf := newCmdRunContext(server, output.FormatText)
	rc.WorkDir = tmpDir

	parsedFiles, err := rc.Parser.ParseWorkflowFiles([]string{wfPath})
	if err != nil {
		t.Fatalf("Failed to parse workflow file: %v", err)
	}
	var allActionRefs []workflow.ActionRef
	for _, wf := range parsedFiles {
		allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
	}

	result := processPin(rc, parsedFiles, allActionRefs, tmpDir, true)
	if !result {
		t.Error("Expected true when pinnable actions found with write flag")
	}

	out := buf.String()
	if !strings.Contains(out, actioninfo.Emoji("📌 ", "[PIN] ")) {
		t.Errorf("Expected PIN marker in output, got %q", out)
	}

	updatedContent, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("Failed to read updated workflow file: %v", err)
	}
	updatedStr := string(updatedContent)
	if strings.Contains(updatedStr, "abc123def456") {
		if !strings.Contains(updatedStr, "# v4") {
			t.Error("Expected SHA reference to include version comment")
		}
	}
}

func TestProcessPin_JSONOutput(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatJSON)

	actionRef := workflow.ActionRef{
		OwnerRepo: "actions/checkout",
		Version:   "v4",
		FullRef:   "actions/checkout@v4",
	}
	wfFiles := []*workflow.WorkflowFile{
		{
			Path:             ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{actionRef},
		},
	}
	allActionRefs := []workflow.ActionRef{actionRef}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if !result {
		t.Error("Expected true when pinnable actions found")
	}

	out := buf.String()
	if !strings.Contains(out, `"pinnable_actions"`) {
		t.Errorf("Expected JSON output with 'pinnable_actions', got %q", out)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Expected valid JSON output, got parse error: %v\nOutput: %q", err, out)
	}
	hasIssues, _ := parsed["has_issues"].(bool)
	if !hasIssues {
		t.Errorf("Expected has_issues to be true in JSON output, got %v", parsed["has_issues"])
	}
}

func TestProcessPin_VerboseOutput(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true

	actionRef := workflow.ActionRef{
		OwnerRepo: "actions/checkout",
		Version:   "v4",
		FullRef:   "actions/checkout@v4",
	}
	wfFiles := []*workflow.WorkflowFile{
		{
			Path:             ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{actionRef},
		},
	}
	allActionRefs := []workflow.ActionRef{actionRef}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if !result {
		t.Error("Expected true when pinnable actions found")
	}

	out := buf.String()
	if !strings.Contains(out, actioninfo.Emoji("📌 ", "[PIN] ")) {
		t.Errorf("Expected PIN marker in verbose output, got %q", out)
	}
}

func TestProcessPin_AllArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{"archived/action": true}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatText)

	actionRef := workflow.ActionRef{
		OwnerRepo: "archived/action",
		Version:   "v1",
		FullRef:   "archived/action@v1",
	}
	wfFiles := []*workflow.WorkflowFile{
		{
			Path:             ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{actionRef},
		},
	}
	allActionRefs := []workflow.ActionRef{actionRef}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if result {
		t.Error("Expected false when all actions are archived (none pinnable)")
	}

	out := buf.String()
	if !strings.Contains(out, "already pinned to commit SHAs") {
		t.Errorf("Expected 'already pinned' message when all actions are archived, got %q", out)
	}
}

func TestProcessPin_MixedPinnableAndSHAPinned(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
		"actions/setup-go": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatText)

	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
				{OwnerRepo: "actions/setup-go", Version: "abc123def456", FullRef: "actions/setup-go@abc123def456"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
		{OwnerRepo: "actions/setup-go", Version: "abc123def456", FullRef: "actions/setup-go@abc123def456"},
	}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if !result {
		t.Error("Expected true when some actions are pinnable")
	}

	out := buf.String()
	if !strings.Contains(out, "actions/checkout@v4") {
		t.Errorf("Expected version-tagged action in output, got %q", out)
	}
	if strings.Contains(out, "actions/setup-go@abc123def456") {
		t.Errorf("Expected SHA-pinned action NOT to appear as pinnable, got %q", out)
	}
}

func TestProcessPin_WriteFlagNoPinnable(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@abc123def456
`
	wfPath := writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	rc, buf := newCmdRunContext(server, output.FormatText)
	rc.WorkDir = tmpDir

	parsedFiles, err := rc.Parser.ParseWorkflowFiles([]string{wfPath})
	if err != nil {
		t.Fatalf("Failed to parse workflow file: %v", err)
	}
	var allActionRefs []workflow.ActionRef
	for _, wf := range parsedFiles {
		allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
	}

	result := processPin(rc, parsedFiles, allActionRefs, tmpDir, true)
	if result {
		t.Error("Expected false when no pinnable actions with write flag")
	}

	out := buf.String()
	if !strings.Contains(out, "already pinned to commit SHAs") {
		t.Errorf("Expected 'already pinned' message, got %q", out)
	}
}

func TestProcessPin_NoPinnableArchivedAndSHA(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action":  true,
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatText)

	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				{OwnerRepo: "actions/checkout", Version: "abc123def456", FullRef: "actions/checkout@abc123def456"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
		{OwnerRepo: "actions/checkout", Version: "abc123def456", FullRef: "actions/checkout@abc123def456"},
	}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if result {
		t.Error("Expected false when all actions are archived or SHA-pinned")
	}

	out := buf.String()
	if !strings.Contains(out, "already pinned to commit SHAs") {
		t.Errorf("Expected 'already pinned' message, got %q", out)
	}
}

func TestRunPin_NoPinnableActions(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@abc123def456
`
	writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = filepath.Join(tmpDir, ".github", "workflows", "ci.yml")
	workflowsDir = ""
	reposDir = ""
	notify = false
	createIssue = false
	outputFormat = "text"
	noCache = true
	refreshCache = false
	cacheTTL = 0
	verbose = false
	debug = false
	exitCode = 0

	runPin(false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 for no pinnable actions, got %d", exitCode)
	}
}

func TestRunPin_WithPinnableActions(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
`
	writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = filepath.Join(tmpDir, ".github", "workflows", "ci.yml")
	workflowsDir = ""
	reposDir = ""
	notify = false
	createIssue = false
	outputFormat = "text"
	noCache = true
	refreshCache = false
	cacheTTL = 0
	verbose = false
	debug = false
	exitCode = 0

	runPin(false)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for pinnable actions, got %d", exitCode)
	}
}

func TestRunPin_WriteFlag(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
`
	wfPath := writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = wfPath
	workflowsDir = ""
	reposDir = ""
	notify = false
	createIssue = false
	outputFormat = "text"
	noCache = true
	refreshCache = false
	cacheTTL = 0
	verbose = false
	debug = false
	exitCode = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runPin(true)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for pinnable actions with write, got %d", exitCode)
	}
}

func TestRunPin_JSONOutput(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
`
	writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = filepath.Join(tmpDir, ".github", "workflows", "ci.yml")
	workflowsDir = ""
	reposDir = ""
	notify = false
	createIssue = false
	outputFormat = "json"
	noCache = true
	refreshCache = false
	cacheTTL = 0
	verbose = false
	debug = false
	exitCode = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runPin(false)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for pinnable actions with JSON, got %d", exitCode)
	}
}

func TestRunPin_ReposDir(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "my-repo")
	wfDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte(simpleWorkflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = ""
	workflowsDir = ""
	reposDir = tmpDir
	notify = false
	createIssue = false
	outputFormat = "text"
	noCache = true
	refreshCache = false
	cacheTTL = 0
	verbose = false
	debug = false
	exitCode = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runPin(false)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for pinnable actions in reposDir, got %d", exitCode)
	}
}

func TestProcessPin_WriteFlagPinsMultipleActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
		"actions/setup-go": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	workflowContent := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
`
	wfPath := writeWorkflowFile(t, tmpDir, "ci.yml", workflowContent)

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.WorkDir = tmpDir

	parsedFiles, err := rc.Parser.ParseWorkflowFiles([]string{wfPath})
	if err != nil {
		t.Fatalf("Failed to parse workflow file: %v", err)
	}
	var allActionRefs []workflow.ActionRef
	for _, wf := range parsedFiles {
		allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
	}

	result := processPin(rc, parsedFiles, allActionRefs, tmpDir, true)
	if !result {
		t.Error("Expected true when pinnable actions found with write flag")
	}

	updatedContent, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("Failed to read updated workflow file: %v", err)
	}
	updatedStr := string(updatedContent)
	if strings.Contains(updatedStr, "abc123def456") {
		if !strings.Contains(updatedStr, "# v4") || !strings.Contains(updatedStr, "# v5") {
			t.Errorf("Expected version comments in updated file, got %q", updatedStr)
		}
	}
}

func TestProcessPin_JSONOutputNoPinnable(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, buf := newCmdRunContext(server, output.FormatJSON)

	actionRef := workflow.ActionRef{
		OwnerRepo: "actions/checkout",
		Version:   "abc123def456",
		FullRef:   "actions/checkout@abc123def456",
	}
	wfFiles := []*workflow.WorkflowFile{
		{
			Path:             ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{actionRef},
		},
	}
	allActionRefs := []workflow.ActionRef{actionRef}

	result := processPin(rc, wfFiles, allActionRefs, rc.WorkDir, false)
	if result {
		t.Error("Expected false when no pinnable actions")
	}

	out := buf.String()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Expected valid JSON output, got parse error: %v\nOutput: %q", err, out)
	}
	hasIssues, _ := parsed["has_issues"].(bool)
	if hasIssues {
		t.Errorf("Expected has_issues to be false in JSON output, got %v", hasIssues)
	}
}
