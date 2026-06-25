package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSong_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		song     Song
		expected bool
	}{
		{
			name: "valid song with artist and title",
			song: Song{
				Artist: "Miles Davis",
				Title:  "So What",
				Album:  "Kind of Blue",
			},
			expected: true,
		},
		{
			name: "valid song minimal fields",
			song: Song{
				Artist: "John Coltrane",
				Title:  "Giant Steps",
			},
			expected: true,
		},
		{
			name: "invalid song missing artist",
			song: Song{
				Title: "So What",
				Album: "Kind of Blue",
			},
			expected: false,
		},
		{
			name: "invalid song missing title",
			song: Song{
				Artist: "Miles Davis",
				Album:  "Kind of Blue",
			},
			expected: false,
		},
		{
			name: "invalid song empty artist",
			song: Song{
				Artist: "",
				Title:  "So What",
			},
			expected: false,
		},
		{
			name: "invalid song empty title",
			song: Song{
				Artist: "Miles Davis",
				Title:  "",
			},
			expected: false,
		},
		{
			name: "invalid song both fields empty",
			song: Song{
				Artist: "",
				Title:  "",
			},
			expected: false,
		},
		{
			name: "valid song with whitespace artist (current behavior)",
			song: Song{
				Artist: "   ",
				Title:  "So What",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.song.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSong_String(t *testing.T) {
	tests := []struct {
		name     string
		song     Song
		expected string
	}{
		{
			name: "song with album",
			song: Song{
				Artist: "Miles Davis",
				Title:  "So What",
				Album:  "Kind of Blue",
			},
			expected: "Miles Davis - So What (Kind of Blue)",
		},
		{
			name: "song without album",
			song: Song{
				Artist: "John Coltrane",
				Title:  "Giant Steps",
			},
			expected: "John Coltrane - Giant Steps",
		},
		{
			name: "song with empty album",
			song: Song{
				Artist: "Billie Holiday",
				Title:  "Strange Fruit",
				Album:  "",
			},
			expected: "Billie Holiday - Strange Fruit",
		},
		{
			name: "song with special characters",
			song: Song{
				Artist: "Artist with (parentheses)",
				Title:  "Title with - dash",
				Album:  "Album with / slash",
			},
			expected: "Artist with (parentheses) - Title with - dash (Album with / slash)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.song.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSongCollection_AddSong(t *testing.T) {
	collection := &SongCollection{
		Songs:       []Song{},
		LastUpdated: time.Now(),
		Source:      "test",
	}

	initialCount := len(collection.Songs)

	song := Song{
		Artist: "Test Artist",
		Title:  "Test Song",
		Album:  "Test Album",
	}

	collection.AddSong(song)

	assert.Equal(t, initialCount+1, len(collection.Songs))

	lastSong := collection.Songs[len(collection.Songs)-1]
	assert.Equal(t, song.Artist, lastSong.Artist)
	assert.Equal(t, song.Title, lastSong.Title)
	assert.Equal(t, song.Album, lastSong.Album)
}

func TestSongCollection_AddSong_EmptyCollection(t *testing.T) {
	collection := &SongCollection{
		Songs:       []Song{},
		LastUpdated: time.Now(),
		Source:      "test",
	}

	assert.Equal(t, 0, len(collection.Songs))

	song := Song{
		Artist: "First Artist",
		Title:  "First Song",
	}

	collection.AddSong(song)

	assert.Equal(t, 1, len(collection.Songs))
}

func TestSongCollection_AddSong_MultipleSongs(t *testing.T) {
	collection := &SongCollection{
		Songs:       []Song{},
		LastUpdated: time.Now(),
		Source:      "test",
	}

	songs := []Song{
		{Artist: "Artist 1", Title: "Song 1"},
		{Artist: "Artist 2", Title: "Song 2"},
		{Artist: "Artist 3", Title: "Song 3"},
	}

	for _, song := range songs {
		collection.AddSong(song)
	}

	assert.Equal(t, len(songs), len(collection.Songs))

	// Verify songs are in the correct order
	for i, expectedSong := range songs {
		actualSong := collection.Songs[i]
		assert.Equal(t, expectedSong.Artist, actualSong.Artist, "Song %d: artist mismatch", i)
		assert.Equal(t, expectedSong.Title, actualSong.Title, "Song %d: title mismatch", i)
	}
}

// Benchmark tests
func BenchmarkSong_IsValid(b *testing.B) {
	song := Song{
		Artist: "Miles Davis",
		Title:  "So What",
		Album:  "Kind of Blue",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		song.IsValid()
	}
}

func BenchmarkSong_String(b *testing.B) {
	song := Song{
		Artist: "Miles Davis",
		Title:  "So What",
		Album:  "Kind of Blue",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = song.String()
	}
}

func BenchmarkSongCollection_AddSong(b *testing.B) {
	collection := &SongCollection{
		Songs:       []Song{},
		LastUpdated: time.Now(),
		Source:      "test",
	}

	song := Song{
		Artist: "Test Artist",
		Title:  "Test Song",
		Album:  "Test Album",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collection.AddSong(song)
	}
}
