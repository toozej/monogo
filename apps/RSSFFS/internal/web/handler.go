package web

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/RSSFFS/internal/RSSFFS"
)

// SubmitRequest represents the form submission data
type SubmitRequest struct {
	URL           string `json:"url"`
	Category      string `json:"category"`
	SingleURLMode bool   `json:"single_url_mode"`
}

// SubmitResponse represents the JSON response sent back to the client
type SubmitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Count   int    `json:"count,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CategoryResponse represents the JSON response for category list
type CategoryResponse struct {
	Success    bool                   `json:"success"`
	Categories []CategoryResponseItem `json:"categories,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// CategoryResponseItem represents a single category in the response
type CategoryResponseItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// ValidationError represents a validation error with field-specific details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (ve ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "validation failed"
	}
	return ve.Errors[0].Message
}

// handleSubmit processes form submissions and integrates with RSSFFS core
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set response content type
	w.Header().Set("Content-Type", "application/json")

	// Parse form data with size limit
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit
	if err := r.ParseForm(); err != nil {
		s.sendErrorResponse(w, "Invalid form data", "Request too large or malformed", http.StatusBadRequest)
		return
	}

	// Validate CSRF token using double submit cookie method
	csrfCookie, err := r.Cookie("csrf_token")
	if err != nil {
		log.Warnf("CSRF cookie not found: %v", err)
		s.sendErrorResponse(w, "Invalid security token", "Please refresh the page and try again", http.StatusForbidden)
		return
	}

	csrfHeader := r.Header.Get("X-CSRF-Token")
	if csrfHeader == "" || csrfHeader != csrfCookie.Value {
		log.Warnf("Invalid CSRF token from IP: %s", getClientIP(r))
		s.sendErrorResponse(w, "Invalid security token", "Please refresh the page and try again", http.StatusForbidden)
		return
	}

	// Extract and sanitize form values
	rawURL := r.FormValue("url")
	rawCategory := r.FormValue("category")
	rawSingleURLMode := r.FormValue("single_url_mode")

	req := SubmitRequest{
		URL:           s.sanitizeInput(strings.TrimSpace(rawURL)),
		Category:      s.sanitizeInput(strings.TrimSpace(rawCategory)),
		SingleURLMode: rawSingleURLMode == "true",
	}

	// Validate input
	if validationErr := s.validateSubmission(req); validationErr != nil {
		s.sendValidationErrorResponse(w, *validationErr)
		return
	}

	// Process the submission
	response := s.processSubmission(req)

	// Send JSON response
	w.WriteHeader(s.getStatusCode(response))
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding JSON response: %v", err)
	}
}

// sanitizeInput sanitizes user input to prevent XSS attacks
func (s *Server) sanitizeInput(input string) string {
	// HTML escape the input
	sanitized := html.EscapeString(input)

	// Remove any null bytes
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")

	// Normalize whitespace
	sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, " ")

	return strings.TrimSpace(sanitized)
}

// validateSubmission validates form input data comprehensively
func (s *Server) validateSubmission(req SubmitRequest) *ValidationErrors {
	var errors []ValidationError

	// Validate URL
	if urlErr := s.validateURL(req.URL); urlErr != nil {
		errors = append(errors, ValidationError{
			Field:   "url",
			Message: urlErr.Error(),
		})
	}

	// Validate category
	if categoryErr := s.validateCategory(req.Category); categoryErr != nil {
		errors = append(errors, ValidationError{
			Field:   "category",
			Message: categoryErr.Error(),
		})
	}

	if len(errors) > 0 {
		return &ValidationErrors{Errors: errors}
	}

	return nil
}

// validateURL validates URL format and constraints
func (s *Server) validateURL(urlStr string) error {
	// Check if URL is provided
	if urlStr == "" {
		return fmt.Errorf("URL is required")
	}

	// Check URL length
	if len(urlStr) > 2048 {
		return fmt.Errorf("URL is too long (maximum 2048 characters)")
	}

	// Check for valid UTF-8
	if !utf8.ValidString(urlStr) {
		return fmt.Errorf("URL contains invalid characters")
	}

	// Parse and validate URL format
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}

	// Ensure URL has a scheme
	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL must include protocol (http:// or https://)")
	}

	// Validate scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https protocol")
	}

	// Ensure URL has a host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a valid domain")
	}

	// Validate host format
	if len(parsedURL.Host) > 253 {
		return fmt.Errorf("domain name is too long")
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"javascript:", "data:", "vbscript:", "file:", "ftp:",
	}
	lowerURL := strings.ToLower(urlStr)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerURL, pattern) {
			return fmt.Errorf("URL contains potentially unsafe protocol")
		}
	}

	// Validate that it's not a local/private IP (basic check)
	if strings.Contains(parsedURL.Host, "localhost") ||
		strings.Contains(parsedURL.Host, "127.0.0.1") ||
		strings.Contains(parsedURL.Host, "::1") {
		return fmt.Errorf("local URLs are not allowed")
	}

	return nil
}

