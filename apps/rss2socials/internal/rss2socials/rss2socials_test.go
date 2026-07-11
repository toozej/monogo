package rss2socials

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
	"github.com/toozej/monogo/apps/rss2socials/internal/db"
	"github.com/toozej/monogo/apps/rss2socials/internal/rss"

	_ "github.com/glebarez/sqlite"
)

type MockRSSChecker struct {
	mock.Mock
}

func (m *MockRSSChecker) CheckRSSFeed(url string) ([]rss.RSSItem, error) {
	args := m.Called(url)
	return args.Get(0).([]rss.RSSItem), args.Error(1)
}

type MockMastodon struct {
	mock.Mock
}

func (m *MockMastodon) GetTootContent(post rss.RSSItem) string {
	args := m.Called(post)
	return args.String(0)
}

func (m *MockMastodon) TootPost(conf config.Config, content string) error {
	args := m.Called(conf, content)
	return args.Error(0)
}

func TestShouldSkipPost(t *testing.T) {
	tests := []struct {
		name                 string
		post                 rss.RSSItem
		skipPrefixCategories []string
		expectedSkip         bool
	}{
		{
			name:                 "No skip categories",
			post:                 rss.RSSItem{Title: "Thoughts on Go", Link: "https://example.com/thoughts-1/"},
			skipPrefixCategories: nil,
			expectedSkip:         false,
		},
		{
			name:                 "Title prefix match case-insensitive",
			post:                 rss.RSSItem{Title: "Thoughts on Go", Link: "https://example.com/post"},
			skipPrefixCategories: []string{"thoughts"},
			expectedSkip:         true,
		},
		{
			name:                 "URL segment prefix match case-insensitive",
			post:                 rss.RSSItem{Title: "My Post", Link: "https://example.com/thoughts-1/"},
			skipPrefixCategories: []string{"Thoughts"},
			expectedSkip:         true,
		},
		{
			name:                 "No match",
			post:                 rss.RSSItem{Title: "My Project", Link: "https://example.com/project-1/"},
			skipPrefixCategories: []string{"Thoughts"},
			expectedSkip:         false,
		},
		{
			name:                 "Multiple skip categories matching second",
			post:                 rss.RSSItem{Title: "Notes on Testing", Link: "https://example.com/post"},
			skipPrefixCategories: []string{"Thoughts", "Notes"},
			expectedSkip:         true,
		},
		{
			name:                 "Partial prefix match on URL segment",
			post:                 rss.RSSItem{Title: "Hello", Link: "https://example.com/thoughts-1/"},
			skipPrefixCategories: []string{"Thoughts"},
			expectedSkip:         true,
		},
		{
			name:                 "Category in middle of URL segment not matched",
			post:                 rss.RSSItem{Title: "Hello", Link: "https://example.com/my-thoughts/"},
			skipPrefixCategories: []string{"Thoughts"},
			expectedSkip:         false,
		},
		{
			name:                 "Empty skip categories list",
			post:                 rss.RSSItem{Title: "Thoughts on Go", Link: "https://example.com/thoughts-1/"},
			skipPrefixCategories: []string{},
			expectedSkip:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipPost(tt.post, tt.skipPrefixCategories)
			assert.Equal(t, tt.expectedSkip, result)
		})
	}
}

func setupTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, db.InitDB())
	t.Cleanup(func() {
		assert.NoError(t, db.CloseDB())
		_ = os.Remove("./tooted_posts.db")
	})
}

func TestHandlePost_NewPost(t *testing.T) {
	setupTestDB(t)

	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/new-post", Content: "content", Title: "New Post"}
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	exists, updated, err := db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.False(t, updated)

	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted)
}

func TestHandlePost_UnchangedPostSkipsPosting(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/unchanged-post", Content: "same content", Title: "Same Post"}

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should post once for new post")

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should NOT post again for unchanged post")
}

func TestHandlePost_NoDuplicatesOnRestart(t *testing.T) {
	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/restart-post", Content: "content", Title: "Restart Post"}

	// First run
	setupTestDB(t)
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should post once for new post")

	// Close DB (simulating application shutdown)
	assert.NoError(t, db.CloseDB())

	// Second run (simulating restart with same DB)
	require.NoError(t, db.InitDB())
	t.Cleanup(func() {
		assert.NoError(t, db.CloseDB())
		_ = os.Remove("./tooted_posts.db")
	})

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should NOT post again after restart for same post")
}

