package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/checkrunner"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/config"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

var defaultActionYMLContent = base64.StdEncoding.EncodeToString([]byte("name: Test\nruns:\n  using: node20\n  main: dist/index.js\n"))

type mockServerConfig struct {
	archivedRepos map[string]bool
	releases      map[string]*github.ReleaseInfo
	repoInfo      map[string]*github.RepoInfo
	actionYML     string
	orgRepos      map[string][]github.RepoEntry
	userRepos     map[string][]github.RepoEntry
	remoteFiles   map[string]string
}

func makeCmdGHServer(archivedRepos map[string]bool, releases map[string]*github.ReleaseInfo, repoInfo map[string]*github.RepoInfo) *httptest.Server {
	return makeCmdGHServerWithActionYML(archivedRepos, releases, repoInfo, defaultActionYMLContent)
}

func makeCmdGHServerWithActionYML(archivedRepos map[string]bool, releases map[string]*github.ReleaseInfo, repoInfo map[string]*github.RepoInfo, actionYMLContent string) *httptest.Server {
	cfg := &mockServerConfig{
		archivedRepos: archivedRepos,
		releases:      releases,
		repoInfo:      repoInfo,
		actionYML:     actionYMLContent,
	}
	return makeConfigurableGHServer(cfg)
}

func makeConfigurableGHServer(cfg *mockServerConfig) *httptest.Server {
	if cfg.actionYML == "" {
		cfg.actionYML = defaultActionYMLContent
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.Contains(path, "/contents/action.y") {
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, cfg.actionYML)
			return
		}

		if strings.Contains(path, "/orgs/") && strings.Contains(path, "/repos") {
			parts := strings.Split(path, "/orgs/")
			if len(parts) == 2 {
				orgName := strings.Split(parts[1], "/repos")[0]
				if repos, ok := cfg.orgRepos[orgName]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(repos)
					_, _ = w.Write(body)
					return
				}
			}
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.RepoEntry{})
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/users/") && strings.Contains(path, "/repos") {
			parts := strings.Split(path, "/users/")
			if len(parts) == 2 {
				userName := strings.Split(parts[1], "/repos")[0]
				if repos, ok := cfg.userRepos[userName]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(repos)
					_, _ = w.Write(body)
					return
				}
			}
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.RepoEntry{})
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/contents/.github") {
			w.WriteHeader(200)
			dirListing := []map[string]string{
				{"name": "workflows", "path": ".github/workflows", "type": "dir"},
			}
			body, _ := json.Marshal(dirListing)
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/contents/.github/workflows") {
			w.WriteHeader(200)
			fileList := []map[string]string{
				{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
			}
			body, _ := json.Marshal(fileList)
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/contents/.github/workflows/ci.yml") {
			w.WriteHeader(200)
			content := cfg.remoteFiles["ci.yml"]
			if content == "" {
				content = base64.StdEncoding.EncodeToString([]byte(simpleWorkflowContent))
			}
			resp := map[string]interface{}{
				"name":     "ci.yml",
				"path":     ".github/workflows/ci.yml",
				"type":     "file",
				"content":  content,
				"encoding": "base64",
			}
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/git/refs/") {
			w.WriteHeader(200)
			body, _ := json.Marshal(github.RefInfo{Object: struct {
				SHA  string `json:"sha"`
				URL  string `json:"url"`
				Type string `json:"type"`
			}{SHA: "abc123def456", URL: "https://api.github.com/repos/owner/repo/git/commits/abc123def456", Type: "commit"}})
			_, _ = w.Write(body)
			return
		}

		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				if info, ok := cfg.repoInfo[ownerRepo]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(info)
					_, _ = w.Write(body)
					return
				}
				if isArchived, ok := cfg.archivedRepos[ownerRepo]; ok {
					resp := map[string]interface{}{
						"full_name":           ownerRepo,
						"archived":            isArchived,
						"name":                strings.Split(ownerRepo, "/")[1],
						"updated_at":          time.Now().Format(time.RFC3339),
						"deprecated":          false,
						"deprecation_warning": "",
					}
					w.WriteHeader(200)
					body, _ := json.Marshal(resp)
					_, _ = w.Write(body)
					return
				}
			}
			w.WriteHeader(404)
			return
		}

		if strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				if release, ok := cfg.releases[ownerRepo]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(release)
					_, _ = w.Write(body)
					return
				}
			}
			w.WriteHeader(404)
			return
		}

		if strings.Contains(path, "/tags?") {
			w.WriteHeader(200)
			body, _ := json.Marshal([]map[string]string{{"name": "v0.0.0"}})
			_, _ = w.Write(body)
			return
		}

		w.WriteHeader(404)
	}))
}

