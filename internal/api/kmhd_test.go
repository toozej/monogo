package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/toozej/kmhd2spotify/pkg/config"
)

func createTestConfig() config.KMHDConfig {
	return config.KMHDConfig{
		APIEndpoint: "https://www.kmhd.org/pf/api/v3/content/fetch/playlist",
		HTTPTimeout: 30,
	}
}

func createTestClient() *KMHDAPIClient {
	cfg := createTestConfig()
	client := NewKMHDAPIClient(cfg)

	// Set logger to error level to reduce test noise
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	client.logger = logger.WithField("component", "kmhd_api_client")

	return client
}

func TestNewKMHDAPIClient(t *testing.T) {
	tests := []struct {
		name     string
		config   config.KMHDConfig
		validate func(*testing.T, *KMHDAPIClient)
	}{
		{
			name: "valid configuration",
			config: config.KMHDConfig{
				APIEndpoint: "https://www.kmhd.org/pf/api/v3/content/fetch/playlist",
				HTTPTimeout: 30,
			},
			validate: func(t *testing.T, client *KMHDAPIClient) {
				assert.NotNil(t, client)
				assert.Equal(t, "https://www.kmhd.org/pf/api/v3/content/fetch/playlist", client.baseURL)
				assert.NotNil(t, client.httpClient)
				assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
				assert.NotNil(t, client.logger)
			},
		},
		{
			name: "empty API endpoint uses default",
			config: config.KMHDConfig{
				APIEndpoint: "",
				HTTPTimeout: 30,
			},
			validate: func(t *testing.T, client *KMHDAPIClient) {
				assert.NotNil(t, client)
				assert.Equal(t, "https://www.kmhd.org/pf/api/v3/content/fetch/playlist", client.baseURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewKMHDAPIClient(tt.config)
			tt.validate(t, client)
		})
	}
}

func TestKMHDAPIClient_buildAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected string
	}{
		{
			name:     "standard date",
			date:     time.Date(2025, 10, 18, 19, 53, 11, 611000000, time.FixedZone("PDT", -7*3600)),
			expected: "https://www.kmhd.org/pf/api/v3/content/fetch/playlist?query=%7B%22day%22%3A%222025-10-18T19%3A53%3A11.611-07%3A00%22%7D",
		},
		{
			name:     "different timezone",
			date:     time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "https://www.kmhd.org/pf/api/v3/content/fetch/playlist?query=%7B%22day%22%3A%222025-01-01T04%3A00%3A00.000-08%3A00%22%7D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient()
			result := client.buildAPIURL(tt.date)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKMHDAPIClient_parseISO8601Timestamp(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   string
		expectError bool
		validate    func(*testing.T, time.Time)
	}{
		{
			name:        "RFC3339 format",
			timestamp:   "2025-10-18T19:53:11Z",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2025, result.Year())
				assert.Equal(t, time.October, result.Month())
				assert.Equal(t, 18, result.Day())
				assert.Equal(t, 19, result.Hour())
				assert.Equal(t, 53, result.Minute())
				assert.Equal(t, 11, result.Second())
			},
		},
		{
			name:        "RFC3339 with timezone",
			timestamp:   "2025-10-18T19:53:11-07:00",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2025, result.Year())
				assert.Equal(t, time.October, result.Month())
				assert.Equal(t, 18, result.Day())
			},
		},
		{
			name:        "with milliseconds",
			timestamp:   "2025-10-18T19:53:11.611Z",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2025, result.Year())
				assert.Equal(t, 611000000, result.Nanosecond())
			},
		},
		{
			name:        "with milliseconds and timezone",
			timestamp:   "2025-10-18T19:53:11.611-07:00",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2025, result.Year())
			},
		},
		{
			name:        "invalid format",
			timestamp:   "invalid-timestamp",
			expectError: true,
			validate:    nil,
		},
		{
			name:        "empty string",
			timestamp:   "",
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient()
			result, err := client.parseISO8601Timestamp(tt.timestamp)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestKMHDAPIClient_mapTrackToSong(t *testing.T) {
	tests := []struct {
		name           string
		artistName     string
		trackName      string
		collectionName string
		startTime      string
		rawJSON        string
		validate       func(*testing.T, *KMHDAPIClient, string, string, string, string, string)
	}{
		{
			name:           "complete track data",
			artistName:     "Miles Davis",
			trackName:      "So What",
			collectionName: "Kind of Blue",
			startTime:      "2025-10-18T19:53:11.611-07:00",
			rawJSON:        `{"artistName":"Miles Davis","trackName":"So What"}`,
			validate: func(t *testing.T, client *KMHDAPIClient, artist, track, album, startTime, rawJSON string) {
				song, err := client.mapTrackToSong(artist, track, album, startTime, rawJSON)
				require.NoError(t, err)
				assert.Equal(t, "Miles Davis", song.Artist)
				assert.Equal(t, "So What", song.Title)
				assert.Equal(t, "Kind of Blue", song.Album)
				assert.Equal(t, rawJSON, song.RawText)
				assert.False(t, song.PlayedAt.IsZero())
			},
		},
		{
			name:           "minimal track data",
			artistName:     "John Coltrane",
			trackName:      "Giant Steps",
			collectionName: "",
			startTime:      "",
			rawJSON:        `{"artistName":"John Coltrane","trackName":"Giant Steps"}`,
			validate: func(t *testing.T, client *KMHDAPIClient, artist, track, album, startTime, rawJSON string) {
				song, err := client.mapTrackToSong(artist, track, album, startTime, rawJSON)
				require.NoError(t, err)
				assert.Equal(t, "John Coltrane", song.Artist)
				assert.Equal(t, "Giant Steps", song.Title)
				assert.Equal(t, "", song.Album)
				assert.False(t, song.PlayedAt.IsZero()) // Should use current time
			},
		},
		{
			name:           "whitespace trimming",
			artistName:     "  Billie Holiday  ",
			trackName:      "  Strange Fruit  ",
			collectionName: "  Lady in Satin  ",
			startTime:      "2025-10-18T19:53:11Z",
			rawJSON:        `{"artistName":"  Billie Holiday  ","trackName":"  Strange Fruit  "}`,
			validate: func(t *testing.T, client *KMHDAPIClient, artist, track, album, startTime, rawJSON string) {
				song, err := client.mapTrackToSong(artist, track, album, startTime, rawJSON)
				require.NoError(t, err)
				assert.Equal(t, "Billie Holiday", song.Artist)
				assert.Equal(t, "Strange Fruit", song.Title)
				assert.Equal(t, "Lady in Satin", song.Album)
			},
		},
		{
			name:           "invalid timestamp uses current time",
			artistName:     "Nina Simone",
			trackName:      "Feeling Good",
			collectionName: "",
			startTime:      "invalid-timestamp",
			rawJSON:        `{"artistName":"Nina Simone","trackName":"Feeling Good"}`,
			validate: func(t *testing.T, client *KMHDAPIClient, artist, track, album, startTime, rawJSON string) {
				before := time.Now()
				song, err := client.mapTrackToSong(artist, track, album, startTime, rawJSON)
				after := time.Now()

				require.NoError(t, err)
				assert.Equal(t, "Nina Simone", song.Artist)
				assert.Equal(t, "Feeling Good", song.Title)
				// Should use current time when timestamp is invalid
				assert.True(t, song.PlayedAt.After(before) || song.PlayedAt.Equal(before))
				assert.True(t, song.PlayedAt.Before(after) || song.PlayedAt.Equal(after))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient()
			tt.validate(t, client, tt.artistName, tt.trackName, tt.collectionName, tt.startTime, tt.rawJSON)
		})
	}
}

func TestKMHDAPIClient_parseTrackObject(t *testing.T) {
	tests := []struct {
		name        string
		rawJSON     string
		expectError bool
		validate    func(*testing.T, *KMHDAPIClient, string)
	}{
		{
			name:        "complete track object",
			rawJSON:     `{"_id":"123","artistName":"Miles Davis","trackName":"So What","collectionName":"Kind of Blue","_start_time":"2025-10-18T19:53:11Z"}`,
			expectError: false,
			validate: func(t *testing.T, client *KMHDAPIClient, rawJSON string) {
				song, err := client.parseTrackObject(json.RawMessage(rawJSON))
				require.NoError(t, err)
				require.NotNil(t, song)
				assert.Equal(t, "Miles Davis", song.Artist)
				assert.Equal(t, "So What", song.Title)
				assert.Equal(t, "Kind of Blue", song.Album)
				assert.True(t, song.IsValid())
			},
		},
		{
			name:        "minimal track object",
			rawJSON:     `{"artistName":"John Coltrane","trackName":"Giant Steps"}`,
			expectError: false,
			validate: func(t *testing.T, client *KMHDAPIClient, rawJSON string) {
				song, err := client.parseTrackObject(json.RawMessage(rawJSON))
				require.NoError(t, err)
				require.NotNil(t, song)
				assert.Equal(t, "John Coltrane", song.Artist)
				assert.Equal(t, "Giant Steps", song.Title)
				assert.True(t, song.IsValid())
			},
		},
		{
			name:        "missing required fields",
			rawJSON:     `{"_id":"123","collectionName":"Some Album"}`,
			expectError: true,
			validate:    nil,
		},
		{
			name:        "malformed JSON",
			rawJSON:     `{"artistName":"Miles Davis","trackName":}`,
			expectError: true,
			validate:    nil,
		},
		{
			name:        "empty artist name",
			rawJSON:     `{"artistName":"","trackName":"So What"}`,
			expectError: true,
			validate:    nil,
		},
		{
			name:        "empty track name",
			rawJSON:     `{"artistName":"Miles Davis","trackName":""}`,
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient()
			song, err := client.parseTrackObject(json.RawMessage(tt.rawJSON))

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, song)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, song)
				if tt.validate != nil {
					tt.validate(t, client, tt.rawJSON)
				}
			}
		})
	}
}

