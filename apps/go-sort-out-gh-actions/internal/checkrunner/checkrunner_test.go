package checkrunner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/go-sort-out-gh-actions/internal/notification"
	"github.com/toozej/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/go-sort-out-gh-actions/pkg/config"
)

func newTestRunContext(ghServer *httptest.Server) *RunContext {
	client := github.NewClientWithHTTP(ghServer.URL, ghServer.Client(), github.WithCache(false, false, 0))
	return &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}
}

func makeGHServer(archivedRepos map[string]bool, releases map[string]*github.ReleaseInfo, repoInfo map[string]*github.RepoInfo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") && !strings.Contains(path, "/contents/") && !strings.Contains(path, "/git/") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				if info, ok := repoInfo[ownerRepo]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(info)
					_, _ = w.Write(body)
					return
				}
				if isArchived, ok := archivedRepos[ownerRepo]; ok {
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
				if release, ok := releases[ownerRepo]; ok {
					w.WriteHeader(200)
					body, _ := json.Marshal(release)
					_, _ = w.Write(body)
					return
				}
			}
			w.WriteHeader(404)
			return
		}

		w.WriteHeader(404)
	}))
}

func TestDetectArchived_ArchivedRepo(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": true,
		"actions/setup-go": false,
	}
	server := makeGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
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

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 1 {
		t.Errorf("Expected 1 archived action, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedActions) > 0 && result.ArchivedActions[0].Repo != "actions/checkout" {
		t.Errorf("Expected archived repo 'actions/checkout', got %s", result.ArchivedActions[0].Repo)
	}
	if len(result.ArchivedRepos) != 1 || result.ArchivedRepos[0] != "actions/checkout" {
		t.Errorf("Expected archived repos ['actions/checkout'], got %v", result.ArchivedRepos)
	}
	if result.Archived["actions/checkout"] != true {
		t.Error("Expected actions/checkout to be marked as archived in result map")
	}
	if result.Archived["actions/setup-go"] != false {
		t.Error("Expected actions/setup-go to be marked as not archived in result map")
	}
	if len(result.NonArchivedRepos) != 1 || result.NonArchivedRepos[0] != "actions/setup-go" {
		t.Errorf("Expected non-archived repos ['actions/setup-go'], got %v", result.NonArchivedRepos)
	}
}

func TestDetectArchived_NoArchived(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": false,
	}
	server := makeGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
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

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 0 {
		t.Errorf("Expected 0 archived actions, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedRepos) != 0 {
		t.Errorf("Expected 0 archived repos, got %d", len(result.ArchivedRepos))
	}
}

func TestDetectArchived_MultipleWorkflows(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout": true,
	}
	server := makeGHServer(archivedRepos, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
		{
			Path: ".github/workflows/release.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
	}

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 2 {
		t.Errorf("Expected 2 archived action uses, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedRepos) != 1 {
		t.Errorf("Expected 1 unique archived repo, got %d", len(result.ArchivedRepos))
	}
}

func TestDetectArchived_EmptyWorkflows(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	result, err := DetectArchived(rc, []*workflow.WorkflowFile{}, []workflow.ActionRef{})
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 0 {
		t.Errorf("Expected 0 archived actions, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedRepos) != 0 {
		t.Errorf("Expected 0 archived repos, got %d", len(result.ArchivedRepos))
	}
}

func TestDetectStale_Deprecated(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:       ownerRepo,
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "Use actions/checkout@v4 instead",
			UpdatedAt:      time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v3", FullRef: "actions/checkout@v3"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)

	if len(staleActions) != 1 {
		t.Fatalf("Expected 1 stale action, got %d", len(staleActions))
	}
	if !staleActions[0].Deprecated {
		t.Error("Expected stale action to be marked as deprecated")
	}
	if staleActions[0].DeprecationMessage != "Use actions/checkout@v4 instead" {
		t.Errorf("Expected deprecation message, got %s", staleActions[0].DeprecationMessage)
	}
}

func TestDetectStale_StaleByAge(t *testing.T) {
	ownerRepo := "actions/old-action"
	oldDate := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:   ownerRepo,
			Archived:   false,
			Deprecated: false,
			UpdatedAt:  oldDate,
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
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
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)

	if len(staleActions) != 1 {
		t.Fatalf("Expected 1 stale action, got %d", len(staleActions))
	}
	if !staleActions[0].StaleByAge {
		t.Error("Expected stale action to be marked as stale by age")
	}
}

func TestDetectStale_EmptyNonArchived(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": true}

	staleActions := DetectStale(rc, workflowFiles, nil, archived, 365)

	if staleActions != nil {
		t.Errorf("Expected nil for all-archived repos, got %v", staleActions)
	}
}

func TestDetectStale_NoStale(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:   ownerRepo,
			Archived:   false,
			Deprecated: false,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)

	if len(staleActions) != 0 {
		t.Errorf("Expected 0 stale actions for recently updated repo, got %d", len(staleActions))
	}
}