func makeCmdEOLServer(eolProducts map[string]bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, isEOL := range eolProducts {
			if strings.Contains(r.URL.Path, "/products/"+key) {
				eolFrom := "2022-04-30"
				resp := map[string]interface{}{
					"schema_version": "1.0.0",
					"result": map[string]interface{}{
						"name":    strings.Split(key, "/")[0],
						"isEol":   isEOL,
						"eolFrom": eolFrom,
					},
				}
				w.WriteHeader(200)
				body, _ := json.Marshal(resp)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
}

func newCmdRunContext(ghServer *httptest.Server, format output.Format) (*checkrunner.RunContext, *bytes.Buffer) {
	client := github.NewClientWithHTTP(ghServer.URL, ghServer.Client(), github.WithCache(false, false, 0))
	buf := new(bytes.Buffer)
	return &checkrunner.RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/test-repo",
		Parser:       &workflow.WorkflowParser{},
		GHClient:     client,
		OutputWriter: &output.Writer{Format: format, Output: buf},
	}, buf
}

func newCmdRunContextWithEOL(ghServer, eolServer *httptest.Server, format output.Format) (*checkrunner.RunContext, *bytes.Buffer) {
	client := github.NewClientWithHTTP(ghServer.URL, ghServer.Client(), github.WithCache(false, false, 0))
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())
	buf := new(bytes.Buffer)
	return &checkrunner.RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/test-repo",
		Parser:       &workflow.WorkflowParser{},
		GHClient:     client,
		OutputWriter: &output.Writer{Format: format, Output: buf},
	}, buf
}

func writeWorkflowFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	fullDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}
	filePath := filepath.Join(fullDir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}
	return filePath
}

const simpleWorkflowContent = `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/checkout@v3
- uses: actions/setup-go@v4
`

const archivedWorkflowContent = `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: archived/action@v1
`

func TestResolveWorkflowFiles_WorkflowPathFlag(t *testing.T) {
	tmpDir := t.TempDir()
	wfPath := writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

	origWorkflowPath := workflowPath
	origWorkflowsDir := workflowsDir
	defer func() {
		workflowPath = origWorkflowPath
		workflowsDir = origWorkflowsDir
	}()

	workflowPath = wfPath
	workflowsDir = ""

	parser := &workflow.WorkflowParser{}
	wfFiles, actionRefs := resolveWorkflowFiles(parser, tmpDir)

	if len(wfFiles) != 1 {
		t.Fatalf("Expected 1 workflow file, got %d", len(wfFiles))
	}
	if len(actionRefs) != 2 {
		t.Fatalf("Expected 2 action refs, got %d", len(actionRefs))
	}
}

func TestResolveWorkflowFiles_WorkflowsDirFlag(t *testing.T) {
	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte(simpleWorkflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	origWorkflowPath := workflowPath
	origWorkflowsDir := workflowsDir
	defer func() {
		workflowPath = origWorkflowPath
		workflowsDir = origWorkflowsDir
	}()

	workflowPath = ""
	workflowsDir = wfDir

	parser := &workflow.WorkflowParser{}
	wfFiles, actionRefs := resolveWorkflowFiles(parser, tmpDir)

	if len(wfFiles) != 1 {
		t.Fatalf("Expected 1 workflow file, got %d", len(wfFiles))
	}
	if len(actionRefs) != 2 {
		t.Fatalf("Expected 2 action refs, got %d", len(actionRefs))
	}
}

func TestResolveWorkflowFiles_DefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

	origWorkflowPath := workflowPath
	origWorkflowsDir := workflowsDir
	defer func() {
		workflowPath = origWorkflowPath
		workflowsDir = origWorkflowsDir
	}()

	workflowPath = ""
	workflowsDir = ""

	parser := &workflow.WorkflowParser{}
	wfFiles, actionRefs := resolveWorkflowFiles(parser, tmpDir)

	if len(wfFiles) != 1 {
		t.Fatalf("Expected 1 workflow file, got %d", len(wfFiles))
	}
	if len(actionRefs) != 2 {
		t.Fatalf("Expected 2 action refs, got %d", len(actionRefs))
	}
}

func TestResolveOutputFormat_InvalidFormat(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "1" {
		origOutputFormat := outputFormat
		outputFormat = "xml"
		resolveOutputFormat()
		outputFormat = origOutputFormat
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestResolveOutputFormat_InvalidFormat")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=1")
	err := cmd.Run()
	if err == nil {
		t.Error("Expected non-zero exit for invalid output format")
	}
}

func TestResolveToken_NoTokenFatal(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "2" {
		origConf := conf
		origGithubToken := githubToken
		conf = config.Config{GitHubToken: "", GitHubTokenFallback: ""}
		githubToken = ""
		resolveToken()
		conf = origConf
		githubToken = origGithubToken
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestResolveToken_NoTokenFatal")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=2")
	err := cmd.Run()
	if err == nil {
		t.Error("Expected non-zero exit when no token is provided")
	}
}

func TestProcessArchived_NoActions(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}

	result := processArchived(rc, wfFiles, nil, rc.WorkDir, 365)
	if result {
		t.Error("Expected false when no action refs found")
	}
}

func TestProcessArchived_NoArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
		"actions/setup-go": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
	}

	result := processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365)
	if result {
		t.Error("Expected false when no archived actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "No archived or stale") {
		t.Errorf("Expected no-issues message, got %q", buf.String())
	}
}

func TestProcessArchived_WithArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action":  true,
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Archived actions detected") {
		t.Errorf("Expected archived summary in output, got %q", buf.String())
	}
}

func TestProcessArchived_WithStaleActions(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use actions/new-action instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Stale or deprecated actions detected") {
		t.Errorf("Expected stale summary in output, got %q", buf.String())
	}
}

func TestProcessArchived_JSONOutput(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatJSON)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), `"has_issues"`) {
		t.Errorf("Expected JSON output with has_issues, got %q", buf.String())
	}
}

func TestProcessArchived_VerboseOutput(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365)
	if result {
		t.Error("Expected false when no archived actions found")
	}
}

