# notes-courier

Notes Courier moves Markdown notes between note platforms. It reads Simplenote through `sncli`, Memos through its HTTP API, or an Obsidian vault on disk. It writes to Memos, an Obsidian vault, Hugo Markdown, or go-vite Markdown.

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
| `TAG_TO_DOWNLOAD` | empty | Only read notes with this tag |
| `SSG_TYPE` | `hugo` | Legacy target setting when `DESTINATION` is empty |
| `OUTPUT_DIR` | empty | Required for Hugo and go-vite targets |
| `AUTHOR` | `root` | Hugo author |
| `VITE_SUBTITLE` | empty | go-vite subtitle fallback |
| `CONTINUOUS_NOTE_TAG` | empty | Comma-separated tags that split a note by line |
| `UNLISTED_TAGS` | empty | Comma-separated tags that mark Hugo notes as unlisted |
| `TITLE_SUBSTITUTIONS` | empty | Ordered `find:replace` pairs for Hugo summaries |
| `POLLING_CYCLE` | `3600` | Seconds between passes; `0` runs one pass |
| `GOTIFY_URL`, `GOTIFY_TOKEN` | empty | Optional Gotify notifications |

Memos and Obsidian imports add a `notes-courier` marker to each note. A later pass updates the marked note. The Obsidian writer refuses to replace an unmarked file with the same name. A target with no source notes is not changed. The Hugo and go-vite writers keep a manifest of generated files and remove stale generated files on later passes.

The Obsidian vault interface reads and writes local Markdown files. It follows the file model used by the [NotesMD CLI Obsidian package](https://github.com/Yakitrak/notesmd-cli/tree/main/pkg/obsidian). Obsidian does not need to run.

## Checks

```sh
make test APP=notes-courier
make pre-commit-run
```
