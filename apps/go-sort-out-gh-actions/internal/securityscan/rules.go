package securityscan

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	rulePullRequestTarget     = "GHA001"
	ruleUntrustedCheckout     = "GHA002"
	ruleUntrustedExpression   = "GHA003"
	rulePrivilegedPermissions = "GHA004"
	ruleSelfHostedRunner      = "GHA005"
	ruleUnpinnedAction        = "GHA006"
	rulePrivilegedCache       = "GHA007"
	ruleHardcodedSecret       = "GHA008"
	ruleBroadSecretExposure   = "GHA009"
	ruleArtifactDownload      = "GHA010"
	ruleArtifactUpload        = "GHA011"
	ruleEnvironmentFile       = "GHA012"
	ruleEval                  = "GHA013"
	ruleDockerSocket          = "GHA014"
	ruleMissingPermissions    = "GHA015"
	checkoutAction            = "actions/checkout@"
	pullRequestTargetEvent    = "pull_request_target"
	workflowRunEvent          = "workflow_run"
	pullRequestEvent          = "pull_request"
)

type pullRequestTargetRule struct{}

func (pullRequestTargetRule) ID() string { return rulePullRequestTarget }

func (pullRequestTargetRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, pullRequestTargetEvent) {
		return nil
	}
	return []Finding{newFinding(rulePullRequestTarget, SeverityHigh, doc.Path, on.Line,
		"workflow uses pull_request_target, which runs in the base repository security context", "Prefer pull_request. If pull_request_target is required, do not check out, execute, or interpolate pull-request-controlled content.")}
}

type untrustedCheckoutRule struct{}

func (untrustedCheckoutRule) ID() string { return ruleUntrustedCheckout }

func (untrustedCheckoutRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, pullRequestTargetEvent) && !hasTrigger(on, workflowRunEvent) {
		return nil
	}

	var findings []Finding
	for _, job := range workflowJobs(doc.Root) {
		steps := mappingValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			uses := scalarValue(mappingValue(step, "uses"))
			ref := scalarValue(mappingValue(mappingValue(step, "with"), "ref"))
			if strings.HasPrefix(uses, checkoutAction) && isUntrustedCheckoutRef(ref) {
				findings = append(findings, newFinding(ruleUntrustedCheckout, SeverityCritical, doc.Path, step.Line,
					"privileged workflow checks out pull-request-controlled code", "Do not check out pull-request head refs in pull_request_target or workflow_run. Run untrusted code only in an unprivileged pull_request workflow."))
			}
		}
	}
	return findings
}

type untrustedExpressionRule struct{}

func (untrustedExpressionRule) ID() string { return ruleUntrustedExpression }

var untrustedExpression = regexp.MustCompile(`(?:github\.event\.(?:issue\.(?:title|body)|comment\.body|discussion\.(?:title|body)|review\.body|pull_request\.(?:title|body|head\.(?:ref|label|repo))|inputs\.[A-Za-z0-9_-]+)|(?:github\.)?inputs\.[A-Za-z0-9_-]+)`)

func (untrustedExpressionRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitMappingValues(doc.Root, "run", func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode || !untrustedExpression.MatchString(node.Value) {
			return
		}
		findings = append(findings, newFinding(ruleUntrustedExpression, SeverityHigh, doc.Path, node.Line,
			"workflow interpolates untrusted GitHub event data into a command or configuration value", "Pass untrusted values through environment variables and quote them in scripts; never interpolate them directly into run commands."))
	})
	return findings
}

type privilegedPermissionsRule struct{}

func (privilegedPermissionsRule) ID() string { return rulePrivilegedPermissions }

func (privilegedPermissionsRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	privilegedTrigger := hasTrigger(on, pullRequestTargetEvent) || hasTrigger(on, workflowRunEvent)
	var findings []Finding
	visitMappingValues(doc.Root, "permissions", func(permissions *yaml.Node) {
		if permissions.Kind == yaml.ScalarNode && strings.EqualFold(permissions.Value, "write-all") {
			findings = append(findings, newFinding(rulePrivilegedPermissions, SeverityHigh, doc.Path, permissions.Line,
				"workflow grants write-all permissions to GITHUB_TOKEN", "Use the least-privilege permissions map and grant write access only to the job that requires it."))
			return
		}
		if privilegedTrigger && hasWritePermission(permissions) {
			findings = append(findings, newFinding(rulePrivilegedPermissions, SeverityCritical, doc.Path, permissions.Line,
				"privileged trigger has write permissions", "Remove write permissions from pull_request_target or workflow_run jobs unless they cannot process untrusted input or code."))
		}
	})
	return findings
}

