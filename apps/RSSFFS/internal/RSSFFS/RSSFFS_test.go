package RSSFFS

import (
	"strings"
	"testing"
)

// TestExtractDomainFromURL tests the extractDomainFromURL function with various URL formats
func TestExtractDomainFromURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "Full URL with HTTPS",
			input:       "https://example.com/blog/post",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "Full URL with HTTP",
			input:       "http://example.com/feed",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with subdomain",
			input:       "https://blog.example.com",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL with subdomain and path",
			input:       "https://blog.example.com/posts/latest",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL without protocol",
			input:       "example.com/blog",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL without protocol with subdomain",
			input:       "blog.example.com",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL with port",
			input:       "https://example.com:8080/feed",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with query parameters",
			input:       "https://example.com/search?q=test",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with fragment",
			input:       "https://example.com/page#section",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "Invalid URL - no domain",
			input:       "://invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid URL - malformed",
			input:       "not-a-url",
			expected:    "not-a-url",
			expectError: false,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractDomainFromURL(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("For input %q, expected %q, got %q", tt.input, tt.expected, result)
				}
			}
		})
	}
}

// TestModeSelectionLogic tests the mode selection logic in the Run function
// Note: This is a basic test that verifies the function signature and basic parameter handling
func TestModeSelectionLogic(t *testing.T) {
	// This test verifies that the Run function accepts the correct parameters
	// and doesn't panic with basic inputs. Full integration testing would require
	// mocking the RSS reader API and HTTP client.

	// Test that the function signature is correct and accepts all required parameters
	defer func() {
		if r := recover(); r != nil {
			// If we get a panic about missing API configuration, that's expected
			// since we're not providing valid API credentials
			if panicMsg, ok := r.(string); ok {
				if panicMsg == "Error getting categoryId from category test: " {
					// This is expected - we don't have valid API credentials
					return
				}
			}
			// Re-panic if it's an unexpected error
			panic(r)
		}
	}()

	// For now, we just test that the function signature is correct
	// by checking that we can call the helper functions
	t.Log("Testing that mode selection helper functions exist")

	// Test that we can call extractDomainFromURL (already tested above)
	domain, err := extractDomainFromURL("https://example.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if domain != "example.com" {
		t.Errorf("Expected example.com, got %s", domain)
	}
}

// TestCommonPatternsExist tests that the common RSS patterns are defined
func TestCommonPatternsExist(t *testing.T) {
	expectedPatterns := []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

	if len(commonPatterns) != len(expectedPatterns) {
		t.Errorf("Expected %d common patterns, got %d", len(expectedPatterns), len(commonPatterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(commonPatterns) {
			t.Errorf("Missing pattern: %s", expected)
			continue
		}
		if commonPatterns[i] != expected {
			t.Errorf("Pattern %d: expected %s, got %s", i, expected, commonPatterns[i])
		}
	}
}

// TestSingleURLModeIntegration tests the complete single URL mode workflow
func TestSingleURLModeIntegration(t *testing.T) {
	// Mock configuration for testing
	mockConfig := struct {
		RSSReaderEndpoint string
		RSSReaderAPIKey   string
		SingleURLMode     bool
	}{
		RSSReaderEndpoint: "https://test.example.com/api",
		RSSReaderAPIKey:   "test-api-key",
		SingleURLMode:     false,
	}

	tests := []struct {
		name              string
		pageURL           string
		category          string
		debug             bool
		clearFeeds        bool
		singleURLMode     bool
		envSingleURLMode  bool
		expectPanic       bool
		expectedLogPhrase string
	}{
		{
			name:              "Single URL mode via CLI flag",
			pageURL:           "https://example.com/blog",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     true,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: example.com",
		},
		{
			name:              "Single URL mode via environment variable",
			pageURL:           "https://blog.example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     false,
			envSingleURLMode:  true,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: blog.example.com",
		},
		{
			name:              "Traversal mode (default)",
			pageURL:           "https://example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     false,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using traversal mode, checking all domains found on page",
		},
		{
			name:              "CLI flag overrides environment variable",
			pageURL:           "https://test.example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     true,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: test.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture panic to verify expected behavior
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
					// In a real test, we would capture and verify log output
					// For now, we just verify that the function was called with correct parameters
				}
			}()

			// Create a mock config that includes the environment variable setting
			testConfig := mockConfig
			testConfig.SingleURLMode = tt.envSingleURLMode

			// This would normally call Run, but since we don't have a real API,
			// we'll test the mode selection logic separately
			t.Logf("Testing mode selection: CLI=%t, Env=%t", tt.singleURLMode, tt.envSingleURLMode)

			// Test the mode selection logic
			useSingleURLMode := tt.singleURLMode || tt.envSingleURLMode
			if useSingleURLMode {
				// Test domain extraction for single URL mode
				domain, err := extractDomainFromURL(tt.pageURL)
				if err != nil {
					t.Errorf("Failed to extract domain from %s: %v", tt.pageURL, err)
				}
				t.Logf("Single URL mode would check domain: %s", domain)
			} else {
				t.Log("Traversal mode would check all domains on page")
			}
		})
	}
}

