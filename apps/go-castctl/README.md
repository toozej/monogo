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
`device_name`, `device_type`, and `device_address`.

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

Castor selects a TV by its `name` and `type`; `address` is optional discovery
metadata returned by `GET /devices` and displayed in the interface.

## Run locally

Install Castor and make it available on `PATH`.
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
contains bitrate and URL but not captured cookies or request headers. Browsers
also differ in native HLS and codec support. As a result, `/receive` works for
browser-playable streams that do not require those headers; pages that require them
possibly still cast successfully while local playback fails.