type selfHostedRunnerRule struct{}

func (selfHostedRunnerRule) ID() string { return ruleSelfHostedRunner }

func (selfHostedRunnerRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, pullRequestEvent) && !hasTrigger(on, pullRequestTargetEvent) {
		return nil
	}
	var findings []Finding
	jobs, _ := workflowJobEntries(doc.Root)
	for name, job := range jobs {
		runsOn := mappingValue(job, "runs-on")
		if hasSelfHostedLabel(runsOn) {
			findings = append(findings, newFinding(ruleSelfHostedRunner, SeverityHigh, doc.Path, runsOn.Line,
				"job "+name+" runs on a self-hosted runner for pull-request events", "Do not run untrusted pull-request code on self-hosted runners. Use GitHub-hosted runners or isolate and ephemerally provision the runner."))
		}
	}
	return findings
}

type unpinnedActionRule struct{}

func (unpinnedActionRule) ID() string { return ruleUnpinnedAction }

var actionReference = regexp.MustCompile(`^[^\s/@]+/[^\s/@]+(?:/[^\s@]+)?@([^\s]+)$`)
var commitSHA = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

func (unpinnedActionRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitNodes(doc.Root, func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode {
			return
		}
		match := actionReference.FindStringSubmatch(node.Value)
		if len(match) != 2 || commitSHA.MatchString(match[1]) {
			return
		}
		findings = append(findings, newFinding(ruleUnpinnedAction, SeverityLow, doc.Path, node.Line,
			"action reference is not pinned to a full commit SHA: "+node.Value, "Pin third-party actions to a full-length commit SHA and retain the version in a comment for readability."))
	})
	return findings
}

type privilegedCacheRule struct{}

func (privilegedCacheRule) ID() string { return rulePrivilegedCache }

func (privilegedCacheRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, pullRequestTargetEvent) {
		return nil
	}

	var findings []Finding
	visitMappingValues(doc.Root, "uses", func(node *yaml.Node) {
		if !strings.HasPrefix(scalarValue(node), "actions/cache@") {
			return
		}
		findings = append(findings, newFinding(rulePrivilegedCache, SeverityHigh, doc.Path, node.Line,
			"pull_request_target workflow uses actions/cache, which can cross the fork-to-base trust boundary", "Do not save or restore shared caches from pull_request_target jobs that process pull-request code. Isolate cache keys and use an unprivileged pull_request workflow."))
	})
	return findings
}

type hardcodedSecretRule struct{}

func (hardcodedSecretRule) ID() string { return ruleHardcodedSecret }

var secretKey = regexp.MustCompile(`(?i)(?:secret|token|password|api[_-]?key|access[_-]?key|private[_-]?key)`)
var knownSecretValue = regexp.MustCompile(`\b(?:github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,})\b`)

func (hardcodedSecretRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitNodes(doc.Root, func(node *yaml.Node) {
		if node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if value.Kind != yaml.ScalarNode || strings.Contains(value.Value, "${{") || value.Value == "" {
				continue
			}
			if secretKey.MatchString(key.Value) || knownSecretValue.MatchString(value.Value) {
				findings = append(findings, newFinding(ruleHardcodedSecret, SeverityHigh, doc.Path, value.Line,
					"workflow contains a literal value that appears to be a credential", "Store credentials in GitHub Secrets or obtain short-lived credentials through OIDC; remove the exposed value and rotate it."))
			}
		}
	})
	return findings
}

type broadSecretExposureRule struct{}

func (broadSecretExposureRule) ID() string { return ruleBroadSecretExposure }

