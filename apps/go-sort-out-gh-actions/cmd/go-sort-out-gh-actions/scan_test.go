package cmd

import (
	"path/filepath"
	"testing"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/securityscan"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func TestScanCommand(t *testing.T) {
	cmd := newScanCmd()
	if cmd.Name() != "scan" {
		t.Errorf("Name() = %q, want scan", cmd.Name())
	}
	if cmd.Flags().Lookup("min-severity") == nil {
		t.Error("expected --min-severity flag")
	}
	if cmd.Args == nil || cmd.Run == nil {
		t.Error("expected scan command Args and Run to be set")
	}
}

func TestParseSeverity(t *testing.T) {
	for _, severity := range []securityscan.Severity{
		securityscan.SeverityLow,
		securityscan.SeverityMedium,
		securityscan.SeverityHigh,
		securityscan.SeverityCritical,
	} {
		got, err := parseSeverity(string(severity))
		if err != nil || got != severity {
			t.Errorf("parseSeverity(%q) = %q, %v", severity, got, err)
		}
	}
	if _, err := parseSeverity("urgent"); err == nil {
		t.Error("parseSeverity() accepted invalid severity")
	}
}

func TestFilterFindings(t *testing.T) {
	findings := []securityscan.Finding{
		{RuleID: "low", Severity: securityscan.SeverityLow},
		{RuleID: "high", Severity: securityscan.SeverityHigh},
		{RuleID: "critical", Severity: securityscan.SeverityCritical},
	}
	filtered := filterFindings(findings, securityscan.SeverityHigh)
	if len(filtered) != 2 || filtered[0].RuleID != "high" || filtered[1].RuleID != "critical" {
		t.Errorf("filterFindings() = %#v, want high and critical", filtered)
	}
}

func TestScanWorkflowPaths(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	workflowPath := writeWorkflowFile(t, workflowDir, "security.yml", `name: Security
on: pull_request_target
permissions: {}
jobs: {}
`)

	findings, err := scanWorkflowPaths(securityscan.NewScanner(), workflow.NewParser(), tmpDir, "", "")
	if err != nil {
		t.Fatalf("scanWorkflowPaths() error = %v", err)
	}
	if len(findings) != 1 || findings[0].Path != workflowPath || findings[0].RuleID != "GHA001" {
		t.Errorf("scanWorkflowPaths() = %#v, want pull_request_target finding", findings)
	}
}