func TestHandlePost_PartialFailureRetries(t *testing.T) {
	setupTestDB(t)

	callCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/partial-fail", Content: "content", Title: "Partial Fail"}

	// First attempt: Mastodon fails
	assert.Error(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, callCount, "Should attempt to post once")

	// Post is stored in DB even though Mastodon failed
	exists, _, err := db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Mastodon was NOT marked as posted since the toot failed
	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.False(t, posted, "Mastodon should NOT be marked posted after failure")

	// Second attempt: Mastodon succeeds (retries because site not marked)
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 2, callCount, "Should retry posting since Mastodon was not marked as posted")

	// Now Mastodon IS marked as posted
	posted, err = db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted, "Mastodon should be marked posted after success")

	// Third attempt: should not post again
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 2, callCount, "Should NOT retry after successful post")
}

func TestHandlePost_PerSiteIndependence(t *testing.T) {
	setupTestDB(t)

	mastodonCallCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mastodonCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/multi-site", Content: "content", Title: "Multi Site"}

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	// Mastodon posted and marked
	assert.Equal(t, 1, mastodonCallCount)
	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted)

	// Bluesky NOT posted (no credentials configured)
	posted, err = db.IsSitePosted(post.Link, "bluesky")
	assert.NoError(t, err)
	assert.False(t, posted, "Bluesky should NOT be marked posted when not configured")

	// Threads NOT posted (no credentials configured)
	posted, err = db.IsSitePosted(post.Link, "threads")
	assert.NoError(t, err)
	assert.False(t, posted, "Threads should NOT be marked posted when not configured")
}

func TestHandlePost_UpdatedPostReposts(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/updated-post", Content: "original", Title: "Updated Post"}

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should post for new post")

	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted, "Mastodon should be marked posted after first post")

	post.Content = "updated content"
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 2, postCount, "Should post again for updated content")
}

func TestHandlePost_UpdatedPostFailureRetriesSameDelivery(t *testing.T) {
	setupTestDB(t)

	callCount := 0
	var updateKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			updateKeys = append(updateKeys, r.Header.Get("Idempotency-Key"))
		}
		if callCount == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	conf := &config.Config{
		MastodonURL: server.URL, MastodonClientKey: "key",
		MastodonClientSecret: "secret", MastodonAccessToken: "token",
	}
	post := rss.RSSItem{Link: "https://example.com/retry-update", Content: "original", Title: "Post"}
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	post.Content = "updated"
	assert.Error(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	pending, err := db.IsUpdatePending(post.Link)
	assert.NoError(t, err)
	assert.True(t, pending)

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 3, callCount, "successful retry must converge without another post")
	if assert.Len(t, updateKeys, 2) {
		assert.NotEmpty(t, updateKeys[0])
		assert.Equal(t, updateKeys[0], updateKeys[1], "retries must use the same remote idempotency key")
	}
	pending, err = db.IsUpdatePending(post.Link)
	assert.NoError(t, err)
	assert.False(t, pending)
}

func TestHandlePost_CategoryMismatchSkips(t *testing.T) {
	setupTestDB(t)

	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	post := rss.RSSItem{Link: "https://example.com/other/new-post", Content: "content", Title: "New Post"}

	lastSegment := path.Base(post.Link)
	if strings.Contains(lastSegment, "tech") {
		t.Skip("Post would match category, not testing mismatch")
	}
}

func TestHandlePost_WithCategoryMatch(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
		Category:             "tech",
	}

	post := rss.RSSItem{Link: "https://example.com/new-post-tech", Content: "content", Title: "New Post"}

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount)
}

func TestHandlePost_MastodonErrorDoesNotMarkPosted(t *testing.T) {
	setupTestDB(t)

	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	post := rss.RSSItem{Link: "https://example.com/error-post", Content: "content", Title: "Error Post"}

	assert.Error(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	exists, _, err := db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, exists, "Post should be stored in DB even on Mastodon error")

	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.False(t, posted, "Mastodon should NOT be marked as posted after error")
}