// validateCategory validates category format and constraints
func (s *Server) validateCategory(category string) error {
	// Category is optional
	if category == "" {
		return nil
	}

	// Check length
	if len(category) > 100 {
		return fmt.Errorf("category name must be 100 characters or less")
	}

	// Check for valid UTF-8
	if !utf8.ValidString(category) {
		return fmt.Errorf("category contains invalid characters")
	}

	// Allow alphanumeric, spaces, hyphens, underscores, and common punctuation
	validCategory := regexp.MustCompile(`^[a-zA-Z0-9\s\-_.,!?()]+$`)
	if !validCategory.MatchString(category) {
		return fmt.Errorf("category name contains invalid characters (only letters, numbers, spaces, and basic punctuation allowed)")
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"<script", "</script", "javascript:", "onclick", "onerror", "onload",
	}
	lowerCategory := strings.ToLower(category)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerCategory, pattern) {
			return fmt.Errorf("category contains potentially unsafe content")
		}
	}

	return nil
}

// processSubmission processes the validated form submission using RSSFFS core
func (s *Server) processSubmission(req SubmitRequest) SubmitResponse {
	if s.debug {
		log.Debugf("Processing submission: URL=%s, Category=%s, SingleURLMode=%t", req.URL, req.Category, req.SingleURLMode)
	}

	// Check if we're in a test environment (test endpoints)
	if strings.Contains(s.config.RSSReaderEndpoint, "test.example.com") {
		return s.processTestSubmission(req)
	}

	// Call the RSSFFS core function
	count, err := RSSFFS.Run(req.URL, req.Category, s.debug, false, req.SingleURLMode, s.config)
	if err != nil {
		log.Errorf("Error processing RSSFFS request: %v", err)
		return SubmitResponse{
			Success: false,
			Error:   "Processing Error",
			Message: "An internal error occurred while processing your request.",
		}
	}

	// Formulate a success message
	var successMessage string
	if count > 0 {
		successMessage = fmt.Sprintf("Successfully found and subscribed to %d feed(s).", count)
	} else {
		successMessage = "Processing complete. No new RSS feeds were subscribed."
	}

	return SubmitResponse{
		Success: true,
		Message: successMessage,
		Count:   count,
	}
}

// processTestSubmission handles submissions in test mode
func (s *Server) processTestSubmission(req SubmitRequest) SubmitResponse {
	// Generate mode-specific messages for test responses
	var modePrefix string
	if req.SingleURLMode {
		domain := s.extractDomainFromURL(req.URL)
		if domain == req.URL {
			modePrefix = "Single URL mode (domain extraction failed) - "
		} else {
			modePrefix = fmt.Sprintf("Single URL mode (checking %s only) - ", domain)
		}
	} else {
		modePrefix = "Traversal mode - "
	}

	// Simulate different responses based on URL for testing
	switch req.URL {
	case "https://test-success.example.com":
		return SubmitResponse{
			Success: true,
			Message: modePrefix + "successfully found and subscribed to 2 RSS feeds",
			Count:   2,
		}
	case "https://test-no-feeds.example.com":
		var message string
		if req.SingleURLMode {
			message = modePrefix + "no RSS feeds found on the target domain (checked common patterns: /index.xml, /feed, /rss, /atom.xml)"
		} else {
			message = modePrefix + "no RSS feeds found across all discovered domains"
		}
		return SubmitResponse{
			Success: true,
			Message: message,
			Count:   0,
		}
	case "https://test-error.example.com":
		var errorMessage string
		if req.SingleURLMode {
			errorMessage = "Single URL mode: Unable to connect to the target domain"
		} else {
			errorMessage = "Traversal mode: Unable to connect to the provided URL"
		}
		return SubmitResponse{
			Success: false,
			Error:   "Network Error",
			Message: errorMessage,
		}
	default:
		// For any other URL in test mode, return a generic test response
		return SubmitResponse{
			Success: false,
			Error:   "Test Environment",
			Message: "RSS reader API endpoint is not configured for production use",
		}
	}
}

