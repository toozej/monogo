# App Configuration (`app.yaml`)

Each app under `apps/<app>` is described by `apps/<app>/app.yaml`. Its values
drive the per-app config that gomplate renders from `templates/app/*.tmpl` and
`templates/common/*.tmpl` (the Dockerfiles, `.goreleaser.yml`,
`docker-compose.yml`, and `.air.toml`), as well as the `make` build targets and
the devcontainer build. Run `make app-generate APP=<app>` after editing
`app.yaml` to regenerate the app's files.

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Project/app name used by GoReleaser and image labels. |
| `binary` | yes | Output binary name. |
| `path` | yes | App source directory, e.g. `apps/monogo`. Drives the scoped Docker `COPY` and the `go vet`/`go test` package set. |
| `mainPath` | yes | Path to the `main` package to build. |
| `description` | yes | Long description used in image labels and metadata. |
| `shortDescription` | yes | Short description. |
| `goImage` | yes | Builder image for the Docker build stages, e.g. `golang:1.26-trixie`. |
| `distrolessImage` | yes | Distroless runtime image for `Dockerfile.distroless`, e.g. `gcr.io/distroless/static-debian13:nonroot`. |
| `distrolessOnly` | no (default `false`) | Builds only the distroless release image. That image receives both the normal (`latest`, version) and distroless (`distroless`, version-distroless) tags. The generated default Dockerfile also uses the distroless runtime. Use this for apps that require CA certificates or timezone data. |
| `cgoEnabled` | no (default `false`) | Toggles `CGO_ENABLED`. See [CGO apps](#cgo-apps). |
| `runtimeImage` | no | Overrides the runtime base in the non-distroless `Dockerfile`. Defaults to `scratch`, or `debian:trixie-slim` when `cgoEnabled` is `true`. |
| `port` | no | When set, `EXPOSE <port>` is emitted in the generated Dockerfiles. |
| `composeUser` | no | User or UID/GID used to run the Compose service. Useful for nonroot CLI containers that write to bind mounts. |
| `composeRestart` | no (default `unless-stopped`) | Docker Compose restart policy. Set to `no` for one-shot CLI apps. |
| `composeEnvironment` | no | Environment entries rendered into the app's Compose service. |
| `composeVolumes` | no | Volume mounts rendered into the app's Compose service. |

## CGO apps

By default apps build with `CGO_ENABLED=0`, producing a static binary that runs
on `scratch`. Set `cgoEnabled: true` to build with cgo instead. The value
trickles down to every place a binary is built:

- `make local-build` (via `APP_CGO_ENABLED` in the `Makefile`),
- both Dockerfile templates (`templates/app/Dockerfile*.tmpl`),
- the GoReleaser build env (`templates/app/.goreleaser.yml.tmpl`),
- the devcontainer build (`.devcontainer/Dockerfile`).

Because a cgo binary is dynamically linked against glibc, enabling it also adjusts
the runtime and release configuration so the result still runs and builds:

- **Non-distroless runtime:** `scratch` → `debian:trixie-slim`, a minimal
  Debian-based glibc runtime. Override with `runtimeImage`.
- **Distroless runtime:** the `static-*` base is swapped for the glibc `base-*`
  variant, e.g. `gcr.io/distroless/static-debian13:nonroot` →
  `gcr.io/distroless/base-debian13:nonroot`. A custom non-`static-*`
  `distrolessImage` is left unchanged.
- **GoReleaser platforms:** builds are trimmed to the native `linux/amd64`
  platform (cross-compiling cgo needs target C toolchains), the macOS
  `universal_binaries` step is dropped, and each `dockers_v2` image is pinned to
  `platforms: [linux/amd64]`.

## Distroless-only apps

Set `distrolessOnly: true` when an app requires runtime data that `scratch`
does not provide, such as CA certificates or timezone data. GoReleaser emits a
single distroless image for these apps and applies both tag families to the same
digest: `latest`/plain version tags and `distroless`/distroless-version tags.
The weekly refresh and signature-verification workflows continue to verify both
stable aliases without building a duplicate scratch image.

### Implementation notes

The cgo/runtime logic is computed in the gomplate templates
(`templates/app/Dockerfile*.tmpl` and `templates/app/.goreleaser.yml.tmpl`), with
parallel parsing in the `Makefile` (`APP_CGO_ENABLED`) and
`.devcontainer/Dockerfile`. Keep those four in sync when changing the schema.