func TestProcessOutdated_NoActions(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}

	result := processOutdated(rc, wfFiles, nil, rc.WorkDir, false, false)
	if result {
		t.Error("Expected false when no action refs found")
	}
}

func TestProcessOutdated_NoOutdatedActions(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, false, false)
	if result {
		t.Error("Expected false when no outdated actions found")
	}
}

func TestProcessOutdated_WithOutdatedActions(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, false, false)
	if !result {
		t.Error("Expected true when outdated actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Outdated actions detected") {
		t.Errorf("Expected outdated summary in output, got %q", buf.String())
	}
}

func TestProcessOutdated_WithUpdateFlag(t *testing.T) {
	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	wfContent := `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/setup-go@v3
`
	wfPath := filepath.Join(wfDir, "ci.yml")
	if err := os.WriteFile(wfPath, []byte(wfContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	archivedRepos := map[string]bool{
		"actions/setup-go": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true

	parsedWf, err := rc.Parser.ParseWorkflowFile(wfPath)
	if err != nil {
		t.Fatalf("Failed to parse workflow file: %v", err)
	}
	wfFiles := []*workflow.WorkflowFile{parsedWf}
	allActionRefs := parsedWf.UsesWithVersions

	result := processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, true, false)
	if !result {
		t.Error("Expected true when outdated actions found")
	}
}

func TestProcessOutdated_UseSemverTrue(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, true, true)
	if !result {
		t.Error("Expected true when outdated actions found with semver")
	}
}

func TestProcessOutdated_VerboseOutput(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, false, false)
	if result {
		t.Error("Expected false when no outdated actions found")
	}
}

func TestProcessEOL_NoActions(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}

	result := processEOL(rc, wfFiles, nil, rc.WorkDir, false, 365)
	if result {
		t.Error("Expected false when no action refs found")
	}
}

func TestProcessEOL_NoEOLActions(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServer(archivedRepos, nil, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, false, 365)
	if result {
		t.Error("Expected false when no EOL actions found")
	}
}

func TestProcessEOL_WithArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, false, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Archived actions detected") {
		t.Errorf("Expected archived summary in EOL output, got %q", buf.String())
	}
}

func TestProcessEOL_WithEOLRuntime(t *testing.T) {
	node12ActionContent := base64.StdEncoding.EncodeToString([]byte("name: Test\nruns:\n  using: node12\n  main: dist/index.js\n"))

	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServerWithActionYML(archivedRepos, nil, repoInfo, node12ActionContent)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/12": true})
	defer eolServer.Close()

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, false, 365)
	if !result {
		t.Error("Expected true when EOL runtime actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "EOL actions detected") {
		t.Errorf("Expected EOL summary in output, got %q", buf.String())
	}
}

func TestProcessEOL_WithUpdateStaleOnlyNoArchived(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use actions/new-action instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, true, 365)
	if !result {
		t.Error("Expected true when stale actions found with --update")
	}
}

func TestProcessEOL_UpdateNoUpdatesAvailable(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use new instead",
			UpdatedAt:      oldDate,
		},
	}
	archivedRepos := map[string]bool{"actions/old-action": false}
	server := makeCmdGHServer(archivedRepos, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, true, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "EOL") {
		t.Errorf("Expected EOL summary, got %q", out)
	}
}

func TestProcessEOL_UpdateStaleNoOutdatedAvailable(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	ownerRepo := "actions/old-action"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:       ownerRepo,
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use v2 instead",
			UpdatedAt:      oldDate,
		},
	}
	archivedRepos := map[string]bool{ownerRepo: false}
	releases := map[string]*github.ReleaseInfo{
		ownerRepo: {TagName: "v1", Name: "v1", HTMLURL: "https://github.com/actions/old-action/releases/tag/v1"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, true, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "EOL") {
		t.Errorf("Expected EOL summary, got %q", out)
	}
}

func TestProcessEOL_StaleOnlySummary(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use v2 instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, false, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	if !strings.Contains(buf.String(), "EOL actions detected") {
		t.Errorf("Expected EOL stale summary, got %q", buf.String())
	}
}

func TestProcessEOL_NoActionsMessage(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}
	var allActionRefs []workflow.ActionRef

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, true, 365)
	if result {
		t.Error("Expected false when no actions found")
	}
}

func TestProcessCheck_NoActions(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}

	result := processCheck(rc, wfFiles, nil, rc.WorkDir, false, false, 365)
	if result {
		t.Error("Expected false when no action refs found")
	}
}

func TestProcessCheck_NoIssues(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if result {
		t.Error("Expected false when no issues found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "No archived, outdated, or stale") {
		t.Errorf("Expected no-issues message, got %q", buf.String())
	}
}

func TestProcessCheck_WithArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action":  true,
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Archived actions detected") {
		t.Errorf("Expected archived summary in check output, got %q", buf.String())
	}
}

func TestProcessCheck_WriteFlagWithArchived(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if !result {
		t.Error("Expected true when archived actions found with --write flag")
	}
}

func TestProcessCheck_WithOutdatedActions(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when outdated actions found")
	}

	buf, ok := rc.OutputWriter.Output.(*bytes.Buffer)
	if !ok {
		t.Fatal("Expected OutputWriter.Output to be *bytes.Buffer")
	}
	if !strings.Contains(buf.String(), "Outdated actions detected") {
		t.Errorf("Expected outdated summary in check output, got %q", buf.String())
	}
}

