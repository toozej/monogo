package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
)

// TestCommentPreservation verifies that ReplaceUsesLine preserves inline
// comments when updating action references. It applies a series of
// replacements to the "after" workflow file and then diffs it against the
// "before" version, asserting the expected transformations.
func TestCommentPreservation(t *testing.T) {
	// Resolve paths relative to this test file's location.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to determine test file path")
	}
	e2eRoot := filepath.Dir(thisFile)
	beforeDir := filepath.Join(e2eRoot, "before", ".github", "workflows")
	afterDir := filepath.Join(e2eRoot, "after", ".github", "workflows")

	// Read the before content for later diff comparison.
	beforeFile := filepath.Join(beforeDir, "ci.yaml")
	afterFile := filepath.Join(afterDir, "ci.yaml")
	beforeContent, err := os.ReadFile(beforeFile)
	if err != nil {
		t.Fatalf("Failed to read before/ci.yaml: %v", err)
	}

	// Start with a fresh copy of before → after content.
	afterContent := make([]byte, len(beforeContent))
	copy(afterContent, beforeContent)

	// Apply replacements simulating what outdated --update --pin would do.
	replacements := []struct {
		oldUse string
		newUse string
		desc   string
	}{
		// 1. Semver tag with nosemgrep comment → SHA pin, nosemgrep preserved
		{
			oldUse: "gitleaks/gitleaks-action@v2",
			newUse: "gitleaks/gitleaks-action@abc123def # v2.3.9",
			desc:   "SHA pin with nosemgrep comment preserved",
		},
		// 2. Subpath action with nosemgrep comment → SHA pin, nosemgrep preserved
		{
			oldUse: "github/codeql-action/init@v4.35.2",
			newUse: "github/codeql-action/init@sha111 # v4.35.4",
			desc:   "Subpath SHA pin with nosemgrep comment preserved",
		},
		// 3. Another subpath action with nosemgrep comment
		{
			oldUse: "github/codeql-action/autobuild@v4.35.2",
			newUse: "github/codeql-action/autobuild@sha222 # v4.35.4",
			desc:   "Subpath SHA pin with nosemgrep comment preserved (autobuild)",
		},
		// 4. Yet another subpath action with nosemgrep comment
		{
			oldUse: "github/codeql-action/analyze@v4.35.2",
			newUse: "github/codeql-action/analyze@sha333 # v4.35.4",
			desc:   "Subpath SHA pin with nosemgrep comment preserved (analyze)",
		},
		// 5. master ref with nosemgrep comment → SHA pin, nosemgrep preserved
		{
			oldUse: "aquasecurity/trivy-action@master",
			newUse: "aquasecurity/trivy-action@sha444 # v0.30.0",
			desc:   "SHA pin master ref with nosemgrep comment preserved",
		},
		// 6. snyk master ref with nosemgrep comment
		{
			oldUse: "snyk/actions/setup@master",
			newUse: "snyk/actions/setup@sha555 # v0.4.0",
			desc:   "SHA pin snyk master ref with nosemgrep comment preserved",
		},
		// 7. Plain semver tag (no comment) → SHA pin
		{
			oldUse: "actions/checkout@v3",
			newUse: "actions/checkout@sha666 # v4.1.2",
			desc:   "SHA pin plain semver tag, no existing comment",
		},
		// 8. Another plain semver tag (no comment) → SHA pin
		{
			oldUse: "actions/setup-go@v5",
			newUse: "actions/setup-go@sha777 # v5.1.0",
			desc:   "SHA pin plain semver tag, no existing comment (setup-go)",
		},
		// 9. Re-pin: old SHA + semver comment → new SHA + updated semver comment
		{
			oldUse: "actions/checkout@oldSHAabc123",
			newUse: "actions/checkout@newSHA888 # v4.1.2",
			desc:   "Re-pin replaces old semver comment with new one",
		},
		// 10. Re-pin: old SHA + semver comment + nosemgrep → updated all
		{
			oldUse: "actions/setup-go@oldSHAdef456",
			newUse: "actions/setup-go@newSHA999 # v5.1.0",
			desc:   "Re-pin replaces old semver comment, preserves nosemgrep",
		},
	}

	for _, r := range replacements {
		afterContent = actioninfo.ReplaceUsesLine(afterContent, r.oldUse, r.newUse)
	}

	// Write the after content so it can be inspected and diffed.
	if err := os.WriteFile(afterFile, afterContent, 0644); err != nil {
		t.Fatalf("Failed to write after/ci.yaml: %v", err)
	}

	afterStr := string(afterContent)

	// Assertion 1: nosemgrep comments must be preserved (not removed, not doubled).
	assertCommentPreserved(t, afterStr, "# nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha.third-party-action-not-pinned-to-commit-sha", "gitleaks nosemgrep")
	assertCommentPreserved(t, afterStr, "# nosemgrep: custom-rule", "snyk/custom nosemgrep")

	// Assertion 2: new SHA references are present.
	assertContains(t, afterStr, "gitleaks/gitleaks-action@abc123def", "gitleaks SHA")
	assertContains(t, afterStr, "github/codeql-action/init@sha111", "codeql init SHA")
	assertContains(t, afterStr, "github/codeql-action/autobuild@sha222", "codeql autobuild SHA")
	assertContains(t, afterStr, "github/codeql-action/analyze@sha333", "codeql analyze SHA")
	assertContains(t, afterStr, "aquasecurity/trivy-action@sha444", "trivy SHA")
	assertContains(t, afterStr, "snyk/actions/setup@sha555", "snyk SHA")
	assertContains(t, afterStr, "actions/checkout@sha666", "checkout SHA (plain)")
	assertContains(t, afterStr, "actions/setup-go@sha777", "setup-go SHA (plain)")
	assertContains(t, afterStr, "actions/checkout@newSHA888", "checkout SHA (re-pin)")
	assertContains(t, afterStr, "actions/setup-go@newSHA999", "setup-go SHA (re-pin)")

	// Assertion 3: new semver comments are present.
	assertContains(t, afterStr, "# v2.3.9", "gitleaks semver comment")
	assertContains(t, afterStr, "# v4.35.4", "codeql semver comment")
	assertContains(t, afterStr, "# v4.1.2", "checkout semver comment")

	// Assertion 4: old references must be gone.
	assertNotContains(t, afterStr, "gitleaks/gitleaks-action@v2", "old gitleaks ref")
	assertNotContains(t, afterStr, "github/codeql-action/init@v4.35.2", "old codeql init ref")
	assertNotContains(t, afterStr, "actions/checkout@v3", "old checkout v3 ref")
	assertNotContains(t, afterStr, "actions/checkout@oldSHAabc123", "old re-pin checkout SHA")
	assertNotContains(t, afterStr, "actions/setup-go@oldSHAdef456", "old re-pin setup-go SHA")

	// Assertion 5: old semver comments from re-pins must be replaced.
	assertNotContains(t, afterStr, "# v3\n", "old re-pin semver comment v3 must be replaced")
	assertNotContains(t, afterStr, "# v5 #", "old re-pin semver comment v5 must be replaced")

	// Assertion 6: unchanged refs must NOT be modified by other replacements.
	// actions/checkout@v6 appears on 3 lines and must survive replacement of
	// actions/checkout@v3 and actions/checkout@oldSHAabc123 without corruption.
	assertContains(t, afterStr, "actions/checkout@v6", "checkout v6 not corrupted")
	// Count occurrences of checkout@v6 — should still be exactly 4
	// (one in each of the gitleaks, codeql, trivy, and snyk jobs).
	checkoutV6Count := countSubstring(afterStr, "actions/checkout@v6")
	if checkoutV6Count != 4 {
		t.Errorf("Expected exactly 4 occurrences of actions/checkout@v6, got %d", checkoutV6Count)
	}

	// Print the diff for visual inspection.
	fmt.Println("\n=== DIFF (before → after) ===")
	printLineDiff(t, string(beforeContent), afterStr)
}

