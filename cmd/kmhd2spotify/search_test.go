package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/toozej/kmhd2spotify/internal/types"
)

func TestSearchSongs(t *testing.T) {
	tests := []struct {
		name     string
		songs    []types.Song
		query    string
		expected []types.Song
	}{
		{
			name:     "empty songs",
			songs:    []types.Song{},
			query:    "test",
			expected: nil,
		},
		{
			name: "match in artist",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
			},
			query: "Taylor",
			expected: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
			},
		},
		{
			name: "match in title",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
			},
			query: "Shape",
			expected: []types.Song{
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
			},
		},
		{
			name: "match in album",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
			},
			query: "Fearless",
			expected: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
			},
		},
		{
			name: "match in raw text",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless", RawText: "Some raw data"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide", RawText: "Other data"},
			},
			query: "raw",
			expected: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless", RawText: "Some raw data"},
			},
		},
		{
			name: "case insensitive match",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
			},
			query: "taylor",
			expected: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
			},
		},
		{
			name: "multiple matches",
			songs: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
				{Artist: "Taylor Swift", Title: "Blank Space", Album: "1989"},
			},
			query: "Taylor",
			expected: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Taylor Swift", Title: "Blank Space", Album: "1989"},
			},
		},
		{
			name:     "no matches",
			songs:    []types.Song{{Artist: "Taylor Swift", Title: "Love Story"}},
			query:    "nonexistent",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := searchSongs(tt.songs, tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplaySearchResults(t *testing.T) {
	tests := []struct {
		name     string
		matches  []types.Song
		query    string
		expected string
	}{
		{
			name:     "empty matches",
			matches:  []types.Song{},
			query:    "test",
			expected: "\n🔍 Search Results for 'test':\nFound 0 matching song(s):\n\n",
		},
		{
			name: "single match without played time",
			matches: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
			},
			query:    "Taylor",
			expected: "\n🔍 Search Results for 'Taylor':\nFound 1 matching song(s):\n\n1. 🎵 Taylor Swift - Love Story (Fearless)\n\n",
		},
		{
			name: "single match with played time",
			matches: []types.Song{
				{
					Artist:   "Taylor Swift",
					Title:    "Love Story",
					Album:    "Fearless",
					PlayedAt: time.Date(2023, 10, 1, 15, 30, 0, 0, time.UTC),
				},
			},
			query:    "Taylor",
			expected: "\n🔍 Search Results for 'Taylor':\nFound 1 matching song(s):\n\n1. 🎵 Taylor Swift - Love Story (Fearless)\n   📅 Played: Oct 1, 2023 15:30\n\n",
		},
		{
			name: "match with raw text",
			matches: []types.Song{
				{
					Artist:  "Taylor Swift",
					Title:   "Love Story",
					Album:   "Fearless",
					RawText: "Some raw data",
				},
			},
			query:    "Taylor",
			expected: "\n🔍 Search Results for 'Taylor':\nFound 1 matching song(s):\n\n1. 🎵 Taylor Swift - Love Story (Fearless)\n   📝 Raw: Some raw data\n\n",
		},
		{
			name: "multiple matches",
			matches: []types.Song{
				{Artist: "Taylor Swift", Title: "Love Story", Album: "Fearless"},
				{Artist: "Ed Sheeran", Title: "Shape of You", Album: "Divide"},
			},
			query:    "test",
			expected: "\n🔍 Search Results for 'test':\nFound 2 matching song(s):\n\n1. 🎵 Taylor Swift - Love Story (Fearless)\n\n2. 🎵 Ed Sheeran - Shape of You (Divide)\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that displaySearchResults doesn't panic
			assert.NotPanics(t, func() {
				displaySearchResults(tt.matches, tt.query)
			})
		})
	}
}

func TestNewSearchCmd(t *testing.T) {
	cmd := newSearchCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "search [query]", cmd.Use)
	assert.Equal(t, "Search for songs in KMHD playlist", cmd.Short)
	assert.Contains(t, cmd.Long, "fuzzy matching")
	assert.NotNil(t, cmd.Run)
}

func TestInitializeKMHDAPIClient(t *testing.T) {
	// This test assumes conf is set up properly from TestMain
	apiClient, err := initializeKMHDAPIClient()

	assert.NoError(t, err)
	assert.NotNil(t, apiClient)
}

func TestInitializeAllServices(t *testing.T) {
	// Skip if Spotify config is not properly set (which is expected in test environment)
	if conf.Spotify.ClientID == "" || conf.Spotify.ClientSecret == "" {
		t.Skip("Skipping test due to missing Spotify credentials")
	}

	kmhdScraper, spotifyService, fuzzySongSearcher, err := initializeAllServices()

	assert.NoError(t, err)
	assert.NotNil(t, kmhdScraper)
	assert.NotNil(t, spotifyService)
	assert.NotNil(t, fuzzySongSearcher)
}
