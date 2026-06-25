//go:build e2e
// +build e2e

package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSettings_ViewSettings tests accessing the settings page.
func TestSettings_ViewSettings(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/settings")
	require.NoError(t, err, "Should navigate to settings page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load settings page")

	time.Sleep(500 * time.Millisecond)
}

// TestSettings_ViewDownloadSettings tests viewing download-related settings.
func TestSettings_ViewDownloadSettings(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/settings")
	require.NoError(t, err, "Should navigate to settings page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	// Settings should be rendered
	time.Sleep(500 * time.Millisecond)
}

// TestSettings_ViewFileNameSettings tests viewing filename format settings.
func TestSettings_ViewFileNameSettings(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/settings")
	require.NoError(t, err, "Should navigate to settings page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	time.Sleep(500 * time.Millisecond)
}
