//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/akhilrex/podgrab/controllers"
	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/service"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	testServer     *httptest.Server
	testServerURL  string
	testBrowser    context.Context
	testBrowserCtx context.Context
	cancel         context.CancelFunc
	skipE2E        bool
)

// checkChromeAvailable checks if Chrome/Chromium is installed.
func checkChromeAvailable() bool {
	browsers := []string{
		"google-chrome-stable",
		"google-chrome",
		"chromium",
		"chromium-browser",
		"chrome",
	}
	for _, browser := range browsers {
		if _, err := exec.LookPath(browser); err == nil {
			return true
		}
	}
	return false
}

// TestMain sets up the test environment before running E2E tests.
func TestMain(m *testing.M) {
	// Check if Chrome is available
	if !checkChromeAvailable() {
		fmt.Println("WARNING: Chrome/Chromium not found. E2E tests will be skipped.")
		fmt.Println("Install Chrome or Chromium to run E2E tests.")
		skipE2E = true
		os.Exit(0)
	}
	// Setup test database
	database := setupTestDatabase()
	defer cleanupTestDatabase(database)

	// Setup test server
	testServer = setupTestServer(database)
	defer testServer.Close()
	testServerURL = testServer.URL

	// Setup browser context
	testBrowserCtx, cancel = chromedp.NewContext(context.Background())
	defer cancel()

	// Run tests
	exitCode := m.Run()

	os.Exit(exitCode)
}

// setupTestDatabase creates an in-memory database for E2E tests.
func setupTestDatabase() *gorm.DB {
	// Set test environment
	tmpDir := os.TempDir()
	os.Setenv("DATA", tmpDir)
	os.Setenv("CONFIG", tmpDir)

	// Create in-memory database
	t := &testing.T{}
	database := db.SetupTestDB(t)

	// Set as global DB
	db.DB = database

	// Create default settings
	setting := &db.Setting{
		DownloadOnAdd:          false,
		InitialDownloadCount:   1,
		AutoDownload:           false,
		MaxDownloadConcurrency: 1,
	}
	database.Create(setting)

	return database
}

// cleanupTestDatabase closes the database connection.
func cleanupTestDatabase(database *gorm.DB) {
	sqlDB, _ := database.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}

// setupSettingsMiddleware creates middleware that loads settings into context.
func setupSettingsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		setting := db.GetOrCreateSetting()
		c.Set("setting", setting)
		c.Next()
	}
}

// setupTemplates loads HTML templates with custom functions (same as main.go).
func setupTemplates() *template.Template {
	// Templates are in the project root client directory
	templatePath := "../client/*"
	funcMap := template.FuncMap{
		"intRange": func(start, end int) []int {
			n := end - start + 1
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = start + i
			}
			return result
		},
		"removeStartingSlash": func(raw string) string {
			if len(raw) > 0 && raw[0] == '/' {
				return raw
			}
			return "/" + raw
		},
		"isDateNull": func(raw time.Time) bool {
			return raw == (time.Time{})
		},
		"formatDate": func(raw time.Time) string {
			if raw == (time.Time{}) {
				return ""
			}
			return raw.Format("Jan 2 2006")
		},
		"naturalDate": func(raw time.Time) string {
			return service.NatualTime(time.Now(), raw)
		},
		"latestEpisodeDate": func(podcastItems []db.PodcastItem) string {
			var latest time.Time
			for _, item := range podcastItems {
				if item.PubDate.After(latest) {
					latest = item.PubDate
				}
			}
			return latest.Format("Jan 2 2006")
		},
		"downloadedEpisodes": func(podcastItems []db.PodcastItem) int {
			count := 0
			for _, item := range podcastItems {
				if item.DownloadStatus == db.Downloaded {
					count++
				}
			}
			return count
		},
		"downloadingEpisodes": func(podcastItems []db.PodcastItem) int {
			count := 0
			for _, item := range podcastItems {
				if item.DownloadStatus == db.NotDownloaded {
					count++
				}
			}
			return count
		},
		"formatFileSize": func(inputSize int64) string {
			size := float64(inputSize)
			const divisor float64 = 1024
			if size < divisor {
				return fmt.Sprintf("%.0f bytes", size)
			}
			size = size / divisor
			if size < divisor {
				return fmt.Sprintf("%.2f KB", size)
			}
			size = size / divisor
			if size < divisor {
				return fmt.Sprintf("%.2f MB", size)
			}
			size = size / divisor
			if size < divisor {
				return fmt.Sprintf("%.2f GB", size)
			}
			size = size / divisor
			return fmt.Sprintf("%.2f TB", size)
		},
		"formatDuration": func(total int) string {
			if total <= 0 {
				return ""
			}
			mins := total / 60
			secs := total % 60
			hrs := 0
			if mins >= 60 {
				hrs = mins / 60
				mins = mins % 60
			}
			if hrs > 0 {
				return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
			}
			return fmt.Sprintf("%02d:%02d", mins, secs)
		},
	}
	return template.Must(template.New("main").Funcs(funcMap).ParseGlob(templatePath))
}

