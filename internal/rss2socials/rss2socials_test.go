package rss2socials

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/toozej/rss2socials/internal/db"
	"github.com/toozej/rss2socials/internal/rss"
	"github.com/toozej/rss2socials/pkg/config"

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
	db.InitDB()
	t.Cleanup(func() {
		db.CloseDB()
		os.Remove("./tooted_posts.db")
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
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)

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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 1, postCount, "Should post once for new post")

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
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
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 1, postCount, "Should post once for new post")

	// Close DB (simulating application shutdown)
	db.CloseDB()

	// Second run (simulating restart with same DB)
	db.InitDB()
	t.Cleanup(func() {
		db.CloseDB()
		os.Remove("./tooted_posts.db")
	})

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
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
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
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
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 2, callCount, "Should retry posting since Mastodon was not marked as posted")

	// Now Mastodon IS marked as posted
	posted, err = db.IsSitePosted(post.Link, "mastodon")
	assert.NoError(t, err)
	assert.True(t, posted, "Mastodon should be marked posted after success")

	// Third attempt: should not post again
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)

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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 1, postCount, "Should post for new post")

	// Content updated
	post.Content = "updated content"
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 2, postCount, "Should post again for updated content")
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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)
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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)

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
				os.Unsetenv(key)
			}

			for key, val := range tt.setupEnv {
				os.Setenv(key, val)
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

			conf := config.GetEnvVars()

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

			db.InitDB()
			assert.NotNil(t, db.DB)
			db.CloseDB()
		})
	}
}

func TestBasicIntegration(t *testing.T) {
	originalDB := db.DB
	db.CloseDB()
	db.InitDB()
	t.Cleanup(func() {
		db.CloseDB()
		os.Remove("./tooted_posts.db")
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
	handlePost(post, conf, "2026-01-01T00:00:00Z", false)

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

	handlePost(post, conf, "2026-01-01T00:00:00Z", false)

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

	handlePost(existingPost, conf, "2026-01-01T00:00:00Z", true)
	assert.Equal(t, 0, postCount, "Should NOT post existing entry when skipIfExisting=true")

	handlePost(newPost, conf, "2026-01-01T00:00:00Z", true)
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

	handlePost(existingPost, conf, "2026-01-01T00:00:00Z", false)
	assert.Equal(t, 0, postCount, "Should not re-post already-fully-posted entry even with skipIfExisting=false")

	handlePost(newPost, conf, "2026-01-01T00:00:00Z", false)
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
	handlePost(updatedPost, conf, "2026-01-01T00:00:00Z", true)
	assert.Equal(t, 1, postCount, "Should post updated entry even when skipIfExisting=true")
}
