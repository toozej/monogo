# gotts-it

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/gotts-it)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo/apps/gotts-it)](https://goreportcard.com/report/github.com/toozej/monogo/apps/gotts-it)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/gotts-it/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/gotts-it/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/gotts-it/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/gotts-it)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/gotts-it/total)

<img src="img/avatar.png" alt="gotts-it avatar"/>

CLI tool that extracts readable article text from a URL or local HTML file and synthesizes speech via a self-hosted Speaches TTS server (OpenAI-compatible API) or Google Translate TTS.

## Usage

1. Start the Speaches TTS server:
   ```
   make speaches-up
   ```
   Wait for the Kokoro model to download on first run.

2. Convert an article URL to speech:
   ```
   gotts-it --url https://en.wikipedia.org/wiki/Readability -o readability.mp3
   ```

3. Convert a local HTML file to speech:
   ```
   gotts-it --file article.html -o article.mp3
   ```

4. Output to a directory (filename derived from article title):
   ```
   gotts-it --url https://en.wikipedia.org/wiki/Readability --output-dir ./output
   ```

5. Use Google Translate TTS backend:
   ```
   gotts-it --url https://en.wikipedia.org/wiki/Readability --tts-backend google --lang en
   ```

6. Server mode (read URLs/files from stdin, one per line):
   ```
   echo "https://en.wikipedia.org/wiki/Readability" | gotts-it server --output-dir /out
   ```

7. See all options:
   ```
   gotts-it --help
   ```

8. Run the full stack with Docker Compose:
   ```
   make up
   ```

9. Run with Docker and comma-separated URLs:
   ```
   make run URLS=https://en.wikipedia.org/wiki/Readability,https://en.wikipedia.org/wiki/Go_(programming_language)
   ```

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `URL` | | Article URL to fetch and convert to speech |
| `FILE` | | Local HTML or text file to convert to speech |
| `OUTPUT` | | Output audio file path (default: derived from article title) |
| `OUTPUT_DIR` | | Directory for output files (filename derived from article title) |
| `OPENAI_BASE_URL` | `http://localhost:8000/v1` | OpenAI-compatible TTS endpoint base URL |
| `OPENAI_API_KEY` | | API key (Speaches ignores it but SDK requires non-empty) |
| `TTS_MODEL` | `speaches-ai/Kokoro-82M-v1.0-ONNX` | TTS model ID |
| `TTS_VOICE` | `af_heart` | Voice ID |
| `TTS_FORMAT` | `mp3` | Output format: mp3, wav, flac, pcm |
| `TTS_SPEED` | `1.0` | Speech speed (0.25–4.0) |
| `TTS_INSTRUCTIONS` | | Optional model instructions |
| `TTS_TIMEOUT` | `5m` | Per-chunk TTS request timeout |
| `FETCH_TIMEOUT` | `30s` | Article URL fetch timeout |
| `TTS_BACKEND` | `openai` | TTS backend: `openai` or `google` |
| `GOOGLE_TRANSLATE_LANG` | `en` | Language code for Google Translate TTS |

## Development

- Build locally: `make local-build`
- Run tests: `make local-test`
- Run linters: `make pre-commit-run`
- Hot-reload: `make local-iterate`

## Features of this project
- Cobra CLI with flags for all TTS and article extraction options
- Article text extraction via go-readability (from URL or local file)
- OpenAI-compatible TTS via the official openai-go SDK
- Google Translate TTS backend (`--tts-backend google --lang en`)
- Sentence-boundary chunking for articles longer than 4096 characters (OpenAI) or 200 characters (Google Translate)
- Audio concatenation: MP3 (naive), WAV (header rewrite), PCM (raw); FLAC deferred
- Server mode: read URLs/files from stdin line-by-line (`gotts-it server`)
- Output directory mode: `--output-dir` for batch output with auto-derived filenames
- Speaches server included in docker-compose.yml
- Goreleaser for cross-platform builds and Docker images
- Signed Docker images with Cosign
- Pre-commit hooks for formatting, linting, and security checks
