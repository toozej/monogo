# go-castctl

`go-castctl` is a local web interface for [Castor](https://github.com/stupside/castor),
built with [go-app v10](https://go-app.dev/).
Paste a page URL. Select or enter a TV endpoint.
Send the discovered video to the TV or resolve it into the browser's HTML video player.

## Routes

Interactive Swagger API documentation is available at
`http://127.0.0.1:8080/swagger/index.html` while the server runs. The generated
Swagger 2.0 document is available at `/swagger/doc.json`.

- `POST /send` starts a Castor session for the selected TV; `POST /cast` is an alias.
- `POST /receive` resolves the best stream for local playback; `POST /view` and
  `POST /watch` are aliases.
- `GET /devices` discovers Castor-compatible devices for the endpoint selector.

The POST routes accept JSON. They also accept HTML form fields named `url`,
`device_name`, `device_type`, `device_address`, and optional `device_id`.

```json
{
  "url": "https://example.com/watch/video",
  "device": {
    "name": "Living Room TV",
    "type": "chromecast",
    "address": "192.168.1.50"
  }
}
```

Castor selects a TV by its `id` and `type` when discovery supplies an ID.
Otherwise, Castor selects it by `name` and `type`.
The interface sends the discovered ID to distinguish devices with the same name.
Supported types are `chromecast`, `dlna`, and `roku`.
The optional `address` field contains discovery metadata.
Castor requires Roku credentials or an app ID for Roku playback.

## Run locally

Install Castor v2.5.0 and make it available on `PATH`.
Also make the tools that Castor requires available: Chromium, ffmpeg, and ffprobe.

```sh
make local APP=go-castctl
./out/go-castctl
# Open http://127.0.0.1:8080
```

Useful settings are `SERVER_HOST`, `SERVER_PORT`, `CASTOR_BINARY`,
`CASTOR_CONFIG`, and `CASTOR_TIMEOUT`. The server defaults to loopback to prevent accidental LAN exposure of unauthenticated casting controls.

The generated container uses Castor's image as its base and host networking on Linux.
Castor requires this network mode to discover devices and let TVs fetch streams that it serves locally. Docker Desktop does not provide equivalent host networking;
use the native binaries on macOS or Windows.

## Local playback limitations

Castor's public interface is currently command-line only. Its dry-run output
contains bitrate and URL but not captured cookies or request headers.
The app selects the first stream in Castor's ranked output.
It reports an unknown bitrate as zero. Browsers
also differ in native HLS and codec support. As a result, `/receive` works for
browser-playable streams that do not require those headers; pages that require them
possibly still cast successfully while local playback fails.

## Castor v2 configuration

The app runs Castor's embedded services through its CLI.
This mode does not require separate API, media, or scraping containers.
The runtime image supplies Chromium settings through the v2 environment keys.

`CASTOR_CONFIG` defaults to empty. Castor then reads its user config directory
and the working directory's `config.yaml`, if those files exist.
Set `CASTOR_CONFIG` only when the specified file exists inside the runtime.
Castor v2 rejects an explicit config path that does not exist.

Migrate existing v1 files and environment variables with the
[Castor migration guide](https://github.com/stupside/castor/blob/v2.5.0/migrations/1.x-to-2.0.md).
For example, `resolver.max_height` becomes `cast.max_height`, and browser
settings move to `servers.scraping.browser`.
The same moves apply to `CASTOR_` environment variables.
The app overrides `device.name`, `device.type`, and `device.id` for each send.
It clears `device.host` so a configured host cannot override the selected TV.
Other Castor settings, including remote service endpoints, remain available.

See the [release compatibility review](docs/castor-v2-review.md) for release diffs and validation details.
