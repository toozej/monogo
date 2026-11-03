# Castor v2 compatibility review

This review covers PR 64 and Castor v2.0.0 through v2.5.0.
The app uses the external Castor CLI. It does not import Castor as a Go module.
The Go dependency updates in PR 64 do not select the Castor version.
The runtime image in `app.yaml` selects that version.

## Release notes and diffs

| Release | Changes relevant to this app | Result |
| --- | --- | --- |
| [v2.0.0](https://github.com/stupside/castor/releases/tag/v2.0.0), [diff](https://github.com/stupside/castor/compare/v1.9.0...v2.0.0) | Castor separates its API, media, and scraping services. The CLI can run them in process. Scan adds device IDs. Dry-run prints ranked streams with optional unknown bitrate and last-resort markers. Configuration moves to `cast` and `servers`. | Keep the CLI integration. Parse the new output. Pass discovered IDs through the web interface. Remove the dry-run placeholder device. Migrate existing user configuration. |
| [v2.1.0](https://github.com/stupside/castor/releases/tag/v2.1.0), [diff](https://github.com/stupside/castor/compare/v2.0.0...v2.1.0) | Codec selection favors copying video over copying audio. Telemetry and npm distribution change. | Castor owns these behaviors. No additional adapter change is required. |
| [v2.2.0](https://github.com/stupside/castor/releases/tag/v2.2.0), [diff](https://github.com/stupside/castor/compare/v2.1.0...v2.2.0) | Media handling accepts HLS MIME aliases. Stop and media cleanup change. npm packages include docs. | Keep process lifetime ownership in the adapter. No CLI format change is required. |
| [v2.3.0](https://github.com/stupside/castor/releases/tag/v2.3.0), [diff](https://github.com/stupside/castor/compare/v2.2.0...v2.3.0) | npm metadata changes. | No adapter change is required. |
| [v2.4.0](https://github.com/stupside/castor/releases/tag/v2.4.0), [diff](https://github.com/stupside/castor/compare/v2.3.0...v2.4.0) | Playback adds a start position, link adaptation, seek support, and pause handling. Ranking considers audio language preferences. | Preserve Castor's ranked order instead of selecting the largest bitrate. Existing CLI invocations remain valid. |
| [v2.5.0](https://github.com/stupside/castor/releases/tag/v2.5.0), [diff](https://github.com/stupside/castor/compare/v2.4.0...v2.5.0) | Castor reads user and working-directory configuration. An explicit missing config file fails. Required-section validation changes. Interactive casting can start without a configured device. | Default `CASTOR_CONFIG` to empty. Retain explicit paths when users supply them. Pin app metadata to v2.5.0 so generation retains the image update. |

See the [migration guide](https://github.com/stupside/castor/blob/v2.5.0/migrations/1.x-to-2.0.md)
for configuration moves. The app README describes configuration and runtime requirements.

## Validation

The adapter tests cover v2 scan output, Roku discovery, device IDs, and target overrides.
The stream tests cover unknown bitrate, last-resort markers, and ranked order.
The HTTP tests cover ID forwarding and Roku selection.

Set `CASTOR_TEST_BINARY` to an absolute path to the Castor v2.5.0 release binary.
Run `make test APP=go-castctl` to include the CLI contract test.
That test runs the actual binary against a local API with protobuf responses.
It also checks target selection with duplicate device names and conflicting config targets.
It does not require Chromium or a physical TV.

The starter and app race tests pass with `GOFLAGS=-mod=readonly`.
This flag avoids the workspace's ignored vendor directory, which contains older PR dependencies.
The starter GoReleaser snapshot build passes.
The app snapshot release builds binaries, archives, Linux packages, and a Homebrew cask.
That snapshot skips before hooks, Docker, signing, and SBOM generation.
The test target builds the browser WebAssembly client before the snapshot.

The Castor v2.5.0 image passes checks for the CLI, Chromium, ffmpeg, and ffprobe.
Its v2 browser environment keys select Chromium and disable the sandbox.
These runtime checks also pass as UID 65532.

Physical TV playback and browser extraction require a network and devices available to the runtime.
The CLI contract test does not validate those operations.
