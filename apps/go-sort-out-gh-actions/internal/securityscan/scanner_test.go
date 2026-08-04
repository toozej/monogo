package securityscan

import (
	"strings"
	"testing"
)

func TestScanner_DefaultRules(t *testing.T) {
	content := []byte(`name: Unsafe workflow
on: pull_request_target
permissions: write-all
jobs:
  test:
    runs-on: [self-hosted, linux]
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - run: echo "${{ github.event.issue.title }}"
      - uses: actions/setup-go@v5
`)

	findings, err := NewScanner().ScanContent(".github/workflows/unsafe.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}

	wantRules := map[string]bool{
		rulePullRequestTarget:     false,
		ruleUntrustedCheckout:     false,
		ruleUntrustedExpression:   false,
		rulePrivilegedPermissions: false,
		ruleSelfHostedRunner:      false,
		ruleUnpinnedAction:        false,
	}
	for _, finding := range findings {
		if _, ok := wantRules[finding.RuleID]; ok {
			wantRules[finding.RuleID] = true
		}
		if finding.Path != ".github/workflows/unsafe.yml" {
			t.Errorf("finding path = %q, want workflow path", finding.Path)
		}
		if finding.Line == 0 {
			t.Errorf("finding %s did not include a source line", finding.RuleID)
		}
		if finding.Remediation == "" {
			t.Errorf("finding %s did not include remediation", finding.RuleID)
		}
	}
	for ruleID, found := range wantRules {
		if !found {
			t.Errorf("expected finding for %s; got %#v", ruleID, findings)
		}
	}
}

func TestScanner_SafeWorkflowHasNoFindings(t *testing.T) {
	content := []byte(`name: CI
on: pull_request
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4
      - run: go test ./...
`)

	findings, err := NewScanner().ScanContent("ci.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("ScanContent() findings = %#v, want none", findings)
	}
}

func TestScanner_RecognizesTriggerForms(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
	}{
		{name: "scalar", workflow: "name: Security\non: pull_request_target\njobs: {}\n"},
		{name: "sequence", workflow: "name: Security\non: [push, pull_request_target]\njobs: {}\n"},
		{name: "mapping", workflow: "name: Security\non:\n  pull_request_target:\n    types: [opened]\njobs: {}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := NewScanner().ScanContent("ci.yml", []byte(tt.workflow))
			if err != nil {
				t.Fatalf("ScanContent() error = %v", err)
			}
			if !containsRule(findings, rulePullRequestTarget) {
				t.Errorf("expected %s finding, got %#v", rulePullRequestTarget, findings)
			}
		})
	}
}

func TestScanner_DetectsPullRequestMergeCheckoutAndCache(t *testing.T) {
	content := []byte(`name: Benchmark
on: pull_request_target
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: refs/pull/${{ github.event.pull_request.number }}/merge
      - uses: actions/cache@v4
        with:
          path: ~/.cache
          key: shared-cache
`)

	findings, err := NewScanner().ScanContent("benchmark.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	for _, ruleID := range []string{ruleUntrustedCheckout, rulePrivilegedCache} {
		if !containsRule(findings, ruleID) {
			t.Errorf("expected %s finding, got %#v", ruleID, findings)
		}
	}
}

func TestScanner_DetectsJobLevelWritePermissions(t *testing.T) {
	content := []byte(`name: Privileged
on: workflow_run
jobs:
  publish:
    permissions:
      id-token: write
    runs-on: ubuntu-latest
    steps: []
`)

	findings, err := NewScanner().ScanContent("publish.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	if !containsRule(findings, rulePrivilegedPermissions) {
		t.Errorf("expected job-level permission finding, got %#v", findings)
	}
}