func assertCommentPreserved(t *testing.T, content, comment, label string) {
	t.Helper()
	if !containsString(content, comment) {
		t.Errorf("FAIL [%s]: Expected comment %q to be preserved in output", label, comment)
	}
}

func assertContains(t *testing.T, content, substr, label string) {
	t.Helper()
	if !containsString(content, substr) {
		t.Errorf("FAIL [%s]: Expected %q in output", label, substr)
	}
}

func assertNotContains(t *testing.T, content, substr, label string) {
	t.Helper()
	if containsString(content, substr) {
		t.Errorf("FAIL [%s]: Did NOT expect %q in output", label, substr)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func countSubstring(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}

// printLineDiff prints a simple line-by-line diff between two strings.
func printLineDiff(t *testing.T, before, after string) {
	t.Helper()
	bLines := splitLines(before)
	aLines := splitLines(after)
	maxLen := len(bLines)
	if len(aLines) > maxLen {
		maxLen = len(aLines)
	}
	for i := 0; i < maxLen; i++ {
		bLine := ""
		aLine := ""
		if i < len(bLines) {
			bLine = bLines[i]
		}
		if i < len(aLines) {
			aLine = aLines[i]
		}
		if bLine != aLine {
			fmt.Printf("-%4d: %s\n", i+1, bLine)
			fmt.Printf("+%4d: %s\n", i+1, aLine)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
