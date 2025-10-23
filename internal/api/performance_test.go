package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// TestLargeJSONResponsePerformance tests API response parsing performance with large JSON responses
func TestLargeJSONResponsePerformance(t *testing.T) {
	tests := []struct {
		name        string
		trackCount  int
		maxDuration time.Duration
	}{
		{
			name:        "small response (10 tracks)",
			trackCount:  10,
			maxDuration: 10 * time.Millisecond,
		},
		{
			name:        "medium response (100 tracks)",
			trackCount:  100,
			maxDuration: 50 * time.Millisecond,
		},
		{
			name:        "large response (1000 tracks)",
			trackCount:  1000,
			maxDuration: 500 * time.Millisecond,
		},
		{
			name:        "very large response (5000 tracks)",
			trackCount:  5000,
			maxDuration: 2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate large JSON response
			tracks := generateTestTracks(tt.trackCount)
			jsonResponse, err := json.Marshal(tracks)
			require.NoError(t, err)

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
				if _, err := w.Write(jsonResponse); err != nil {
					http.Error(w, "Failed to write response", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			// Create client
			client := createTestClient()
			client.baseURL = server.URL

			// Measure parsing performance
			start := time.Now()
			collection, err := client.FetchPlaylist(time.Now())
			duration := time.Since(start)

			// Validate results
			require.NoError(t, err)
			require.NotNil(t, collection)
			assert.Equal(t, tt.trackCount, len(collection.Songs))
			assert.True(t, duration < tt.maxDuration,
				"Parsing took %v, expected less than %v for %d tracks",
				duration, tt.maxDuration, tt.trackCount)

			t.Logf("Parsed %d tracks in %v (%.2f tracks/ms)",
				tt.trackCount, duration, float64(tt.trackCount)/float64(duration.Nanoseconds()/1e6))
		})
	}
}

// TestMemoryUsageDuringJSONProcessing validates memory usage during JSON processing
func TestMemoryUsageDuringJSONProcessing(t *testing.T) {
	// Force garbage collection before test
	runtime.GC()
	runtime.GC()

	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Generate large response (1000 tracks)
	tracks := generateTestTracks(1000)
	jsonResponse, err := json.Marshal(tracks)
	require.NoError(t, err)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
		if _, err := w.Write(jsonResponse); err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client and process response
	client := createTestClient()
	client.baseURL = server.URL

	collection, err := client.FetchPlaylist(time.Now())
	require.NoError(t, err)
	require.NotNil(t, collection)

	// Measure memory after processing
	runtime.ReadMemStats(&memAfter)

	// Calculate memory usage
	memUsed := memAfter.Alloc - memBefore.Alloc
	memUsedMB := float64(memUsed) / (1024 * 1024)

	// Validate reasonable memory usage (should be less than 10MB for 1000 tracks)
	assert.True(t, memUsedMB < 10.0,
		"Memory usage too high: %.2f MB for 1000 tracks", memUsedMB)

	t.Logf("Memory used: %.2f MB for %d tracks (%.2f KB per track)",
		memUsedMB, len(collection.Songs), (memUsedMB*1024)/float64(len(collection.Songs)))
}

// TestHourlySyncBehavior tests system behavior over extended periods with hourly sync
func TestHourlySyncBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extended sync test in short mode")
	}

	// Create test server that tracks requests
	requestCount := 0
	var requestTimes []time.Time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		requestTimes = append(requestTimes, time.Now())

		// Return different songs each time to simulate real behavior
		tracks := generateTestTracks(5)
		for i := range tracks {
			tracks[i]["artistName"] = fmt.Sprintf("Artist %d", requestCount)
			tracks[i]["trackName"] = fmt.Sprintf("Track %d", i+1)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(tracks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := createTestClient()
	client.baseURL = server.URL

	// Simulate multiple sync cycles over a short period (using seconds instead of hours for testing)
	testDuration := 5 * time.Second
	syncInterval := 1 * time.Second

	start := time.Now()
	for time.Since(start) < testDuration {
		collection, err := client.FetchPlaylist(time.Now())
		require.NoError(t, err)
		require.NotNil(t, collection)
		assert.Equal(t, 5, len(collection.Songs))

		time.Sleep(syncInterval)
	}

	// Validate sync behavior
	assert.True(t, requestCount >= 4, "Expected at least 4 requests in %v", testDuration)
	assert.True(t, requestCount <= 6, "Expected at most 6 requests in %v", testDuration)

	// Validate request timing intervals
	for i := 1; i < len(requestTimes); i++ {
		interval := requestTimes[i].Sub(requestTimes[i-1])
		assert.True(t, interval >= 900*time.Millisecond,
			"Request interval too short: %v", interval)
		assert.True(t, interval <= 1100*time.Millisecond,
			"Request interval too long: %v", interval)
	}

	t.Logf("Completed %d sync cycles in %v", requestCount, testDuration)
}

// TestDuplicatePreventionEffectiveness monitors duplicate prevention effectiveness with real data
func TestDuplicatePreventionEffectiveness(t *testing.T) {
	// Create test server that returns some duplicate tracks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mix of new and duplicate tracks
		tracks := []map[string]interface{}{
			{
				"artistName":     "Miles Davis",
				"trackName":      "So What",
				"collectionName": "Kind of Blue",
				"_start_time":    "2025-10-18T19:53:11Z",
			},
			{
				"artistName":  "John Coltrane",
				"trackName":   "Giant Steps",
				"_start_time": "2025-10-18T20:00:00Z",
			},
			{
				"artistName":     "Miles Davis", // Duplicate artist
				"trackName":      "So What",     // Duplicate song
				"collectionName": "Kind of Blue",
				"_start_time":    "2025-10-18T20:05:00Z",
			},
			{
				"artistName":  "Billie Holiday",
				"trackName":   "Strange Fruit",
				"_start_time": "2025-10-18T20:10:00Z",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(tracks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := createTestClient()
	client.baseURL = server.URL

	// Fetch playlist multiple times to test duplicate handling
	collections := make([]*types.SongCollection, 3)
	for i := 0; i < 3; i++ {
		collection, err := client.FetchPlaylist(time.Now())
		require.NoError(t, err)
		require.NotNil(t, collection)
		collections[i] = collection
	}

	// Validate that each collection contains the same songs (API returns same data)
	for i := 1; i < len(collections); i++ {
		assert.Equal(t, len(collections[0].Songs), len(collections[i].Songs),
			"Collection %d has different song count", i)
	}

	// Validate that duplicate songs are present in the raw data
	// (duplicate prevention happens at the sync level, not API level)
	collection := collections[0]
	assert.Equal(t, 4, len(collection.Songs), "Expected 4 songs including duplicates")

	// Count occurrences of "So What" by Miles Davis
	soWhatCount := 0
	for _, song := range collection.Songs {
		if song.Artist == "Miles Davis" && song.Title == "So What" {
			soWhatCount++
		}
	}
	assert.Equal(t, 2, soWhatCount, "Expected 2 occurrences of 'So What' in raw API data")

	t.Logf("Successfully processed %d songs with %d duplicates detected",
		len(collection.Songs), soWhatCount-1)
}

// TestAPIErrorRecovery tests system behavior during API outages or errors
func TestAPIErrorRecovery(t *testing.T) {
	errorCount := 0
	successCount := 0

	// Create test server that simulates intermittent failures
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorCount++

		// Fail first 3 requests, then succeed
		if errorCount <= 3 {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Return successful response
		successCount++
		tracks := generateTestTracks(2)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(tracks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := createTestClient()
	client.baseURL = server.URL

	// Test error recovery behavior
	var lastErr error
	var collection *types.SongCollection

	for i := 0; i < 5; i++ {
		var err error
		collection, err = client.FetchPlaylist(time.Now())
		lastErr = err

		if err == nil {
			break
		}

		// Brief pause between retries
		time.Sleep(100 * time.Millisecond)
	}

	// Validate recovery behavior
	assert.NoError(t, lastErr, "Expected eventual success after retries")
	assert.NotNil(t, collection, "Expected valid collection after recovery")
	assert.Equal(t, 2, len(collection.Songs), "Expected 2 songs after recovery")
	assert.Equal(t, 1, successCount, "Expected exactly 1 successful request")

	t.Logf("Recovered after %d failed attempts, got %d songs",
		errorCount-1, len(collection.Songs))
}

// TestConcurrentAPIRequests tests behavior under concurrent load
func TestConcurrentAPIRequests(t *testing.T) {
	var requestCount int64

	// Create test server that tracks concurrent requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		tracks := generateTestTracks(3)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(tracks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create multiple clients
	clients := make([]*KMHDAPIClient, 5)
	for i := range clients {
		clients[i] = createTestClient()
		clients[i].baseURL = server.URL
	}

	// Make concurrent requests
	results := make(chan error, len(clients))
	start := time.Now()

	for _, client := range clients {
		go func(c *KMHDAPIClient) {
			collection, err := c.FetchPlaylist(time.Now())
			if err != nil {
				results <- err
				return
			}
			if len(collection.Songs) != 3 {
				results <- fmt.Errorf("expected 3 songs, got %d", len(collection.Songs))
				return
			}
			results <- nil
		}(client)
	}

	// Wait for all requests to complete
	for i := 0; i < len(clients); i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent request %d failed", i)
	}

	duration := time.Since(start)

	// Validate concurrent behavior
	finalRequestCount := atomic.LoadInt64(&requestCount)
	assert.Equal(t, int64(len(clients)), finalRequestCount, "Expected all requests to complete")
	assert.True(t, duration < 200*time.Millisecond,
		"Concurrent requests took too long: %v", duration)

	t.Logf("Completed %d concurrent requests in %v", finalRequestCount, duration)
}

// generateTestTracks creates test track data for performance testing
func generateTestTracks(count int) []map[string]interface{} {
	tracks := make([]map[string]interface{}, count)

	artists := []string{"Miles Davis", "John Coltrane", "Billie Holiday", "Nina Simone", "Duke Ellington"}
	albums := []string{"Kind of Blue", "Giant Steps", "Lady in Satin", "Pastel Blues", "Ellington at Newport"}

	for i := 0; i < count; i++ {
		tracks[i] = map[string]interface{}{
			"_id":            fmt.Sprintf("track_%d", i),
			"_duration":      180000 + (i * 1000), // Vary duration
			"_start_time":    time.Now().Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			"artistName":     artists[i%len(artists)],
			"trackName":      fmt.Sprintf("Track %d", i+1),
			"collectionName": albums[i%len(albums)],
		}
	}

	return tracks
}

// BenchmarkLargeJSONParsing benchmarks JSON parsing performance
func BenchmarkLargeJSONParsing(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("tracks_%d", size), func(b *testing.B) {
			// Generate test data once
			tracks := generateTestTracks(size)
			jsonData, err := json.Marshal(tracks)
			require.NoError(b, err)

			client := createTestClient()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var apiResponse APIResponse
				err := json.Unmarshal(jsonData, &apiResponse)
				require.NoError(b, err)

				_, err = client.parseResponse(apiResponse)
				require.NoError(b, err)
			}
		})
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	client := createTestClient()
	tracks := generateTestTracks(100)
	jsonData, err := json.Marshal(tracks)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var apiResponse APIResponse
		_ = json.Unmarshal(jsonData, &apiResponse)
		_, _ = client.parseResponse(apiResponse)
	}
}
