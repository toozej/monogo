package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/toozej/RSSFFS/pkg/config"
)

// Helper function to create a request with a valid CSRF token (cookie and header)
func newCSRFRequest(method, path, body string, token string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		req.Header.Set("X-CSRF-Token", token)
		// nosemgrep: go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly, go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	}
	return req
}

func TestHandleSubmit(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)

	testCases := []struct {
		name           string
		method         string
		formData       url.Values
		token          string // Token for header/cookie
		expectedStatus int
		expectJSON     bool
	}{
		{
			name:           "Invalid method",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
			expectJSON:     false,
		},
		{
			name:           "Missing CSRF cookie and header",
			method:         "POST",
			formData:       url.Values{"url": {"https://example.com"}},
			token:          "", // No token
			expectedStatus: http.StatusForbidden,
			expectJSON:     true,
		},
		{
			name:           "Mismatched CSRF token",
			method:         "POST",
			formData:       url.Values{"url": {"https://example.com"}},
			token:          "wrong-token",
			expectedStatus: http.StatusForbidden,
			expectJSON:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := ""
			if tc.formData != nil {
				body = tc.formData.Encode()
			}
			req := newCSRFRequest(tc.method, "/submit", body, tc.token)

			// For mismatched token test, set a different cookie value
			if tc.name == "Mismatched CSRF token" {
				req.Header.Set("X-CSRF-Token", "header-token")
				// nosemgrep: go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly, go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
				req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
			}

			w := httptest.NewRecorder()
			server.handleSubmit(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectJSON {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Errorf("Expected Content-Type to contain 'application/json', got %s", contentType)
				}

				var response SubmitResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal JSON response: %v", err)
				}

				if response.Success {
					t.Error("Expected response.Success to be false")
				}
			}
		})
	}
}

func TestHandleSubmitValidation(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)
	token, _ := GenerateCSRFToken()

	testCases := []struct {
		name           string
		url            string
		category       string
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "Valid submission",
			url:            "https://test-success.example.com",
			category:       "test",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "Invalid URL",
			url:            "not-a-url",
			category:       "test",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "Empty URL",
			url:            "",
			category:       "test",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "URL too long",
			url:            "https://example.com/" + strings.Repeat("a", 2048),
			category:       "test",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "Invalid category",
			url:            "https://example.com",
			category:       strings.Repeat("a", 101), // Too long
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{
				"url":      {tc.url},
				"category": {tc.category},
			}

			req := newCSRFRequest("POST", "/submit", formData.Encode(), token)
			w := httptest.NewRecorder()
			server.handleSubmit(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			var response SubmitResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal JSON response: %v", err)
			}

			if response.Success != tc.expectSuccess {
				t.Errorf("Expected success %v, got %v", tc.expectSuccess, response.Success)
			}
		})
	}
}

