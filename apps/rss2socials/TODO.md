# TODO

## Bluesky: custom PDS host support for tests and self-hosted PDS

The `botsky` library (`github.com/davhofer/botsky`) does not expose its internal
`xrpc.Client` field, so we cannot set a custom PDS host at client creation time.
This means:

- **Testing**: We cannot point `botsky.Client` at a local `httptest` mock server.
  The client always talks to `https://bsky.social` (the hardcoded `ApiEntryway`).
  Integration tests that hit the real Bluesky API are skipped by default (use
  `-run TestPost_Integration` without `-short`). Unit tests currently only cover
  input validation (missing handle/appkey).

- **Self-hosted PDS**: The `BLUESKY_PDS` config field is read but not yet passed
  into the botsky client. When the botsky library adds a `WithPDS` option,
  a `SetHost` method on `Client`, or otherwise exposes the xrpc host, update
  `internal/bluesky/bluesky.go:NewClient` to set the PDS host from
  `conf.BlueskyPDS` so that self-hosted PDS instances and test mocks work
  correctly.

Upstream issue: https://github.com/davhofer/botsky — consider filing a feature
request or contributing a PR to expose a `SetHost` or `WithPDS` option.
