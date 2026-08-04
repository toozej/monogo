package securityscan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Scanner applies a collection of workflow-security rules.
type Scanner struct {
	rules []Rule
}

// NewScanner creates a scanner with the built-in rules when no rules are supplied.
// Supplying rules makes it possible for callers to add or replace checks without
// changing scanner orchestration.
func NewScanner(rules ...Rule) *Scanner {
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	return &Scanner{rules: rules}
}

// DefaultRules returns the maintained set of workflow-security checks.
func DefaultRules() []Rule {
	return []Rule{
		pullRequestTargetRule{},
		untrustedCheckoutRule{},
		untrustedExpressionRule{},
		privilegedPermissionsRule{},
		selfHostedRunnerRule{},
		unpinnedActionRule{},
		privilegedCacheRule{},
		hardcodedSecretRule{},
		broadSecretExposureRule{},
		artifactDownloadRule{},
		artifactUploadRule{},
		environmentFileRule{},
		evalRule{},
		dockerSocketRule{},
		missingPermissionsRule{},
		workflowRunRule{},
		issueCommentRule{},
		checkoutCredentialsRule{},
		longLivedCredentialRule{},
		unprotectedDeploymentRule{},
		secretInterpolationRule{},
	}
}

// ScanFile scans one workflow file.
func (s *Scanner) ScanFile(path string) ([]Finding, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open workflow directory for %s: %w", path, err)
	}
	defer func() { _ = root.Close() }()

	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("open workflow file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read workflow file %s: %w", path, err)
	}
	return s.ScanContent(path, content)
}

// ScanContent scans YAML workflow content and records findings with source lines.
func (s *Scanner) ScanContent(path string, content []byte) ([]Finding, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parse workflow file %s: %w", path, err)
	}

	doc := Document{Path: path, Root: &root}
	var findings []Finding
	for _, rule := range s.rules {
		findings = append(findings, rule.Scan(doc)...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].RuleID < findings[j].RuleID
	})
	return findings, nil
}