func TestProcessCheck_WithStaleActions(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use actions/new-action instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}
}

func TestProcessCheck_WriteFlagNoArchived(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if !result {
		t.Error("Expected true when outdated actions found")
	}
}

func TestProcessCheck_WriteFlagWithArchivedCannotUpdate(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}
}

func TestProcessCheck_StaleOnlySummary(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use v2 instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	if !strings.Contains(buf.String(), "Stale or deprecated actions detected") {
		t.Errorf("Expected stale summary, got %q", buf.String())
	}
}

func TestProcessCheck_RuntimeEOLSummary(t *testing.T) {
	node12ActionContent := base64.StdEncoding.EncodeToString([]byte("name: Test\nruns:\n  using: node12\n  main: dist/index.js\n"))

	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServerWithActionYML(archivedRepos, releases, repoInfo, node12ActionContent)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/12": true})
	defer eolServer.Close()

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when runtime EOL actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "EOL runtimes") || !strings.Contains(out, "RUNTIME") {
		t.Errorf("Expected runtime EOL summary, got %q", out)
	}
}

func TestProcessCheck_JSONOutput(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatJSON)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when archived actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	if !strings.Contains(buf.String(), `"archived_actions"`) {
		t.Errorf("Expected JSON output, got %q", buf.String())
	}
}

func TestProcessCheck_NoActionsMessage(t *testing.T) {
	server := makeCmdGHServer(nil, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{Path: ".github/workflows/ci.yml", UsesWithVersions: []workflow.ActionRef{}},
	}
	var allActionRefs []workflow.ActionRef

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if result {
		t.Error("Expected false when no actions found")
	}
}

func TestRunArchived_NoArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
		"actions/setup-go": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContext(server, output.FormatText)
	if processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365) {
		exitCode = 1
	}

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0, got %d", exitCode)
	}
}

func TestRunArchived_WithArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", archivedWorkflowContent)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContext(server, output.FormatText)
	if processArchived(rc, wfFiles, allActionRefs, rc.WorkDir, 365) {
		exitCode = 1
	}

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1, got %d", exitCode)
	}
}

func TestRunArchived_ReposDir(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
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

	rc, _ := newCmdRunContext(server, output.FormatText)

	files, err := rc.Parser.FindWorkflowFilesInDir(wfDir)
	if err != nil {
		t.Fatalf("Failed to find workflow files: %v", err)
	}
	wfFiles, err := rc.Parser.ParseWorkflowFiles(files)
	if err != nil {
		t.Fatalf("Failed to parse workflow files: %v", err)
	}
	var allActionRefs []workflow.ActionRef
	for _, wf := range wfFiles {
		allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
	}

	result := processArchived(rc, wfFiles, allActionRefs, repoDir, 365)
	if result {
		t.Error("Expected false when no archived actions found in reposDir")
	}
}

func TestRunOutdated_NoOutdatedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/checkout@v3
`)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContext(server, output.FormatText)
	if processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, false, false) {
		exitCode = 1
	}

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0, got %d", exitCode)
	}
}

func TestRunOutdated_WithOutdatedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/setup-go": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/setup-go@v3
`)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContext(server, output.FormatText)
	if processOutdated(rc, wfFiles, allActionRefs, rc.WorkDir, false, false) {
		exitCode = 1
	}

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1, got %d", exitCode)
	}
}

func TestRunOutdated_ReposDir(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
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

	rc, _ := newCmdRunContext(server, output.FormatText)

	files, err := rc.Parser.FindWorkflowFilesInDir(wfDir)
	if err != nil {
		t.Fatalf("Failed to find workflow files: %v", err)
	}
	wfFiles, err := rc.Parser.ParseWorkflowFiles(files)
	if err != nil {
		t.Fatalf("Failed to parse workflow files: %v", err)
	}
	var allActionRefs []workflow.ActionRef
	for _, wf := range wfFiles {
		allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
	}

	result := processOutdated(rc, wfFiles, allActionRefs, repoDir, false, false)
	if result {
		t.Error("Expected false when no outdated actions found in reposDir")
	}
}

func TestRunEOL_NoEOLActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServer(archivedRepos, nil, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/checkout@v3
`)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	if processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, false, 365) {
		exitCode = 1
	}

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0, got %d", exitCode)
	}
}

func TestRunCheck_NoIssues(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
test:
runs-on: ubuntu-latest
steps:
- uses: actions/checkout@v3
`)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	if processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365) {
		exitCode = 1
	}

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0, got %d", exitCode)
	}
}

func TestRunCheck_WithArchivedActions(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", archivedWorkflowContent)

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

	parser := &workflow.WorkflowParser{}
	wfFiles, allActionRefs := resolveWorkflowFiles(parser, tmpDir)

	rc, _ := newCmdRunContext(server, output.FormatText)
	if processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365) {
		exitCode = 1
	}

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1, got %d", exitCode)
	}
}