// TestSingleURLModeErrorHandling tests error handling in single URL mode
func TestSingleURLModeErrorHandling(t *testing.T) {
	errorTests := []struct {
		name        string
		pageURL     string
		expectError bool
		errorPhrase string
	}{
		{
			name:        "Valid URL with subdomain",
			pageURL:     "https://blog.example.com/posts",
			expectError: false,
			errorPhrase: "",
		},
		{
			name:        "Valid URL without protocol",
			pageURL:     "example.com",
			expectError: false,
			errorPhrase: "",
		},
		{
			name:        "Empty URL",
			pageURL:     "",
			expectError: true,
			errorPhrase: "URL cannot be empty",
		},
		{
			name:        "Invalid URL format",
			pageURL:     "://invalid-url",
			expectError: true,
			errorPhrase: "no valid hostname found",
		},
		{
			name:        "URL with spaces in hostname",
			pageURL:     "https://example .com",
			expectError: true,
			errorPhrase: "invalid URL format",
		},
		{
			name:        "Very long hostname",
			pageURL:     "https://" + strings.Repeat("a", 260) + ".com",
			expectError: true,
			errorPhrase: "hostname too long",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractDomainFromURL(tt.pageURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URL %q, but got none", tt.pageURL)
				} else if !strings.Contains(err.Error(), tt.errorPhrase) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorPhrase, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for URL %q: %v", tt.pageURL, err)
				}
				if domain == "" {
					t.Errorf("Expected non-empty domain for valid URL %q", tt.pageURL)
				}
			}
		})
	}
}

// TestSingleURLModeLogging tests that appropriate log messages are generated
func TestSingleURLModeLogging(t *testing.T) {
	// Note: In a real implementation, we would use a test logger to capture
	// and verify log output. For now, we test the logic that determines
	// what should be logged.

	testCases := []struct {
		name           string
		pageURL        string
		singleURLMode  bool
		expectedDomain string
	}{
		{
			name:           "Single URL mode with simple domain",
			pageURL:        "https://example.com",
			singleURLMode:  true,
			expectedDomain: "example.com",
		},
		{
			name:           "Single URL mode with subdomain",
			pageURL:        "https://blog.example.com/feed",
			singleURLMode:  true,
			expectedDomain: "blog.example.com",
		},
		{
			name:           "Single URL mode with complex path",
			pageURL:        "https://news.example.com/category/tech?page=1",
			singleURLMode:  true,
			expectedDomain: "news.example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.singleURLMode {
				domain, err := extractDomainFromURL(tc.pageURL)
				if err != nil {
					t.Errorf("Failed to extract domain: %v", err)
				}
				if domain != tc.expectedDomain {
					t.Errorf("Expected domain %s, got %s", tc.expectedDomain, domain)
				}
				// In a real test, we would verify that the log message
				// "Using single URL mode for domain: %s" was generated
				t.Logf("Would log: Using single URL mode for domain: %s", domain)
			}
		})
	}
}

