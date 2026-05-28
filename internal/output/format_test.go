package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
		wantErr  bool
	}{
		{"text", FormatText, false},
		{"json", FormatJSON, false},
		{"xml", "", true},
		{"", "", true},
		{"TEXT", "", true},
	}

	for _, tt := range tests {
		got, err := ParseFormat(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseFormat(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFormat(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestWriterJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatJSON, Output: &buf}

	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "owner/repo", Workflow: "ci.yml", Uses: "owner/repo@v1"},
		},
		ArchivedRepos: []string{"owner/repo"},
		HasIssues:     true,
		Summary:       "Archived actions detected",
	}

	w.WriteCheckResult(co)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result["has_issues"] != true {
		t.Errorf("has_issues = %v, want true", result["has_issues"])
	}
	if _, ok := result["archived_actions"]; !ok {
		t.Error("archived_actions field missing from JSON output")
	}
	if _, ok := result["summary"]; !ok {
		t.Error("summary field missing from JSON output")
	}
}

func TestWriterJSONEmptyResult(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatJSON, Output: &buf}

	co := &CheckOutput{
		HasIssues: false,
	}

	w.WriteCheckResult(co)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result["has_issues"] != false {
		t.Errorf("has_issues = %v, want false", result["has_issues"])
	}
}

func TestWriterTextOutput(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatText, Output: &buf}

	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "owner/repo", Workflow: "ci.yml", Uses: "owner/repo@v1"},
		},
		ArchivedRepos: []string{"owner/repo"},
		HasIssues:     true,
		Summary:       "Test summary",
	}

	w.WriteCheckResult(co)

	output := buf.String()
	if !strings.Contains(output, "archived") && !strings.Contains(output, "ARCHIVED") {
		t.Errorf("Text output should contain archived info, got: %s", output)
	}
	if !strings.Contains(output, "Test summary") {
		t.Errorf("Text output should contain summary, got: %s", output)
	}
}

func TestWriterTextEmptyResult(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatText, Output: &buf}

	co := &CheckOutput{
		HasIssues:       false,
		NoIssuesMessage: "No issues found",
	}

	w.WriteCheckResult(co)

	output := buf.String()
	if !strings.Contains(output, "No issues found") {
		t.Errorf("Text output for no-issues result should contain noIssuesMessage, got: %q", output)
	}
}

func TestJSONOutputAllCategories(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatJSON, Output: &buf}

	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
		},
		ArchivedRepos: []string{"archived/repo"},
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use other instead"},
		},
		RuntimeEOL: []actioninfo.RuntimeEOLActionInfo{
			{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "14", EOLDate: time.Date(2023, 4, 30, 0, 0, 0, 0, time.UTC)},
		},
		OutdatedActions: []actioninfo.OutdatedActionInfo{
			{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v2", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
		},
		HasIssues: true,
		Summary:   "Issues found",
	}

	w.WriteCheckResult(co)

	var result CheckOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v\nOutput: %s", err, buf.String())
	}

	if len(result.ArchivedActions) != 1 {
		t.Errorf("archived_actions length = %d, want 1", len(result.ArchivedActions))
	}
	if len(result.StaleActions) != 1 {
		t.Errorf("stale_actions length = %d, want 1", len(result.StaleActions))
	}
	if len(result.RuntimeEOL) != 1 {
		t.Errorf("runtime_eol_actions length = %d, want 1", len(result.RuntimeEOL))
	}
	if len(result.OutdatedActions) != 1 {
		t.Errorf("outdated_actions length = %d, want 1", len(result.OutdatedActions))
	}
	if !result.HasIssues {
		t.Error("has_issues = false, want true")
	}
}

func TestNewWriter(t *testing.T) {
	w := NewWriter(FormatText)
	if w.Format != FormatText {
		t.Errorf("NewWriter(FormatText).Format = %q, want %q", w.Format, FormatText)
	}
	if w.Output != os.Stdout {
		t.Errorf("NewWriter(FormatText).Output = %v, want os.Stdout", w.Output)
	}

	wj := NewWriter(FormatJSON)
	if wj.Format != FormatJSON {
		t.Errorf("NewWriter(FormatJSON).Format = %q, want %q", wj.Format, FormatJSON)
	}
	if wj.Output != os.Stdout {
		t.Errorf("NewWriter(FormatJSON).Output = %v, want os.Stdout", wj.Output)
	}
}

