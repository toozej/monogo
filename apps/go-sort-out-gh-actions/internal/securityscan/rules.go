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
	ruleWorkflowRun           = "GHA016"
	ruleIssueComment          = "GHA017"
	ruleCheckoutCredentials   = "GHA018"
	ruleLongLivedCredential   = "GHA019"
	ruleUnprotectedDeployment = "GHA020"
	ruleSecretInterpolation   = "GHA021"
	checkoutAction            = "actions/checkout@"
	pullRequestTargetEvent    = "pull_request_target"
	workflowRunEvent          = "workflow_run"
	pullRequestEvent          = "pull_request"
	issueCommentEvent         = "issue_comment"
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
	if !hasPrivilegedTrigger(on) {
		return nil
	}

	var findings []Finding
	for _, job := range workflowJobs(doc.Root) {
		steps := mappingValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			uses := strings.ToLower(scalarValue(mappingValue(step, "uses")))
			ref := scalarValue(mappingValue(mappingValue(step, "with"), "ref"))
			run := scalarValue(mappingValue(step, "run"))
			if (strings.HasPrefix(uses, checkoutAction) && isUntrustedCheckoutRef(ref)) || untrustedShellCheckout.MatchString(run) {
				findings = append(findings, newFinding(ruleUntrustedCheckout, SeverityCritical, doc.Path, step.Line,
					"privileged workflow checks out pull-request-controlled code", "Do not check out pull-request refs in pull_request_target, workflow_run, or issue_comment. Run untrusted code in an unprivileged pull_request workflow, or bind authorized execution to an immutable reviewed commit SHA."))
			}
		}
	}
	return findings
}

type untrustedExpressionRule struct{}

func (untrustedExpressionRule) ID() string { return ruleUntrustedExpression }

var untrustedExpression = regexp.MustCompile(`(?i)(?:github\.(?:event(?:\.[A-Za-z0-9_-]+)*|head_ref|base_ref|ref|ref_name|actor|triggering_actor)|(?:github\.)?inputs\.[A-Za-z0-9_-]+|(?:needs|steps)\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_-]+|matrix\.[A-Za-z0-9_-]+)`)

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
	privilegedTrigger := hasPrivilegedTrigger(on)
	var findings []Finding
	scan := func(permissions *yaml.Node) {
		if permissions == nil {
			return
		}
		if permissions.Kind == yaml.ScalarNode && strings.EqualFold(permissions.Value, "write-all") {
			findings = append(findings, newFinding(rulePrivilegedPermissions, SeverityHigh, doc.Path, permissions.Line,
				"workflow grants write-all permissions to GITHUB_TOKEN", "Use the least-privilege permissions map and grant write access only to the job that requires it."))
		} else if privilegedTrigger && hasWritePermission(permissions) {
			findings = append(findings, newFinding(rulePrivilegedPermissions, SeverityCritical, doc.Path, permissions.Line,
				"privileged trigger has write permissions", "Remove write permissions from pull_request_target, workflow_run, or issue_comment jobs unless they cannot process untrusted input or code."))
		}
	}

	scan(mappingValue(rootMapping(doc.Root), "permissions"))
	for _, job := range workflowJobs(doc.Root) {
		scan(mappingValue(job, "permissions"))
	}
	return findings
}

type selfHostedRunnerRule struct{}

func (selfHostedRunnerRule) ID() string { return ruleSelfHostedRunner }

func (selfHostedRunnerRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, pullRequestEvent) && !hasPrivilegedTrigger(on) {
		return nil
	}
	var findings []Finding
	jobs, _ := workflowJobEntries(doc.Root)
	for name, job := range jobs {
		runsOn := mappingValue(job, "runs-on")
		if hasSelfHostedLabel(runsOn) {
			findings = append(findings, newFinding(ruleSelfHostedRunner, SeverityHigh, doc.Path, runsOn.Line,
				"job "+name+" may run untrusted input or code on a self-hosted runner", "Use GitHub-hosted runners for untrusted events. Otherwise use isolated runner groups, ephemeral runners, and restricted network access with no persistent secrets."))
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
			"action reference is not pinned to a full commit SHA: "+node.Value, "Pin the action to a full commit SHA, verify that the commit belongs to the intended repository, and use Dependabot or Renovate with a release cooldown to keep it updated."))
	})
	return findings
}