func (broadSecretExposureRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitNodes(doc.Root, func(node *yaml.Node) {
		if node.Kind == yaml.ScalarNode && strings.Contains(strings.ToLower(node.Value), "tojson(secrets)") {
			findings = append(findings, newFinding(ruleBroadSecretExposure, SeverityHigh, doc.Path, node.Line,
				"workflow exposes the entire secrets context to a runner", "Pass only the named secret required by a single step; never serialize the complete secrets context."))
		}
		if node.Kind == yaml.MappingNode && strings.EqualFold(scalarValue(mappingValue(node, "secrets")), "inherit") {
			secrets := mappingValue(node, "secrets")
			findings = append(findings, newFinding(ruleBroadSecretExposure, SeverityMedium, doc.Path, secrets.Line,
				"reusable workflow inherits every available secret", "Declare and pass only the secrets required by the reusable workflow."))
		}
	})

	root := rootMapping(doc.Root)
	if exposesSecrets(mappingValue(root, "env")) {
		findings = append(findings, newFinding(ruleBroadSecretExposure, SeverityMedium, doc.Path, mappingValue(root, "env").Line,
			"workflow-level environment exposes secrets to every job", "Move secret environment variables to the individual step that needs them."))
	}
	jobs, _ := workflowJobEntries(doc.Root)
	for name, job := range jobs {
		if exposesSecrets(mappingValue(job, "env")) {
			findings = append(findings, newFinding(ruleBroadSecretExposure, SeverityMedium, doc.Path, mappingValue(job, "env").Line,
				"job "+name+" exposes secrets to every step", "Move secret environment variables to the individual step that needs them."))
		}
	}
	return findings
}

type artifactDownloadRule struct{}

func (artifactDownloadRule) ID() string { return ruleArtifactDownload }

func (artifactDownloadRule) Scan(doc Document) []Finding {
	if !hasTrigger(workflowTriggerNode(doc.Root), workflowRunEvent) {
		return nil
	}
	var findings []Finding
	for _, job := range workflowJobs(doc.Root) {
		for _, step := range jobSteps(job) {
			if !strings.HasPrefix(scalarValue(mappingValue(step, "uses")), "actions/download-artifact@") || mappingValue(mappingValue(step, "with"), "path") != nil {
				continue
			}
			findings = append(findings, newFinding(ruleArtifactDownload, SeverityHigh, doc.Path, step.Line,
				"workflow_run downloads an artifact into the workspace without an isolated path", "Treat workflow_run artifacts as untrusted: download into runner.temp, verify integrity, and do not execute artifacts from pull-request workflows."))
		}
	}
	return findings
}

type artifactUploadRule struct{}

func (artifactUploadRule) ID() string { return ruleArtifactUpload }

func (artifactUploadRule) Scan(doc Document) []Finding {
	var findings []Finding
	for _, job := range workflowJobs(doc.Root) {
		for _, step := range jobSteps(job) {
			if !strings.HasPrefix(scalarValue(mappingValue(step, "uses")), "actions/upload-artifact@") {
				continue
			}
			path := strings.TrimSpace(scalarValue(mappingValue(mappingValue(step, "with"), "path")))
			if path != "." && path != "./" && !strings.Contains(path, "github.workspace") {
				continue
			}
			findings = append(findings, newFinding(ruleArtifactUpload, SeverityMedium, doc.Path, step.Line,
				"artifact upload includes the entire workspace and can expose credentials or configuration", "Upload only the required build output and explicitly exclude secret-bearing files such as .env and configuration files."))
		}
	}
	return findings
}

type environmentFileRule struct{}

func (environmentFileRule) ID() string { return ruleEnvironmentFile }

func (environmentFileRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitMappingValues(doc.Root, "run", func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode || !untrustedExpression.MatchString(node.Value) || (!strings.Contains(node.Value, "GITHUB_ENV") && !strings.Contains(node.Value, "GITHUB_PATH")) {
			return
		}
		findings = append(findings, newFinding(ruleEnvironmentFile, SeverityHigh, doc.Path, node.Line,
			"workflow writes attacker-controlled data to GITHUB_ENV or GITHUB_PATH", "Do not write untrusted data to environment files; validate it in a trusted action or pass it as a scoped, quoted environment variable."))
	})
	return findings
}

type evalRule struct{}

func (evalRule) ID() string { return ruleEval }

var evalCommand = regexp.MustCompile(`(?m)(?:^|[;&|]\s*)eval\s`)

func (evalRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitMappingValues(doc.Root, "run", func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode || !evalCommand.MatchString(node.Value) {
			return
		}
		findings = append(findings, newFinding(ruleEval, SeverityHigh, doc.Path, node.Line,
			"workflow uses eval, which executes dynamically constructed shell code", "Avoid eval. Pass inputs through step environment variables and invoke commands with fixed argument handling."))
	})
	return findings
}

