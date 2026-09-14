# gocicle

Gocicle schedules container jobs across Linux runners. PostgreSQL stores the execution queue and configuration. The control plane serves a Go WebAssembly interface, a JSON API, and live events. Runners use outbound HTTPS connections and a local Docker or Podman socket.

For a new installation, follow [Set up a new instance](docs/setup.md). The guide covers database and key creation, HTTPS, forge OAuth registration, the first administrator, and runner access.

## Build and test

```sh
make local APP=gocicle
make test APP=gocicle
make local-build APP=gocicle
make release-test APP=gocicle
make demo APP=gocicle
apps/gocicle/tests/acceptance.sh
GOCICLE_TEST_BROWSER=1 apps/gocicle/tests/acceptance.sh
apps/gocicle/tests/deployment.sh
```

`make local APP=gocicle` runs the repository's dependency updates, vendoring, checks, tests, native build, WebAssembly build, and release build. It can change the root module dependencies. The narrower targets run individual checks. The local binary is `out/gocicle`.

`make demo APP=gocicle` builds the app and runs [demo.sh](demo.sh). The offline demo validates job YAML, creates a temporary encryption key, and translates supported GitHub Actions and Jenkins examples. It also checks rejection of Jenkins hashed cron. The demo does not need PostgreSQL, a runtime, or provider credentials. It removes all temporary output on exit. Use the acceptance script to test job execution.

The acceptance script creates an isolated PostgreSQL container. It tests Docker execution and tests Podman when the executable is available. It removes its containers and temporary directories when it exits. The optional browser test installs pinned Playwright and Chromium in its temporary directory. It requires Node.js and Chromium's system libraries. It tests onboarding, job edits, schedule previews, and live events through the actual API. Unit tests use provider fixtures. Live OAuth tests require registered provider applications and user consent.

The deployment test checks the nginx and Caddy examples with temporary certificates and isolated containers. It verifies HTTPS, forwarded protocol headers, and incremental event delivery. The server tests also check shutdown with an active stream.

Gocicle builds and publishes only distroless container images. `make docker-build APP=gocicle` and the development Compose example use `Dockerfile.distroless`. Releases use `Dockerfile.goreleaser.distroless`. Both normal and distroless release tags refer to the same image. The app does not generate `Dockerfile` or `Dockerfile.goreleaser`.

The weekly image refresh discovers released apps from their tags. Gocicle enters that workflow after its first `apps/gocicle/vX.Y.Z` release tag.

## Control plane installation