func TestHandleSubmitSingleURLMode(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)
	token, _ := GenerateCSRFToken()

	testCases := []struct {
		name              string
		url               string
		category          string
		singleURLMode     string
		expectedSingleURL bool
		description       string
	}{
		{
			name:              "Single URL mode enabled",
			url:               "https://test-success.example.com",
			category:          "test",
			singleURLMode:     "true",
			expectedSingleURL: true,
			description:       "Checkbox checked should enable single URL mode",
		},
		{
			name:              "Single URL mode disabled",
			url:               "https://test-success.example.com",
			category:          "test",
			singleURLMode:     "",
			expectedSingleURL: false,
			description:       "Checkbox unchecked should disable single URL mode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{
				"url":      {tc.url},
				"category": {tc.category},
			}

			if tc.singleURLMode != "" {
				formData.Set("single_url_mode", tc.singleURLMode)
			}

			req := newCSRFRequest("POST", "/submit", formData.Encode(), token)
			w := httptest.NewRecorder()
			server.handleSubmit(w, req)

			var response SubmitResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal JSON response: %v", err)
			}

			if tc.expectedSingleURL {
				if !strings.Contains(response.Message, "test-success.example.com only") {
					t.Errorf("Expected single URL mode message, got: %s", response.Message)
				}
			} else {
				if !strings.Contains(response.Message, "Traversal mode") {
					t.Errorf("Expected traversal mode message, got: %s", response.Message)
				}
			}
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"text\x00with\x00nulls", "textwithnulls"},
		{"  multiple   spaces  ", "multiple spaces"},
		{"", ""},
		{"text\nwith\nnewlines", "text with newlines"},
		{"text\twith\ttabs", "text with tabs"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := server.sanitizeInput(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		url         string
		expectError bool
		description string
	}{
		{"https://example.com", false, "Valid HTTPS URL"},
		{"http://example.com", false, "Valid HTTP URL"},
		{"", true, "Empty URL"},
		{"not-a-url", true, "Invalid URL format"},
		{"ftp://example.com", true, "Invalid protocol"},
		{"javascript:alert('xss')", true, "Dangerous protocol"},
		{"https://localhost", true, "Local URL"},
		{"https://127.0.0.1", true, "Local IP"},
		{"https://example.com/" + strings.Repeat("a", 2048), true, "URL too long"},
		{"https://", true, "Missing host"},
		{"example.com", true, "Missing protocol"},
		{"https://example.com/path?query=value", false, "URL with path and query"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := server.validateURL(tc.url)
			hasError := err != nil

			if hasError != tc.expectError {
				if tc.expectError {
					t.Errorf("Expected error for URL %q, but got none", tc.url)
				} else {
					t.Errorf("Expected no error for URL %q, but got: %v", tc.url, err)
				}
			}
		})
	}
}

func TestValidateCategory(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		category    string
		expectError bool
		description string
	}{
		{"", false, "Empty category (optional)"},
		{"valid category", false, "Valid category"},
		{"Category-123", false, "Category with numbers and hyphens"},
		{"Category_with_underscores", false, "Category with underscores"},
		{"Category, with punctuation!", false, "Category with punctuation"},
		{strings.Repeat("a", 101), true, "Category too long"},
		{"<script>alert('xss')</script>", true, "Category with script tag"},
		{"category with\x00null", true, "Category with null byte"},
		{"valid category 123", false, "Category with numbers"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := server.validateCategory(tc.category)
			hasError := err != nil

			if hasError != tc.expectError {
				if tc.expectError {
					t.Errorf("Expected error for category %q, but got none", tc.category)
				} else {
					t.Errorf("Expected no error for category %q, but got: %v", tc.category, err)
				}
			}
		})
	}
}

func TestValidateSubmission(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		request     SubmitRequest
		expectError bool
		description string
	}{
		{
			SubmitRequest{URL: "https://example.com", Category: "test"},
			false,
			"Valid submission",
		},
		{
			SubmitRequest{URL: "https://example.com", Category: "test", SingleURLMode: true},
			false,
			"Valid submission with single URL mode",
		},
		{
			SubmitRequest{URL: "", Category: "test"},
			true,
			"Empty URL",
		},
		{
			SubmitRequest{URL: "https://example.com", Category: strings.Repeat("a", 101)},
			true,
			"Invalid category",
		},
		{
			SubmitRequest{URL: "not-a-url", Category: "test"},
			true,
			"Invalid URL",
		},
		{
			SubmitRequest{URL: "not-a-url", Category: "test", SingleURLMode: true},
			true,
			"Invalid URL with single URL mode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			validationErr := server.validateSubmission(tc.request)
			hasError := validationErr != nil

			if hasError != tc.expectError {
				if tc.expectError {
					t.Errorf("Expected error for request %+v, but got none", tc.request)
				} else {
					t.Errorf("Expected no error for request %+v, but got: %v", tc.request, validationErr)
				}
			}

			// Check if validation errors are properly structured
			if hasError {
				if len(validationErr.Errors) == 0 {
					t.Error("Expected validation errors to contain error details")
				}
			}
		})
	}
}

