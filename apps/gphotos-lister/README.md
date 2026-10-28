# gphotos-lister

Create a text file with the original filenames of photos that have at least one heart in a Google Photos album.
Hearts from any album participant qualify.
The app excludes videos and removes duplicate filenames.
The app supports a CLI and an embedded web UI based on the RSSFFS server pattern.

## Requirements

Enable access through a link for the Google Photos album.
Supply the full album URL with its `key` parameter, or the short album link.
The app uses anonymous HTTP requests.
It does not require Chrome, a Google account, OAuth tokens, account cookies, or saved account credentials.
It does not read or write a browser profile.

The app reads the Google Photos web interface and its internal requests.
Google removed general album access from the public Photos API in 2025.
See [Google API changes](https://developers.google.com/photos/support/updates).
The internal protocol can change.
The app returns an error if it cannot identify the response or heart target.
It does not interpret an unreadable album as an empty selection.

## Build

Run this command from the repository root:

```sh
make local-build APP=gphotos-lister
```

## CLI

```sh
./out/gphotos-lister \
  --album 'https://photos.google.com/share/ALBUM?key=KEY' \
  --output liked-photos.txt \
  --prefix '/photos/' \
  --suffix '.original'
```

The app also accepts the album URL as a positional argument.
The app also accepts short `https://photos.app.goo.gl/…` links.

An example output line is `/photos/IMG_1234.JPG.original`.
The suffix follows the complete filename and its extension.
An album with no hearts produces an empty text file.

The CLI replaces a current output file only after it writes the new list successfully.
The output parent directory must exist.

### Debug log output

```sh
./out/gphotos-lister --debug \
  --album 'https://photos.app.goo.gl/SHARE_ID' \
  --output liked-photos.txt
```

Set `GPHOTOS_DEBUG=true` to enable the same logs through the environment.
Use `serve --debug` to log web requests to Google Photos.
The app sends debug logs to stderr.
Logs show request status, duration, album pages, photo counts, activity types, and heart matches.
Logs exclude album URLs, access keys, account data, comments, and original filenames.

For an empty list, check `photos_total`, `album_hearts`, `hearted_photo_ids_total`, and `matched_photos`.
An album heart applies to the whole album and does not select each photo.
Private Favorites stars differ from hearts in shared albums.

### Copy local originals

```sh
./out/gphotos-lister \
  --album 'https://photos.google.com/share/ALBUM?key=KEY' \
  --copy \
  --source /photos/originals \
  --destination /photos/selected
```

The app searches the source directory and its subdirectories.
It matches the exact original filename with its extension and letter case.
Prefixes and suffixes affect only the text list.
Copies keep the original filenames in the destination directory.
The app creates the destination directory if it does not exist.

The app copies regular files into new files.
It does not move files or create symlinks or hard links.
It skips source symlinks and excludes a destination directory inside the source tree.
Source and destination directories must differ.
The app replaces current destination files by default.
Use `--no-overwrite` to skip current destination entries.

The app reports filenames that are absent or have multiple source matches.
It does not choose between ambiguous matches.
A copy error returns a nonzero exit status.
The CLI retains the filename list if a local copy fails.
Successful copies can remain after a later copy fails.

## Web UI

```sh
./out/gphotos-lister serve
```

1. Open `http://127.0.0.1:8080`.
2. Enter the album URL and optional prefix or suffix.
3. Select `Copy matching local files` if you want local copies.
4. Enter the source and destination paths if you selected copies. These paths refer to the server machine.
5. Select **Create filename list**.
6. Select **Download liked-photos.txt**.

The page shows copy counts and absent or ambiguous filenames.

The server binds to localhost by default.
Set both web credentials before you bind to a remote address:

```sh
GPHOTOS_WEB_USERNAME=user \
GPHOTOS_WEB_PASSWORD='your-password' \
./out/gphotos-lister serve --host 0.0.0.0 --port 8080
```

Use a TLS reverse proxy for remote access.
Each form request requires a CSRF token.
The generated container uses a distroless runtime with CA certificates.
It supports the same anonymous requests as the native binary.
Web credentials protect this app.
They are separate from Google account credentials.

For Compose, copy `.env.sample` to `.env` in the app directory.
Set both web credentials in that file.
Then run:

```sh
docker compose -f apps/gphotos-lister/docker-compose.yml up
```

Mount a source directory and a writable destination directory to use local copies inside a container.

## Environment variables

CLI flags override environment variables. Explicit empty strings and false values from CLI flags also override the environment.
The shared configuration loader also reads a `.env` file in the current directory.
See [.env.sample](.env.sample).

| CLI flag | Environment variable | Default |
| --- | --- | --- |
| `--album` | `GPHOTOS_ALBUM` | Empty |
| `--output`, `-o` | `GPHOTOS_OUTPUT` | `liked-photos.txt` |
| `--prefix` | `GPHOTOS_PREFIX` | Empty |
| `--suffix` | `GPHOTOS_SUFFIX` | Empty |
| `--copy` | `GPHOTOS_COPY` | `false` |
| `--source` | `GPHOTOS_SOURCE` | Empty |
| `--destination` | `GPHOTOS_DESTINATION` | Empty |
| `--no-overwrite` | `GPHOTOS_NO_OVERWRITE` | `false` |
| `--debug` | `GPHOTOS_DEBUG` | `false` |
| `--timeout` | `GPHOTOS_TIMEOUT` | `15m` |
| `--host` | `GPHOTOS_WEB_HOST` | `127.0.0.1` |
| `--port` | `GPHOTOS_WEB_PORT` | `8080` |
| `--web-username` | `GPHOTOS_WEB_USERNAME` | Empty |
| `--web-password` | `GPHOTOS_WEB_PASSWORD` | Empty |

## Development

The Make target `make new-app APP=gphotos-lister` created the app.
The CI app matrix includes the app.
The weekly Docker workflow discovers released apps from their tags.
It requires no static matrix entry.

```sh
make test APP=gphotos-lister
make release-test APP=gphotos-lister
make pre-commit-run
```

Unit tests use fake Google sessions and temporary local directories.
They cover pagination, original filenames, list formatting, configuration precedence, web requests, and copy outcomes.
Live checks use anonymous public album links.
Unit tests use local fixtures for hearts and original filenames.

The album and filename field references come from
[google_photos_web_client](https://github.com/xob0t/google_photos_web_client).
The anonymous request shapes follow the public Google Photos web client.
The reader uses `snAcKc` for albums, `wQ6iqd` for activity, and `fDcn4b` for original filenames.
It selects photo heart events and excludes album hearts and comments.
The server uses the same embedded HTML, CSS, JavaScript, and Cobra `serve` approach as [RSSFFS](../RSSFFS/README.md).