Use the [installation procedure](docs/setup.md#prepare-the-control-plane) before starting the service. Run `gocicle migrate` explicitly. The control plane does not create or update its schema at startup.

Complete the [one-time OAuth setup](docs/setup.md#register-login-providers) for each forge that users will use for sign-in. The guide includes GitHub, GitLab, Codeberg, Forgejo, SourceHut, Tangled, and Generic Git. Each registered provider has its own callback URL. Tangled instead publishes an AT Protocol client metadata document. Generic Git provides repository access only.

Run `gocicle bootstrap` immediately before the [first administrator sign-in](docs/setup.md#create-the-first-administrator). Its invitation expires after one hour. Later users need administrator approval or an invitation. Repository credentials and runner grants require separate setup, even after OAuth sign-in succeeds.

## Projects and jobs

Create a project with an HTTPS or SSH Git URL and a branch. Grant its owner access to at least one runner. In the import screen, enter that runner's ID and select **Read repository YAML**. The runner reads `.gocicle.yaml` in a temporary checkout. Review the preview. Supply explicit credential mappings and current versions for conflicting jobs. Import the jobs. Enable each validated job from its editor.

You can also paste YAML into the import screen or use the CLI:

```sh
export GOCICLE_API_URL=https://gocicle.example.com
export GOCICLE_TOKEN=REPLACE_ME
gocicle jobs validate .gocicle.yaml
gocicle jobs import PROJECT_ID .gocicle.yaml --mapping mapping.json --key import-2026-09-11
gocicle jobs list PROJECT_ID
gocicle jobs get JOB_ID
gocicle jobs run JOB_ID
gocicle jobs cancel RUN_ID
gocicle jobs export PROJECT_ID > exported.gocicle.yaml
```

Example mapping file:

```json
{
  "credentials": {"repository-write": "CREDENTIAL_ID"},
  "versions": {"update-dependencies": 3}
}
```

Omit `versions` for new jobs. Map every credential reference, including references that already contain a destination credential ID. Imported jobs remain disabled. UI edits update PostgreSQL. They do not commit repository YAML. See [the portable example](examples/.gocicle.yaml).

A job has exactly one Git or local source. It uses exactly one image or Dockerfile. A Dockerfile and its build context must stay within the project source. The Dockerfile must also stay within its build context. Build contexts reject symlinks and special files. They exclude `.git` entries.

Commands use an absolute executable path and an argument array. To execute shell text, set the executable to `/bin/sh` or another explicit shell. Environment values and named secret references occupy separate maps.

```yaml
environment:
  MODE: maintenance
secrets:
  API_TOKEN: CREDENTIAL_ID
container:
  dockerfile: Dockerfile.jobs
  buildContext: .
```

Cron expressions have five fields. The timezone defaults to UTC. Jobs can be paused without disabling manual runs. Defaults are a 30-minute timeout, two CPUs, 2 GiB of memory, and 256 PIDs. The maximum timeout is 24 hours. One run can be active per job. Overlapping occurrences appear as skipped runs. A downtime interval appears as a missed history entry. The scheduler advances to the next future occurrence without replaying work.

## Credentials and preferences

Create a credential through `POST /api/v1/secrets`. Responses return metadata and IDs. They never return credential values.

Environment credential:

```json
{"name":"API token","kind":"environment","value":{"value":"REPLACE_ME"}}
```

HTTPS Git credential:

```json
{
  "name":"Repository write",
  "kind":"https",
  "value":{
    "repository":"https://github.com/example/project.git",
    "username":"example",
    "password":"REPLACE_ME"
  }
}
```

An SSH credential uses kind `ssh`. Its value has `repository`, `privateKey`, and `knownHosts` fields. Obtain host keys through a trusted channel. Gocicle requires strict host-key verification. HTTPS and SSH credentials must match the assigned repository URL exactly.

The credential owner grants project use through `PUT /api/v1/secrets/CREDENTIAL_ID/grants/PROJECT_ID` with `{"allow":true}`. Use `false` to revoke the grant. Revocation blocks future assignments and requests cancellation of running work at the next heartbeat. Operators cannot retrieve credential values.

Jobs inherit their project owner's preferences. Project overrides take precedence. Manual and scheduled runs resolve preferences in the same way. Omitted fields inherit. Explicit `false` disables a boolean preference. An explicit empty `notifications` array disables inherited destinations.

```json
{
  "commitName":"Maintenance Bot",
  "commitEmail":"maintenance@example.com",
  "notifyFailure":true,
  "notifyRecovery":true,
  "notifySuccess":false,
  "notifications":[]
}
```

Commit identity belongs to project preferences. It does not change the owner's login identity. Notification destinations support Gotify, Slack, Telegram, Discord, Pushover, and Pushbullet. Store transport credentials as a `notification` secret. A destination uses `name`, `secretID`, and `config` with `type` and an optional `endpoint`. Gotify endpoints must use HTTPS and appear in `GOCICLE_NOTIFICATION_HOSTS` as exact host names, including nonstandard ports.

Notifications use a database outbox. Delivery attempts use bounded backoff and stop after eight attempts. Delivery failure does not change the run result. Failures and recovery notifications are enabled by default when destinations are configured.

## Runner installation

Install host Git, OpenSSH, and a container runtime. Run the runner as a dedicated Linux account. The generated app image serves the control plane. Install runners as host services so workspace paths refer to the runtime host.

The administrator creates an enrollment token:

```text
POST /api/v1/admin/enrollments
{"name":"runner-one","labels":["linux","maintenance"],"approvedRoots":["/srv/gocicle-jobs"]}
```

Enroll the host once:

```sh
export GOCICLE_API_URL=https://gocicle.example.com
export GOCICLE_STATE_DIR=/var/lib/gocicle-runner
gocicle runner --enroll REPLACE_ME
```

The runner stores its credential in `credential.json` with mode `0600`. A state-directory lock prevents two runner processes from sharing that file. Grant a project owner access with `PUT /api/v1/admin/runners/RUNNER_ID/grants/USER_ID` and `{"allow":true}`. Use a separate state directory and enrollment for each host.

Runners send heartbeats every 15 seconds. Leases last 60 seconds. A runner stops work when it cannot maintain its lease. The control plane records unresolved execution as lost. It does not retry a run that may have started. A user must request another run explicitly.

The runner persists ordered events in a bounded spool. It retries delivery after temporary failures. On restart, it removes labeled orphan containers and temporary checkouts. It also removes recorded run-built images. Docker and Podman base-image caches remain available. Run-built images use no-cache builds and are removed after execution.

Git metadata, SSH keys, and askpass files remain outside the mounted checkout. The runner disables Git hooks and inherited Git configuration. Successful jobs can create a commit and push to the original source branch. Clean worktrees do not create commits. Pushes use normal fast-forward rules. A rejected push produces `push_failure`. Gocicle does not merge, rebase, or force push. Pushes to the same configured repository URL and branch are serialized.

Containers run with the runner account's UID and GID. Images must support that identity. Job containers receive no runtime socket or runner home mount. Rootless Podman uses `keep-id` and private SELinux relabeling (`Z`) for source mounts. Configure subordinate UID and GID ranges before enabling the Podman service. See [the Podman service documentation](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html).

For local jobs, use this source shape:

```yaml
source:
  local:
    runnerID: RUNNER_ID
    hostPath: /srv/gocicle-jobs/project
    containerPath: /workspace
    readOnly: false
```

The runner resolves symlinks and checks the approved roots. Local writes persist on the host. Local jobs cannot push changes. Gocicle serializes local assignments on each runner to protect overlapping paths. This is more restrictive than serializing only overlapping writable mounts.

## API and administration

Swagger documentation is available at `/swagger/`. All JSON endpoints use `/api/v1`. Lists return at most 100 records. Use `offset` for the next page. Send `If-Match: VERSION` for updates and deletions. Conflicting versions return HTTP 409. Run creation and imports require `Idempotency-Key`.

| API area | Operations |
|---|---|
| `/projects` | Create and list accessible projects. Rename or delete projects by ID |
| `/projects/{id}/jobs` | List, import, and export project jobs |
| `/jobs/{id}` | Read, revise, or delete a job |
| `/jobs/{id}/runs` | Start a manual run |
| `/runs/{id}` | Read the result, cancel, or inspect logs, metrics, and events |
| `/projects/{id}/inspections` | Request repository YAML through a runner |
| `/preferences` | Update the authenticated user's preferences |
| `/projects/{id}/preferences` | Update project overrides |
| `/projects/{id}/memberships/{user}` | Set owner, operator, or viewer access |
| `/connections`, `/ssh-keys`, `/notifications` | Manage credential-backed settings |
| `/tokens` | Create tokens and list their metadata. Revoke a token by ID |
| `/admin/users`, `/admin/runners`, `/admin/providers` | Manage instance access and provider configuration |
| `/admin/configuration` | Export or import configuration without secret values |
| `/admin/audit` | Read administrative audit events |

Viewers can read project settings and run data. Operators can edit jobs and control runs. Owners can also manage project membership, preferences, and credential grants. Administrators manage instance settings, user approval, provider instances, and runners. API tokens also need the relevant `read`, `write`, or `admin` scope. Administrator status does not bypass a token's scope.

Browser changes require a secure session cookie, a same-origin request, and `X-CSRF-Token`. The browser obtains the token from the secure CSRF cookie. Automation sends `Authorization: Bearer TOKEN`. Create scoped tokens through `POST /api/v1/tokens` with `scopes` and `expiresAt`. Tokens expire within one year.

Rename a project through `PUT /projects/{id}` with `{"name":"new-name"}` and its current version. Use job imports to change the source. A source change must include every existing job and its conflict version. This keeps YAML exports consistent with the stored job revisions.

Create disabled users through `POST /admin/users` with a `name`. Disable users before deletion. Users with retained resource references cannot be deleted. Provider updates use `POST /admin/providers` with the existing `id` and `version`. Create a new provider instance to change its type or origin. Instances with linked identities or connections cannot be deleted.

Provider updates also accept `PUT /admin/providers/{id}` with `If-Match`. Use the same configuration fields as the create operation.

Use `gocicle api METHOD /api/v1/PATH --data request.json --revision VERSION --key KEY` for operations without a dedicated CLI subcommand. The browser's settings editor accepts the same JSON fields. For a connection, `config` contains `providerID`. For an SSH key record, it contains public display metadata such as `publicKey` and `fingerprint`.

Configuration imports require explicit credential mappings. They create disabled users and disabled jobs. Existing IDs cause a transaction conflict. After import, an administrator can bind a disabled user's first identity through `/admin/users/{id}/identity` with `providerID` and `subject`. The administrator must then approve that user. Additional identities require a login initiated from an authenticated session. Credential owners must grant imported projects access before their jobs can be enabled.

Logs have a 10 MiB limit per run. The UI retains at most 1 MiB of a live log in browser memory. The API provides the retained log in pages. Metrics include available CPU, memory, and network counters. Missing runtime counters remain unavailable. The control plane retains finished runs, logs, and metrics for 30 days by default.

## Translation

```sh
gocicle translate gha .github/workflows/update.yaml \
  --source https://github.com/example/project.git --branch main \
  --labels '{"ubuntu-latest":["linux","maintenance"]}' \
  --credentials '{"WRITE_TOKEN":"CREDENTIAL_ID"}' --output translated

gocicle translate jenkins Jenkinsfile \
  --source https://github.com/example/project.git \
  --labels '{"jenkins":["linux"]}' --output translated
```

The translator writes `.gocicle.yaml` and executable scripts under `.gocicle/scripts`. Add these files to the repository before running translated jobs. Existing output files are not overwritten. `--image` supplies an explicit missing image mapping.

Supported GHA inputs include schedules, explicit checkout, literal environment values, mapped secrets, a container image, and sequential shell steps. Supported Jenkins inputs include one Declarative Docker agent, literal cron, literal environment values, and sequential shell stages. The translator reports unsupported fields and code with file locations. Matrices, job dependencies, reusable actions, services, dynamic Groovy, plugins, and hashed Jenkins cron remain unsupported. `--draft` writes paused jobs with diagnostics. Incomplete conversion still returns a nonzero exit status.

## Operations

Use the example [control plane unit](deploy/gocicle.service), [Docker runner unit](deploy/gocicle-runner.service), or [rootless Podman user unit](deploy/gocicle-runner-podman.service). Enroll runners before enabling their units. For rootless Podman, enable `podman.socket` with `systemctl --user`. Enable user lingering if the runner must continue after logout. Systemd shutdown sends SIGTERM. The runner stops its container and removes its temporary workspace before it exits.

The [nginx example](deploy/nginx.conf) and [Caddy example](deploy/Caddyfile) proxy to `127.0.0.1:8080`. Replace their host names and certificate paths. Set the same HTTPS origin in `GOCICLE_PUBLIC_URL` and the provider callback registration. Set `GOCICLE_TRUSTED_PROXIES` to the proxy's exact CIDR ranges. The server ignores forwarded identity and scheme headers from other addresses. Disable proxy buffering for server-sent events. Keep streaming timeouts above five minutes and polling timeouts above 25 seconds.

The [development Compose example](deploy/compose.dev.yaml) keeps PostgreSQL data in a named volume. Set `GOCICLE_POSTGRES_PASSWORD`, `GOCICLE_PUBLIC_URL`, and `GOCICLE_KEY_FILE` before starting it. Apply migrations with `docker compose -f apps/gocicle/deploy/compose.dev.yaml run --rm gocicle migrate`. The control plane does not change the schema at startup. Supply an HTTPS proxy for browser login.

Back up PostgreSQL with `pg_dump`. Back up the external encryption key file separately. A database backup without its key versions cannot restore encrypted credentials. Stop scheduling before a coordinated restoration. Restore the database and matching keys, apply migrations, then start the control plane and runners. Expired leases prevent recovered old work from being reassigned automatically.

To rotate keys, add a new 32-byte base64 key to the key file and change `active` to its version name. Keep old versions in `keys`. Run `gocicle rotate-keys`. Restart the control plane with the updated key file. Retain old keys until backups and pending OAuth states no longer require them. API tokens, runner credentials, and session tokens are stored as hashes.