// TestRunSetup tests the setup logic of Run (flag parsing, config loading, DB init)
func TestRunSetup(t *testing.T) {
	tests := []struct {
		name             string
		setupEnv         map[string]string
		debugFlag        bool
		feedURLFlag      string
		intervalFlag     int
		categoryFlag     string
		expectedDebug    bool
		expectedFeedURL  string
		expectedInterval int
		expectedCategory string
	}{
		{
			name: "Default config from env vars",
			setupEnv: map[string]string{
				"MASTODON_URL":           "https://mastodon.com",
				"MASTODON_CLIENT_KEY":    "clientkey",
				"MASTODON_CLIENT_SECRET": "clientsecret",
				"MASTODON_ACCESS_TOKEN":  "token",
				"GOTIFY_URL":             "https://gotify.com",
				"GOTIFY_TOKEN":           "gotifytoken",
				"FEED_URL":               "https://default.com/rss",
				"INTERVAL":               "10",
				"CATEGORY":               "",
				"DEBUG":                  "false",
			},
			debugFlag:        false,
			feedURLFlag:      "",
			intervalFlag:     0,
			categoryFlag:     "",
			expectedDebug:    false,
			expectedFeedURL:  "https://default.com/rss",
			expectedInterval: 10,
			expectedCategory: "",
		},
		{
			name: "Flag overrides",
			setupEnv: map[string]string{
				"MASTODON_URL":           "https://mastodon.com",
				"MASTODON_CLIENT_KEY":    "clientkey",
				"MASTODON_CLIENT_SECRET": "clientsecret",
				"MASTODON_ACCESS_TOKEN":  "token",
				"GOTIFY_URL":             "https://gotify.com",
				"GOTIFY_TOKEN":           "gotifytoken",
				"FEED_URL":               "https://env.com/rss",
				"INTERVAL":               "5",
				"CATEGORY":               "envcat",
				"DEBUG":                  "false",
			},
			debugFlag:        false,
			feedURLFlag:      "https://flag.com/rss",
			intervalFlag:     15,
			categoryFlag:     "flagcat",
			expectedDebug:    false,
			expectedFeedURL:  "https://flag.com/rss",
			expectedInterval: 15,
			expectedCategory: "flagcat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv := []string{"MASTODON_URL", "MASTODON_CLIENT_KEY", "MASTODON_CLIENT_SECRET", "MASTODON_ACCESS_TOKEN", "GOTIFY_URL", "GOTIFY_TOKEN", "FEED_URL", "INTERVAL", "CATEGORY", "DEBUG"}
			for _, key := range clearEnv {
				_ = os.Unsetenv(key)
			}

			for key, val := range tt.setupEnv {
				_ = os.Setenv(key, val)
			}

			cmd := &cobra.Command{}
			cmd.Flags().BoolP("debug", "d", false, "Enable debug logging")
			cmd.Flags().StringP("feed-url", "f", "", "")
			cmd.Flags().IntP("interval", "i", 60, "")
			cmd.Flags().StringP("category", "c", "", "")
			assert.NoError(t, cmd.Flags().Set("debug", fmt.Sprintf("%t", tt.debugFlag)))
			if tt.feedURLFlag != "" {
				assert.NoError(t, cmd.Flags().Set("feed-url", tt.feedURLFlag))
			}
			if tt.intervalFlag > 0 {
				assert.NoError(t, cmd.Flags().Set("interval", fmt.Sprintf("%d", tt.intervalFlag)))
			}
			if tt.categoryFlag != "" {
				assert.NoError(t, cmd.Flags().Set("category", tt.categoryFlag))
			}

			conf, err := config.GetEnvVars()
			if err != nil {
				t.Fatalf("GetEnvVars() failed: %v", err)
			}

			debug, err := cmd.Flags().GetBool("debug")
			assert.NoError(t, err)
			if debug {
				conf.Debug = true
			}
			assert.Equal(t, tt.expectedDebug, conf.Debug)

			feedURL := conf.FeedURL
			if tt.feedURLFlag != "" {
				feedURL = tt.feedURLFlag
			}
			if feedURL == "" {
				t.Fatal("RSS feed URL is required")
			}

			interval := conf.Interval
			if tt.intervalFlag > 0 {
				interval = tt.intervalFlag
			}
			if interval <= 0 {
				interval = 60
			}

			category := conf.Category
			if tt.categoryFlag != "" {
				category = tt.categoryFlag
			}

			assert.Equal(t, tt.expectedFeedURL, feedURL)
			assert.Equal(t, tt.expectedInterval, interval)
			assert.Equal(t, tt.expectedCategory, category)

			require.NoError(t, db.InitDB())
			assert.NotNil(t, db.DB)
			assert.NoError(t, db.CloseDB())
		})
	}
}