func TestKMHDAPIClient_parseResponse(t *testing.T) {
	tests := []struct {
		name          string
		apiResponse   APIResponse
		expectedSongs int
		validate      func(*testing.T, *KMHDAPIClient, APIResponse)
	}{
		{
			name: "valid response with multiple tracks",
			apiResponse: APIResponse{
				json.RawMessage(`{"artistName":"Miles Davis","trackName":"So What","collectionName":"Kind of Blue"}`),
				json.RawMessage(`{"artistName":"John Coltrane","trackName":"Giant Steps"}`),
				json.RawMessage(`{"artistName":"Billie Holiday","trackName":"Strange Fruit","collectionName":"Lady in Satin"}`),
			},
			expectedSongs: 3,
			validate: func(t *testing.T, client *KMHDAPIClient, response APIResponse) {
				collection, err := client.parseResponse(response)
				require.NoError(t, err)
				assert.Equal(t, 3, len(collection.Songs))
				assert.Equal(t, "kmhd_api", collection.Source)
				assert.False(t, collection.LastUpdated.IsZero())

				// Verify first song
				assert.Equal(t, "Miles Davis", collection.Songs[0].Artist)
				assert.Equal(t, "So What", collection.Songs[0].Title)
				assert.Equal(t, "Kind of Blue", collection.Songs[0].Album)
			},
		},
		{
			name: "response with invalid tracks",
			apiResponse: APIResponse{
				json.RawMessage(`{"artistName":"Miles Davis","trackName":"So What"}`),
				json.RawMessage(`{"artistName":"","trackName":"Invalid Song"}`),  // Invalid: empty artist
				json.RawMessage(`{"artistName":"John Coltrane","trackName":""}`), // Invalid: empty title
				json.RawMessage(`{"artistName":"Billie Holiday","trackName":"Strange Fruit"}`),
			},
			expectedSongs: 2, // Only 2 valid songs
			validate: func(t *testing.T, client *KMHDAPIClient, response APIResponse) {
				collection, err := client.parseResponse(response)
				require.NoError(t, err)
				assert.Equal(t, 2, len(collection.Songs))
			},
		},
		{
			name:          "empty response",
			apiResponse:   APIResponse{},
			expectedSongs: 0,
			validate: func(t *testing.T, client *KMHDAPIClient, response APIResponse) {
				collection, err := client.parseResponse(response)
				require.NoError(t, err)
				assert.Equal(t, 0, len(collection.Songs))
				assert.Equal(t, "kmhd_api", collection.Source)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient()
			tt.validate(t, client, tt.apiResponse)
		})
	}
}