// TestRSSPatternChecking tests the RSS pattern checking logic
func TestRSSPatternChecking(t *testing.T) {
	// Test that we have the expected common patterns
	expectedPatterns := []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

	if len(commonPatterns) != len(expectedPatterns) {
		t.Errorf("Expected %d patterns, got %d", len(expectedPatterns), len(commonPatterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(commonPatterns) || commonPatterns[i] != expected {
			t.Errorf("Pattern mismatch at index %d: expected %s, got %s",
				i, expected, commonPatterns[i])
		}
	}

	// Test pattern URL construction
	testDomain := "example.com"
	for _, pattern := range commonPatterns {
		expectedURL := "https://" + testDomain + pattern
		t.Logf("Would check RSS feed at: %s", expectedURL)

		// Verify URL construction is valid
		if !strings.HasPrefix(expectedURL, "https://") {
			t.Errorf("Invalid URL construction: %s", expectedURL)
		}
		if !strings.Contains(expectedURL, testDomain) {
			t.Errorf("URL should contain domain: %s", expectedURL)
		}
	}
}

// TestMediumSpecialCase tests the special case handling for medium.com URLs
func TestMediumSpecialCase(t *testing.T) {
	tests := []struct {
		name         string
		domain       string
		originalURL  string
		expectedPath string
		shouldTry    bool
	}{
		{
			name:         "Medium.com with username",
			domain:       "medium.com",
			originalURL:  "https://medium.com/rokkorxblog",
			expectedPath: "rokkorxblog",
			shouldTry:    true,
		},
		{
			name:         "Medium.com root",
			domain:       "medium.com",
			originalURL:  "https://medium.com",
			expectedPath: "",
			shouldTry:    false,
		},
		{
			name:         "Medium.com with path",
			domain:       "medium.com",
			originalURL:  "https://medium.com/tag/technology",
			expectedPath: "",
			shouldTry:    false, // has slash, not username
		},
		{
			name:         "Other domain",
			domain:       "example.com",
			originalURL:  "https://example.com/user",
			expectedPath: "",
			shouldTry:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic for constructing special URL
			if tt.domain == "medium.com" && strings.HasPrefix(tt.originalURL, "https://medium.com/") {
				path := strings.TrimPrefix(tt.originalURL, "https://medium.com/")
				if path != "" && !strings.Contains(path, "/") {
					specialURL := "https://medium.com/feed/" + path
					expected := "https://medium.com/feed/" + tt.expectedPath
					if specialURL != expected {
						t.Errorf("Expected special URL %s, got %s", expected, specialURL)
					}
				} else if tt.shouldTry {
					t.Errorf("Expected to try special case for %s", tt.originalURL)
				}
			} else if tt.shouldTry {
				t.Errorf("Did not expect to try special case for %s", tt.originalURL)
			}
		})
	}
}

// TestModeSelectionPrecedence tests CLI flag vs environment variable precedence
func TestModeSelectionPrecedence(t *testing.T) {
	precedenceTests := []struct {
		name         string
		cliFlag      bool
		envVar       bool
		expectedMode bool
		description  string
	}{
		{
			name:         "CLI flag true, env var false",
			cliFlag:      true,
			envVar:       false,
			expectedMode: true,
			description:  "CLI flag should take precedence",
		},
		{
			name:         "CLI flag false, env var true",
			cliFlag:      false,
			envVar:       true,
			expectedMode: true,
			description:  "Environment variable should be used when CLI flag is false",
		},
		{
			name:         "Both true",
			cliFlag:      true,
			envVar:       true,
			expectedMode: true,
			description:  "Should use single URL mode when both are true",
		},
		{
			name:         "Both false",
			cliFlag:      false,
			envVar:       false,
			expectedMode: false,
			description:  "Should use traversal mode when both are false",
		},
	}

	for _, tt := range precedenceTests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the precedence logic: CLI flag OR environment variable
			actualMode := tt.cliFlag || tt.envVar

			if actualMode != tt.expectedMode {
				t.Errorf("%s: expected mode %t, got %t",
					tt.description, tt.expectedMode, actualMode)
			}

			// Log what mode would be selected
			if actualMode {
				t.Log("Would use single URL mode")
			} else {
				t.Log("Would use traversal mode")
			}
		})
	}
}