type privilegedCacheRule struct{}

func (privilegedCacheRule) ID() string { return rulePrivilegedCache }

func (privilegedCacheRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	var findings []Finding
	jobs, _ := workflowJobEntries(doc.Root)
	for name, job := range jobs {
		if !hasPrivilegedTrigger(on) && !hasReleaseTrigger(on) && !isDeploymentJob(name, job) {
			continue
		}
		for _, step := range jobSteps(job) {
			if !stepUsesCache(step) {
				continue
			}
			findings = append(findings, newFinding(rulePrivilegedCache, SeverityHigh, doc.Path, step.Line,
				"privileged or release workflow uses a shared dependency cache", "Do not save or restore caches in release jobs or jobs that cross trust boundaries. Keep untrusted work in a pull_request workflow and rebuild release inputs from trusted sources."))
		}
	}
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
			valueLower := strings.ToLower(strings.TrimSpace(value.Value))
			permissionValue := valueLower == "read" || valueLower == "write" || valueLower == "none" || valueLower == "true" || valueLower == "false" || valueLower == "null"
			credentialKey := !strings.EqualFold(key.Value, "id-token") && secretKey.MatchString(key.Value)
			if knownSecretValue.MatchString(value.Value) || (credentialKey && !permissionValue) {
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
			uses := strings.ToLower(scalarValue(mappingValue(step, "uses")))
			run := strings.ToLower(scalarValue(mappingValue(step, "run")))
			if !strings.Contains(uses, "download-artifact@") && !strings.Contains(run, "gh run download") {
				continue
			}
			findings = append(findings, newFinding(ruleArtifactDownload, SeverityHigh, doc.Path, step.Line,
				"workflow_run consumes an artifact that may have been produced by untrusted code", "Download into runner.temp, verify the source workflow, repository, branch, conclusion, and artifact integrity, and never execute untrusted artifact contents in the privileged workflow."))
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
			if !isBroadArtifactPath(path) {
				continue
			}
			findings = append(findings, newFinding(ruleArtifactUpload, SeverityMedium, doc.Path, step.Line,
				"artifact upload includes broad or credential-bearing workspace content", "Upload only the required build output and explicitly exclude .git, .env, credentials, configuration, logs, and other secret-bearing files."))
		}
	}
	return findings
}

type environmentFileRule struct{}

func (environmentFileRule) ID() string { return ruleEnvironmentFile }

func (environmentFileRule) Scan(doc Document) []Finding {
	var findings []Finding
	privileged := hasPrivilegedTrigger(workflowTriggerNode(doc.Root))
	for _, job := range workflowJobs(doc.Root) {
		for _, step := range jobSteps(job) {
			run := mappingValue(step, "run")
			if run == nil || run.Kind != yaml.ScalarNode || !writesEnvironmentFile(run.Value) {
				continue
			}
			hasUntrustedData := untrustedExpression.MatchString(run.Value) ||
				nodeMatches(mappingValue(step, "env"), untrustedExpression) ||
				privileged && readsFileIntoCommand(run.Value)
			if !hasUntrustedData {
				continue
			}
			findings = append(findings, newFinding(ruleEnvironmentFile, SeverityHigh, doc.Path, run.Line,
				"workflow writes attacker-controlled data to an environment file", "Do not write untrusted data to GITHUB_ENV or GITHUB_PATH. Validate single-line values and use action outputs or scoped, quoted step environment variables instead."))
		}
	}
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
		if mappingValue(job, "permissions") == nil {
			return []Finding{newFinding(ruleMissingPermissions, SeverityMedium, doc.Path, root.Line,
				"workflow leaves one or more jobs on repository-default GITHUB_TOKEN permissions", "Set workflow permissions: {} as a baseline, then grant only the required read or write permissions to individual jobs.")}
		}
	}
	return nil
}

type workflowRunRule struct{}

func (workflowRunRule) ID() string { return ruleWorkflowRun }

