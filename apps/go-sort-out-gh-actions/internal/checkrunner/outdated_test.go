package checkrunner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func TestLatestEligibleRelease(t *testing.T) {
	cutoff := time.Date(2026, 10, 7, 6, 37, 0, 0, time.UTC)
	tests := []struct {
		name     string
		releases []github.ReleaseInfo
		want     string
		wantErr  bool
	}{
		{
			name: "highest eligible version despite API order and recent release",
			releases: []github.ReleaseInfo{
				{TagName: "v1.0.0", PublishedAt: cutoff.Add(-48 * time.Hour).Format(time.RFC3339)},
				{TagName: "v3.0.0", PublishedAt: cutoff.Add(time.Second).Format(time.RFC3339)},
				{TagName: "v2.0.0", PublishedAt: cutoff.Format(time.RFC3339)},
			},
			want: "v2.0.0",
		},
		{
			name: "all releases in cooldown",
			releases: []github.ReleaseInfo{
				{TagName: "v2.0.0", PublishedAt: cutoff.Add(time.Second).Format(time.RFC3339)},
			},
		},
		{
			name: "draft prerelease and non-SemVer are excluded",
			releases: []github.ReleaseInfo{
				{TagName: "v9.0.0", Draft: true},
				{TagName: "v8.0.0", Prerelease: true},
				{TagName: "v7.0.0-beta"},
				{TagName: "nightly"},
				{TagName: "v1.0.0", PublishedAt: cutoff.Format(time.RFC3339)},
			},
			want: "v1.0.0",
		},
		{name: "missing date fails", releases: []github.ReleaseInfo{{TagName: "v1.0.0"}}, wantErr: true},
		{name: "invalid date fails", releases: []github.ReleaseInfo{{TagName: "v1.0.0", PublishedAt: "invalid"}}, wantErr: true},
		{name: "no published releases"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := latestEligibleRelease(tt.releases, cutoff)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			tag := ""
			if got != nil {
				tag = got.TagName
			}
			if tag != tt.want {
				t.Fatalf("tag = %q, want %q", tag, tt.want)
			}
		})
	}
}

func workflowWithRefs(refs ...string) string {
	content := "jobs:\n  test:\n    steps:\n"
	for _, ref := range refs {
		content += "      - uses: " + ref + "\n"
	}
	return content
}