func TestBasicIntegration(t *testing.T) {
	originalDB := db.DB
	assert.NoError(t, db.CloseDB())
	require.NoError(t, db.InitDB())
	t.Cleanup(func() {
		assert.NoError(t, db.CloseDB())
		_ = os.Remove("./tooted_posts.db")
		db.DB = originalDB
	})

	token := "test-token"
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  token,
	}

	post := rss.RSSItem{Link: "https://test.com/new-post", Content: "test content", Title: "Test Post"}
	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	exists, updated, err := db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.False(t, updated)

	posted, err := db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted)

	post.Content = "updated content"
	existsBefore, updatedBefore, err := db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, existsBefore)
	assert.True(t, updatedBefore)

	assert.NoError(t, handlePost(post, conf, "2026-01-01T00:00:00Z", false))

	exists, updated, err = db.HasPostChanged(post.Link, post.Content)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.False(t, updated)
}

func TestHandlePost_SkipExistingOnFirstCycle(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	existingPost := rss.RSSItem{Link: "https://example.com/existing-post", Content: "old content", Title: "Existing Post"}
	newPost := rss.RSSItem{Link: "https://example.com/new-post", Content: "new content", Title: "New Post"}

	if err := db.StoreTootedPost(existingPost.Link, existingPost.Content, "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("Failed to seed existing post: %v", err)
	}
	if err := db.MarkSitePosted(existingPost.Link, "mastodon"); err != nil {
		t.Fatalf("Failed to mark existing post as posted: %v", err)
	}

	assert.NoError(t, handlePost(existingPost, conf, "2026-01-01T00:00:00Z", true))
	assert.Equal(t, 0, postCount, "Should NOT post existing entry when skipIfExisting=true")

	assert.NoError(t, handlePost(newPost, conf, "2026-01-01T00:00:00Z", true))
	assert.Equal(t, 1, postCount, "Should post truly new entry even when skipIfExisting=true")
}

func TestHandlePost_PostAllWhenSkipDisabled(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	existingPost := rss.RSSItem{Link: "https://example.com/existing-post2", Content: "old content", Title: "Existing Post"}
	newPost := rss.RSSItem{Link: "https://example.com/new-post2", Content: "new content", Title: "New Post"}

	if err := db.StoreTootedPost(existingPost.Link, existingPost.Content, "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("Failed to seed existing post: %v", err)
	}
	if err := db.MarkSitePosted(existingPost.Link, "mastodon"); err != nil {
		t.Fatalf("Failed to mark existing post as posted: %v", err)
	}

	assert.NoError(t, handlePost(existingPost, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 0, postCount, "Should not re-post already-fully-posted entry even with skipIfExisting=false")

	assert.NoError(t, handlePost(newPost, conf, "2026-01-01T00:00:00Z", false))
	assert.Equal(t, 1, postCount, "Should post new entry with skipIfExisting=false")
}

func TestHandlePost_FirstCycleSkipOnlyExistingUnchanged(t *testing.T) {
	setupTestDB(t)

	postCount := 0
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "123456"}); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mastodonServer.Close()

	conf := &config.Config{
		MastodonURL:          mastodonServer.URL,
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-token",
	}

	updatedPost := rss.RSSItem{Link: "https://example.com/updated-first-cycle", Content: "original", Title: "Updated Post"}
	if err := db.StoreTootedPost(updatedPost.Link, "original", "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("Failed to seed post: %v", err)
	}

	updatedPost.Content = "updated content"
	assert.NoError(t, handlePost(updatedPost, conf, "2026-01-01T00:00:00Z", true))
	assert.Equal(t, 1, postCount, "Should post updated entry even when skipIfExisting=true")
}