func TestProcessSubmission(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, true) // Debug mode

	req := SubmitRequest{
		URL:      "https://example.com",
		Category: "test",
	}

	// Note: This test will call the actual RSSFFS.Run function which will fail
	// because the test endpoint doesn't exist. We expect this to return an error response.
	response := server.processSubmission(req)

	// Since RSSFFS.Run will fail with the test endpoint, we expect an error response
	if response.Success {
		t.Error("Expected response to indicate failure due to invalid test endpoint")
	}

	if response.Error == "" && response.Message == "" {
		t.Error("Expected error response to have either an error or message")
	}
}

func TestProcessTestSubmissionModes(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)

	testCases := []struct {
		name              string
		request           SubmitRequest
		expectedSuccess   bool
		expectedInMessage string
		description       string
	}{
		{
			name: "Test success with single URL mode",
			request: SubmitRequest{
				URL:           "https://test-success.example.com",
				Category:      "test",
				SingleURLMode: true,
			},
			expectedSuccess:   true,
			expectedInMessage: "test-success.example.com only",
			description:       "Single URL mode should include domain in message",
		},
		{
			name: "Test success with traversal mode",
			request: SubmitRequest{
				URL:           "https://test-success.example.com",
				Category:      "test",
				SingleURLMode: false,
			},
			expectedSuccess:   true,
			expectedInMessage: "Traversal mode",
			description:       "Traversal mode should include mode in message",
		},
		{
			name: "Test no feeds with single URL mode",
			request: SubmitRequest{
				URL:           "https://test-no-feeds.example.com",
				Category:      "test",
				SingleURLMode: true,
			},
			expectedSuccess:   true,
			expectedInMessage: "test-no-feeds.example.com only",
			description:       "Single URL mode should include domain even when no feeds found",
		},
		{
			name: "Test no feeds with traversal mode",
			request: SubmitRequest{
				URL:           "https://test-no-feeds.example.com",
				Category:      "test",
				SingleURLMode: false,
			},
			expectedSuccess:   true,
			expectedInMessage: "Traversal mode",
			description:       "Traversal mode should include mode even when no feeds found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := server.processSubmission(tc.request)

			if response.Success != tc.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tc.expectedSuccess, response.Success)
			}

			if !strings.Contains(response.Message, tc.expectedInMessage) {
				t.Errorf("Expected message to contain %q, got: %s", tc.expectedInMessage, response.Message)
			}
		})
	}
}

func TestExtractDomainFromURL(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		url      string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"https://blog.example.com", "blog.example.com"},
		{"https://example.com/path/to/page", "example.com"},
		{"https://example.com:8080", "example.com"},
		{"http://subdomain.example.com/path?query=value", "subdomain.example.com"},
		{"invalid-url", "invalid-url"}, // Should return original if parsing fails
		{"", ""},                       // Should return empty string
	}

	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			result := server.extractDomainFromURL(tc.url)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestSendErrorResponse(t *testing.T) {
	server := &Server{}

	w := httptest.NewRecorder()
	server.sendErrorResponse(w, "Test Error", "Test message", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Check that the response was written (status code was set)
	if w.Code == 0 {
		t.Error("Expected status code to be set")
	}

	var response SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal JSON response: %v", err)
	}

	if response.Success {
		t.Error("Expected response.Success to be false")
	}

	if response.Error != "Test Error" {
		t.Errorf("Expected error %q, got %q", "Test Error", response.Error)
	}

	if response.Message != "Test message" {
		t.Errorf("Expected message %q, got %q", "Test message", response.Message)
	}
}

func TestSendValidationErrorResponse(t *testing.T) {
	server := &Server{}

	validationErrors := ValidationErrors{
		Errors: []ValidationError{
			{Field: "url", Message: "URL is required"},
			{Field: "category", Message: "Category is too long"},
		},
	}

	w := httptest.NewRecorder()
	server.sendValidationErrorResponse(w, validationErrors)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal JSON response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Expected response.success to be false")
	}

	if validationErrs, ok := response["validation_errors"].([]interface{}); !ok || len(validationErrs) != 2 {
		t.Error("Expected validation_errors to contain 2 errors")
	}
}

