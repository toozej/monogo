package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDB(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	assert.NotNil(t, DB, "DB should be initialized")

	sqlDB, err := DB.DB()
	require.NoError(t, err)
	err = sqlDB.Ping()
	assert.NoError(t, err, "DB should be reachable")
}

func TestCloseDB(t *testing.T) {
	InitDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	assert.NotNil(t, DB, "DB should be initialized before close")

	CloseDB()

	sqlDB, err := DB.DB()
	if err == nil {
		assert.Error(t, sqlDB.Ping(), "DB should be closed and unreachable")
	}
}

func TestInitDB_CustomPath(t *testing.T) {
	customPath := "./test_custom.db"
	InitDB(customPath)
	defer CloseDB()
	defer func() { _ = os.Remove(customPath) }()

	assert.NotNil(t, DB, "DB should be initialized with custom path")
}

func TestStoreTootedPost_NewPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/test-post", "Test post content", "2026-01-01T00:00:00Z")
	assert.NoError(t, err)

	var post TootedPost
	result := DB.Where("link = ?", "https://example.com/test-post").First(&post)
	assert.NoError(t, result.Error)
	assert.Equal(t, "https://example.com/test-post", post.Link)
	assert.NotEmpty(t, post.ContentHash)
	assert.Equal(t, "2026-01-01T00:00:00Z", post.StartupTime)
}

func TestStoreTootedPost_UpdateExisting(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/test-post", "Original content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	err = StoreTootedPost("https://example.com/test-post", "Updated content", "2026-01-02T00:00:00Z")
	assert.NoError(t, err)

	var post TootedPost
	result := DB.Where("link = ?", "https://example.com/test-post").First(&post)
	assert.NoError(t, result.Error)
	assert.Equal(t, "2026-01-02T00:00:00Z", post.StartupTime)
}

func TestHasPostChanged_NewPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	exists, updated, err := HasPostChanged("https://example.com/test-post-2", "Test post 2 content")
	assert.NoError(t, err)
	assert.False(t, exists, "Expected post to be new")
	assert.False(t, updated, "Expected post to be new, not updated")
}

func TestHasPostChanged_UpdatedPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/test-post", "Original content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	exists, updated, err := HasPostChanged("https://example.com/test-post", "Updated content")
	assert.NoError(t, err)
	assert.True(t, exists, "Expected post to exist")
	assert.True(t, updated, "Expected post to be updated")
}

func TestHasPostChanged_UnchangedPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/test-post", "Test post content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	exists, updated, err := HasPostChanged("https://example.com/test-post", "Test post content")
	assert.NoError(t, err)
	assert.True(t, exists, "Expected post to exist")
	assert.False(t, updated, "Expected post to be unchanged")
}

func TestMarkSitePosted(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/mark-test", "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	sites := []string{"mastodon", "bluesky", "threads"}
	for _, site := range sites {
		posted, err := IsSitePosted("https://example.com/mark-test", site)
		require.NoError(t, err)
		assert.False(t, posted, "Expected %s to not be posted yet", site)

		err = MarkSitePosted("https://example.com/mark-test", site)
		require.NoError(t, err)

		posted, err = IsSitePosted("https://example.com/mark-test", site)
		require.NoError(t, err)
		assert.True(t, posted, "Expected %s to be posted after marking", site)
	}
}

func TestMarkSitePosted_UnknownSite(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/test", "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	err = MarkSitePosted("https://example.com/test", "unknown_site")
	assert.Error(t, err, "Expected error for unknown site")
}

func TestMarkSitePosted_NonExistentLink(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := MarkSitePosted("https://example.com/nonexistent", "mastodon")
	assert.Error(t, err, "Expected error when marking non-existent link")
}

func TestIsSitePosted_UnknownSite(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	_, err := IsSitePosted("https://example.com/test", "unknown_site")
	assert.Error(t, err, "Expected error for unknown site")
}

func TestIsSitePosted_NonExistentLink(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	posted, err := IsSitePosted("https://example.com/nonexistent", "mastodon")
	assert.NoError(t, err)
	assert.False(t, posted, "Expected non-existent link to not be posted")
}

