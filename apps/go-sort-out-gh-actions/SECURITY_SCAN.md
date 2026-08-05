# Workflow security scan coverage

The `scan` command performs offline static analysis of GitHub Actions workflow
YAML. It reports a stable rule ID, severity, source location, explanation, and
remediation for each finding.

The rule set below deduplicates the recurring issues and recommendations from
the security references at the end of this document. Several articles describe
the same underlying trust-boundary failure using different incidents or names;
those cases intentionally map to one rule or one closely related group of
rules.

## Deduplicated coverage

| Risk | Rules | What is detected | Primary remediation |
| --- | --- | --- | --- |
| Privileged execution of untrusted code | `GHA001`, `GHA002`, `GHA016`, `GHA017` | Dangerous `pull_request_target`, `workflow_run`, and `issue_comment` triggers, including pull request refs checked out by `actions/checkout`, `gh pr checkout`, or Git commands | Run untrusted code in an unprivileged `pull_request` workflow. Keep privileged work separate, authorize actors, and bind any approved checkout to an immutable reviewed commit SHA. |
| Script and environment injection | `GHA003`, `GHA012`, `GHA013` | Event data, branch refs, dispatch inputs, matrix values, and step/job outputs interpolated into `run`; untrusted data written to `GITHUB_ENV` or `GITHUB_PATH`; shell `eval` | Pass untrusted values through a step-level environment variable or typed action input, quote them, validate expected formats, and use action outputs instead of environment files where possible. |
| Excessive token privilege | `GHA004`, `GHA015` | `write-all`, write grants on privileged triggers, and jobs left on repository-default `GITHUB_TOKEN` permissions | Set workflow-level `permissions: {}` and grant only the required permissions to individual jobs. |
| Mutable or untrusted action dependencies | `GHA006` | Actions and reusable workflows not pinned to a full commit SHA | Pin the full SHA, verify that it belongs to the intended repository, and automate reviewed updates with a release cooldown. |
| Shared-cache poisoning | `GHA007` | Explicit caches in privileged, release, deployment, or publishing jobs, including setup-action cache options | Do not share caches across trust boundaries or use caches for release inputs. Rebuild privileged outputs from trusted sources. |
| Credential storage and exposure | `GHA008`, `GHA009`, `GHA018`, `GHA019`, `GHA021` | Literal credentials, the complete secrets context, `secrets: inherit`, workflow/job-wide secret environments, persisted checkout credentials, likely long-lived secret references, and secrets interpolated into shell commands | Prefer OIDC or trusted publishing, pass only named secrets at step scope, use protected environment secrets, set `persist-credentials: false`, avoid shell interpolation, and rotate any exposed credential. |
| Artifact poisoning or leakage | `GHA010`, `GHA011` | Artifacts consumed across a `workflow_run` boundary and uploads of the workspace, `.git`, or environment files | Treat upstream artifacts as untrusted, validate their source and integrity, isolate downloads in `runner.temp`, never execute untrusted contents, and upload only explicit build outputs. |
| Untrusted runner or host access | `GHA005`, `GHA014` | Self-hosted runners reachable from untrusted or privileged event chains and Docker socket exposure | Prefer GitHub-hosted runners. Otherwise use restricted runner groups, ephemeral isolated hosts, minimal network access, and no Docker socket mount. |
| Ungated deployment credentials | `GHA020` | Deployment, release, or publishing jobs with secrets or write permission but no GitHub environment | Target a protected environment, require reviewers, and bind deployment credentials to that environment. |

Severity is attached to each finding. Some rules vary severity by context; for
example, a write grant becomes critical when paired with a privileged trigger.

## Complementary controls

An offline workflow-file scan cannot establish repository settings, runtime
behavior, dependency provenance, or external infrastructure state. The source
recommendations below are therefore accounted for as complementary controls:

- Require approval for all external contributors; protect branches with review,
  status-check, signed-commit, and `CODEOWNERS` requirements.
- Restrict allowed actions and reusable workflows, enforce SHA pinning at the
  organization level, prefer reviewed GitHub or verified-creator actions, and
  check pinned SHAs for impostor commits. Scan action code and transitive
  dependencies for vulnerabilities.
- Use Dependabot or Renovate with a minimum release age while allowing security
  fixes through promptly. Action maintainers should use immutable releases.
- Configure protected environments with required reviewers. The scanner can
  see an `environment` reference but cannot inspect its protection rules.
- Prefer OIDC to stored cloud credentials, and constrain each provider trust
  policy to the expected organization, repository, ref or environment, and
  audience so another workflow cannot mint an equivalent token.
- Restrict runner egress, isolate self-hosted runner groups by trust level,
  monitor runner behavior, and destroy ephemeral runners after every job.
- Treat AI assistants as untrusted interpreters: isolate event content, grant
  the minimum tools, permissions, secrets, and network access, and require
  human approval for consequential actions. The general injection, permission,
  and secret-scope rules apply, but arbitrary AI tooling cannot be identified
  reliably from YAML alone.
- Enable workflow static analysis in pull requests and scheduled scans, require
  high/critical findings to pass before merge, and maintain an incident response
  procedure for credential revocation, log/artifact review, and compromise
  containment.
- Minimize artifact and log retention and never rely on log masking to prevent
  a compromised workflow from using or transforming a secret.

## Sources

- [OpenSSF: Mitigating Attack Vectors in GitHub Workflows](https://openssf.org/blog/2024/08/12/mitigating-attack-vectors-in-github-workflows/)
- [Datadog Security Labs: The case for GitHub Actions security after recent supply chain attacks](https://securitylabs.datadoghq.com/articles/case-for-github-actions-security/)
- [2AM Security: Threat Modeling GitHub](https://2amsecurity.substack.com/p/threat-modeling-github-how-vulnerable)
- [GitHub: Disrupting supply chain attacks on npm and GitHub Actions](https://github.blog/security/supply-chain-security/disrupting-supply-chain-attacks-on-npm-and-github-actions/)
- [OWASP GitHub Actions Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/GitHub_Actions_Security_Cheat_Sheet.html)
- [Wiz: Hardening GitHub Actions](https://www.wiz.io/blog/github-actions-security-guide)