// shortRunTestServers builds a test RSS feed server with `numItems` items and
// a mastodon-compatible HTTP server that increments `mastodonCalls` on every
// successful POST. Both servers must be Close()'d by the caller via t.Cleanup.
func shortRunTestServers(t *testing.T, numItems int, mastodonCalls *int32) (rssURL, mastodonURL string) {
	t.Helper()

	feed := rss.RSSFeed{}
	feed.Channel.Title = "Test"
	for i := 0; i < numItems; i++ {
		feed.Channel.Items = append(feed.Channel.Items, rss.RSSItem{
			Title:   fmt.Sprintf("Post %d", i),
			Link:    fmt.Sprintf("https://example.com/post-%d", i),
			Content: fmt.Sprintf("Content %d", i),
		})
	}

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		if err := xml.NewEncoder(w).Encode(feed); err != nil {
			t.Logf("failed to encode rss feed: %v", err)
		}
	}))
	t.Cleanup(rssServer.Close)

	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(mastodonCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "1"}); err != nil {
				t.Logf("failed to encode mastodon response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(mastodonServer.Close)

	return rssServer.URL, mastodonServer.URL
}

// setupRunTestDB sets DB_PATH to an isolated temp file per test so that
// Run()'s internal db.InitDB() does not collide with other tests.
// It returns the db file path so callers can pass it to db.InitDB or
// set conf.DBPath.
func setupRunTestDB(t *testing.T) string {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "tooted_posts.db")
	t.Setenv("DB_PATH", dbFile)
	t.Cleanup(func() {
		// Run() defers db.CloseDB(); ensure DB is closed before cleanup
		// in case the test bailed out early. Best-effort.
		if db.DB != nil {
			sqlDB, _ := db.DB.DB()
			_ = sqlDB.Close()
			db.DB = nil
		}
	})
	return dbFile
}

func TestRun_ShortRunPostsThreeMostRecent(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 10, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s; SHORT_RUN should not sleep before exit")
	}

	assert.Equal(t, int32(3), atomic.LoadInt32(&mastodonCalls),
		"SHORT_RUN with empty DB should post the 3 most recent items")
}

func TestRun_ShortRunWithFewerThanThreeItems(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 2, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(2), atomic.LoadInt32(&mastodonCalls),
		"SHORT_RUN should post all available items when fewer than 3")
}

func TestRun_ShortRunWithExactlyThreeItems(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 3, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(3), atomic.LoadInt32(&mastodonCalls),
		"SHORT_RUN should post all 3 items when feed has exactly 3")
}

// TestRun_ShortRunExitsWithoutSleeping verifies that Run() returns quickly
// when SHORT_RUN=true, without waiting for the configured Interval.
// This regression-tests a bug where the sleep happened before the SHORT_RUN
// exit check, causing the app to hang for `Interval` minutes after processing.
func TestRun_ShortRunExitsWithoutSleeping(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 5, &mastodonCalls)

	// Use a deliberately large Interval so that, if the sleep happened
	// before the exit, the test would time out instead of completing.
	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60, // 60 minutes
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	start := time.Now()
	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s; it should not sleep for Interval before exiting in SHORT_RUN mode")
	}

	elapsed := time.Since(start)
	assert.Less(t, elapsed, 5*time.Second,
		"SHORT_RUN should exit promptly, well under the configured Interval")
	assert.Equal(t, int32(3), atomic.LoadInt32(&mastodonCalls),
		"SHORT_RUN should still post the 3 most recent items before exiting")
}

func TestRun_ShortRunSkipsAlreadyPostedItems(t *testing.T) {
	dbFile := setupRunTestDB(t)

	// Pre-seed the DB with the most recent post (post-0) marked as
	// already posted to mastodon. SHORT_RUN should skip it and post
	// only the next two items (post-1, post-2).
	require.NoError(t, db.InitDB(dbFile))
	if err := db.StoreTootedPost("https://example.com/post-0", "Content 0", "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("seed StoreTootedPost failed: %v", err)
	}
	if err := db.MarkSitePosted("https://example.com/post-0", "mastodon"); err != nil {
		t.Fatalf("seed MarkSitePosted failed: %v", err)
	}
	assert.NoError(t, db.CloseDB())

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 10, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   true,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(2), atomic.LoadInt32(&mastodonCalls),
		"SHORT_RUN should skip already-posted items and post the next two new items")
}