func (workflowRunRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, workflowRunEvent) {
		return nil
	}
	return []Finding{newFinding(ruleWorkflowRun, SeverityMedium, doc.Path, on.Line,
		"workflow uses workflow_run, which crosses from one workflow's trust context into a potentially privileged workflow", "Prefer workflow_call when possible. If workflow_run is required, use explicit minimal permissions and treat all artifacts, outputs, caches, and source refs from the triggering workflow as untrusted.")}
}

type issueCommentRule struct{}

func (issueCommentRule) ID() string { return ruleIssueComment }

func (issueCommentRule) Scan(doc Document) []Finding {
	on := workflowTriggerNode(doc.Root)
	if !hasTrigger(on, issueCommentEvent) {
		return nil
	}
	return []Finding{newFinding(ruleIssueComment, SeverityMedium, doc.Path, on.Line,
		"workflow uses issue_comment, which can be triggered by untrusted users and introduce authorization or checkout TOCTOU flaws", "Authorize the triggering actor, bind approval to an immutable reviewed commit SHA, and prefer an authorized pull_request label event over comment commands.")}
}

type checkoutCredentialsRule struct{}

func (checkoutCredentialsRule) ID() string { return ruleCheckoutCredentials }

func (checkoutCredentialsRule) Scan(doc Document) []Finding {
	var findings []Finding
	for _, job := range workflowJobs(doc.Root) {
		for _, step := range jobSteps(job) {
			uses := strings.ToLower(scalarValue(mappingValue(step, "uses")))
			if !strings.HasPrefix(uses, checkoutAction) {
				continue
			}
			persist := strings.ToLower(strings.TrimSpace(scalarValue(mappingValue(mappingValue(step, "with"), "persist-credentials"))))
			if persist == "false" || persist == "${{ false }}" {
				continue
			}
			findings = append(findings, newFinding(ruleCheckoutCredentials, SeverityMedium, doc.Path, step.Line,
				"actions/checkout persists the workflow credential in the local Git configuration", "Set persist-credentials: false unless a later git command requires the token; grant that operation a narrowly scoped credential only for the step that needs it."))
		}
	}
	return findings
}

type longLivedCredentialRule struct{}

func (longLivedCredentialRule) ID() string { return ruleLongLivedCredential }

var secretReference = regexp.MustCompile(`(?i)secrets\.([A-Za-z0-9_]+)`)
var longLivedSecretName = regexp.MustCompile(`(?i)(?:^|_)(?:PAT|TOKEN|PASSWORD|PRIVATE_KEY|API_KEY|ACCESS_KEY(?:_ID)?|SECRET_ACCESS_KEY|CLIENT_SECRET)(?:$|_)`)

func (longLivedCredentialRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitNodes(doc.Root, func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode {
			return
		}
		for _, match := range secretReference.FindAllStringSubmatch(node.Value, -1) {
			if len(match) != 2 || strings.EqualFold(match[1], "GITHUB_TOKEN") || !longLivedSecretName.MatchString(match[1]) {
				continue
			}
			findings = append(findings, newFinding(ruleLongLivedCredential, SeverityMedium, doc.Path, node.Line,
				"workflow references a secret whose name suggests a long-lived credential: "+match[1], "Replace static credentials with OIDC or trusted publishing where supported. Otherwise scope the credential narrowly, store it as an environment secret, require approval, and rotate it regularly."))
			break
		}
	})
	return findings
}

type unprotectedDeploymentRule struct{}

func (unprotectedDeploymentRule) ID() string { return ruleUnprotectedDeployment }

func (unprotectedDeploymentRule) Scan(doc Document) []Finding {
	root := rootMapping(doc.Root)
	rootPermissions := mappingValue(root, "permissions")
	jobs, _ := workflowJobEntries(doc.Root)
	var findings []Finding
	for name, job := range jobs {
		if !isDeploymentJob(name, job) || mappingValue(job, "uses") != nil || mappingValue(job, "environment") != nil {
			continue
		}
		if !exposesSecrets(job) && !hasWritePermission(rootPermissions) && !hasWritePermission(mappingValue(job, "permissions")) {
			continue
		}
		findings = append(findings, newFinding(ruleUnprotectedDeployment, SeverityMedium, doc.Path, job.Line,
			"deployment or publishing job "+name+" uses credentials or write permissions without a GitHub environment", "Target a protected GitHub environment, require reviewers for critical deployments, and store deployment credentials as environment-level secrets."))
	}
	return findings
}