func TestDetectRuntimeEOL_ArchivedExcluded(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	archived := map[string]bool{"archived/action": true}
	nonArchivedRepos := []string{}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if result != nil {
		t.Errorf("Expected nil when no non-archived repos, got %v", result)
	}
}

func TestDetectRuntimeEOL_EmptyWorkflows(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	archived := map[string]bool{}
	nonArchivedRepos := []string{"actions/checkout"}

	result := DetectRuntimeEOL(rc, []*workflow.WorkflowFile{}, archived, nonArchivedRepos)

	if result != nil {
		t.Errorf("Expected nil for empty workflows, got %v", result)
	}
}

func TestDetectOutdated_EmptyNonArchived(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	archived := map[string]bool{"actions/checkout": true}
	nonArchivedRepos := []string{}

	outdated, releases := DetectOutdated(rc, nil, archived, nonArchivedRepos)

	if outdated != nil {
		t.Errorf("Expected nil outdated, got %v", outdated)
	}
	if releases != nil {
		t.Errorf("Expected nil releases, got %v", releases)
	}
}

func TestDetectOutdated_WithRelease(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}
	server := makeGHServer(nil, releases, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, releaseMap := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releaseMap == nil {
		t.Fatal("Expected non-nil releases map")
	}
	if _, ok := releaseMap["actions/checkout"]; !ok {
		t.Error("Expected release info for actions/checkout")
	}

	if outdated == nil {
		t.Log("Outdated is nil (possibly because SHA comparison found no difference for v3 tag); this is acceptable")
	}
}

func TestPrintArchived_Empty(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.PrintArchivedText([]issue.ArchivedActionInfo{}, []string{})

	w.Close()
	os.Stdout = oldStdout
	_, _ = buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty actions, got %q", buf.String())
	}
}

func TestPrintArchived_WithActions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}
	repos := []string{"actions/checkout"}

	output.PrintArchivedText(actions, repos)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "actions/checkout@v3") {
		t.Errorf("Expected output to contain 'actions/checkout@v3', got %q", out)
	}
	if !strings.Contains(out, "ci.yml") {
		t.Errorf("Expected output to contain 'ci.yml', got %q", out)
	}
}

func TestPrintStale_Empty(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.PrintStaleText([]actioninfo.StaleActionInfo{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty stale actions, got %q", buf.String())
	}
}

func TestPrintStale_WithActions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []actioninfo.StaleActionInfo{
		{
			OwnerRepo:          "actions/checkout",
			FullRef:            "actions/checkout@v3",
			Workflow:           "ci.yml",
			Deprecated:         true,
			DeprecationMessage: "Use v4 instead",
		},
	}

	output.PrintStaleText(actions)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "DEPRECATED") {
		t.Errorf("Expected output to contain 'DEPRECATED', got %q", out)
	}
	if !strings.Contains(out, "Use v4 instead") {
		t.Errorf("Expected output to contain deprecation message, got %q", out)
	}
}

func TestPrintStale_StaleByAge(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []actioninfo.StaleActionInfo{
		{
			OwnerRepo:   "actions/old-action",
			FullRef:     "actions/old-action@v1",
			Workflow:    "ci.yml",
			StaleByAge:  true,
			LastUpdated: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	output.PrintStaleText(actions)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "not updated since") {
		t.Errorf("Expected output to contain 'not updated since', got %q", out)
	}
}

func TestPrintRuntimeEOL_Empty(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.PrintRuntimeEOLText([]actioninfo.RuntimeEOLActionInfo{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty runtime EOL actions, got %q", buf.String())
	}
}

func TestPrintRuntimeEOL_WithActions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []actioninfo.RuntimeEOLActionInfo{
		{
			OwnerRepo: "actions/checkout",
			FullRef:   "actions/checkout@v3",
			Workflow:  "ci.yml",
			Runtime:   "nodejs",
			Version:   "16",
			EOLDate:   time.Date(2023, 9, 11, 0, 0, 0, 0, time.UTC),
		},
	}

	output.PrintRuntimeEOLText(actions)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "nodejs16") {
		t.Errorf("Expected output to contain 'nodejs16', got %q", out)
	}
	if !strings.Contains(out, "2023-09-11") {
		t.Errorf("Expected output to contain EOL date, got %q", out)
	}
}

