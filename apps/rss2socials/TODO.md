# TODO

## Bluesky: Support for Custom PDS Hosts in Tests and Self-hosted Deployments

The `botsky` library (`github.com/davhofer/botsky`) does not expose its internal `xrpc.Client` field.
We therefore cannot set a custom PDS host at client creation time.

### Tests

We cannot point `botsky.Client` at a local `httptest` mock server.
The client always contacts `https://bsky.social`, the hardcoded `ApiEntryway`.
The default test run skips integration tests that contact the real Bluesky API.
Use `-run TestPost_Integration` without `-short` to run them.
Unit tests currently cover only input validation for an absent handle or app key.

### Self-hosted PDS

The application reads the `BLUESKY_PDS` config field but does not yet pass it to the botsky client.
Wait for botsky to expose the xrpc host through a `WithPDS` option, a `SetHost` method on `Client`, or an equivalent interface.
Update `internal/bluesky/bluesky.go:NewClient` to set the PDS host from `conf.BlueskyPDS` when that interface becomes available.
This change will enable self-hosted PDS instances and test mocks.

Upstream issue: https://github.com/davhofer/botsky
Consider a feature request or PR to expose a `SetHost` or `WithPDS` option.
