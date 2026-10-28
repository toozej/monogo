# notes-courier

Notes Courier moves Markdown notes between note platforms.
It reads Simplenote through `sncli`, Memos through its HTTP API, or an Obsidian vault on disk.
It writes to Memos, an Obsidian vault, Hugo Markdown, or go-vite Markdown.
See the [Obsidian migration guide](docs/obsidian-migration.md) for metadata mapping, scheduled exports, conflict recovery, and OneDrive settings.

## Run

Set the source with `BACKEND` and the target with `DESTINATION`. Set `POLLING_CYCLE=0` for one pass.

```sh
BACKEND=simplenote SN_USERNAME=user@example.com SN_PASSWORD=secret \
  DESTINATION=memos MEMOS_URL=https://memos.example.com MEMOS_TOKEN=secret \
  POLLING_CYCLE=0 notes-courier
```

```sh
BACKEND=memos MEMOS_URL=https://memos.example.com MEMOS_TOKEN=secret \
  DESTINATION=obsidian OBSIDIAN_VAULT_PATH=/notes OBSIDIAN_FOLDER=Imported \
  POLLING_CYCLE=0 notes-courier
```

```sh
BACKEND=obsidian OBSIDIAN_VAULT_PATH=/notes \
  DESTINATION=hugo OUTPUT_DIR=/site/content/posts \
  POLLING_CYCLE=0 notes-courier
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `BACKEND` | `simplenote` | Source: `simplenote`, `memos`, `usememos`, or `obsidian` |
| `DESTINATION` | value of `SSG_TYPE` | Target: `hugo`, `vite`, `memos`, `usememos`, or `obsidian` |
| `SN_USERNAME`, `SN_PASSWORD` | empty | Credentials passed to `sncli` |
| `SN_CLI_PATH` | `sncli` | Path to the `sncli` program |
| `MEMOS_URL`, `MEMOS_TOKEN` | empty | Memos base URL and bearer token |
| `MEMOS_VISIBILITY` | `PRIVATE` | Visibility for new Memos notes: `PRIVATE`, `PROTECTED`, or `PUBLIC` |
| `OBSIDIAN_VAULT_PATH` | empty | Local Obsidian vault directory |
| `OBSIDIAN_FOLDER` | vault root | Folder for imported Obsidian notes |
| `OBSIDIAN_DELETE_MISSING` | `false` | Remove the unchanged imports absent from the source export |
| `TAG_TO_DOWNLOAD` | empty | Only read notes with this tag |
| `SSG_TYPE` | `hugo` | Legacy target option when `DESTINATION` is empty |
| `OUTPUT_DIR` | empty | Required for Hugo and go-vite. Also selects the Obsidian output folder when `OBSIDIAN_VAULT_PATH` is empty. |
| `AUTHOR` | `root` | Hugo author |
| `VITE_SUBTITLE` | empty | go-vite subtitle fallback |
| `CONTINUOUS_NOTE_TAG` | empty | Comma-separated tags that split a note by line |
| `UNLISTED_TAGS` | empty | Comma-separated tags that mark Hugo notes as unlisted |
| `TITLE_SUBSTITUTIONS` | empty | Ordered `find:replace` pairs for Hugo summaries |
| `POLLING_CYCLE` | `3600` | Seconds between passes; `0` runs one pass |
| `GOTIFY_URL`, `GOTIFY_TOKEN` | empty | Optional Gotify notifications |

Memos imports add a `notes-courier` marker to each note.
A later pass updates the marked note.
An empty source export does not change a Memos destination.
The Hugo and go-vite writers keep a manifest of generated files.
They remove stale generated files on later passes.

Obsidian exports preserve the Markdown body and put source metadata in YAML properties.
Filenames derive from source IDs, so title changes and duplicate titles do not create duplicate files.

Simplenote exports use `simplenote-<key>.md`.
Memos and Obsidian exports use their source name and a SHA-256 hash of their source ID.

The exporter checks for local edits before replacement or deletion.
It retains absent source notes unless `OBSIDIAN_DELETE_MISSING=true`.

An empty successful export removes unchanged imports only when you enable deletion.
Site-specific settings do not split or modify Obsidian exports.

`DESTINATION=obsidian` and the legacy `SSG_TYPE=obsidian` option select the same exporter.
`DESTINATION` takes precedence over `SSG_TYPE`.
`OBSIDIAN_VAULT_PATH` and `OBSIDIAN_FOLDER` take precedence over `OUTPUT_DIR`.
The exporter retains `.notes2ssg-obsidian.json` and its lock name for compatibility with current migrations.
The Obsidian reader uses local Markdown files and skips hidden files and symbolic links.
Obsidian does not need to run.

## Checks

```sh
make test APP=notes-courier
make pre-commit-run
```
