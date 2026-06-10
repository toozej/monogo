# Major Version Tag Management

These two workflows manage the floating major version tags (e.g., `v1`) so that consumers can pin to a major version and automatically receive minor and patch updates.

## Workflows

### `update-major-version.yaml` — Automatic

Triggers on every non-prerelease GitHub release publication. Parses the major version from the release tag and force-moves the corresponding major version tag.

**Example:** Publishing release `v1.4.0` automatically moves the `v1` tag to point at `v1.4.0`.

### `rollback-major-version.yaml` — Manual

Triggered via the Actions UI (workflow dispatch). Allows a maintainer to move a major version tag to any existing semver tag.

**Inputs:**

| Input | Description | Example |
|-------|-------------|---------|
| `target` | The semver tag to roll back to | `v1.3.0` |
| `major_version` | The major version tag to move | `v1` |

## Safety guards

### Automatic workflow (`update-major-version.yaml`)

1. **Prerelease skip** — the job does not run for prereleases, so tags like `v2.0.0-rc.1` won't move the `v2` tag.
2. **Tag prefix check** — the job only runs when the release tag starts with `v`.
3. **Strict semver validation** — the parse step rejects tags that don't match `vX.Y.Z` exactly, failing the workflow for malformed tags.

### Rollback workflow (`rollback-major-version.yaml`)

1. **Input validation** — `major_version` must match `vN` and `target` must match `vX.Y.Z`.
2. **Tag existence check** — the workflow verifies the target tag exists in the repository before attempting to move the major tag.

### GoReleaser interaction (`release.yaml`)

Force-pushing a major version tag (e.g., `v1`) is a tag push event. Without protection, it would match `release.yaml`'s trigger and start a GoReleaser build on a non-semver tag. Two guards prevent this:

1. **Tag glob filter** — `release.yaml` uses `v[0-9]*.*.*` instead of `v*`, which requires at least two dots in the tag. Tags like `v1` don't match.
2. **Semver validation step** — the first step in the `goreleaser` job rejects any tag that doesn't match `vX.Y.Z` exactly, as a belt-and-suspenders check.

## Examples

### Normal release

1. Push a tag: `git tag v1.4.0 && git push origin v1.4.0`
2. The `release.yaml` workflow runs GoReleaser, which creates the GitHub release.
3. The `update-major-version.yaml` workflow triggers on the release, parses `v1`, and moves the `v1` tag to `v1.4.0`.
4. The `v1` tag push does **not** re-trigger `release.yaml` because `v1` doesn't match `v[0-9]*.*.*`.

### Rollback

1. A problem is discovered in `v1.4.0`.
2. Navigate to **Actions > Rollback Major Version > Run workflow**.
3. Enter `v1.3.2` as the **target** and `v1` as the **major_version**.
4. The `v1` tag is moved back to `v1.3.2`.
5. The `v1` tag push does **not** trigger `release.yaml` for the same reason as above.