func TestKMHDAPIClient_FetchPlaylist_HTTPErrors(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "404 not found",
			statusCode:    http.StatusNotFound,
			responseBody:  "Not Found",
			expectError:   true,
			errorContains: "API returned status 404",
		},
		{
			name:          "500 internal server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  "Internal Server Error",
			expectError:   true,
			errorContains: "API returned status 500",
		},
		{
			name:          "invalid JSON response",
			statusCode:    http.StatusOK,
			responseBody:  `{"invalid": json}`,
			expectError:   true,
			errorContains: "failed to decode JSON response",
		},
		{
			name:         "valid JSON response",
			statusCode:   http.StatusOK,
			responseBody: `[{"artistName":"Miles Davis","trackName":"So What"}]`,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
				if _, err := w.Write([]byte(tt.responseBody)); err != nil {
					http.Error(w, "Failed to write response", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			// Create client with test server URL
			client := createTestClient()
			client.baseURL = server.URL

			// Test FetchPlaylist
			collection, err := client.FetchPlaylist(time.Now())

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, collection)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collection)
			}
		})
	}
}

func TestKMHDAPIClient_FetchPlaylist_Success(t *testing.T) {
	// Create test server with valid response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "application/json, text/plain, */*", r.Header.Get("Accept"))
		// Verify that a Chrome user agent is being used (should contain Chrome and be appropriate for the OS)
		userAgent := r.Header.Get("User-Agent")
		assert.Contains(t, userAgent, "Chrome")
		assert.Contains(t, userAgent, "Mozilla/5.0")

		// Verify query parameter
		query := r.URL.Query().Get("query")
		assert.Contains(t, query, "day")

		// Return valid response
		response := `[
			{"artistName":"Miles Davis","trackName":"So What","collectionName":"Kind of Blue","_start_time":"2025-10-18T19:53:11Z"},
			{"artistName":"John Coltrane","trackName":"Giant Steps","_start_time":"2025-10-18T20:00:00Z"}
		]`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := createTestClient()
	client.baseURL = server.URL

	// Test FetchPlaylist
	collection, err := client.FetchPlaylist(time.Now())

	require.NoError(t, err)
	require.NotNil(t, collection)
	assert.Equal(t, 2, len(collection.Songs))
	assert.Equal(t, "kmhd_api", collection.Source)
	assert.False(t, collection.LastUpdated.IsZero())

	// Verify first song
	song1 := collection.Songs[0]
	assert.Equal(t, "Miles Davis", song1.Artist)
	assert.Equal(t, "So What", song1.Title)
	assert.Equal(t, "Kind of Blue", song1.Album)
	assert.True(t, song1.IsValid())

	// Verify second song
	song2 := collection.Songs[1]
	assert.Equal(t, "John Coltrane", song2.Artist)
	assert.Equal(t, "Giant Steps", song2.Title)
	assert.True(t, song2.IsValid())
}

