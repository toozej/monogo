# E2E Tests

This directory contains end-to-end (E2E) tests for Podgrab using chromedp for
browser automation.

## Overview

E2E tests verify complete user workflows by running a real Podgrab server and
automating browser interactions. Tests use an in-memory SQLite database and
chromedp for browser automation.

## Running E2E Tests

E2E tests are tagged with `e2e` build tag to separate them from unit and
integration tests:

```bash
# Run all E2E tests
go test -tags=e2e -v ./e2e_test/...

# Run specific test file
go test -tags=e2e -v ./e2e_test/podcast_workflow_test.go

# Run with timeout (E2E tests can be slow)
go test -tags=e2e -timeout 5m -v ./e2e_test/...
```

## Prerequisites

E2E tests require:

- Chrome or Chromium browser installed on the system
- chromedp will automatically find and use the installed browser
- No additional setup required for headless mode
- **Auto-Skip**: Tests automatically skip with a warning message if Chrome is
  not found
  - Install Chrome: `brew install --cask google-chrome` (macOS)
  - Install Chromium: `sudo apt-get install chromium-browser` (Ubuntu)

## Test Structure

### Test Files

- **setup_test.go**: Test infrastructure, browser setup, helper functions
- **podcast_workflow_test.go**: Podcast management workflows (9 tests)
- **episode_workflow_test.go**: Episode viewing and filtering workflows (6
  tests)
- **settings_test.go**: Settings page tests (3 tests)
- **responsive_test.go**: Responsive design tests (3 tests)

**Total: 21 E2E tests**

### Test Approach

E2E tests use:

- Real Podgrab HTTP server via httptest.Server
- Real SQLite in-memory database for isolation
- Real browser automation with chromedp
- Page navigation and element interaction
- Basic visual verification

## Test Categories

### Podcast Workflows

- View home page
- View podcasts list
- View podcast details
- Navigate between pages
- Page load verification

### Episode Workflows

- View episode details
- View downloaded episodes
- View played/unplayed status
- View bookmarked episodes
- Episode filtering
- Episode pagination

### Settings

- View settings page
- View download settings
- View filename settings

### Responsive Design

- Mobile viewport (375x667)
- Tablet viewport (768x1024)
- Desktop viewport (1920x1080)

## Browser Configuration

Tests use chromedp with:

- **Headless mode**: Enabled by default
- **Viewport**: Configurable per test
- **Timeout**: 30 seconds per operation
- **Screenshots**: Captured on test failure (saved to /tmp)

## Helper Functions

**Navigation**:

- `navigateToPage(ctx, path)`: Navigate to URL
- `waitForElement(ctx, selector)`: Wait for element visibility
- `waitForURL(ctx, pattern)`: Wait for URL pattern

**Interaction**:

- `clickElement(ctx, selector)`: Click element
- `fillInput(ctx, selector, value)`: Fill input field
- `getElementText(ctx, selector, text)`: Get element text
- `getElementCount(ctx, selector)`: Count matching elements

**Utilities**:

- `takeScreenshot(ctx, t)`: Capture screenshot
- `newBrowserContext(t)`: Create test browser context

## Known Limitations

1. **Browser Dependency**: Tests require Chrome/Chromium installed
1. **Headless Only**: Currently configured for headless mode
1. **Limited Interaction**: Tests focus on page load and basic verification
1. **No WebSocket Testing**: Real-time features not fully tested
1. **Single Browser**: Tests only Chrome (not Firefox, Safari)

## Future Enhancements

Potential improvements:

- Add form submission tests (add podcast, update settings)
- Test WebSocket real-time updates
- Add multi-browser support (Firefox, Safari)
- Test audio player interaction
- Add visual regression testing
- Test touch gestures for mobile
- Add accessibility testing (axe-core)

## Troubleshooting

**Chrome not found**:

```
Install Chrome or Chromium:
- macOS: brew install --cask google-chrome
- Ubuntu: sudo apt install chromium-browser
- Windows: Download from google.com/chrome
```

**Tests timeout**:

```
Increase timeout: go test -tags=e2e -timeout 10m
Check if Chrome is running properly
```

**Screenshot location**:

```
Failed test screenshots saved to: /tmp/podgrab-e2e-{TestName}.png
```

## Integration with CI/CD

For CI/CD pipelines:

```yaml
- name: Install Chrome
  run: |
    wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
    sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list'
    sudo apt update
    sudo apt install google-chrome-stable

- name: Run E2E tests
  run: go test -tags=e2e -v ./e2e_test/...
```

## Test Maintenance

When updating E2E tests:

1. Keep tests focused on user workflows
1. Avoid testing implementation details
1. Use meaningful test names describing user actions
1. Add screenshots on failure for debugging
1. Keep tests independent and isolated
1. Clean up test data after each test