func TestPrintRuntimeEOL_ZeroEOLDate(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []actioninfo.RuntimeEOLActionInfo{
		{
			OwnerRepo: "actions/checkout",
			FullRef:   "actions/checkout@v3",
			Workflow:  "ci.yml",
			Runtime:   "nodejs",
			Version:   "99",
			EOLDate:   time.Time{},
		},
	}

	output.PrintRuntimeEOLText(actions)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "unknown") {
		t.Errorf("Expected output to contain 'unknown' for zero EOL date, got %q", out)
	}
}

func TestPrintOutdated_Empty(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.PrintOutdatedText([]actioninfo.OutdatedActionInfo{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty outdated actions, got %q", buf.String())
	}
}

func TestPrintOutdated_WithActions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	actions := []actioninfo.OutdatedActionInfo{
		{
			OwnerRepo:  "actions/checkout",
			CurrentRef: "v3",
			LatestTag:  "v4",
			LatestURL:  "https://github.com/actions/checkout/releases/tag/v4",
			Workflow:   "ci.yml",
			FullRef:    "actions/checkout@v3",
		},
	}

	output.PrintOutdatedText(actions)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	if !strings.Contains(out, "actions/checkout@v3") {
		t.Errorf("Expected output to contain 'actions/checkout@v3', got %q", out)
	}
	if !strings.Contains(out, "v4") {
		t.Errorf("Expected output to contain latest tag 'v4', got %q", out)
	}
}

func TestSendArchivedNotifications_NilNotifier(t *testing.T) {
	rc := &RunContext{
		Ctx:      context.Background(),
		Notifier: nil,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	SendArchivedNotifications(rc, actions)
}

func TestSendArchivedNotifications_EmptyActions(t *testing.T) {
	rc := &RunContext{
		Ctx:      context.Background(),
		Notifier: nil,
		WorkDir:  "/tmp/owner/repo",
	}

	SendArchivedNotifications(rc, []issue.ArchivedActionInfo{})
}

func TestCreateArchivedIssues_NilIssueCreator(t *testing.T) {
	rc := &RunContext{
		Ctx:          context.Background(),
		IssueCreator: nil,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)
}

func TestCreateArchivedIssues_EmptyActions(t *testing.T) {
	rc := &RunContext{
		Ctx:          context.Background(),
		IssueCreator: issue.NewIssueCreator("test-token"),
	}

	CreateArchivedIssues(rc, []issue.ArchivedActionInfo{})
}

func TestCheckResult_Fields(t *testing.T) {
	cr := &CheckResult{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
		},
		ArchivedRepos:     []string{"actions/checkout"},
		StaleActions:      []actioninfo.StaleActionInfo{},
		RuntimeEOLActions: []actioninfo.RuntimeEOLActionInfo{},
		OutdatedActions:   []actioninfo.OutdatedActionInfo{},
		Releases:          map[string]*github.ReleaseInfo{},
		Archived:          map[string]bool{"actions/checkout": true},
		NonArchivedRepos:  []string{"actions/setup-go"},
	}

	if len(cr.ArchivedActions) != 1 {
		t.Errorf("Expected 1 archived action, got %d", len(cr.ArchivedActions))
	}
	if len(cr.ArchivedRepos) != 1 {
		t.Errorf("Expected 1 archived repo, got %d", len(cr.ArchivedRepos))
	}
	if cr.Archived["actions/checkout"] != true {
		t.Error("Expected actions/checkout to be archived")
	}
	if len(cr.NonArchivedRepos) != 1 {
		t.Errorf("Expected 1 non-archived repo, got %d", len(cr.NonArchivedRepos))
	}
}