func TestScanner_DetectsAdditionalCommonWorkflowRisks(t *testing.T) {
	content := []byte(`name: Dangerous deployment
on: workflow_run
env:
  DEPLOY_TOKEN: literal-production-token
jobs:
  receive-artifact:
    runs-on: ubuntu-latest
    env:
      CLOUD_TOKEN: ${{ secrets.CLOUD_TOKEN }}
    steps:
      - uses: actions/download-artifact@v4
      - uses: actions/upload-artifact@v4
        with:
          name: workspace
          path: .
      - run: echo "${{ github.event.inputs.environment }}" >> "$GITHUB_ENV"
      - run: eval "$USER_INPUT"
      - run: docker run -v /var/run/docker.sock:/var/run/docker.sock tool
  call-deploy:
    uses: example/deploy/.github/workflows/deploy.yml@v1
    secrets: inherit
`)

	findings, err := NewScanner().ScanContent("dangerous.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	for _, ruleID := range []string{
		ruleHardcodedSecret,
		ruleBroadSecretExposure,
		ruleArtifactDownload,
		ruleArtifactUpload,
		ruleEnvironmentFile,
		ruleEval,
		ruleDockerSocket,
		ruleMissingPermissions,
	} {
		if !containsRule(findings, ruleID) {
			t.Errorf("expected %s finding, got %#v", ruleID, findings)
		}
	}
}

func TestScanner_DetectsEntireSecretsContext(t *testing.T) {
	content := []byte(`name: Secret dump
on: push
permissions: {}
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - env:
          SECRETS: ${{ toJson(secrets) }}
        run: deploy
`)

	findings, err := NewScanner().ScanContent("secrets.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	if !containsRule(findings, ruleBroadSecretExposure) {
		t.Errorf("expected %s finding, got %#v", ruleBroadSecretExposure, findings)
	}
}

func TestScanner_ExpandedUntrustedInputsOnlyApplyToRun(t *testing.T) {
	content := []byte(`name: Inputs
on: workflow_dispatch
permissions: {}
jobs:
  safe:
    runs-on: ubuntu-latest
    steps:
      - uses: example/action@${{ inputs.version }}
        with:
          name: ${{ github.event.inputs.name }}
      - run: echo "${{ inputs.name }}"
`)

	findings, err := NewScanner().ScanContent("inputs.yml", content)
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	if countRule(findings, ruleUntrustedExpression) != 1 {
		t.Errorf("expected exactly one %s finding, got %#v", ruleUntrustedExpression, findings)
	}
}

func TestScanner_PermissionsAtWorkflowOrJobLevelAvoidsMissingPermissionsFinding(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
	}{
		{
			name:     "workflow permissions",
			workflow: "name: CI\npermissions: {}\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps: []\n",
		},
		{
			name:     "job permissions",
			workflow: "name: CI\njobs:\n  test:\n    permissions:\n      contents: read\n    runs-on: ubuntu-latest\n    steps: []\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := NewScanner().ScanContent("ci.yml", []byte(tt.workflow))
			if err != nil {
				t.Fatalf("ScanContent() error = %v", err)
			}
			if containsRule(findings, ruleMissingPermissions) {
				t.Errorf("did not expect %s finding, got %#v", ruleMissingPermissions, findings)
			}
		})
	}
}

func TestScanner_CustomRules(t *testing.T) {
	scanner := NewScanner(testRule{})
	findings, err := scanner.ScanContent("ci.yml", []byte("name: CI\n"))
	if err != nil {
		t.Fatalf("ScanContent() error = %v", err)
	}
	if len(findings) != 1 || findings[0].RuleID != "TEST001" {
		t.Errorf("ScanContent() findings = %#v, want custom rule finding", findings)
	}
}

func TestScanner_InvalidYAML(t *testing.T) {
	_, err := NewScanner().ScanContent("broken.yml", []byte("jobs: ["))
	if err == nil || !strings.Contains(err.Error(), "parse workflow file broken.yml") {
		t.Errorf("ScanContent() error = %v, want wrapped parse error", err)
	}
}

type testRule struct{}

func (testRule) ID() string { return "TEST001" }

func (testRule) Scan(doc Document) []Finding {
	return []Finding{{RuleID: "TEST001", Severity: SeverityLow, Path: doc.Path, Message: "test"}}
}

func containsRule(findings []Finding, ruleID string) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return true
		}
	}
	return false
}

func countRule(findings []Finding, ruleID string) int {
	count := 0
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			count++
		}
	}
	return count
}
