# Internal packages

The `releases` package selects, downloads, verifies, and stores forge release assets.
It also manages the cache, directory lock, and release retention.

`Source` provides forge-neutral release and asset records.
`Sources` selects a client from the repository hostname.
GitHub uses the official Go client.
GitLab and Codeberg use their REST APIs.
Tangled uses public AT Protocol records and annotated tag artifacts.
Only GitLab supports nested repository namespaces.
GitLab links can omit asset sizes.
A nil asset size disables the size comparison and preserves checksum checks.