func TestRun_ShortRunSecondCycleDoesNotRepost(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 10, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   true,
		DBPath:               dbFile,
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("First Run() did not exit within 5s")
	}
	firstRunCalls := atomic.LoadInt32(&mastodonCalls)
	assert.Equal(t, int32(0), firstRunCalls, "First snapshot should be seeded without posting")

	assert.NoError(t, db.CloseDB())

	done2 := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done2)
	}()
	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("Second Run() did not exit within 5s")
	}
	secondRunCalls := atomic.LoadInt32(&mastodonCalls) - firstRunCalls
	assert.Equal(t, int32(0), secondRunCalls,
		"Second cycle should NOT re-post items that were already posted in the first cycle")
}

func TestRun_ShortRunPostsToAllConfiguredSites(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32
	rssURL, mastodonURL := shortRunTestServers(t, 10, &mastodonCalls)

	// Configure SocialSites explicitly so EnabledSites() returns only the
	// sites we mock-test (mastodon). Adding bluesky/threads here would
	// require live credentials and is gated by their handlePost checks.
	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	// Expect 3 RSS items × 1 site = 3 mastodon posts.
	assert.Equal(t, int32(3), atomic.LoadInt32(&mastodonCalls), "Each enabled site should receive a post per RSS item processed")
}

// pubDateTestServers builds a test RSS feed server with items that have
// specific pubDate values and a mastodon-compatible HTTP server.
func pubDateTestServers(t *testing.T, items []rss.RSSItem, mastodonCalls *int32) (rssURL, mastodonURL string) {
	t.Helper()

	feed := rss.RSSFeed{}
	feed.Channel.Title = "Test"
	feed.Channel.Items = items

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		if err := xml.NewEncoder(w).Encode(feed); err != nil {
			t.Logf("failed to encode rss feed: %v", err)
		}
	}))
	t.Cleanup(rssServer.Close)

	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(mastodonCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"id": "1"}); err != nil {
				t.Logf("failed to encode mastodon response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(mastodonServer.Close)

	return rssServer.URL, mastodonServer.URL
}

func TestRun_PostNewEntriesOnly_SkipsOldPubDates(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32

	now := time.Now().UTC()
	oldTime := now.Add(-48 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")
	recentTime := now.Add(1 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")

	items := []rss.RSSItem{
		{Title: "Old Post", Link: "https://example.com/old-post", Content: "old content", PubDate: oldTime},
		{Title: "New Post", Link: "https://example.com/new-post", Content: "new content", PubDate: recentTime},
	}

	rssURL, mastodonURL := pubDateTestServers(t, items, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   true,
		DBPath:               dbFile,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(0), atomic.LoadInt32(&mastodonCalls),
		"PostNewEntriesOnly should seed the entire first snapshot without posting")
}

func TestRun_PostNewEntriesOnly_AllowsNewPubDates(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32

	now := time.Now().UTC()
	futureTime := now.Add(1 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")

	items := []rss.RSSItem{
		{Title: "Future Post", Link: "https://example.com/future-post", Content: "future content", PubDate: futureTime},
	}

	rssURL, mastodonURL := pubDateTestServers(t, items, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   true,
		DBPath:               dbFile,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(0), atomic.LoadInt32(&mastodonCalls),
		"PostNewEntriesOnly should seed future-dated entries in the first snapshot without posting")
}

func TestRun_PostNewEntriesOnly_NoPubDatePostsAll(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32

	items := []rss.RSSItem{
		{Title: "No Date Post", Link: "https://example.com/no-date-post", Content: "content"},
	}

	rssURL, mastodonURL := pubDateTestServers(t, items, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   true,
		DBPath:               dbFile,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(0), atomic.LoadInt32(&mastodonCalls),
		"PostNewEntriesOnly should seed entries without pubDate instead of posting them")
}

func TestRun_PostNewEntriesOnlyDisabled_PostsAllRegardlessOfPubDate(t *testing.T) {
	dbFile := setupRunTestDB(t)

	var mastodonCalls int32

	now := time.Now().UTC()
	oldTime := now.Add(-48 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")

	items := []rss.RSSItem{
		{Title: "Old Post", Link: "https://example.com/old-post-dis", Content: "old content", PubDate: oldTime},
	}

	rssURL, mastodonURL := pubDateTestServers(t, items, &mastodonCalls)

	conf := config.Config{
		FeedURL:              rssURL,
		Interval:             60,
		ShortRun:             true,
		PostNewEntriesOnly:   false,
		DBPath:               dbFile,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          mastodonURL,
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}

	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(conf))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit within 5s")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&mastodonCalls),
		"When PostNewEntriesOnly is disabled, posts should not be filtered by pubDate")
}

func TestRun_PostNewEntriesOnlyPostsUnseenBackdatedItem(t *testing.T) {
	dbFile := setupRunTestDB(t)
	oldDate := time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC1123Z)
	items := []rss.RSSItem{{Title: "Baseline", Link: "https://example.com/baseline", Content: "old", PubDate: oldDate}}

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		feed := rss.RSSFeed{}
		feed.Channel.Items = items
		_ = xml.NewEncoder(w).Encode(feed)
	}))
	defer rssServer.Close()
	var calls int32
	mastodonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer mastodonServer.Close()

	conf := config.Config{
		FeedURL: rssServer.URL, Interval: 1, ShortRun: true, PostNewEntriesOnly: true,
		DBPath: dbFile, SocialSites: []string{"mastodon"}, MastodonURL: mastodonServer.URL,
		MastodonClientKey: "key", MastodonClientSecret: "secret", MastodonAccessToken: "token",
	}
	assert.NoError(t, Run(conf))
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "initial snapshot must only be seeded")

	items = append(items, rss.RSSItem{
		Title: "Published during downtime", Link: "https://example.com/downtime",
		Content: "new to this installation", PubDate: oldDate,
	})
	assert.NoError(t, Run(conf))
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "DB absence, not pubDate, identifies a new item")
}

