# Migrate Simplenote notes to Obsidian

`notes-courier` exports Simplenote notes into a local Obsidian vault.
Use `DESTINATION=obsidian` to select this exporter.

The legacy `SSG_TYPE=obsidian` setting selects the same exporter when `DESTINATION` is empty.
It repeats the export during the migration validation period.

Simplenote remains the source for imported notes until you stop the exporter.
The Obsidian OneDrive Sync plugin distributes vault changes between clients.

You do not need the Obsidian CLI.

## Implementation plan

1. Read `sncli export` JSON to preserve note IDs, dates, tags, and complete content.
2. Convert metadata into flat Obsidian properties above the original Markdown.
3. Use source IDs for filenames so title changes do not create duplicate files.
4. Track exported content hashes to detect vault edits before replacement or deletion.
5. Repeat exports on a configured interval and retry failed passes.
6. Retain notes absent from the source during validation unless you explicitly enable deletion.

## Prepare the host

Run the exporter on one computer that can write to the local vault.
Keep that vault open in Obsidian when you want the plugin to sync changes.
The exporter can write files while Obsidian does not run.
The plugin can sync those changes when you open the vault again.

1. Install Python and [sncli](https://github.com/insanum/sncli#installation).
2. Build the app from the repository root with `make local-build APP=notes-courier`.
3. Copy `out/notes-courier` into `~/.local/bin/notes-courier`.
4. Create `~/.config/notes-courier-obsidian/`.
5. Copy [the example configuration](../examples/obsidian.env.example) into that directory as `.env`.
6. Set the Simplenote credentials and absolute paths in `.env`.
7. Restrict access with `chmod 600 ~/.config/notes-courier-obsidian/.env`.

Set `OBSIDIAN_VAULT_PATH` to a current vault, such as `/home/you/Documents/MyVault`.
Set `OBSIDIAN_FOLDER=Simplenote` to use a dedicated import folder inside that vault.
Current configurations can continue to use `OUTPUT_DIR=/home/you/Documents/MyVault/Simplenote` when `OBSIDIAN_VAULT_PATH` is empty.
Keep the configuration outside the vault so OneDrive does not sync your credentials.
Use `SN_CLI_PATH` to select the installed `sncli` executable.

For example, `pipx install sncli` usually installs it under `~/.local/bin/`.

`sncli` also needs a writable home directory for its local cache.
Use its cache only for this exporter during migration.
Do not edit cached notes through `sncli`.
The export command performs `sncli`'s normal server sync before it reads the notes.
The exporter never submits Obsidian edits to Simplenote.

The repository's minimal Docker images do not include Python or `sncli`.
Use the native binary for this workflow.

## Run the first export

Run these commands after you edit the configuration:

```sh
cd ~/.config/notes-courier-obsidian
POLLING_CYCLE=0 ~/.local/bin/notes-courier
```

The app reads `.env` from the current directory.
Process environment variables that already have values take precedence over `.env` values.
Leave `TAG_TO_DOWNLOAD` empty to export all active notes.
Set it to an exact Simplenote tag to validate a subset first.
The JSON export applies this filter locally, without a regular-expression search.
Deleted Simplenote notes do not enter the active export.

Open the imported folder in Obsidian.
Check the representative notes for Unicode titles, tables, code blocks, dates, and tags.
Check the imported notes on another client after the OneDrive plugin completes its sync.

## Metadata mapping

Obsidian stores properties in YAML at the start of a Markdown file.
`tags` and `aliases` use lists.
See the official [properties documentation](https://help.obsidian.md/properties).

| Simplenote value | Obsidian output |
| --- | --- |
| Server `key` | `simplenote_id` and filename `simplenote-<key>.md` |
| First non-empty content line | `title` and one `aliases` entry. The exporter removes `# ` at the start of this metadata only. |
| `creationDate` | `created`, as a UTC date and time |
| `modificationDate` | `modified`, as a UTC date and time |
| `tags` | Obsidian-compatible `tags`, plus original values in `simplenote_tags` |
| `systemtags` | Original values in `simplenote_system_tags`, which include `pinned` and `markdown` when present |
| Source service | `source: "simplenote"` |
| `content` | Original Markdown body, which includes its title line, whitespace, tables, and links |

Unknown dates do not produce date properties.
Empty notes use `Untitled` as their metadata title.
The body remains unchanged.

Pinned status remains source metadata. It does not create an Obsidian bookmark.
YAML already inside the source content remains in the body. The exporter does not merge it into the generated properties.
Links remain unchanged. The exporter does not download linked files or rewrite Simplenote links.

File-system timestamps describe the exported files. The front matter preserves the source dates.

The `tags` conversion removes `#` characters at the start and changes `:` to `/` for nested tags.
It replaces unsupported characters, which include spaces, with hyphens.
It adds `tag-` to tags that contain only numbers.
It uses `untagged` if conversion produces an empty tag.

It removes duplicate tags without regard to case.
The original values remain available in `simplenote_tags`.
See Obsidian's [tag rules](https://help.obsidian.md/tags).

For example, these Simplenote tags:

```text
blog:go, two words, 123
```

produce these properties:

```yaml
---
title: "Example note"
aliases:
  - "Example note"
source: "simplenote"
simplenote_id: "abc123"
created: 2023-09-01T02:33:35Z
modified: 2023-09-02T03:44:55Z
tags:
  - "blog/go"
  - "two-words"
  - "tag-123"
simplenote_tags:
  - "blog:go"
  - "two words"
  - "123"
simplenote_system_tags:
  - "markdown"
---
```

Duplicate titles produce separate files because their source IDs differ.
Title edits update properties inside the same file.
Aliases let Obsidian find the note by its current title.
Site-specific settings such as `CONTINUOUS_NOTE_TAG`, `UNLISTED_TAGS`, `TITLE_SUBSTITUTIONS`, and `AUTHOR` do not affect Obsidian exports.
The migration preserves one source note per file.

## Repeat exports during validation

Run the app without a one-shot override:

```sh
cd ~/.config/notes-courier-obsidian
~/.local/bin/notes-courier
```

With the example configuration, the app waits 300 seconds after each pass.
A failed pass prints an error and retries after the same interval.
After a failed pass, one-shot mode returns an error without another scheduled attempt.
Unchanged exports do not rewrite note files or state.

On Linux, use the supplied service to run the exporter under systemd:

```sh
mkdir -p ~/.config/systemd/user
cp /path/to/monogo/apps/notes-courier/examples/notes-courier-obsidian.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now notes-courier-obsidian.service
journalctl --user -u notes-courier-obsidian.service -f
```

The service runs while your user manager is active.
Other schedulers can run `POLLING_CYCLE=0` from the configuration directory.
Run only one exporter for this folder and `sncli` cache.

## OneDrive settings

Install and connect [OneDrive Sync](https://community.obsidian.md/plugins/onedrive-sync) in each Obsidian client.
Use the same remote vault location on each client.
The plugin handles OneDrive authentication. `notes-courier` only writes local files.

Add these patterns to the vault's `.syncIgnore` file.
Replace `Simplenote/` if you selected a different import folder:

```text
Simplenote/.notes2ssg-obsidian.json
Simplenote/.notes2ssg-obsidian.lock/
Simplenote/.notes2ssg-tmp-*
```

These patterns keep export state and temporary files on the export computer.
The exporter retains the `notes2ssg` state and lock names for compatibility with current migrations.
Back up `.notes2ssg-obsidian.json` with the imported notes.
The exporter uses this file to identify previous output and detect local edits.
Do not run a second sync tool against the same local vault folder.
The plugin documents this restriction in its [advanced configuration guide](https://github.com/JeffSteinbok/obsidian-onedrive/blob/HEAD/docs/ADVANCED.md).

## Conflicts, deletion, and recovery

Simplenote controls imported content while exports continue.
New source notes create files. Source edits replace unchanged imports.
If you remove an imported file locally, the next export recreates it if its source note remains active.
If you rename an imported file locally, the next export retains that renamed copy and recreates the original filename.
Keep filenames stable during validation.

If you edit an imported file in Obsidian, the next export reports a conflict.
The exporter checks all destinations before it changes notes or removes files.
A conflict found in the initial checks prevents changes to notes or state during that pass.
The exporter checks each file again before replacement or deletion.
It also refuses to replace an unrelated file with a matching destination name.

To resolve a conflict:

1. Preserve the edited copy outside the imported folder.
2. Apply any required changes in Simplenote if Simplenote must retain them.
3. Remove the conflicted imported file.
4. Run the export again.

The default `OBSIDIAN_DELETE_MISSING=false` retains imports absent from the active source export.
This includes deleted notes and notes that no longer match `TAG_TO_DOWNLOAD`.
Set `OBSIDIAN_DELETE_MISSING=true` only when you want those files removed.
The exporter removes only files recorded in its state, and only if their content matches the last export.
An empty successful source export can therefore remove all unchanged imports if you enable deletion.
Malformed JSON and failed source commands do not trigger deletion.

The exporter creates `.notes2ssg-obsidian.lock/` during each write pass.
An interrupted process can leave this lock directory.
Stop the service and confirm that no exporter remains active before you remove a stale lock.
Then restart the service.

Do not remove the state file to resolve a content conflict.
An I/O failure can occur after some file replacements.
A file change during the pass can also stop an export after earlier replacements.
A later pass can recover files that already match the source export.

## Complete the migration

1. Finish validation on your Obsidian clients.
2. Run one final export from Simplenote.
3. Stop the exporter or disable its scheduler.
4. Wait for the OneDrive plugin to sync the final changes.
5. Edit the imported notes in Obsidian.

For the systemd example, stop scheduled exports with:

```sh
systemctl --user disable --now notes-courier-obsidian.service
```

The imported Markdown remains in the vault after you stop the exporter.
