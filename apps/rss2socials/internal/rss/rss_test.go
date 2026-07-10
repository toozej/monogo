package rss

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Table-driven tests for CheckRSSFeed with various scenarios
func TestCheckRSSFeed(t *testing.T) {
	tests := []struct {
		name          string
		xmlContent    string
		statusCode    int
		expectedPosts int
		expectedError bool
		expectedTitle string
	}{
		{
			name: "Valid RSS feed",
			xmlContent: `
				<rss>
					<channel>
						<title>Test Blog</title>
						<item>
							<title>Test Post</title>
							<link>https://example.com/test-post</link>
							<description>This is a test post</description>
						</item>
						<item>
							<title>Second Post</title>
							<link>https://example.com/second-post</link>
							<description>Second test post</description>
						</item>
					</channel>
				</rss>`,
			statusCode:    200,
			expectedPosts: 2,
			expectedError: false,
			expectedTitle: "Test Post",
		},
		{
			name: "Empty RSS feed",
			xmlContent: `
				<rss>
					<channel>
						<title>Empty Blog</title>
					</channel>
				</rss>`,
			statusCode:    200,
			expectedPosts: 0,
			expectedError: false,
		},
		{
			name:          "Invalid XML",
			xmlContent:    `Invalid XML content`,
			statusCode:    200,
			expectedPosts: 0,
			expectedError: true,
		},
		{
			name:          "HTTP error 404",
			xmlContent:    ``,
			statusCode:    404,
			expectedPosts: 0,
			expectedError: true,
		},
		{
			name: "RSS with different structure",
			xmlContent: `
				<rss>
					<channel>
						<title>Different Blog</title>
						<item>
							<title>Different Post</title>
							<link>https://example.com/different-post</link>
							<description>Different test post</description>
							<enclosure url="https://example.com/image.jpg" type="image/jpeg"/>
						</item>
					</channel>
				</rss>`,
			statusCode:    200,
			expectedPosts: 1,
			expectedError: false,
			expectedTitle: "Different Post",
		},
		{
			name: "Malformed URL in RSS",
			xmlContent: `
				<rss>
					<channel>
						<title>Malformed Blog</title>
						<item>
							<title>Malformed Post</title>
							<link>invalid-url</link>
							<description>Malformed URL test</description>
						</item>
					</channel>
				</rss>`,
			statusCode:    200,
			expectedPosts: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockHTTPServer(tt.xmlContent, tt.statusCode)
			defer server.Close()

			posts, err := CheckRSSFeed(server.URL)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPosts, len(posts))

			if tt.expectedPosts > 0 && tt.expectedTitle != "" {
				assert.Equal(t, tt.expectedTitle, posts[0].Title)
			}
		})
	}
}

func TestCheckRSSFeedRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxFeedBytes+1)))
	}))
	defer server.Close()

	if _, err := CheckRSSFeed(server.URL); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("CheckRSSFeed() error = %v, want size-limit error", err)
	}
}

func TestCheckRSSFeedRejectsTooManyItems(t *testing.T) {
	feed := RSSFeed{}
	for i := 0; i <= maxFeedItems; i++ {
		feed.Channel.Items = append(feed.Channel.Items, RSSItem{
			Title: "post", Link: "https://example.com/post", Content: "content",
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = xml.NewEncoder(w).Encode(feed)
	}))
	defer server.Close()

	if _, err := CheckRSSFeed(server.URL); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("CheckRSSFeed() error = %v, want item-limit error", err)
	}
}

// Test hash content function
func TestHashContent(t *testing.T) {
	content := "This is a test post"
	actualHash := HashContent(content)

	expectedHash := [32]byte{171, 214, 38, 231, 215, 166, 144, 206, 157, 133, 112, 100, 123, 136, 149, 247, 102, 45, 79, 114, 7, 254, 136, 203, 103, 200, 223, 156, 18, 75, 167, 165}

	assert.Equal(t, expectedHash[:], actualHash[:])
}

func TestParsePubDate(t *testing.T) {
	tests := []struct {
		name        string
		pubDate     string
		expectError bool
		expectYear  int
	}{
		{
			name:        "RFC 1123 format with numeric timezone",
			pubDate:     "Wed, 31 Jul 2024 21:06:33 +0000",
			expectError: false,
			expectYear:  2024,
		},
		{
			name:        "RFC 1123 format with named timezone",
			pubDate:     "Wed, 31 Jul 2024 21:06:33 UTC",
			expectError: false,
			expectYear:  2024,
		},
		{
			name:        "RFC 1123Z format",
			pubDate:     "Wed, 31 Jul 2024 21:06:33 -0700",
			expectError: false,
			expectYear:  2024,
		},
		{
			name:        "Single-digit day",
			pubDate:     "Wed, 2 Aug 2024 21:06:33 +0000",
			expectError: false,
			expectYear:  2024,
		},
		{
			name:        "Empty pubDate",
			pubDate:     "",
			expectError: true,
		},
		{
			name:        "Invalid date string",
			pubDate:     "not-a-date",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := RSSItem{PubDate: tt.pubDate}
			parsed, err := item.ParsePubDate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectYear, parsed.Year())
			}
		})
	}
}

func TestCheckRSSFeedWithPubDate(t *testing.T) {
	xmlContent := `
<rss>
<channel>
<title>Test Blog</title>
<item>
<title>Post One</title>
<link>https://example.com/post-1</link>
<description>Content 1</description>
<pubDate>Wed, 31 Jul 2024 21:06:33 +0000</pubDate>
</item>
<item>
<title>Post Two</title>
<link>https://example.com/post-2</link>
<description>Content 2</description>
<pubDate>Thu, 01 Aug 2024 12:00:00 +0000</pubDate>
</item>
<item>
<title>Post Without Date</title>
<link>https://example.com/post-3</link>
<description>Content 3</description>
</item>
</channel>
</rss>`

	server := mockHTTPServer(xmlContent, 200)
	defer server.Close()

	posts, err := CheckRSSFeed(server.URL)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(posts))
	assert.Equal(t, "Wed, 31 Jul 2024 21:06:33 +0000", posts[0].PubDate)
	assert.Equal(t, "Thu, 01 Aug 2024 12:00:00 +0000", posts[1].PubDate)
	assert.Equal(t, "", posts[2].PubDate)
}

// Helper function to mock an HTTP server
func mockHTTPServer(response string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
		_, _ = w.Write([]byte(response))

	}))
}