// Benchmark tests
func BenchmarkKMHDAPIClient_parseTrackObject(b *testing.B) {
	client := createTestClient()
	rawJSON := json.RawMessage(`{"artistName":"Miles Davis","trackName":"So What","collectionName":"Kind of Blue","_start_time":"2025-10-18T19:53:11Z"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.parseTrackObject(rawJSON)
	}
}

func BenchmarkKMHDAPIClient_parseISO8601Timestamp(b *testing.B) {
	client := createTestClient()
	timestamp := "2025-10-18T19:53:11.611-07:00"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.parseISO8601Timestamp(timestamp)
	}
}

func BenchmarkKMHDAPIClient_buildAPIURL(b *testing.B) {
	client := createTestClient()
	date := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.buildAPIURL(date)
	}
}
func TestKMHDAPIClient_ScrapePlaylist(t *testing.T) {
	// Create a test server that returns valid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `[
			{
				"_id": "track1",
				"_duration": 180000,
				"_start_time": "2023-10-01T15:30:00.000-07:00",
				"artistName": "John Coltrane",
				"trackName": "Giant Steps",
				"collectionName": "Giant Steps"
			}
		]`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := createTestClient()
	client.baseURL = server.URL

	// Test ScrapePlaylist method (compatibility interface)
	collection, err := client.ScrapePlaylist()

	require.NoError(t, err)
	require.NotNil(t, collection)
	assert.Equal(t, 1, len(collection.Songs))
	assert.Equal(t, "John Coltrane", collection.Songs[0].Artist)
	assert.Equal(t, "Giant Steps", collection.Songs[0].Title)
	assert.Equal(t, "Giant Steps", collection.Songs[0].Album)
	assert.Equal(t, "kmhd_api", collection.Source)
}