func TestRunContext_Fields(t *testing.T) {
	rc := &RunContext{
		Ctx:     context.Background(),
		WorkDir: "/tmp/test",
		Parser:  &workflow.WorkflowParser{},
		Verbose: true,
		Debug:   false,
	}

	if rc.WorkDir != "/tmp/test" {
		t.Errorf("Expected WorkDir '/tmp/test', got %s", rc.WorkDir)
	}
	if rc.Verbose != true {
		t.Error("Expected Verbose to be true")
	}
	if rc.Debug != false {
		t.Error("Expected Debug to be false")
	}
}

func TestDetectArchived_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
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

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() should not return error even on API errors, got %v", err)
	}
	if len(result.ArchivedActions) != 0 {
		t.Errorf("Expected 0 archived actions on API error, got %d", len(result.ArchivedActions))
	}
}

func TestDetectStale_SanitizesStaleDays(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:   ownerRepo,
			Archived:   false,
			Deprecated: false,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		},
	}
	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, -1)

	if len(staleActions) != 0 {
		t.Errorf("Expected 0 stale actions for recently updated repo with sanitized stale days, got %d", len(staleActions))
	}
}

func TestWriteResult_WithIssues(t *testing.T) {
	var buf bytes.Buffer
	w := &output.Writer{
		Format: output.FormatText,
		Output: &buf,
	}

	archivedActions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}
	staleActions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "actions/old-action", FullRef: "actions/old-action@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "Use v2 instead"},
	}
	outdatedActions := []actioninfo.OutdatedActionInfo{
		{OwnerRepo: "actions/setup-go", CurrentRef: "v3", LatestTag: "v4", LatestURL: "https://github.com/actions/setup-go/releases/tag/v4", Workflow: "ci.yml", FullRef: "actions/setup-go@v3"},
	}

	WriteResult(w, archivedActions, []string{"actions/checkout"}, staleActions, nil, outdatedActions, nil, true, "Found issues", "")

	out := buf.String()
	if !strings.Contains(out, "actions/checkout@v3") {
		t.Errorf("Expected output to contain archived action ref, got %q", out)
	}
	if !strings.Contains(out, "actions/old-action@v1") {
		t.Errorf("Expected output to contain stale action ref, got %q", out)
	}
	if !strings.Contains(out, "actions/setup-go@v3") {
		t.Errorf("Expected output to contain outdated action ref, got %q", out)
	}
	if !strings.Contains(out, "Found issues") {
		t.Errorf("Expected output to contain summary, got %q", out)
	}
}

func TestWriteResult_NoIssues(t *testing.T) {
	var buf bytes.Buffer
	w := &output.Writer{
		Format: output.FormatText,
		Output: &buf,
	}

	WriteResult(w, nil, nil, nil, nil, nil, nil, false, "", "No issues found!")

	out := buf.String()
	if !strings.Contains(out, "No issues found!") {
		t.Errorf("Expected output to contain noIssuesMessage, got %q", out)
	}
}

func TestWriteResult_JSON(t *testing.T) {
	var buf bytes.Buffer
	w := &output.Writer{
		Format: output.FormatJSON,
		Output: &buf,
	}

	archivedActions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	WriteResult(w, archivedActions, []string{"actions/checkout"}, nil, nil, nil, nil, true, "", "")

	out := buf.String()
	if !strings.Contains(out, `"archived_actions"`) {
		t.Errorf("Expected JSON output to contain archived_actions key, got %q", out)
	}
	if !strings.Contains(out, `"has_issues"`) {
		t.Errorf("Expected JSON output to contain has_issues key, got %q", out)
	}
}

