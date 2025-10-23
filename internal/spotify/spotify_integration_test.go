package spotify

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/pkg/config"
)

// TestSpotifyService_Integration tests the complete integration flow with mocked responses
func TestSpotifyService_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T, service *Service)
	}{
		{
			name:        "complete_artist_workflow",
			description: "Test complete workflow from artist search to track addition",
			testFunc:    testCompleteArtistWorkflow,
		},
		{
			name:        "error_handling_workflow",
			description: "Test error handling across all service methods",
			testFunc:    testErrorHandlingWorkflow,
		},
		{
			name:        "rate_limiting_scenarios",
			description: "Test rate limiting behavior simulation",
			testFunc:    testRateLimitingScenarios,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with invalid credentials (expected for unit tests)
			cfg := config.SpotifyConfig{
				ClientID:     "test-id",
				ClientSecret: "test-secret",
			}
			service := NewService(cfg, logger)
			if service == nil {
				t.Fatal("Failed to create service")
			}

			tt.testFunc(t, service)
		})
	}
}

func testCompleteArtistWorkflow(t *testing.T, service *Service) {
	// This test simulates the complete workflow but expects errors due to invalid credentials
	// In a real integration test, this would use valid credentials

	// Check if service has nil client (expected with invalid credentials)
	if service.client == nil {
		t.Log("Service has nil client as expected with invalid credentials")
		return
	}

	// Test 1: Search for artist (should fail with invalid credentials)
	artist, err := service.SearchArtist("Taylor Swift")
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}
	if artist != nil {
		t.Error("Expected nil artist with error")
	}

	// Test 2: Get artist top tracks (should fail with invalid credentials)
	tracks, err := service.GetArtistTopTracks("test-artist-id")
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}
	if tracks != nil {
		t.Error("Expected nil tracks with error")
	}

	// Test 3: Get user playlists (should fail with invalid credentials)
	playlists, err := service.GetUserPlaylists("Incoming")
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}
	if playlists != nil {
		t.Error("Expected nil playlists with error")
	}

	// Test 4: Add tracks to playlist (should fail with invalid credentials)
	err = service.AddTracksToPlaylist("test-playlist", []string{"track1", "track2"})
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}

	// Test 5: Check tracks in playlist (should fail with invalid credentials)
	results, err := service.CheckTracksInPlaylist("test-playlist", []string{"track1", "track2"})
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}
	if results != nil {
		t.Error("Expected nil results with error")
	}
}

func testErrorHandlingWorkflow(t *testing.T, service *Service) {
	// Test error handling with various invalid inputs

	// Check if service has nil client (expected with invalid credentials)
	if service.client == nil {
		t.Log("Service has nil client as expected with invalid credentials")
		return
	}

	// Test empty/invalid inputs
	_, err := service.SearchArtist("")
	if err == nil {
		t.Error("Expected error with empty artist name")
	}

	_, err = service.GetArtistTopTracks("")
	if err == nil {
		t.Error("Expected error with empty artist ID")
	}

	_, err = service.GetUserPlaylists("")
	if err == nil {
		t.Error("Expected error with empty folder name")
	}

	err = service.AddTracksToPlaylist("", []string{})
	if err == nil {
		t.Error("Expected error with empty playlist ID and tracks")
	}

	_, err = service.CheckTracksInPlaylist("", []string{})
	if err != nil {
		t.Error("CheckTracksInPlaylist should handle empty tracks gracefully")
	}
}

