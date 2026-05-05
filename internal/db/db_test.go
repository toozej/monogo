package db

import (
	"os"
	"testing"
)

func TestInitDB(t *testing.T) {
	InitDB()
	defer CloseDB()

	_, err := DB.Exec("SELECT 1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestStoreTootedPost_NewPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/test-post", "Test post content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestHasPostChanged_NewPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	exists, updated, err := HasPostChanged("https://example.com/test-post-2", "Test post 2 content")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if exists {
		t.Errorf("Expected post to be new")
	}

	if updated {
		t.Errorf("Expected post to be new, not updated")
	}
}

func TestHasPostChanged_UpdatedPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/test-post", "Original content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	exists, updated, err := HasPostChanged("https://example.com/test-post", "Updated content")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !exists {
		t.Errorf("Expected post to exist")
	}

	if !updated {
		t.Errorf("Expected post to be updated")
	}
}

func TestHasPostChanged_UnchangedPost(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/test-post", "Test post content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	exists, updated, err := HasPostChanged("https://example.com/test-post", "Test post content")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !exists {
		t.Errorf("Expected post to exist")
	}

	if updated {
		t.Errorf("Expected post to be unchanged")
	}
}

func TestMarkSitePosted(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/mark-test", "content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("StoreTootedPost failed: %v", err)
	}

	sites := []string{"mastodon", "bluesky", "threads"}
	for _, site := range sites {
		posted, err := IsSitePosted("https://example.com/mark-test", site)
		if err != nil {
			t.Fatalf("IsSitePosted(%s) failed: %v", site, err)
		}
		if posted {
			t.Errorf("Expected %s to not be posted yet", site)
		}

		err = MarkSitePosted("https://example.com/mark-test", site)
		if err != nil {
			t.Fatalf("MarkSitePosted(%s) failed: %v", site, err)
		}

		posted, err = IsSitePosted("https://example.com/mark-test", site)
		if err != nil {
			t.Fatalf("IsSitePosted(%s) after mark failed: %v", site, err)
		}
		if !posted {
			t.Errorf("Expected %s to be posted after marking", site)
		}
	}
}

func TestMarkSitePosted_UnknownSite(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := MarkSitePosted("https://example.com/test", "unknown_site")
	if err == nil {
		t.Error("Expected error for unknown site, got nil")
	}
}

func TestIsSitePosted_UnknownSite(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	_, err := IsSitePosted("https://example.com/test", "unknown_site")
	if err == nil {
		t.Error("Expected error for unknown site, got nil")
	}
}

func TestIsSitePosted_NonExistentLink(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	posted, err := IsSitePosted("https://example.com/nonexistent", "mastodon")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if posted {
		t.Error("Expected non-existent link to not be posted")
	}
}

func TestMarkSitePosted_Independence(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/indep-test", "content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("StoreTootedPost failed: %v", err)
	}

	err = MarkSitePosted("https://example.com/indep-test", "mastodon")
	if err != nil {
		t.Fatalf("MarkSitePosted(mastodon) failed: %v", err)
	}

	mastodonPosted, err := IsSitePosted("https://example.com/indep-test", "mastodon")
	if err != nil {
		t.Fatalf("IsSitePosted(mastodon) failed: %v", err)
	}
	if !mastodonPosted {
		t.Error("Expected mastodon to be posted")
	}

	blueskyPosted, err := IsSitePosted("https://example.com/indep-test", "bluesky")
	if err != nil {
		t.Fatalf("IsSitePosted(bluesky) failed: %v", err)
	}
	if blueskyPosted {
		t.Error("Expected bluesky to NOT be posted when only mastodon was marked")
	}

	threadsPosted, err := IsSitePosted("https://example.com/indep-test", "threads")
	if err != nil {
		t.Fatalf("IsSitePosted(threads) failed: %v", err)
	}
	if threadsPosted {
		t.Error("Expected threads to NOT be posted when only mastodon was marked")
	}
}

func TestStoreTootedPost_ResetsSiteFlags(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	err := StoreTootedPost("https://example.com/reset-test", "original", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("StoreTootedPost failed: %v", err)
	}

	err = MarkSitePosted("https://example.com/reset-test", "mastodon")
	if err != nil {
		t.Fatalf("MarkSitePosted failed: %v", err)
	}

	mastodonPosted, err := IsSitePosted("https://example.com/reset-test", "mastodon")
	if err != nil {
		t.Fatalf("IsSitePosted failed: %v", err)
	}
	if !mastodonPosted {
		t.Fatal("Expected mastodon to be posted after marking")
	}

	err = StoreTootedPost("https://example.com/reset-test", "updated content", "2026-01-02T00:00:00Z")
	if err != nil {
		t.Fatalf("StoreTootedPost (update) failed: %v", err)
	}

	mastodonPosted, err = IsSitePosted("https://example.com/reset-test", "mastodon")
	if err != nil {
		t.Fatalf("IsSitePosted after re-store failed: %v", err)
	}
	if mastodonPosted {
		t.Error("Expected mastodon_posted to be reset to 0 after StoreTootedPost with updated content")
	}
}

func TestIsFirstCycle(t *testing.T) {
	InitDB()
	defer CloseDB()
	defer os.Remove("./tooted_posts.db")

	if !IsFirstCycle() {
		t.Error("Expected IsFirstCycle() to be true on empty DB")
	}

	err := StoreTootedPost("https://example.com/first-cycle-test", "content", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("StoreTootedPost failed: %v", err)
	}

	if IsFirstCycle() {
		t.Error("Expected IsFirstCycle() to be false after storing a post")
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Remove("./tooted_posts.db")
	os.Exit(code)
}
