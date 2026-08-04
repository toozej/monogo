// Package securityscan detects risky GitHub Actions workflow configurations.
package securityscan

import "gopkg.in/yaml.v3"

// Severity describes the impact of a security finding.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Finding is a security concern found in a workflow file.
type Finding struct {
	RuleID      string   `json:"rule_id"`
	Severity    Severity `json:"severity"`
	Path        string   `json:"path"`
	Line        int      `json:"line"`
	Message     string   `json:"message"`
	Remediation string   `json:"remediation"`
}

// Document is the parsed representation of one GitHub Actions workflow.
type Document struct {
	Path string
	Root *yaml.Node
}

// Rule is a single, independently extensible workflow-security check.
type Rule interface {
	ID() string
	Scan(Document) []Finding
}
