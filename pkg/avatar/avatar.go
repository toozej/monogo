// Package avatar provides a shared, hidden "avatar" cobra command that renders
// an application's mascot image in the terminal using the go-termimg library.
//
// Every monorepo app wires the same command via avatar.NewCommand("<app>").
// The command renders ./img/avatar.png (or a --url/--path override) and falls
// back to ASCII art labeled with the app name when the terminal cannot render
// images or the source is unavailable.
package avatar

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/blacktop/go-termimg"
)

// DefaultAvatarURL is the URL to the project's GitHub avatar/mascot image.
const DefaultAvatarURL = "https://github.com/toozej.png"

// DefaultAvatarPath is a local fallback path for the avatar image. It is
// resolved relative to the working directory, so each app renders its own
// ./img/avatar.png.
const DefaultAvatarPath = "./img/avatar.png"

const maxAvatarSize = 10 * 1024 * 1024

const avatarHTTPTimeout = 10 * time.Second

var httpGet = func(url string) (*http.Response, error) {
	client := &http.Client{Timeout: avatarHTTPTimeout}
	return client.Get(url)
}

func validateAvatarURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("URL scheme must be https, got %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("URL must have a hostname")
	}
	return nil
}

// RenderFromURL fetches an image from the given URL and renders it in the terminal.
func RenderFromURL(url string, width, height int, w io.Writer) error {
	if err := validateAvatarURL(url); err != nil {
		return fmt.Errorf("failed to fetch avatar from %s: %w", url, err)
	}
	resp, err := httpGet(url)
	if err != nil {
		return fmt.Errorf("failed to fetch avatar from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch avatar from %s: HTTP %d", url, resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "avatar-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxAvatarSize)); err != nil {
		if cerr := tmpFile.Close(); cerr != nil {
			return fmt.Errorf("failed to save avatar image: %w (close error: %v)", err, cerr)
		}
		return fmt.Errorf("failed to save avatar image: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	return RenderFromFile(tmpFile.Name(), width, height, w)
}

// renderImage is a variable for the termimg rendering function, allowing it
// to be replaced in tests without requiring a real terminal or image file.
var renderImage = func(path string, width, height int) (string, error) {
	img, err := termimg.Open(path)
	if err != nil {
		return "", err
	}
	return img.Width(width).Height(height).Render()
}

// RenderFromFile reads an image from the given path and renders it in the terminal.
func RenderFromFile(path string, width, height int, w io.Writer) error {
	rendered, err := renderImage(path, width, height)
	if err != nil {
		return fmt.Errorf("failed to render avatar image from %s: %w", path, err)
	}

	_, err = fmt.Fprint(w, rendered)
	return err
}

// PrintFallback prints a text-based avatar fallback, labeled with appName,
// when image rendering fails.
func PrintFallback(w io.Writer, appName string) {
	_, _ = fmt.Fprintf(w, `
    ____ ___ ___ _____
   / ___||_ _| / _ \___ /
   | |    | | | | | ||_ \
   | |___ | | | |_| |__) |
    \____|___| \___/____/

    %s
`, appName)
}