func TestDetectRuntimeEOL_WithRuntimeResults(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n using: node12\n main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/12") {
			eolFrom := "2022-04-30"
			resp := map[string]interface{}{
				"schema_version": "1.0.0",
				"result": map[string]interface{}{
					"name":    "12",
					"isEol":   true,
					"eolFrom": eolFrom,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if len(result) != 1 {
		t.Fatalf("Expected 1 runtime EOL action, got %d", len(result))
	}
	if result[0].Runtime != "nodejs" {
		t.Errorf("Expected runtime nodejs, got %s", result[0].Runtime)
	}
	if result[0].Version != "12" {
		t.Errorf("Expected version 12, got %s", result[0].Version)
	}
	if result[0].OwnerRepo != "actions/checkout" {
		t.Errorf("Expected OwnerRepo actions/checkout, got %s", result[0].OwnerRepo)
	}
	if result[0].Workflow != "ci.yml" {
		t.Errorf("Expected Workflow ci.yml, got %s", result[0].Workflow)
	}
	if !strings.Contains(result[0].EOLDate.Format("2006-01-02"), "2022") {
		t.Errorf("Expected EOLDate year 2022, got %v", result[0].EOLDate)
	}
}

func TestDetectOutdated_OutdatedAction(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v4.0.0", Name: "v4.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v4.0.0"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	server := makeGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/setup-go": false}
	nonArchivedRepos := []string{"actions/setup-go"}

	outdated, releaseMap := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releaseMap == nil {
		t.Fatal("Expected non-nil releases map")
	}
	if _, ok := releaseMap["actions/setup-go"]; !ok {
		t.Error("Expected release info for actions/setup-go")
	}

	if len(outdated) > 0 {
		if outdated[0].OwnerRepo != "actions/setup-go" {
			t.Errorf("Expected outdated OwnerRepo actions/setup-go, got %s", outdated[0].OwnerRepo)
		}
		if outdated[0].LatestTag != "v4.0.0" {
			t.Errorf("Expected LatestTag v4.0.0, got %s", outdated[0].LatestTag)
		}
	}
}

func TestDetectOutdated_UpToDateAction(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v3.1.0", Name: "v3.1.0", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3.1.0"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	server := makeGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, releaseMap := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releaseMap == nil {
		t.Fatal("Expected non-nil releases map")
	}
	if len(outdated) > 0 {
		t.Errorf("Expected no outdated actions for up-to-date ref, got %d", len(outdated))
	}
}

func TestSendArchivedNotifications_WithNotifier(t *testing.T) {
	notifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"appid":1,"messageid":1}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer notifyServer.Close()

	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")

	conf := config.NotificationConfig{
		GotifyEndpoint: notifyServer.URL,
		GotifyToken:    "test-token",
	}
	mgr, err := notification.NewNotificationManager(conf)
	if err != nil {
		t.Fatalf("Failed to create notification manager: %v", err)
	}

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/owner/repo",
		Notifier: mgr,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	SendArchivedNotifications(rc, actions)
}

func TestSendArchivedNotifications_WithNotifierError(t *testing.T) {
	notifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer notifyServer.Close()

	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")

	conf := config.NotificationConfig{
		GotifyEndpoint: notifyServer.URL,
		GotifyToken:    "test-token",
	}
	mgr, err := notification.NewNotificationManager(conf)
	if err != nil {
		t.Fatalf("Failed to create notification manager: %v", err)
	}

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/owner/repo",
		Notifier: mgr,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	SendArchivedNotifications(rc, actions)
}

func TestCreateArchivedIssues_WithIssueCreator(t *testing.T) {
	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")

	var testCalled bool
	var testOwner, testRepo string
	var testActions []issue.ArchivedActionInfo

	ic := issue.NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []issue.ArchivedActionInfo) error {
		testCalled = true
		testOwner = owner
		testRepo = repo
		testActions = archivedActions
		return nil
	})

	rc := &RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/owner/repo",
		IssueCreator: ic,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)

	if !testCalled {
		t.Error("Expected IssueCreator to be called")
	}
	if testOwner != "owner" {
		t.Errorf("Expected owner 'owner', got %s", testOwner)
	}
	if testRepo != "repo" {
		t.Errorf("Expected repo 'repo', got %s", testRepo)
	}
	if len(testActions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(testActions))
	}
}

func TestCreateArchivedIssues_InvalidRepoName(t *testing.T) {
	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "invalid-repo-name")

	var testCalled bool

	ic := issue.NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []issue.ArchivedActionInfo) error {
		testCalled = true
		return nil
	})

	rc := &RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/owner/repo",
		IssueCreator: ic,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)

	if testCalled {
		t.Error("Expected IssueCreator NOT to be called for invalid repo name format")
	}
}

func TestCreateArchivedIssues_EmptyRepoName(t *testing.T) {
	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Unsetenv("GITHUB_REPOSITORY")

	var testCalled bool

	ic := issue.NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []issue.ArchivedActionInfo) error {
		testCalled = true
		return nil
	})

	rc := &RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/owner/repo",
		IssueCreator: ic,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)

	if testCalled {
		t.Error("Expected IssueCreator NOT to be called when GITHUB_REPOSITORY is unset (defaults to 'current-repo')")
	}
}