type secretInterpolationRule struct{}

func (secretInterpolationRule) ID() string { return ruleSecretInterpolation }

var secretInterpolation = regexp.MustCompile(`(?i)\$\{\{[^}\n]*(?:secrets\.[A-Za-z0-9_]+|github\.token)`)

func (secretInterpolationRule) Scan(doc Document) []Finding {
	var findings []Finding
	visitMappingValues(doc.Root, "run", func(node *yaml.Node) {
		if node.Kind != yaml.ScalarNode || !secretInterpolation.MatchString(node.Value) {
			return
		}
		findings = append(findings, newFinding(ruleSecretInterpolation, SeverityHigh, doc.Path, node.Line,
			"workflow interpolates a credential directly into a shell command", "Pass the named secret through this step's env map, quote the environment variable, and ensure the command cannot print it or expose it in process arguments."))
	})
	return findings
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
	ref = strings.ToLower(ref)
	return strings.Contains(ref, "github.event.pull_request.") ||
		strings.Contains(ref, "github.event.issue.") ||
		strings.Contains(ref, "github.event.workflow_run.head_") ||
		strings.Contains(ref, "github.head_ref") ||
		strings.Contains(ref, "refs/pull/")
}

var untrustedShellCheckout = regexp.MustCompile(`(?im)\bgh\s+pr\s+checkout\b|\bgit\s+(?:checkout|switch|fetch)\b[^\n]*(?:github\.(?:event|head_ref)|(?:refs/)?pull/)`)

func hasPrivilegedTrigger(on *yaml.Node) bool {
	return hasTrigger(on, pullRequestTargetEvent) || hasTrigger(on, workflowRunEvent) || hasTrigger(on, issueCommentEvent)
}

func hasReleaseTrigger(on *yaml.Node) bool {
	if hasTrigger(on, "release") || hasTrigger(on, "registry_package") {
		return true
	}
	if on == nil || on.Kind != yaml.MappingNode {
		return false
	}
	push := mappingValue(on, "push")
	return mappingValue(push, "tags") != nil || mappingValue(push, "tags-ignore") != nil
}

func isDeploymentJob(name string, job *yaml.Node) bool {
	label := strings.ToLower(name + " " + scalarValue(mappingValue(job, "name")))
	for _, keyword := range []string{"deploy", "publish", "release"} {
		if strings.Contains(label, keyword) {
			return true
		}
	}
	return false
}

func stepUsesCache(step *yaml.Node) bool {
	uses := strings.ToLower(scalarValue(mappingValue(step, "uses")))
	if strings.HasPrefix(uses, "actions/cache@") || strings.Contains(uses, "/cache@") {
		return true
	}
	with := mappingValue(step, "with")
	for _, key := range []string{"cache", "cache-from", "cache-to"} {
		value := strings.ToLower(strings.TrimSpace(scalarValue(mappingValue(with, key))))
		if value != "" && value != "false" && value != "${{ false }}" {
			return true
		}
	}
	return false
}

func isBroadArtifactPath(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if path == "." || path == "./" || path == "**" || path == "**/*" || strings.Contains(path, "github.workspace") {
		return true
	}
	for _, line := range strings.Split(path, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "!") {
			continue
		}
		if line == ".git" || strings.HasPrefix(line, ".git/") || line == ".env" || strings.HasPrefix(line, ".env.") {
			return true
		}
	}
	return false
}

func writesEnvironmentFile(command string) bool {
	command = strings.ToUpper(command)
	return strings.Contains(command, "GITHUB_ENV") || strings.Contains(command, "GITHUB_PATH") || strings.Contains(command, "::SET-ENV")
}

func readsFileIntoCommand(command string) bool {
	command = strings.ToLower(command)
	return strings.Contains(command, "$(cat ") || strings.Contains(command, "`cat ")
}

func nodeMatches(node *yaml.Node, expression *regexp.Regexp) bool {
	matched := false
	visitNodes(node, func(child *yaml.Node) {
		if child.Kind == yaml.ScalarNode && expression.MatchString(child.Value) {
			matched = true
		}
	})
	return matched
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
