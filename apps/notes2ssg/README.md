# notes2ssg

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/notes2ssg)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

notes2ssg fetches notes from **Simplenote** or **Usememos** and converts them into **Hugo-formatted Markdown** files.

## Features

- **Multi-backend support**: Fetch notes from Simplenote (via `sncli`) or Usememos (via REST API).
- **Tag-based filtering**: Download only notes matching a specific tag.
- **Multi-SSG output**: Generates Markdown for Hugo (YAML front matter: `title`, `author`, `date`, `url`, `summary`, `categories`, `unlisted`) and go-vite (template front matter: `slug`, `title`, `subtitle`, `date`).
- **Continuous note splitting**: Automatically split a single note into multiple notes, one per line.
- **Unlisted tags**: Mark notes as `unlisted` based on configured tags.
- **Title substitutions**: Map titles keywords to custom summary text.
- **Safe repeat exports**: Only writes changed files. It removes stale files that its output manifest lists.
- **Push notifications**: Optional Gotify integration for success and failure alerts.
- **Retry logic**: Simplenote backend retries with exponential backoff and jitter.

## Layout

```
apps/notes2ssg/
  app.yaml                 # build/release metadata consumed by Make + GoReleaser
  main.go                  # entrypoint that defers to cmd/notes2ssg
  cmd/notes2ssg/           # cobra CLI (root command + subcommands)
  internal/
    backend/               # Backend interface (Note struct)
    config/                # app-specific config struct backed by pkg/config
    converter/             # orchestrates fetch -> parse -> format -> write
    hugo/                  # Hugo front matter and Markdown formatter
    note/                  # note parser and continuous-note splitter
    simplenote/            # Simplenote backend (sncli wrapper)
    usememos/              # Usememos backend (HTTP API client)
    gotify/                # Gotify notification client
  demo.sh                  # smoke-test script run via `make APP=notes2ssg demo`
```

## Usage

```sh
# Export notes repeatedly. The default polling cycle is 3600 seconds.
notes2ssg

# Run one export pass and exit.
POLLING_CYCLE=0 notes2ssg

# Enable debug logging
notes2ssg --debug

# Print version/build metadata
notes2ssg version

# Print man page
notes2ssg man
```

## Configuration

All configuration is passed via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND` | `simplenote` | Notes backend to use: `simplenote` or `usememos` |
| `SN_USERNAME` | — | Simplenote username |
| `SN_PASSWORD` | — | Simplenote password |
| `SN_CLI_PATH` | `sncli` | Path to the `sncli` binary |
| `MEMOS_URL` | — | Base URL of the Usememos instance |
| `MEMOS_TOKEN` | — | API token for Usememos |
| `TAG_TO_DOWNLOAD` | — | Tag used to filter notes for export |
| `CONTINUOUS_NOTE_TAG` | — | Comma-separated continuous-note tags. Each matching note is split into one note per line. A tag such as `blog:thoughts` becomes the `thoughts` category. |
| `UNLISTED_TAGS` | — | Comma-separated tags that mark notes as `unlisted` |
| `TITLE_SUBSTITUTIONS` | — | Comma-separated, ordered `find:replace` pairs for summary generation. The first matching pair wins. |
| `SSG_TYPE` | `hugo` | Static site generator type (`hugo` or `vite`) |
| `VITE_SUBTITLE` | — | Default subtitle used in go-vite front matter |
| `INPUT_DIR` | — | Directory for temporary/raw input files |
| `OUTPUT_DIR` | — | Required directory where generated Markdown files are written. The app records generated filenames in `.notes2ssg-manifest.json`. |
| `AUTHOR` | `root` | Author name used in front matter |
| `POLLING_CYCLE` | `3600` | Sleep duration between exports in seconds. Set `0` to export once and exit. |
| `GOTIFY_URL` | — | Gotify server base URL for notifications |
| `GOTIFY_TOKEN` | — | Gotify application token |
| `DEBUG` | `false` | Enable debug-level logging |

## Common workflows

```sh
make test        APP=notes2ssg        # run unit tests
make local-build APP=notes2ssg        # produce ./out/notes2ssg
make local-run   APP=notes2ssg        # run the binary against ./apps/notes2ssg/.env
make demo        APP=notes2ssg        # exercise the freshly built binary
make release-test APP=notes2ssg       # goreleaser snapshot build
make release     APP=notes2ssg TYPE=patch  # tag and push a release
```
