# lego-stego

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/lego-stego)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

Go-based CLI tool for [Steganography](https://en.wikipedia.org/wiki/Steganography)

<img src="img/avatar.png" alt="lego-stego avatar" style="background-color: #FFFFFF;" />

## Features

- QR code steganography
- Embed arbitrary files
- AES-GCM encryption (Argon2id KDF)
- Reed–Solomon error correction
- Adaptive noise-aware embedding
- Randomized embedding (password-derived)
- PNG support (lossless)
- Decode QR codes automatically

## Usage

### Embed QR
```
lego-stego embed -i carrier.png -o out.png -u https://example.com --password secret
```

### Extract QR
```
lego-stego extract -i out.png -o qr.png --password secret
```

### Hide File
```
lego-stego hide -i carrier.png -f secret.bin -o out.png --password secret
```

### Reveal File
```
lego-stego reveal -i out.png -o secret.bin --password secret
```

## Security Notes
Always use a strong password
PNG only (lossless required)
Resistant to casual inspection, not nation-state steganalysis

## changes required to use this as a starter template
- set up new repository in quay.io web console
    - (DockerHub and GitHub Container Registry do this automatically on first push and publish)
    - name must match Git repo name
    - Grant `write` permissions to the robot user named in `QUAY_USERNAME`.
    - Admin permissions for your quay.io account are an expected prerequisite.
- set built packages visibility in GitHub packages to public
    - navigate to https://github.com/users/$USERNAME/packages/container/$REPO/settings
    - scroll down to "Danger Zone"
    - change visibility to public