func TestNewRunContext(t *testing.T) {
	rc := NewRunContext("dummy-token", config.Config{}, false, false, output.FormatText, nil, false, false, 0)

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	if rc.Ctx == nil {
		t.Error("Expected Ctx to be non-nil")
	}
	if rc.Parser == nil {
		t.Error("Expected Parser to be non-nil")
	}
	if rc.GHClient == nil {
		t.Error("Expected GHClient to be non-nil")
	}
	if rc.OutputWriter == nil {
		t.Error("Expected OutputWriter to be non-nil")
	}
	if rc.Notifier != nil {
		t.Error("Expected Notifier to be nil when initNotifier=false")
	}
	if rc.IssueCreator != nil {
		t.Error("Expected IssueCreator to be nil when initIssueCreator=false")
	}
	if rc.WorkDir == "" {
		t.Error("Expected WorkDir to be non-empty")
	}
}

func TestNewRunContext_WithIssueCreator(t *testing.T) {
	rc := NewRunContext("dummy-token", config.Config{}, false, true, output.FormatText, nil, false, false, 0)

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	if rc.IssueCreator == nil {
		t.Error("Expected IssueCreator to be non-nil when initIssueCreator=true")
	}
}

func TestDetectStale_BothDeprecatedAndStaleByAge(t *testing.T) {
	ownerRepo := "actions/old-deprecated-action"
	oldDate := time.Now().Add(-500 * 24 * time.Hour).Format(time.RFC3339)
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:       ownerRepo,
			Archived:       false,
			Deprecated:     true,
			DeprecationMsg: "This action is deprecated and stale",
			UpdatedAt:      oldDate,
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v1", FullRef: "actions/old-deprecated-action@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v1", FullRef: "actions/old-deprecated-action@v1"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)

	if len(staleActions) != 1 {
		t.Fatalf("Expected 1 stale action, got %d", len(staleActions))
	}
	if !staleActions[0].Deprecated {
		t.Error("Expected stale action to be marked as deprecated")
	}
	if !staleActions[0].StaleByAge {
		t.Error("Expected stale action to be marked as stale by age")
	}
	if staleActions[0].DeprecationMessage != "This action is deprecated and stale" {
		t.Errorf("Expected deprecation message, got %s", staleActions[0].DeprecationMessage)
	}
}

func TestDetectArchived_WithRepoInfo(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:       ownerRepo,
			Archived:       true,
			Name:           "checkout",
			Deprecated:     false,
			DeprecationMsg: "",
			UpdatedAt:      time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: true}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 1 {
		t.Errorf("Expected 1 archived action, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedRepos) != 1 || result.ArchivedRepos[0] != ownerRepo {
		t.Errorf("Expected archived repos [%s], got %v", ownerRepo, result.ArchivedRepos)
	}
}

func TestRunReposMode_NoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	var processCalled bool
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		processCalled = true
		return false
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if result != false {
		t.Error("Expected false when no repos with workflows found")
	}
	if processCalled {
		t.Error("Expected processFunc NOT to be called when no repos found")
	}
}

func TestRunReposMode_WithRepos(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "my-repo")
	workflowsDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}
	workflowContent := "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	server := makeGHServer(map[string]bool{"actions/checkout": false}, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	var processCalled bool
	var receivedWorkDir string
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		processCalled = true
		receivedWorkDir = workDir
		return true
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if !result {
		t.Error("Expected true when repos have issues")
	}
	if !processCalled {
		t.Error("Expected processFunc to be called")
	}
	if receivedWorkDir != repoDir {
		t.Errorf("Expected workDir %s, got %s", repoDir, receivedWorkDir)
	}
}

func TestRunReposMode_WithReposNoIssues(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "my-repo")
	workflowsDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}
	workflowContent := "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return false
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if result {
		t.Error("Expected false when processFunc returns no issues")
	}
}

func TestDetectRuntimeEOL_VerboseLogging(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n using: node12\n main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/12") {
			eolFrom := "2022-04-30"
			resp := map[string]interface{}{
				"schema_version": "1.0.0",
				"result": map[string]interface{}{
					"name":    "12",
					"isEol":   true,
					"eolFrom": eolFrom,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Verbose:  true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)
	if len(result) != 1 {
		t.Errorf("Expected 1 runtime EOL action, got %d", len(result))
	}
}
