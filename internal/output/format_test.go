package output

import (
	"bytes"
	"encoding/json"
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