func TestFprintArchivedText(t *testing.T) {
	var buf bytes.Buffer
	actions := []issue.ArchivedActionInfo{
		{Repo: "owner/repo", Workflow: "ci.yml", Uses: "owner/repo@v1"},
	}
	repos := []string{"owner/repo"}

	FprintArchivedText(&buf, actions, repos)

	output := buf.String()
	if !strings.Contains(output, "Found 1 archived GitHub Actions in 1 uses") {
		t.Errorf("expected archived header, got: %s", output)
	}
	if !strings.Contains(output, "ci.yml") {
		t.Errorf("expected workflow name, got: %s", output)
	}
	if !strings.Contains(output, "owner/repo@v1") {
		t.Errorf("expected action uses, got: %s", output)
	}
}

func TestFprintArchivedText_MultipleWorkflows(t *testing.T) {
	var buf bytes.Buffer
	actions := []issue.ArchivedActionInfo{
		{Repo: "a/repo", Workflow: "deploy.yml", Uses: "a/repo@v1"},
		{Repo: "b/repo", Workflow: "ci.yml", Uses: "b/repo@v2"},
	}
	repos := []string{"a/repo", "b/repo"}

	FprintArchivedText(&buf, actions, repos)

	output := buf.String()
	ciIdx := strings.Index(output, "ci.yml")
	deployIdx := strings.Index(output, "deploy.yml")
	if ciIdx == -1 || deployIdx == -1 {
		t.Fatalf("expected both workflows in output, got: %s", output)
	}
	if deployIdx < ciIdx {
		t.Errorf("expected workflows sorted alphabetically (ci.yml before deploy.yml), got: %s", output)
	}
	if !strings.Contains(output, "Found 2 archived GitHub Actions in 2 uses") {
		t.Errorf("expected count of 2 repos and 2 uses, got: %s", output)
	}
}

func TestFprintStaleText_Deprecated(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use other instead"},
	}

	FprintStaleText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "DEPRECATED") {
		t.Errorf("expected DEPRECATED in output, got: %s", output)
	}
	if !strings.Contains(output, "use other instead") {
		t.Errorf("expected deprecation message in output, got: %s", output)
	}
	if !strings.Contains(output, "ci.yml") {
		t.Errorf("expected workflow name in output, got: %s", output)
	}
}

func TestFprintStaleText_StaleByAge(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "old/repo", FullRef: "old/repo@v2", Workflow: "build.yml", StaleByAge: true, LastUpdated: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
	}

	FprintStaleText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "not updated since") {
		t.Errorf("expected 'not updated since' in output, got: %s", output)
	}
	if !strings.Contains(output, "2024-06-15") {
		t.Errorf("expected last updated date in output, got: %s", output)
	}
}

func TestFprintStaleText_MixedActions(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use v2"},
		{OwnerRepo: "old/repo", FullRef: "old/repo@v3", Workflow: "ci.yml", StaleByAge: true, LastUpdated: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC)},
	}

	FprintStaleText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "DEPRECATED") {
		t.Errorf("expected DEPRECATED in output, got: %s", output)
	}
	if !strings.Contains(output, "use v2") {
		t.Errorf("expected deprecation message, got: %s", output)
	}
	if !strings.Contains(output, "not updated since") {
		t.Errorf("expected stale-by-age text, got: %s", output)
	}
	if !strings.Contains(output, "2023-01-10") {
		t.Errorf("expected stale date, got: %s", output)
	}
}

func TestFprintRuntimeEOLText_WithActions(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.RuntimeEOLActionInfo{
		{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "14", EOLDate: time.Date(2023, 4, 30, 0, 0, 0, 0, time.UTC)},
	}

	FprintRuntimeEOLText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "node") {
		t.Errorf("expected runtime name in output, got: %s", output)
	}
	if !strings.Contains(output, "14") {
		t.Errorf("expected version in output, got: %s", output)
	}
	if !strings.Contains(output, "2023-04-30") {
		t.Errorf("expected EOL date in output, got: %s", output)
	}
	if !strings.Contains(output, "ci.yml") {
		t.Errorf("expected workflow name in output, got: %s", output)
	}
}

func TestFprintRuntimeEOLText_ZeroEOLDate(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.RuntimeEOLActionInfo{
		{OwnerRepo: "eol/repo", FullRef: "eol/repo@v2", Workflow: "ci.yml", Runtime: "node", Version: "12", EOLDate: time.Time{}},
	}

	FprintRuntimeEOLText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "unknown") {
		t.Errorf("expected 'unknown' for zero EOL date, got: %s", output)
	}
}

func TestFprintOutdatedText_WithActions(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.OutdatedActionInfo{
		{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v2", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
	}

	FprintOutdatedText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "outdated/repo@v1") {
		t.Errorf("expected current ref in output, got: %s", output)
	}
	if !strings.Contains(output, "v2") {
		t.Errorf("expected latest tag in output, got: %s", output)
	}
	if !strings.Contains(output, "ci.yml") {
		t.Errorf("expected workflow name in output, got: %s", output)
	}
}

