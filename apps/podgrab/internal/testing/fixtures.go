// Package testhelpers provides test fixtures and mock data for unit and integration tests.
// This package contains RSS feed examples, test data constants, and helper utilities
// for creating consistent test scenarios.
package testhelpers

import "fmt"

// ValidRSSFeed is a valid RSS 2.0 podcast feed for testing.
const ValidRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Test Podcast</title>
    <description>A podcast for testing purposes</description>
    <itunes:author>Test Author</itunes:author>
    <itunes:image href="https://example.com/podcast-image.jpg"/>
    <link>https://example.com</link>
    <item>
      <title>Episode 1: Introduction</title>
      <description>The first test episode</description>
      <pubDate>Mon, 15 Jan 2024 10:00:00 GMT</pubDate>
      <enclosure url="https://example.com/episode1.mp3" length="25000000" type="audio/mpeg"/>
      <guid>test-podcast-episode-1</guid>
      <itunes:duration>1800</itunes:duration>
      <itunes:image href="https://example.com/episode1.jpg"/>
    </item>
    <item>
      <title>Episode 2: Deep Dive</title>
      <description>The second test episode with more content</description>
      <pubDate>Mon, 22 Jan 2024 10:00:00 GMT</pubDate>
      <enclosure url="https://example.com/episode2.mp3" length="30000000" type="audio/mpeg"/>
      <guid>test-podcast-episode-2</guid>
      <itunes:duration>2400</itunes:duration>
      <itunes:image href="https://example.com/episode2.jpg"/>
    </item>
  </channel>
</rss>`

// InvalidXMLFeed is an invalid XML document for testing error handling.
const InvalidXMLFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Broken Podcast
    <!-- Missing closing tags -->
  </channel>
`

// EmptyRSSFeed is a valid RSS feed with no episodes.
const EmptyRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Empty Podcast</title>
    <description>A podcast with no episodes yet</description>
    <itunes:author>Test Author</itunes:author>
    <itunes:image href="https://example.com/empty-podcast.jpg"/>
    <link>https://example.com/empty</link>
  </channel>
</rss>`

// RSSFeedWithItunesExtensions is a feed with comprehensive iTunes namespace tags.
const RSSFeedWithItunesExtensions = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
     xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"
     xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Advanced Test Podcast</title>
    <description>A podcast with iTunes extensions</description>
    <itunes:author>Advanced Test Author</itunes:author>
    <itunes:subtitle>Extended metadata testing</itunes:subtitle>
    <itunes:summary>This podcast includes comprehensive iTunes namespace tags for testing.</itunes:summary>
    <itunes:image href="https://example.com/advanced-podcast.jpg"/>
    <itunes:category text="Technology">
      <itunes:category text="Podcasting"/>
    </itunes:category>
    <itunes:explicit>no</itunes:explicit>
    <itunes:type>episodic</itunes:type>
    <link>https://example.com/advanced</link>
    <item>
      <title>Episode 1: Advanced Features</title>
      <description>Testing advanced RSS features</description>
      <content:encoded><![CDATA[<p>Rich HTML content goes here</p>]]></content:encoded>
      <pubDate>Wed, 10 Jan 2024 12:00:00 GMT</pubDate>
      <enclosure url="https://example.com/advanced-ep1.mp3" length="40000000" type="audio/mpeg"/>
      <guid isPermaLink="false">advanced-podcast-episode-1</guid>
      <itunes:episodeType>full</itunes:episodeType>
      <itunes:episode>1</itunes:episode>
      <itunes:season>1</itunes:season>
      <itunes:duration>00:45:00</itunes:duration>
      <itunes:image href="https://example.com/advanced-ep1.jpg"/>
      <itunes:explicit>no</itunes:explicit>
    </item>
  </channel>
</rss>`

// RSSFeedWithSpecialCharacters tests encoding and sanitization.
const RSSFeedWithSpecialCharacters = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Podcast with Special Characters: &lt;&gt;&amp;"'</title>
    <description>Testing &amp; encoding with special characters</description>
    <itunes:author>Author &amp; Co.</itunes:author>
    <itunes:image href="https://example.com/special-chars.jpg"/>
    <link>https://example.com/special</link>
    <item>
      <title>Episode: "Quotes" &amp; &lt;Brackets&gt;</title>
      <description>Testing special characters in episode &amp; title</description>
      <pubDate>Fri, 05 Jan 2024 14:00:00 GMT</pubDate>
      <enclosure url="https://example.com/special.mp3" length="20000000" type="audio/mpeg"/>
      <guid>special-chars-episode-1</guid>
      <itunes:duration>1200</itunes:duration>
    </item>
  </channel>
</rss>`

// GenerateLargeRSSFeed creates an RSS feed with the specified number of episodes for pagination testing.
func GenerateLargeRSSFeed(episodeCount int) string {
	header := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Large Test Podcast</title>
    <description>A podcast with many episodes for pagination testing</description>
    <itunes:author>Test Author</itunes:author>
    <itunes:image href="https://example.com/large-podcast.jpg"/>
    <link>https://example.com/large</link>`

	episodes := ""
	for i := 1; i <= episodeCount; i++ {
		episodeNum := fmt.Sprintf("%d", i)
		episodes += `
    <item>
      <title>Episode ` + episodeNum + `</title>
      <description>Episode ` + episodeNum + ` description</description>
      <pubDate>Mon, 01 Jan 2024 10:00:00 GMT</pubDate>
      <enclosure url="https://example.com/episode` + episodeNum + `.mp3" length="25000000" type="audio/mpeg"/>
      <guid>large-podcast-episode-` + episodeNum + `</guid>
      <itunes:duration>1800</itunes:duration>
    </item>`
	}

	footer := `
  </channel>
</rss>`

	return header + episodes + footer
}

// MockItunesSearchResponse is a mock response from iTunes API search.
const MockItunesSearchResponse = `{
  "resultCount": 2,
  "results": [
    {
      "collectionId": 123456,
      "collectionName": "Test Podcast from iTunes",
      "artistName": "iTunes Test Author",
      "artworkUrl600": "https://is1-ssl.mzstatic.com/image/thumb/Podcasts/test.jpg",
      "feedUrl": "https://example.com/itunes-feed.xml",
      "releaseDate": "2024-01-15T10:00:00Z",
      "trackCount": 10,
      "collectionViewUrl": "https://podcasts.apple.com/podcast/id123456"
    },
    {
      "collectionId": 789012,
      "collectionName": "Another Test Podcast",
      "artistName": "Another Test Author",
      "artworkUrl600": "https://is1-ssl.mzstatic.com/image/thumb/Podcasts/another.jpg",
      "feedUrl": "https://example.com/another-feed.xml",
      "releaseDate": "2024-01-10T08:00:00Z",
      "trackCount": 25,
      "collectionViewUrl": "https://podcasts.apple.com/podcast/id789012"
    }
  ]
}`

// MockItunesEmptyResponse is a mock iTunes API response with no results.
const MockItunesEmptyResponse = `{
  "resultCount": 0,
  "results": []
}`