type dockerSocketRule struct{}

func (dockerSocketRule) ID() string { return ruleDockerSocket }

func (dockerSocketRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitNodes(doc.Root, func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode || !strings.Contains(node.Value, "/var/run/docker.sock") {
			return
		}
		findings = append(findings, newFinding(ruleDockerSocket, SeverityHigh, doc.Path, node.Line,
			"workflow exposes the Docker socket, which grants control of the runner host", "Do not mount or pass the Docker socket to workflow containers; use an isolated builder or rootless alternative."))
	})
	return findings
}

type missingPermissionsRule struct{}

func (missingPermissionsRule) ID() string { return ruleMissingPermissions }

func (missingPermissionsRule) Scan(doc Document) []Finding {
	root := rootMapping(doc.Root)
	if mappingValue(root, "permissions") != nil {
		return nil
	}
	jobs, _ := workflowJobEntries(doc.Root)
	for _, job := range jobs {
		if mappingValue(job, "permissions") != nil {
			return nil
		}
	}
	return []Finding{newFinding(ruleMissingPermissions, SeverityMedium, doc.Path, root.Line,
		"workflow does not explicitly restrict GITHUB_TOKEN permissions", "Set workflow permissions: {} as a baseline, then grant only the required read or write permissions to individual jobs.")}
}

func newFinding(ruleID string, severity Severity, path string, line int, message, remediation string) Finding {
	return Finding{RuleID: ruleID, Severity: severity, Path: path, Line: line, Message: message, Remediation: remediation}
}

func rootMapping(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0]
	}
	return root
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func scalarValue(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func workflowTriggerNode(root *yaml.Node) *yaml.Node {
	return mappingValue(rootMapping(root), "on")
}

func hasTrigger(node *yaml.Node, trigger string) bool {
	if node == nil {
		return false
	}
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Value == trigger
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if hasTrigger(child, trigger) {
				return true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == trigger {
				return true
			}
		}
	}
	return false
}

func workflowJobs(root *yaml.Node) []*yaml.Node {
	_, jobs := workflowJobEntries(root)
	return jobs
}

func workflowJobEntries(root *yaml.Node) (map[string]*yaml.Node, []*yaml.Node) {
	jobs := mappingValue(rootMapping(root), "jobs")
	entries := make(map[string]*yaml.Node)
	var values []*yaml.Node
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return entries, values
	}
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		entries[jobs.Content[i].Value] = jobs.Content[i+1]
		values = append(values, jobs.Content[i+1])
	}
	return entries, values
}

func isUntrustedCheckoutRef(ref string) bool {
	return strings.Contains(ref, "github.event.pull_request.head.") ||
		strings.Contains(ref, "github.event.workflow_run.head_") ||
		(strings.Contains(ref, "refs/pull/") && strings.Contains(ref, "github.event.pull_request."))
}

func hasWritePermission(permissions *yaml.Node) bool {
	if permissions == nil {
		return false
	}
	if permissions.Kind == yaml.ScalarNode {
		return strings.EqualFold(permissions.Value, "write-all")
	}
	if permissions.Kind != yaml.MappingNode {
		return false
	}
	for i := 1; i < len(permissions.Content); i += 2 {
		if strings.EqualFold(permissions.Content[i].Value, "write") {
			return true
		}
	}
	return false
}

func hasSelfHostedLabel(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode {
		return strings.Contains(strings.ToLower(node.Value), "self-hosted")
	}
	for _, child := range node.Content {
		if hasSelfHostedLabel(child) {
			return true
		}
	}
	return false
}

func exposesSecrets(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode {
		return strings.Contains(strings.ToLower(node.Value), "secrets.")
	}
	for _, child := range node.Content {
		if exposesSecrets(child) {
			return true
		}
	}
	return false
}

func jobSteps(job *yaml.Node) []*yaml.Node {
	steps := mappingValue(job, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return nil
	}
	return steps.Content
}

func visitNodes(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for _, child := range node.Content {
		visitNodes(child, visit)
	}
}

func visitMappingValues(node *yaml.Node, key string, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				visit(node.Content[i+1])
			}
			visitMappingValues(node.Content[i+1], key, visit)
		}
		return
	}
	for _, child := range node.Content {
		visitMappingValues(child, key, visit)
	}
}
