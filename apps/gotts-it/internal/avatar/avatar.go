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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch avatar from %s: HTTP %d", url, resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "avatar-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

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

// PrintFallback prints a text-based avatar fallback when image rendering fails.
func PrintFallback(w io.Writer) {
	fmt.Fprint(w, `
    ____ ___ ___ _____
   / ___||_ _| / _ \___ /
   | |    | | | | | ||_ \
   | |___ | | | |_| |__) |
    \____|___| \___/____/

    gotts-it
`)
}