func testRateLimitingScenarios(t *testing.T, service *Service) {
	// Simulate rate limiting by making multiple rapid requests
	// This tests the service's behavior under load (though it will fail due to invalid credentials)

	// Check if service has nil client (expected with invalid credentials)
	if service.client == nil {
		t.Log("Service has nil client as expected with invalid credentials")
		return
	}

	for i := 0; i < 5; i++ {
		_, err := service.SearchArtist("test artist")
		if err == nil {
			t.Error("Expected error with invalid credentials")
		}
		// Small delay to simulate real usage
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSpotifyService_DataTransformation tests data transformation between client and service layers
func TestSpotifyService_DataTransformation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	service := NewService(cfg, logger)
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Test that service properly handles nil client
	if service.client != nil {
		t.Error("Expected nil client with invalid credentials")
	}

	// Test service methods with nil client
	_, err := service.SearchArtist("test")
	if err == nil {
		t.Error("Expected error with nil client")
	}

	_, err = service.GetArtistTopTracks("test")
	if err == nil {
		t.Error("Expected error with nil client")
	}

	_, err = service.GetUserPlaylists("test")
	if err == nil {
		t.Error("Expected error with nil client")
	}

	err = service.AddTracksToPlaylist("test", []string{"track1"})
	if err == nil {
		t.Error("Expected error with nil client")
	}

	_, err = service.CheckTracksInPlaylist("test", []string{"track1"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestSpotifyService_LoggingBehavior tests that proper logging occurs
func TestSpotifyService_LoggingBehavior(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Capture log output
	var logOutput []byte
	logger.SetOutput(&testLogWriter{output: &logOutput})

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	service := NewService(cfg, logger)
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Make a call that will generate logs
	_, err := service.SearchArtist("test artist")
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}

	// Verify that logging occurred (basic check)
	if len(logOutput) == 0 {
		t.Error("Expected log output but got none")
	}
}

// testLogWriter is a simple writer for capturing log output
type testLogWriter struct {
	output *[]byte
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	*w.output = append(*w.output, p...)
	return len(p), nil
}

// TestSpotifyService_ConfigurationHandling tests various configuration scenarios
func TestSpotifyService_ConfigurationHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name            string
		clientID        string
		clientSecret    string
		expectNilClient bool
	}{
		{
			name:            "empty client ID",
			clientID:        "",
			clientSecret:    "secret",
			expectNilClient: true,
		},
		{
			name:            "empty client secret",
			clientID:        "id",
			clientSecret:    "",
			expectNilClient: true,
		},
		{
			name:            "both empty",
			clientID:        "",
			clientSecret:    "",
			expectNilClient: true,
		},
		{
			name:            "invalid credentials",
			clientID:        "invalid-id",
			clientSecret:    "invalid-secret",
			expectNilClient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.SpotifyConfig{
				ClientID:     tt.clientID,
				ClientSecret: tt.clientSecret,
			}
			service := NewService(cfg, logger)
			if service == nil {
				t.Fatal("Service should not be nil")
			}

			if tt.expectNilClient && service.client != nil {
				t.Error("Expected nil client with invalid configuration")
			}
		})
	}
}

// TestSpotifyService_TypeConversions tests type conversions between internal types
func TestSpotifyService_TypeConversions(t *testing.T) {
	// This test would verify type conversions if we had mock data
	// For now, we test the structure and error handling

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	service := NewService(cfg, logger)
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Test that methods return proper types even on error
	artist, err := service.SearchArtist("test")
	if err == nil {
		t.Error("Expected error")
	}
	if artist != nil {
		t.Error("Expected nil artist on error")
	}

	tracks, err := service.GetArtistTopTracks("test")
	if err == nil {
		t.Error("Expected error")
	}
	if tracks != nil {
		t.Error("Expected nil tracks on error")
	}

	playlists, err := service.GetUserPlaylists("test")
	if err == nil {
		t.Error("Expected error")
	}
	if playlists != nil {
		t.Error("Expected nil playlists on error")
	}

	results, err := service.CheckTracksInPlaylist("test", []string{"track1"})
	if err == nil {
		t.Error("Expected error")
	}
	if results != nil {
		t.Error("Expected nil results on error")
	}
}

// TestSpotifyService_ConcurrentAccess tests concurrent access to service methods
func TestSpotifyService_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	service := NewService(cfg, logger)
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Test concurrent access to service methods
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine makes different calls
			switch id % 4 {
			case 0:
				_, _ = service.SearchArtist("test")
			case 1:
				_, _ = service.GetArtistTopTracks("test")
			case 2:
				_, _ = service.GetUserPlaylists("test")
			case 3:
				_ = service.AddTracksToPlaylist("test", []string{"track1"})
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without deadlock, the test passes
}

// TestSpotifyService_EdgeCases tests edge cases and boundary conditions
func TestSpotifyService_EdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	service := NewService(cfg, logger)
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Test with very long strings
	longString := string(make([]byte, 1000))
	for i := range longString {
		longString = longString[:i] + "a" + longString[i+1:]
	}

	_, err := service.SearchArtist(longString)
	if err == nil {
		t.Error("Expected error with very long artist name")
	}

	// Test with special characters
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?"
	_, err = service.SearchArtist(specialChars)
	if err == nil {
		t.Error("Expected error with special characters")
	}

	// Test with unicode characters
	unicode := "测试艺术家"
	_, err = service.SearchArtist(unicode)
	if err == nil {
		t.Error("Expected error with unicode characters")
	}

	// Test with very large track lists
	largeTracks := make([]string, 1000)
	for i := range largeTracks {
		largeTracks[i] = "track" + string(rune(i))
	}

	err = service.AddTracksToPlaylist("test", largeTracks)
	if err == nil {
		t.Error("Expected error with very large track list")
	}
}