func TestMarkSitePosted_Independence(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/indep-test", "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	err = MarkSitePosted("https://example.com/indep-test", "mastodon")
	require.NoError(t, err)

	mastodonPosted, err := IsSitePosted("https://example.com/indep-test", "mastodon")
	require.NoError(t, err)
	assert.True(t, mastodonPosted, "Expected mastodon to be posted")

	blueskyPosted, err := IsSitePosted("https://example.com/indep-test", "bluesky")
	require.NoError(t, err)
	assert.False(t, blueskyPosted, "Expected bluesky to NOT be posted when only mastodon was marked")

	threadsPosted, err := IsSitePosted("https://example.com/indep-test", "threads")
	require.NoError(t, err)
	assert.False(t, threadsPosted, "Expected threads to NOT be posted when only mastodon was marked")
}

func TestStoreTootedPost_PreservesSiteFlagsOnConflict(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/reset-test", "original", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	err = MarkSitePosted("https://example.com/reset-test", "mastodon")
	require.NoError(t, err)

	mastodonPosted, err := IsSitePosted("https://example.com/reset-test", "mastodon")
	require.NoError(t, err)
	assert.True(t, mastodonPosted, "Expected mastodon to be posted after marking")

	err = StoreTootedPost("https://example.com/reset-test", "updated content", "2026-01-02T00:00:00Z")
	require.NoError(t, err)

	mastodonPosted, err = IsSitePosted("https://example.com/reset-test", "mastodon")
	require.NoError(t, err)
	assert.True(t, mastodonPosted, "Expected mastodon_posted to be preserved after StoreTootedPost with updated content")
}

func TestIsFirstCycle(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	assert.True(t, IsFirstCycle(), "Expected IsFirstCycle() to be true on empty DB")

	err := StoreTootedPost("https://example.com/first-cycle-test", "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	assert.False(t, IsFirstCycle(), "Expected IsFirstCycle() to be false after storing a post")
}

func TestIsFirstCycle_AfterDelete(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	err := StoreTootedPost("https://example.com/delete-test", "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.False(t, IsFirstCycle())

	DB.Where("link = ?", "https://example.com/delete-test").Delete(&TootedPost{})
	assert.True(t, IsFirstCycle(), "Expected IsFirstCycle() to be true after deleting all posts")
}

func TestStoreTootedPost_ContentHashConsistency(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	link := "https://example.com/hash-test"
	content := "consistent content"
	err := StoreTootedPost(link, content, "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	exists, updated, err := HasPostChanged(link, content)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.False(t, updated, "Same content should not be detected as updated")

	exists, updated, err = HasPostChanged(link, "different content")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, updated, "Different content should be detected as updated")
}

func TestMarkSitePosted_AllSitesSequentially(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	link := "https://example.com/all-sites"
	err := StoreTootedPost(link, "content", "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	for _, site := range []string{"mastodon", "bluesky", "threads"} {
		posted, err := IsSitePosted(link, site)
		require.NoError(t, err)
		assert.False(t, posted, "Expected %s to not be posted initially", site)
	}

	err = MarkSitePosted(link, "mastodon")
	require.NoError(t, err)
	err = MarkSitePosted(link, "bluesky")
	require.NoError(t, err)
	err = MarkSitePosted(link, "threads")
	require.NoError(t, err)

	for _, site := range []string{"mastodon", "bluesky", "threads"} {
		posted, err := IsSitePosted(link, site)
		require.NoError(t, err)
		assert.True(t, posted, "Expected %s to be posted after marking all", site)
	}
}

func TestTootedPost_GormModel(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer func() { _ = os.Remove("./tooted_posts.db") }()

	post := TootedPost{
		Link:           "https://example.com/model-test",
		ContentHash:    "abc123",
		Timestamp:      "2026-01-01T00:00:00Z",
		StartupTime:    "2026-01-01T00:00:00Z",
		MastodonPosted: false,
		BlueskyPosted:  false,
		ThreadsPosted:  false,
	}
	result := DB.Create(&post)
	require.NoError(t, result.Error)

	var retrieved TootedPost
	err := DB.Where("link = ?", "https://example.com/model-test").First(&retrieved).Error
	require.NoError(t, err)
	assert.Equal(t, post.Link, retrieved.Link)
	assert.Equal(t, post.ContentHash, retrieved.ContentHash)
	assert.False(t, retrieved.MastodonPosted)
	assert.False(t, retrieved.BlueskyPosted)
	assert.False(t, retrieved.ThreadsPosted)
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.Remove("./tooted_posts.db")
	_ = os.Remove("./test_custom.db")
	os.Exit(code)
}