func TestDetectOutdatedFiltered(t *testing.T) {
	oldSHA := strings.Repeat("a", 40)
	targetSHA := strings.Repeat("b", 40)
	newSHA := strings.Repeat("c", 40)
	workflowPath := ".github/workflows/ci.yaml"
	oldRef := "owner/action/path@v1.0.0"
	tests := []struct {
		name       string
		current    string
		base       string
		head       string
		file       string
		author     string
		branch     string
		state      string
		status     string
		ageOnly    bool
		disableAge bool
		apiError   string
		wantCount  int
		wantErr    bool
	}{
		{name: "uncovered update", head: workflowWithRefs(oldRef), wantCount: 1},
		{name: "covered tag update", head: workflowWithRefs("owner/action/path@v2.1.0")},
		{name: "covered SHA update", head: workflowWithRefs("owner/action/path@" + targetSHA)},
		{name: "covered newer SHA update", head: workflowWithRefs("owner/action/path@" + newSHA)},
		{name: "covered major alias", head: workflowWithRefs("owner/action/path@v2")},
		{name: "PR filter without age filter", head: workflowWithRefs("owner/action/path@v2.1.0"), disableAge: true},
		{name: "PR filter alone retains unrelated update", head: workflowWithRefs(oldRef), disableAge: true, wantCount: 1},
		{name: "older PR does not cover target", head: workflowWithRefs("owner/action/path@v2.0.0"), wantCount: 1},
		{name: "older SHA does not cover target", head: workflowWithRefs("owner/action/path@" + oldSHA), wantCount: 1},
		{name: "different action path", head: workflowWithRefs("owner/action/other@v2.1.0"), wantCount: 1},
		{name: "different workflow", file: ".github/workflows/other.yaml", head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "same basename in another directory", file: ".github/workflows/nested/ci.yaml", head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "wrong author", author: "someone", head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "wrong target branch", branch: "other", head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "closed PR", state: "closed", head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "partially updated workflow", head: workflowWithRefs(oldRef, "owner/action/path@v2.1.0"), wantCount: 1},
		{name: "base does not contain scanned ref", base: workflowWithRefs("owner/action/path@v1.1.0"), head: workflowWithRefs("owner/action/path@v2.1.0"), wantCount: 1},
		{name: "removed workflow stays visible", status: "removed", wantCount: 1},
		{name: "current newer SHA is not downgraded", current: "owner/action/path@" + newSHA, ageOnly: true},
		{name: "current older SHA stays outdated", current: "owner/action/path@" + oldSHA, ageOnly: true, wantCount: 1},
		{name: "current newer tag is not downgraded", current: "owner/action/path@v3.0.0", ageOnly: true},
		{name: "release API failure", apiError: "/repos/owner/action/releases", wantErr: true},
		{name: "PR API failure", apiError: "/repos/test/repo/pulls", wantErr: true},
		{name: "PR files API failure", apiError: "/repos/test/repo/pulls/1/files", wantErr: true},
		{name: "workflow API failure", apiError: "/repos/test/repo/contents/" + workflowPath, wantErr: true},
		{name: "release SHA API failure", current: "owner/action/path@" + oldSHA, apiError: "/repos/owner/action/git/refs/tags/v2.1.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, base, file, author, branch, state, status := tt.current, tt.base, tt.file, tt.author, tt.branch, tt.state, tt.status
			if current == "" {
				current = oldRef
			}
			if base == "" {
				base = workflowWithRefs(current)
			}
			if file == "" {
				file = workflowPath
			}
			if author == "" {
				author = "dependabot[bot]"
			}
			if branch == "" {
				branch = "main"
			}
			if state == "" {
				state = "open"
			}
			if status == "" {
				status = "modified"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tt.apiError {
					http.Error(w, "API failure", http.StatusForbidden)
					return
				}
				switch r.URL.Path {
				case "/repos/owner/action/releases/latest":
					_ = json.NewEncoder(w).Encode(github.ReleaseInfo{TagName: "v2.1.0"})
				case "/repos/owner/action/releases":
					_ = json.NewEncoder(w).Encode([]github.ReleaseInfo{
						{TagName: "v3.0.0", PublishedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
						{TagName: "v2.1.0", PublishedAt: time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)},
						{TagName: "v1.0.0", PublishedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)},
					})
				case "/repos/test/repo/pulls":
					if r.URL.Query().Get("base") != "main" || r.URL.Query().Get("state") != "open" {
						t.Errorf("unexpected PR query: %s", r.URL.RawQuery)
					}
					_ = json.NewEncoder(w).Encode([]github.PullRequest{{
						Number: 1, State: state, User: github.Owner{Login: author},
						Base: github.PullRequestRef{Ref: branch, SHA: "base-sha"},
						Head: github.PullRequestRef{SHA: "head-sha", Repo: github.RepoEntry{FullName: "test/repo"}},
					}})
				case "/repos/test/repo/pulls/1/files":
					_ = json.NewEncoder(w).Encode([]github.PullRequestFile{
						{Filename: "package-lock.json", Status: "modified"},
						{Filename: file, Status: status},
					})
				case "/repos/test/repo/contents/" + workflowPath:
					content := base
					if r.URL.Query().Get("ref") == "head-sha" {
						content = tt.head
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"content": base64.StdEncoding.EncodeToString([]byte(content)), "encoding": "base64"})
				default:
					if strings.HasPrefix(r.URL.Path, "/repos/owner/action/git/refs/tags/") {
						tag := strings.TrimPrefix(r.URL.Path, "/repos/owner/action/git/refs/tags/")
						sha := map[string]string{"v1.0.0": oldSHA, "v2.1.0": targetSHA, "v2": targetSHA, "v3.0.0": newSHA}[tag]
						if sha != "" {
							_, _ = fmt.Fprintf(w, `{"object":{"sha":%q,"type":"commit"}}`, sha)
							return
						}
					}
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			rc := newTestRunContext(server)
			rc.OutdatedOptions = OutdatedOptions{MinReleaseAgeDays: 7, IgnoreDependabotPRs: !tt.ageOnly, Repository: "test/repo", BaseBranch: "main"}
			if tt.disableAge {
				rc.OutdatedOptions.MinReleaseAgeDays = 0
			}
			parsed, err := rc.Parser.ParseWorkflowContent(filepath.Join(rc.WorkDir, workflowPath), workflowWithRefs(current))
			if err != nil {
				t.Fatal(err)
			}
			actions, releases, err := DetectOutdatedFiltered(rc, []*workflow.WorkflowFile{parsed}, nil, []string{"owner/action"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if len(actions) != tt.wantCount {
				t.Fatalf("actions = %+v, want count %d", actions, tt.wantCount)
			}
			if err == nil && releases["owner/action"].TagName != "v2.1.0" {
				t.Fatalf("wrong eligible release: %+v", releases)
			}
		})
	}
}