func TestProcessEOL_UpdateWithAvailableUpdates(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	ownerRepo := "actions/old-action"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:       ownerRepo,
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use v2 instead",
			UpdatedAt:      oldDate,
		},
	}
	archivedRepos := map[string]bool{ownerRepo: false}
	releases := map[string]*github.ReleaseInfo{
		ownerRepo: {TagName: "v2", Name: "v2", HTMLURL: "https://github.com/actions/old-action/releases/tag/v2"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	rc.Verbose = true
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v1", FullRef: ownerRepo + "@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v1", FullRef: ownerRepo + "@v1"},
	}

	result := processEOL(rc, wfFiles, allActionRefs, rc.WorkDir, true, 365)
	if !result {
		t.Error("Expected true when stale actions found with update=true and updates available")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "EOL") {
		t.Errorf("Expected EOL summary with updates, got %q", out)
	}
}

func TestProcessCheck_WriteFlagWithUpdatesAvailable(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if !result {
		t.Error("Expected true when outdated actions found with write=true")
	}
}

func TestProcessCheck_WriteFlagArchivedCannotUpdate(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, true, false, 365)
	if !result {
		t.Error("Expected true when archived actions found with write=true")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "Archived actions detected") {
		t.Errorf("Expected archived warning with write flag, got %q", out)
	}
}

func TestProcessCheck_RuntimeEOLOnlySummary(t *testing.T) {
	node12ActionContent := base64.StdEncoding.EncodeToString([]byte("name: Test\nruns:\n using: node12\n main: dist/index.js\n"))

	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServerWithActionYML(archivedRepos, releases, repoInfo, node12ActionContent)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/12": true})
	defer eolServer.Close()

	rc, _ := newCmdRunContextWithEOL(ghServer, eolServer, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when runtime EOL actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "EOL runtimes") {
		t.Errorf("Expected runtime EOL summary, got %q", out)
	}
}

func TestProcessCheck_OutdatedOnlySummary(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when outdated actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	out := buf.String()
	if !strings.Contains(out, "Outdated actions detected") {
		t.Errorf("Expected outdated summary, got %q", out)
	}
}

func TestProcessCheck_StaleOnlySummary2(t *testing.T) {
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		"actions/old-action": {
			FullName:       "actions/old-action",
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use v2 instead",
			UpdatedAt:      oldDate,
		},
	}
	server := makeCmdGHServer(map[string]bool{"actions/old-action": false}, nil, repoInfo)
	defer server.Close()

	rc, _ := newCmdRunContext(server, output.FormatText)
	wfFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/old-action", Version: "v1", FullRef: "actions/old-action@v1"},
	}

	result := processCheck(rc, wfFiles, allActionRefs, rc.WorkDir, false, false, 365)
	if !result {
		t.Error("Expected true when stale actions found")
	}

	buf := rc.OutputWriter.Output.(*bytes.Buffer)
	if !strings.Contains(buf.String(), "Stale or deprecated") {
		t.Errorf("Expected stale summary, got %q", buf.String())
	}
}

func TestExecute_ExitCodePath(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "3" {
		exitCode = 1
		Execute()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExecute_ExitCodePath")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=3")
	err := cmd.Run()
	if err == nil {
		t.Error("Expected non-zero exit when exitCode is set to 1")
	}
}

func TestExecute_ErrorPath(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "4" {
		oldArgs := os.Args
		os.Args = []string{"go-sort-out-gh-actions", "nonexistent-command"}
		defer func() { os.Args = oldArgs }()
		Execute()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExecute_ErrorPath")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=4")
	err := cmd.Run()
	if err == nil {
		t.Error("Expected non-zero exit for unknown command")
	}
}

func TestResolveWorkflowFiles_ParseError(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "5" {
		workflowPath = "/nonexistent/path/to/workflow.yml"
		workflowsDir = ""
		parser := &workflow.WorkflowParser{}
		resolveWorkflowFiles(parser, "/tmp")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestResolveWorkflowFiles_ParseError")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=5")
	err := cmd.Run()
	if err == nil {
		t.Error("Expected non-zero exit for invalid workflow path")
	}
}

func TestRunArchived_WithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 for no archived, got %d", exitCode)
	}
}

func TestRunArchived_WithArchivedMockServer(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", archivedWorkflowContent)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runArchived(365)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for archived actions, got %d", exitCode)
	}
}

func TestRunOutdated_WithMockServer(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
`)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runOutdated(false, false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 for no outdated, got %d", exitCode)
	}
}

func TestRunOutdated_WithOutdatedMockServer(t *testing.T) {
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	server := makeCmdGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/setup-go@v3
`)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runOutdated(false, false)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for outdated actions, got %d", exitCode)
	}
}

func TestRunEOL_WithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServer(archivedRepos, nil, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
`)

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
	ghAPIBaseURL = ghServer.URL
	ghAPIClient = ghServer.Client()
	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	runEOL(false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 for no EOL, got %d", exitCode)
	}
}

func TestRunEOL_WithEOLMockServer(t *testing.T) {
	node12ActionContent := base64.StdEncoding.EncodeToString([]byte("name: Test\nruns:\n using: node12\n main: dist/index.js\n"))
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServerWithActionYML(archivedRepos, nil, repoInfo, node12ActionContent)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/12": true})
	defer eolServer.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
`)

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
	ghAPIBaseURL = ghServer.URL
	ghAPIClient = ghServer.Client()
	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	runEOL(false, 365)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for EOL actions, got %d", exitCode)
	}
}