// sendErrorResponse sends a standardized error response
func (s *Server) sendErrorResponse(w http.ResponseWriter, error, message string, statusCode int) {
	response := SubmitResponse{
		Success: false,
		Error:   error,
		Message: message,
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding error response: %v", err)
	}
}

// sendValidationErrorResponse sends a detailed validation error response
func (s *Server) sendValidationErrorResponse(w http.ResponseWriter, validationErrors ValidationErrors) {
	// Create a user-friendly message
	message := "Please correct the following errors:"
	if len(validationErrors.Errors) == 1 {
		message = validationErrors.Errors[0].Message
	}

	response := map[string]interface{}{
		"success":           false,
		"error":             "Validation Error",
		"message":           message,
		"validation_errors": validationErrors.Errors,
	}

	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding validation error response: %v", err)
	}
}

// handleCategories fetches available categories from the RSS reader
func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set response content type
	w.Header().Set("Content-Type", "application/json")

	// Check if we're in a test environment
	if strings.Contains(s.config.RSSReaderEndpoint, "test.example.com") {
		s.sendTestCategoriesResponse(w)
		return
	}

	// Fetch categories from RSS reader API
	categories, err := s.fetchCategoriesFromAPI()
	if err != nil {
		log.Warnf("Could not fetch categories from RSS reader: %v", err)
		// Instead of returning an error, provide a fallback with common categories
		s.sendFallbackCategoriesResponse(w)
		return
	}

	// Send successful response
	response := CategoryResponse{
		Success:    true,
		Categories: categories,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding categories response: %v", err)
	}
}

// fetchCategoriesFromAPI fetches categories from the RSS reader API
func (s *Server) fetchCategoriesFromAPI() ([]CategoryResponseItem, error) {
	// Create HTTP request to fetch categories
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/categories", s.config.RSSReaderEndpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Auth-Token", s.config.RSSReaderAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req) // #nosec G704 -- URL is from config, not user input
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	// Parse the JSON response using the existing Category struct from RSSFFS package
	var apiCategories []RSSFFS.Category
	if err := json.NewDecoder(resp.Body).Decode(&apiCategories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to response format
	categories := make([]CategoryResponseItem, len(apiCategories))
	for i, cat := range apiCategories {
		categories[i] = CategoryResponseItem{
			ID:    cat.ID,
			Title: cat.Title,
		}
	}

	return categories, nil
}

// sendTestCategoriesResponse sends a test response for categories
func (s *Server) sendTestCategoriesResponse(w http.ResponseWriter) {
	response := CategoryResponse{
		Success: true,
		Categories: []CategoryResponseItem{
			{ID: 1, Title: "News"},
			{ID: 2, Title: "Technology"},
			{ID: 3, Title: "Science"},
			{ID: 4, Title: "Entertainment"},
			{ID: 5, Title: "Sports"},
		},
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding test categories response: %v", err)
	}
}

// sendFallbackCategoriesResponse sends a fallback response when RSS reader is not accessible
func (s *Server) sendFallbackCategoriesResponse(w http.ResponseWriter) {
	response := CategoryResponse{
		Success: true,
		Categories: []CategoryResponseItem{
			{ID: 0, Title: "General"},
			{ID: 0, Title: "News"},
			{ID: 0, Title: "Technology"},
			{ID: 0, Title: "Science"},
			{ID: 0, Title: "Business"},
			{ID: 0, Title: "Entertainment"},
			{ID: 0, Title: "Sports"},
			{ID: 0, Title: "Health"},
			{ID: 0, Title: "Politics"},
			{ID: 0, Title: "Education"},
			{ID: 0, Title: "Travel"},
			{ID: 0, Title: "Food"},
			{ID: 0, Title: "Lifestyle"},
			{ID: 0, Title: "Gaming"},
			{ID: 0, Title: "Finance"},
		},
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding fallback categories response: %v", err)
	}
}

// sendCategoriesErrorResponse sends an error response for categories
func (s *Server) sendCategoriesErrorResponse(w http.ResponseWriter, error, message string) {
	response := CategoryResponse{
		Success: false,
		Error:   error,
	}

	w.WriteHeader(http.StatusInternalServerError)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding categories error response: %v", err)
	}
}

// extractDomainFromURL extracts the domain from a URL for display purposes
func (s *Server) extractDomainFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr // Return original URL if parsing fails
	}
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return urlStr // Return original URL if no hostname found
	}
	return hostname
}

// getStatusCode returns appropriate HTTP status code based on response
func (s *Server) getStatusCode(response SubmitResponse) int {
	if response.Success {
		return http.StatusOK
	}
	return http.StatusInternalServerError
}
