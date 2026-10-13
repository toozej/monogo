# `/scripts`

Scripts to perform various build, install, analysis, etc operations.

These scripts keep the root level Makefile small and simple.

`manage-go-tools.sh` installs the Go tools pinned in `tools/go-tools.tsv` into
`.tools/bin`. This includes Vale, installed from `github.com/vale-cli/vale/v3/cmd/vale`
at the manifest version.
Use `go version -m .tools/bin/vale` to inspect its pinned module version.
Vale's source build reports `master` with `--version`.

`manage-binary-tools.sh` installs the releases pinned in
`tools/binary-tools.tsv` into `.tools/bin`. Each row supplies the tool name,
GitHub repository, release tag, four platform assets, and an optional archive
member. The platform order is Linux AMD64, Linux ARM64, macOS AMD64, macOS ARM64.

Use `-` or omit the final column for a direct executable, such as OpenGrep.
For `.tar.gz`, `.tgz`, or `.zip` assets, specify the binary path inside the archive.
Asset names can contain `{version}`, which expands to the tag without a leading
`v`. Member paths can contain `{archive}`, which expands to the asset name without
its archive suffix. The updater preserves these patterns when it changes tags.

The installer extracts only the specified member. It checks the binary with
`--version` before replacing an installed tool. Failed downloads or extraction
leave the installed binary and its stamp unchanged. Temporary files are removed
on exit. A stamp records the release, asset, and member to detect pin changes.
Run `make binary-tools-test` to test direct executables, archives, platform
selection, cached installs, failed replacements, and manifest updates without
network access.

`check-docs.sh` provides the common Vale and lychee entry point for Make and
pre-commit. It selects Markdown files, excludes generated and downloaded content,
and uses the repository installer before each check. Vale also synchronizes its
pinned package before it checks prose. See the root
[documentation check instructions](../README.md#documentation-checks).