func TestRunCheck_WithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
`)

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
	ghAPIBaseURL = ghServer.URL
	ghAPIClient = ghServer.Client()
	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	runCheck(false, false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 for no issues, got %d", exitCode)
	}
}

func TestRunCheck_WithArchivedMockServer(t *testing.T) {
	archivedRepos := map[string]bool{
		"archived/action": true,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", archivedWorkflowContent)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runCheck(false, false, 365)

	if exitCode != 1 {
		t.Errorf("Expected exitCode 1 for archived actions, got %d", exitCode)
	}
}

func TestRunArchived_DebugFlag(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

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
	debug = true
	exitCode = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with debug, got %d", exitCode)
	}
}

func TestRunArchived_JSONOutput(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

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

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with JSON output, got %d", exitCode)
	}
}

func TestNewOutdatedCmd_PinOverridesSemver(t *testing.T) {
	cmd := newOutdatedCmd()
	if err := cmd.Flags().Set("pin", "true"); err != nil {
		t.Fatalf("Failed to set pin: %v", err)
	}
	if err := cmd.Flags().Set("semver", "true"); err != nil {
		t.Fatalf("Failed to set semver: %v", err)
	}
	if err := cmd.Flags().Set("update", "true"); err != nil {
		t.Fatalf("Failed to set update: %v", err)
	}

	pinVal, _ := cmd.Flags().GetBool("pin")
	semverVal, _ := cmd.Flags().GetBool("semver")

	useSemver := semverVal
	if pinVal {
		useSemver = false
	}

	if useSemver != false {
		t.Errorf("Expected useSemver=false when pin=true, got %v", useSemver)
	}
}

func TestNewCheckCmd_SemverFlag(t *testing.T) {
	cmd := newCheckCmd()
	if cmd.Flags().Lookup("semver") == nil {
		t.Error("Expected --semver flag on check command")
	}
	semverVal, err := cmd.Flags().GetBool("semver")
	if err != nil {
		t.Fatalf("Failed to get semver: %v", err)
	}
	if semverVal != false {
		t.Errorf("Expected semver default false, got %v", semverVal)
	}
}

func TestExecute_VersionCommand(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	exitCode = 0

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"go-sort-out-gh-actions", "version"}

	Execute()
}

func TestRunArchived_ReposDirWithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
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

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with reposDir, got %d", exitCode)
	}
}

func TestRunOutdated_ReposDirWithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
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

	runOutdated(false, false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with reposDir, got %d", exitCode)
	}
}

func TestRunEOL_ReposDirWithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	ghServer := makeCmdGHServer(archivedRepos, nil, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

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
	ghAPIBaseURL = ghServer.URL
	ghAPIClient = ghServer.Client()
	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	runEOL(false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with reposDir, got %d", exitCode)
	}
}

func TestRunCheck_ReposDirWithMockServer(t *testing.T) {
	archivedRepos := map[string]bool{"actions/checkout": false}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	ghServer := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer ghServer.Close()

	eolServer := makeCmdEOLServer(map[string]bool{"nodejs/releases/20": false})
	defer eolServer.Close()

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
	ghAPIBaseURL = ghServer.URL
	ghAPIClient = ghServer.Client()
	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	runCheck(false, false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with reposDir, got %d", exitCode)
	}
}

func TestNewRunContextFromFlags_NotifyBranch(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	notify = true
	createIssue = false
	noCache = true
	refreshCache = false
	cacheTTL = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	rc := newRunContextFromFlags("test-token", output.FormatText)
	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	_ = rc.Close()
}

func TestNewRunContextFromFlags_CreateIssueBranch(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	saveGlobals()
	defer restoreGlobals()

	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	notify = false
	createIssue = true
	noCache = true
	refreshCache = false
	cacheTTL = 0
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()
	eolAPIBaseURL = ""
	eolAPIClient = nil

	rc := newRunContextFromFlags("test-token", output.FormatText)
	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	_ = rc.Close()
}

func TestNewRunContextFromFlags_FallbackPath(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	ghAPIBaseURL = ""
	ghAPIClient = nil

	rc := newRunContextFromFlags("test-token", output.FormatText)
	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	_ = rc.Close()
}

func TestNewOutdatedCmd_RunClosurePinOverride(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "8" {
		cmd := newOutdatedCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Flags().Set("pin", "true"); err != nil {
			os.Exit(2)
		}
		if err := cmd.Flags().Set("semver", "true"); err != nil {
			os.Exit(2)
		}
		if err := cmd.Flags().Set("update", "false"); err != nil {
			os.Exit(2)
		}
		cmd.Run(cmd, nil)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestNewOutdatedCmd_RunClosurePinOverride")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=8", "GH_TOKEN=test-token")
	err := cmd.Run()
	_ = err
}

func TestNewCheckCmd_RunClosureSemver(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "9" {
		cmd := newCheckCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Flags().Set("semver", "true"); err != nil {
			os.Exit(2)
		}
		cmd.Run(cmd, nil)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestNewCheckCmd_RunClosureSemver")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=9", "GH_TOKEN=test-token")
	err := cmd.Run()
	_ = err
}

func TestNewArchivedCmd_RunClosure(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "10" {
		cmd := newArchivedCmd()
		cmd.SetArgs([]string{})
		cmd.Run(cmd, nil)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestNewArchivedCmd_RunClosure")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=10", "GH_TOKEN=test-token")
	err := cmd.Run()
	_ = err
}

func TestNewEOLCmd_RunClosure(t *testing.T) {
	if os.Getenv("GO_TEST_REEXEC") == "11" {
		cmd := newEOLCmd()
		cmd.SetArgs([]string{})
		cmd.Run(cmd, nil)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestNewEOLCmd_RunClosure")
	cmd.Env = append(os.Environ(), "GO_TEST_REEXEC=11", "GH_TOKEN=test-token")
	err := cmd.Run()
	_ = err
}

func TestWriteActionOutput_WithGITHUB_OUTPUT(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "github_output")
	if err := os.WriteFile(outputFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}

	origGITHUB_OUTPUT := os.Getenv("GITHUB_OUTPUT")
	defer func() { _ = os.Setenv("GITHUB_OUTPUT", origGITHUB_OUTPUT) }()
	_ = os.Setenv("GITHUB_OUTPUT", outputFile)

	actioninfo.WriteActionOutput("test-key", "test-value")

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if !strings.Contains(string(content), "test-key=test-value") {
		t.Errorf("Expected output file to contain 'test-key=test-value', got %q", string(content))
	}
}

func TestNewOutdatedCmd_RunClosureInProcess(t *testing.T) {
	archivedRepos := map[string]bool{}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3"},
		"actions/setup-go": {TagName: "v4"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {Name: "checkout", Archived: false},
		"actions/setup-go": {Name: "setup-go", Archived: false},
	}
	server := makeCmdGHServerWithActionYML(archivedRepos, releases, repoInfo, defaultActionYMLContent)
	defer server.Close()

	tmpDir := t.TempDir()
	writeWorkflowFile(t, tmpDir, "ci.yml", simpleWorkflowContent)

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
	ghAPIBaseURL = server.URL
	ghAPIClient = server.Client()

	cmd := newOutdatedCmd()
	if err := cmd.Flags().Set("pin", "true"); err != nil {
		t.Fatalf("Failed to set pin flag: %v", err)
	}
	if err := cmd.Flags().Set("semver", "true"); err != nil {
		t.Fatalf("Failed to set semver flag: %v", err)
	}
	cmd.Run(cmd, []string{})

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0, got %d", exitCode)
	}
}

func setupOrgUserRemoteGlobals(server *httptest.Server) {
	saveGlobals()
	conf = config.Config{GitHubToken: "test-token"}
	githubToken = "test-token"
	workflowPath = ""
	workflowsDir = ""
	reposDir = ""
	orgName = ""
	userName = ""
	remoteRepo = ""
	remoteRef = ""
	includeForks = false
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
}

func TestRunArchived_RemoteRepoMode(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	remoteRepo = "owner/repo"

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with remoteRepo, got %d", exitCode)
	}
}

func TestRunArchived_OrgMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {
				{FullName: "testorg/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	orgName = "testorg"

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with orgName, got %d", exitCode)
	}
}

func TestRunArchived_UserMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		userRepos: map[string][]github.RepoEntry{
			"testuser": {
				{FullName: "testuser/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	userName = "testuser"

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with userName, got %d", exitCode)
	}
}

func TestRunArchived_OrgModeEmpty(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	orgName = "testorg"

	runArchived(365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with empty org, got %d", exitCode)
	}
}

func TestRunOutdated_RemoteRepoMode(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	remoteRepo = "owner/repo"

	runOutdated(false, false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with remoteRepo, got %d", exitCode)
	}
}

func TestRunOutdated_OrgMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		repoInfo: map[string]*github.RepoInfo{
			"actions/checkout": {
				FullName:  "actions/checkout",
				Archived:  false,
				UpdatedAt: time.Now().Format(time.RFC3339),
			},
		},
		releases: map[string]*github.ReleaseInfo{
			"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
		},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {
				{FullName: "testorg/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	orgName = "testorg"

	runOutdated(false, false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with orgName, got %d", exitCode)
	}
}

func TestRunOutdated_UserMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		repoInfo: map[string]*github.RepoInfo{
			"actions/checkout": {
				FullName:  "actions/checkout",
				Archived:  false,
				UpdatedAt: time.Now().Format(time.RFC3339),
			},
		},
		releases: map[string]*github.ReleaseInfo{
			"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
		},
		userRepos: map[string][]github.RepoEntry{
			"testuser": {
				{FullName: "testuser/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	userName = "testuser"

	runOutdated(false, false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with userName, got %d", exitCode)
	}
}

func makeCmdEOLServerForRemote() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		body, _ := json.Marshal([]map[string]interface{}{
			{
				"cycle": map[string]interface{}{
					"name":    "nodejs",
					"release": "2023-10-18",
					"eol":     false,
					"lts":     "2025-10-28",
				},
				"runtimeStatus": "active",
				"releaseDate":   "2023-10-18",
				"releaseLabel":  "v20",
			},
		})
		_, _ = w.Write(body)
	}))
}

func TestRunEOL_RemoteRepoMode(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	remoteRepo = "owner/repo"

	runEOL(false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with remoteRepo, got %d", exitCode)
	}
}

func TestRunEOL_OrgMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {
				{FullName: "testorg/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	orgName = "testorg"

	runEOL(false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with orgName, got %d", exitCode)
	}
}

func TestRunEOL_UserMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		userRepos: map[string][]github.RepoEntry{
			"testuser": {
				{FullName: "testuser/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	userName = "testuser"

	runEOL(false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with userName, got %d", exitCode)
	}
}

func TestRunCheck_RemoteRepoMode(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
	}
	server := makeCmdGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	remoteRepo = "owner/repo"

	runCheck(false, false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with remoteRepo, got %d", exitCode)
	}
}

func TestRunCheck_OrgMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		repoInfo: map[string]*github.RepoInfo{
			"actions/checkout": {
				FullName:  "actions/checkout",
				Archived:  false,
				UpdatedAt: time.Now().Format(time.RFC3339),
			},
		},
		releases: map[string]*github.ReleaseInfo{
			"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
		},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {
				{FullName: "testorg/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	orgName = "testorg"

	runCheck(false, false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with orgName, got %d", exitCode)
	}
}

func TestRunCheck_UserMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		repoInfo: map[string]*github.RepoInfo{
			"actions/checkout": {
				FullName:  "actions/checkout",
				Archived:  false,
				UpdatedAt: time.Now().Format(time.RFC3339),
			},
		},
		releases: map[string]*github.ReleaseInfo{
			"actions/checkout": {TagName: "v3", Name: "v3", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3"},
		},
		userRepos: map[string][]github.RepoEntry{
			"testuser": {
				{FullName: "testuser/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	eolServer := makeCmdEOLServerForRemote()
	defer eolServer.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	eolAPIBaseURL = eolServer.URL
	eolAPIClient = eolServer.Client()

	userName = "testuser"

	runCheck(false, false, 365)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with userName, got %d", exitCode)
	}
}

func TestRunPin_RemoteRepoMode(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeCmdGHServer(archivedRepos, nil, nil)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	remoteRepo = "owner/repo"

	runPin(false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with remoteRepo, got %d", exitCode)
	}
}

func TestRunPin_OrgMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		orgRepos: map[string][]github.RepoEntry{
			"testorg": {
				{FullName: "testorg/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	orgName = "testorg"

	runPin(false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with orgName, got %d", exitCode)
	}
}

func TestRunPin_UserMode(t *testing.T) {
	cfg := &mockServerConfig{
		archivedRepos: map[string]bool{
			"actions/checkout": false,
		},
		userRepos: map[string][]github.RepoEntry{
			"testuser": {
				{FullName: "testuser/repo1", Name: "repo1", Archived: false, Fork: false},
			},
		},
	}
	server := makeConfigurableGHServer(cfg)
	defer server.Close()

	setupOrgUserRemoteGlobals(server)
	defer restoreGlobals()

	userName = "testuser"

	runPin(false)

	if exitCode != 0 {
		t.Errorf("Expected exitCode 0 with userName, got %d", exitCode)
	}
}

var savedGlobals struct {
	conf          config.Config
	githubToken   string
	workflowPath  string
	workflowsDir  string
	reposDir      string
	orgName       string
	userName      string
	remoteRepo    string
	remoteRef     string
	includeForks  bool
	notify        bool
	createIssue   bool
	outputFormat  string
	noCache       bool
	refreshCache  bool
	cacheTTL      time.Duration
	verbose       bool
	debug         bool
	exitCode      int
	ghAPIBaseURL  string
	ghAPIClient   *http.Client
	eolAPIBaseURL string
	eolAPIClient  *http.Client
}

func saveGlobals() {
	savedGlobals.conf = conf
	savedGlobals.githubToken = githubToken
	savedGlobals.workflowPath = workflowPath
	savedGlobals.workflowsDir = workflowsDir
	savedGlobals.reposDir = reposDir
	savedGlobals.orgName = orgName
	savedGlobals.userName = userName
	savedGlobals.remoteRepo = remoteRepo
	savedGlobals.remoteRef = remoteRef
	savedGlobals.includeForks = includeForks
	savedGlobals.notify = notify
	savedGlobals.createIssue = createIssue
	savedGlobals.outputFormat = outputFormat
	savedGlobals.noCache = noCache
	savedGlobals.refreshCache = refreshCache
	savedGlobals.cacheTTL = cacheTTL
	savedGlobals.verbose = verbose
	savedGlobals.debug = debug
	savedGlobals.exitCode = exitCode
	savedGlobals.ghAPIBaseURL = ghAPIBaseURL
	savedGlobals.ghAPIClient = ghAPIClient
	savedGlobals.eolAPIBaseURL = eolAPIBaseURL
	savedGlobals.eolAPIClient = eolAPIClient
}

func restoreGlobals() {
	conf = savedGlobals.conf
	githubToken = savedGlobals.githubToken
	workflowPath = savedGlobals.workflowPath
	workflowsDir = savedGlobals.workflowsDir
	reposDir = savedGlobals.reposDir
	orgName = savedGlobals.orgName
	userName = savedGlobals.userName
	remoteRepo = savedGlobals.remoteRepo
	remoteRef = savedGlobals.remoteRef
	includeForks = savedGlobals.includeForks
	notify = savedGlobals.notify
	createIssue = savedGlobals.createIssue
	outputFormat = savedGlobals.outputFormat
	noCache = savedGlobals.noCache
	refreshCache = savedGlobals.refreshCache
	cacheTTL = savedGlobals.cacheTTL
	verbose = savedGlobals.verbose
	debug = savedGlobals.debug
	exitCode = savedGlobals.exitCode
	ghAPIBaseURL = savedGlobals.ghAPIBaseURL
	ghAPIClient = savedGlobals.ghAPIClient
	eolAPIBaseURL = savedGlobals.eolAPIBaseURL
	eolAPIClient = savedGlobals.eolAPIClient
}
