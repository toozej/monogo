//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

// TestResponsive_MobileView tests the mobile viewport rendering.
func TestResponsive_MobileView(t *testing.T) {
	// Create browser context with mobile viewport
	opts := newExecAllocatorOpts(chromedp.WindowSize(375, 667)) // iPhone SE dimensions

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err := navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate to home page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page in mobile view")
}

// TestResponsive_TabletView tests the tablet viewport rendering.
func TestResponsive_TabletView(t *testing.T) {
	// Create browser context with tablet viewport
	opts := newExecAllocatorOpts(chromedp.WindowSize(768, 1024)) // iPad dimensions

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err := navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate to home page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page in tablet view")
}

// TestResponsive_DesktopView tests the desktop viewport rendering.
func TestResponsive_DesktopView(t *testing.T) {
	// Create browser context with desktop viewport
	opts := newExecAllocatorOpts(chromedp.WindowSize(1920, 1080)) // Full HD desktop

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err := navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate to home page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page in desktop view")
}
