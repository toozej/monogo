# App Configuration (`app.yaml`)

The `apps/<app>/app.yaml` file describes each app under `apps/<app>`. Its values
drive the per-app config that gomplate renders from `templates/app/*.tmpl` and
`templates/common/*.tmpl` (the Dockerfiles, `.goreleaser.yml`,
`docker-compose.yml`, and `.air.toml`), as well as the `make` build targets.
The shared devcontainer can then build or develop any configured app. Run
`make app-generate APP=<app>` after you edit `app.yaml` to regenerate the app's
files.

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Project or app name used by GoReleaser and image labels. |
| `binary` | yes | Output binary name. |
| `path` | yes | App source directory, for example `apps/monogo`. Drives the scoped Docker `COPY` and the set of packages for `go vet` and `go test`. |
| `mainPath` | yes | Path to the `main` package to build. |
| `description` | yes | Long description used in image labels and metadata. |
| `shortDescription` | yes | Short description. |
| `goImage` | yes | Builder image for the Docker build stages, for example `golang:1.27.3-trixie`. |
| `distrolessImage` | yes | Distroless runtime image for `Dockerfile.distroless`, for example `gcr.io/distroless/static-debian13:nonroot`. |
| `distrolessOnly` | no (default `false`) | Builds only the distroless release image. That image receives both the normal (`latest`, version) and distroless (`distroless`, version-distroless) tags. The generated default Dockerfile also uses the distroless runtime. Use this for apps that require CA certificates or timezone data. |
| `dockerPlatforms` | no | Restricts the container images from GoReleaser to the listed OCI platforms, such as `linux/amd64` and `linux/arm64`. Use this when a custom runtime base lacks published images for some default platforms. |
| `cgoEnabled` | no (default `false`) | Toggles `CGO_ENABLED`. See [CGO apps](#cgo-apps). |
| `runtimeImage` | no | Overrides the runtime base in the non-distroless `Dockerfile`. Defaults to `scratch`, or `debian:trixie-slim` when `cgoEnabled` is `true`. |
| `port` | no | When set, the generator emits `EXPOSE <port>` in Dockerfiles. |
| `swaggerEnabled` | no (default `false`) | Generates and serves Swagger 2.0 documentation for an app with a user-facing HTTP API. See [Swagger API documentation](#swagger-api-documentation). |
| `swaggerGeneralInfo` | when `swaggerEnabled` is `true` | Go source file, relative to the app directory, with the general Swag annotations. |
| `wasmMainPath` | no | Main package for a go-app WebAssembly client. Make, Docker, and GoReleaser build it to `apps/<app>/internal/server/web/app.wasm` before they build the native binary so the server can embed it. |
| `writableDirs` | no | Runtime directories created with ownership for the image's nonroot user. This lets empty named volumes inherit usable permissions. |
| `workDir` | no | Runtime work directory for generated Dockerfiles. Set this to a writable directory when an app writes relative paths. |
| `compose.user` | no | User, or UID and GID used to run the Compose service. Useful for nonroot CLI containers that write to bind mounts. |
| `compose.restart` | no (default `unless-stopped`) | Restart policy for Docker Compose. Set to `no` for one-shot CLI apps. |
| `compose.networkMode` | no | Compose network mode, such as `host` for LAN discovery and casting applications. Do not combine host networking with published `ports`. |
| `compose.environment` | no | Environment variables emitted for the app service in `docker-compose.yml`. |
| `compose.envFile` | no | Environment files emitted with Compose's `env_file` setting. |
| `compose.command` | no | Command and arguments emitted for the app service. |
| `compose.stdinOpen` | no | Keeps standard input open for an interactive or stream-processing service. |
| `compose.tty` | no | Allocates a TTY, commonly paired with `stdinOpen` to keep an interactive service active. |
| `compose.ports` | no | Port mappings emitted for the app service, such as `8080:8080`. |
| `compose.volumes` | no | Bind mounts or named-volume mappings emitted for the app service. |
| `compose.namedVolumes` | no | Named volumes declared at the top level of `docker-compose.yml`. |
| `compose.sidecars` | no | Named companion services. Each sidecar supports `image`, `containerName`, `restart`, `environment`, and `ports`. |

The `compose` fields are useful for server apps whose generated development
deployment needs persistent writable paths or a published port. For example:

```yaml
port: 8080
writableDirs:
  - /config
  - /assets
compose:
  environment:
    CONFIG: /config
    DATA: /assets
  envFile:
    - .env
  command:
    - serve
  ports:
    - 8080:8080
  volumes:
    - app-config:/config
    - app-assets:/assets
  namedVolumes:
    - app-config
    - app-assets
```

## Swagger API documentation

Apps with a user-facing HTTP API opt into Swagger generation in `app.yaml`:

```yaml
swaggerEnabled: true
swaggerGeneralInfo: internal/server/server.go
```

Add [Swag declarative comments](https://github.com/swaggo/swag#declarative-comments-format)
to that general-info file and to each API handler. `make app-generate APP=<app>`
then writes the generated Go package, JSON specification, and YAML specification
to `apps/<app>/docs/`. The app must import the generated Go package so the runtime Swagger UI can resolve its specification.

Use `make swagger-generate APP=<app>` to regenerate only the Swagger artifacts.
The command is a no-op for apps that do not opt in. Git tracks generated specifications. The normal generation and pre-commit workflows keep them current.

## CGO apps

By default, apps build with `CGO_ENABLED=0` and produce a static binary that runs on `scratch`.
Set `cgoEnabled: true` to build with CGO instead.
This value affects every build location:

- `make local-build` (via `APP_CGO_ENABLED` in the `Makefile`),
- both Dockerfile templates (`templates/app/Dockerfile*.tmpl`),
- the GoReleaser build env (`templates/app/.goreleaser.yml.tmpl`).

CGO links the binary dynamically against glibc.
The CGO option also adjusts runtime and release configuration so the result still runs and builds:

**Non-distroless runtime:** `scratch` becomes `debian:trixie-slim`, a minimal Debian-based glibc runtime.
Override this value with `runtimeImage`.

**Distroless runtime:** The generator replaces the `static-*` base with the glibc `base-*` variant.
For example, `gcr.io/distroless/static-debian13:nonroot` becomes `gcr.io/distroless/base-debian13:nonroot`.
It preserves a custom non-`static-*` `distrolessImage`.

**GoReleaser platforms:** The generator restricts builds to native `linux/amd64` because CGO cross-compilation needs target C toolchains.
It removes the macOS `universal_binaries` step.
It pins each `dockers_v2` image to `platforms: [linux/amd64]`.

## Distroless-only apps

Set `distrolessOnly: true` when an app requires runtime data that `scratch`
does not provide, such as CA certificates or timezone data. GoReleaser emits a
single distroless image for these apps and applies both tag families to the same
digest: `latest`/plain version tags and `distroless`/distroless-version tags.
The weekly refresh and signature-verification workflows continue to verify both
stable aliases without a duplicate build of the scratch image.

### Implementation notes

The gomplate templates compute the CGO and runtime logic
(`templates/app/Dockerfile*.tmpl` and `templates/app/.goreleaser.yml.tmpl`), with
parallel parsing in the `Makefile` (`APP_CGO_ENABLED`). Keep those three in sync
when you change the schema. The devcontainer invokes the Make targets above, so it
does not duplicate individual app build settings.