func TestRun_ShortRunReturnsFeedFailure(t *testing.T) {
	dbFile := setupRunTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	conf := config.Config{
		FeedURL: server.URL, Interval: 1, ShortRun: true, DBPath: dbFile,
		SocialSites: []string{"mastodon"}, MastodonURL: "https://social.example.com",
		MastodonClientKey: "key", MastodonClientSecret: "secret", MastodonAccessToken: "token",
	}
	if err := Run(conf); err == nil {
		t.Fatal("Run() error = nil, want feed failure")
	}
}

func TestRunReturnsDatabaseOpenFailure(t *testing.T) {
	conf := config.Config{
		FeedURL: "https://example.com/feed.xml", Interval: 1, ShortRun: true,
		DBPath:      filepath.Join(t.TempDir(), "missing", "database.sqlite"),
		SocialSites: []string{"mastodon"}, MastodonURL: "https://social.example.com",
		MastodonClientKey: "key", MastodonClientSecret: "secret", MastodonAccessToken: "token",
	}
	if err := Run(conf); err == nil {
		t.Fatal("Run() error = nil, want database-open failure")
	}
}

func TestRunContextPacesFeedFailures(t *testing.T) {
	dbFile := setupRunTestDB(t)
	var calls int32
	firstCall := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		select {
		case firstCall <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunContext(ctx, config.Config{
			FeedURL: server.URL, Interval: 1, DBPath: dbFile,
			SocialSites: []string{"mastodon"}, MastodonURL: "https://social.example.com",
			MastodonClientKey: "key", MastodonClientSecret: "secret", MastodonAccessToken: "token",
		})
	}()
	<-firstCall
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "feed failures must wait before retrying")
	cancel()
	assert.ErrorIs(t, <-done, context.Canceled)
}

func TestNewestPostsSortsByPublicationDate(t *testing.T) {
	posts := []rss.RSSItem{
		{Link: "https://example.com/old", PubDate: "Mon, 01 Jan 2024 00:00:00 +0000"},
		{Link: "https://example.com/new", PubDate: "Wed, 03 Jan 2024 00:00:00 +0000"},
		{Link: "https://example.com/middle", PubDate: "Tue, 02 Jan 2024 00:00:00 +0000"},
	}
	got := newestPosts(posts, 2)
	assert.Equal(t, []string{"https://example.com/new", "https://example.com/middle"},
		[]string{got[0].Link, got[1].Link})
}