// setupTestServer creates a test HTTP server with the Podgrab application.
func setupTestServer(database *gorm.DB) *httptest.Server {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(setupSettingsMiddleware())

	// Load HTML templates with custom functions (same as main.go)
	router.SetHTMLTemplate(setupTemplates())

	// Register routes (simplified version of main.go)
	router.GET("/", controllers.HomePage)
	router.GET("/podcasts", controllers.HomePage)
	router.GET("/search", controllers.Search)
	router.GET("/add", controllers.AddPage)
	router.GET("/podcast/:id", controllers.PodcastPage)
	router.GET("/episodes", controllers.AllEpisodesPage)
	router.GET("/settings", controllers.SettingsPage)

	// API routes
	api := router.Group("/api")
	{
		api.GET("/podcasts", controllers.GetAllPodcasts)
		api.POST("/podcasts", controllers.AddPodcast)
		api.GET("/podcasts/:id", controllers.GetPodcastById)
		api.DELETE("/podcasts/:id", controllers.DeletePodcastById)
		api.GET("/podcasts/:id/items", controllers.GetPodcastItemsByPodcastId)
		api.PATCH("/podcastItems/:id", controllers.PatchPodcastItemById)
		api.GET("/podcastItems", controllers.GetAllPodcastItems)
		api.GET("/tags", controllers.GetAllTags)
		api.POST("/tags", controllers.AddTag)
		api.PATCH("/settings", controllers.UpdateSetting)
	}

	server := httptest.NewServer(router)
	return server
}

// requireChrome skips the test if Chrome is not available.
func requireChrome(t *testing.T) {
	t.Helper()
	if skipE2E {
		t.Skip("Chrome/Chromium not installed - skipping E2E test")
	}
}

// newBrowserContext creates a new browser context for a test.
func newBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	requireChrome(t)

	ctx, cancel := chromedp.NewContext(testBrowserCtx)

	// Set timeout for test operations
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)

	cleanup := func() {
		timeoutCancel()
		cancel()
	}

	return ctx, cleanup
}

// navigateToPage navigates to a page relative to the test server.
func navigateToPage(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s%s", testServerURL, path)
	return chromedp.Run(ctx, chromedp.Navigate(url))
}

// waitForElement waits for an element to be visible on the page.
func waitForElement(ctx context.Context, selector string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// clickElement clicks an element on the page.
func clickElement(ctx context.Context, selector string) error {
	return chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// fillInput fills an input field with text.
func fillInput(ctx context.Context, selector, value string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Clear(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, value, chromedp.ByQuery),
	)
}

// getElementText gets the text content of an element.
func getElementText(ctx context.Context, selector string, text *string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Text(selector, text, chromedp.ByQuery),
	)
}

// getElementCount gets the count of elements matching a selector.
func getElementCount(ctx context.Context, selector string) (int, error) {
	var count int
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll(`+"`"+selector+"`"+`).length`, &count),
	)
	return count, err
}

// waitForURL waits for the URL to match a pattern.
func waitForURL(ctx context.Context, urlPattern string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Location(&urlPattern),
	)
}

// takeScreenshot captures a screenshot of the current page.
func takeScreenshot(ctx context.Context, t *testing.T) {
	var buf []byte
	err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		t.Logf("Failed to capture screenshot: %v", err)
		return
	}

	filename := fmt.Sprintf("/tmp/podgrab-e2e-%s.png", t.Name())
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		t.Logf("Failed to save screenshot: %v", err)
		return
	}

	t.Logf("Screenshot saved: %s", filename)
}