func TestGetStatusCode(t *testing.T) {
	server := &Server{}

	testCases := []struct {
		response       SubmitResponse
		expectedStatus int
	}{
		{SubmitResponse{Success: true}, http.StatusOK},
		{SubmitResponse{Success: false}, http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		status := server.getStatusCode(tc.response)
		if status != tc.expectedStatus {
			t.Errorf("Expected status %d, got %d", tc.expectedStatus, status)
		}
	}
}
func TestHandleCategories(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		config         config.Config
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Invalid method",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "Test environment",
			method: "GET",
			config: config.Config{
				RSSReaderEndpoint: "https://test.example.com",
				RSSReaderAPIKey:   "test-key",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"success":true`,
		},
		{
			name:   "Fallback when RSS reader not accessible",
			method: "GET",
			config: config.Config{
				RSSReaderEndpoint: "https://unreachable.example.com",
				RSSReaderAPIKey:   "test-key",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"success":true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				config: tt.config,
				debug:  false,
			}

			req := httptest.NewRequest(tt.method, "/categories", nil)
			w := httptest.NewRecorder()

			server.handleCategories(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handleCategories() status = %v, want %v", w.Code, tt.expectedStatus)
			}

			if tt.expectedBody != "" && !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("handleCategories() body = %v, want to contain %v", w.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestSendTestCategoriesResponse(t *testing.T) {
	server := &Server{}
	w := httptest.NewRecorder()

	server.sendTestCategoriesResponse(w)

	if w.Code != http.StatusOK {
		t.Errorf("sendTestCategoriesResponse() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response CategoryResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if len(response.Categories) == 0 {
		t.Error("Expected categories to be populated")
	}

	// Check that we have expected test categories
	expectedCategories := []string{"News", "Technology", "Science", "Entertainment", "Sports"}
	for i, expected := range expectedCategories {
		if i >= len(response.Categories) {
			t.Errorf("Missing category at index %d", i)
			continue
		}
		if response.Categories[i].Title != expected {
			t.Errorf("Category[%d].Title = %v, want %v", i, response.Categories[i].Title, expected)
		}
	}
}

func TestSendCategoriesErrorResponse(t *testing.T) {
	server := &Server{}
	w := httptest.NewRecorder()

	server.sendCategoriesErrorResponse(w, "Test Error", "Test message")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("sendCategoriesErrorResponse() status = %v, want %v", w.Code, http.StatusInternalServerError)
	}

	var response CategoryResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("Expected success to be false")
	}

	if response.Error != "Test Error" {
		t.Errorf("Error = %v, want %v", response.Error, "Test Error")
	}
}
func TestSendFallbackCategoriesResponse(t *testing.T) {
	server := &Server{}
	w := httptest.NewRecorder()

	server.sendFallbackCategoriesResponse(w)

	if w.Code != http.StatusOK {
		t.Errorf("sendFallbackCategoriesResponse() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response CategoryResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if len(response.Categories) == 0 {
		t.Error("Expected fallback categories to be populated")
	}

	// Check that we have expected fallback categories
	expectedCategories := []string{"General", "News", "Technology", "Science", "Business"}
	for i, expected := range expectedCategories {
		if i >= len(response.Categories) {
			t.Errorf("Missing category at index %d", i)
			continue
		}
		if response.Categories[i].Title != expected {
			t.Errorf("Category[%d].Title = %v, want %v", i, response.Categories[i].Title, expected)
		}
		// Fallback categories should have ID = 0
		if response.Categories[i].ID != 0 {
			t.Errorf("Category[%d].ID = %v, want 0 (fallback)", i, response.Categories[i].ID)
		}
	}
}