func TestFprintOutdatedText_WithFullRef(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.OutdatedActionInfo{
		{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v3", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
	}

	FprintOutdatedText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "outdated/repo@v1 (latest: v3)") {
		t.Errorf("expected full ref with latest tag, got: %s", output)
	}

	var buf2 bytes.Buffer
	actionsNoFullRef := []actioninfo.OutdatedActionInfo{
		{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v3", Workflow: "ci.yml", FullRef: ""},
	}

	FprintOutdatedText(&buf2, actionsNoFullRef)

	output2 := buf2.String()
	if !strings.Contains(output2, "outdated/repo@v1 (latest: v3)") {
		t.Errorf("expected owner/repo@currentRef fallback when FullRef empty, got: %s", output2)
	}
}

func TestWriteText_OnlyStale(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatText, Output: &buf}

	co := &CheckOutput{
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use v2"},
		},
		HasIssues: true,
	}

	w.WriteCheckResult(co)

	output := buf.String()
	if !strings.Contains(output, "stale") && !strings.Contains(output, "STALE") {
		t.Errorf("expected stale output, got: %s", output)
	}
	if strings.Contains(output, "archived") || strings.Contains(output, "ARCHIVED") {
		t.Errorf("should not contain archived output, got: %s", output)
	}
}

func TestWriteText_OnlyOutdated(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatText, Output: &buf}

	co := &CheckOutput{
		OutdatedActions: []actioninfo.OutdatedActionInfo{
			{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v2", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
		},
		HasIssues: true,
	}

	w.WriteCheckResult(co)

	output := buf.String()
	if !strings.Contains(output, "outdated") && !strings.Contains(output, "WARN") {
		t.Errorf("expected outdated output, got: %s", output)
	}
}

func TestWriteText_OnlyRuntimeEOL(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Format: FormatText, Output: &buf}

	co := &CheckOutput{
		RuntimeEOL: []actioninfo.RuntimeEOLActionInfo{
			{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "14", EOLDate: time.Date(2023, 4, 30, 0, 0, 0, 0, time.UTC)},
		},
		HasIssues: true,
	}

	w.WriteCheckResult(co)

	output := buf.String()
	if !strings.Contains(output, "EOL") && !strings.Contains(output, "RUNTIME") {
		t.Errorf("expected runtime EOL output, got: %s", output)
	}
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write failed")
}

func TestWriteCheckResult_JSONEncodingError(t *testing.T) {
	w := &Writer{Format: FormatJSON, Output: failingWriter{}}

	co := &CheckOutput{
		HasIssues: true,
	}

	w.writeJSON(co)
}

func TestPrintArchivedText_CallsFprint(t *testing.T) {
	PrintArchivedText([]issue.ArchivedActionInfo{}, []string{})
}

func TestPrintStaleText_CallsFprint(t *testing.T) {
	PrintStaleText([]actioninfo.StaleActionInfo{})
}

func TestPrintRuntimeEOLText_CallsFprint(t *testing.T) {
	PrintRuntimeEOLText([]actioninfo.RuntimeEOLActionInfo{})
}

func TestPrintOutdatedText_CallsFprint(t *testing.T) {
	PrintOutdatedText([]actioninfo.OutdatedActionInfo{})
}

func TestFprintStaleText_DeprecatedNoMessage(t *testing.T) {
	var buf bytes.Buffer
	actions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: ""},
	}

	FprintStaleText(&buf, actions)

	output := buf.String()
	if !strings.Contains(output, "DEPRECATED") {
		t.Errorf("expected DEPRECATED in output, got: %s", output)
	}
	if strings.Contains(output, ": ") && strings.Contains(output, "DEPRECATED: ") {
		t.Errorf("should not have deprecation message colon when message empty, got: %s", output)
	}
}

func TestFprintArchivedText_MultipleActionsPerWorkflow(t *testing.T) {
	var buf bytes.Buffer
	actions := []issue.ArchivedActionInfo{
		{Repo: "a/repo", Workflow: "ci.yml", Uses: "a/repo@v1"},
		{Repo: "b/repo", Workflow: "ci.yml", Uses: "b/repo@v2"},
	}
	repos := []string{"a/repo", "b/repo"}

	FprintArchivedText(&buf, actions, repos)

	output := buf.String()
	if !strings.Contains(output, "Found 2 archived GitHub Actions in 2 uses") {
		t.Errorf("expected header with 2 repos and 2 uses, got: %s", output)
	}
	if !strings.Contains(output, "a/repo@v1") {
		t.Errorf("expected first action in output, got: %s", output)
	}
	if !strings.Contains(output, "b/repo@v2") {
		t.Errorf("expected second action in output, got: %s", output)
	}
}
