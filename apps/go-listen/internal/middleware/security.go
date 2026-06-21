package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const maxFormSize = 1 << 20 // 1 MB

// SecurityMiddleware provides various security protections
type SecurityMiddleware struct {
	logger      *log.Logger
	rateLimiter *RateLimiter
	csrfTokens  map[string]time.Time
	csrfMutex   sync.RWMutex
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(logger *log.Logger, rateLimiter *RateLimiter) *SecurityMiddleware {
	sm := &SecurityMiddleware{
		logger:      logger,
		rateLimiter: rateLimiter,
		csrfTokens:  make(map[string]time.Time),
	}

	// Start cleanup goroutine for expired CSRF tokens
	go sm.cleanupExpiredTokens()

	return sm
}

// SecurityHeaders adds security headers to all responses
func (sm *SecurityMiddleware) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://api.spotify.com; frame-src https://open.spotify.com;")

		// Only add HSTS for HTTPS
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimit implements rate limiting per IP address
func (sm *SecurityMiddleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		if !sm.rateLimiter.Allow(clientIP) {
			sm.logger.WithFields(log.Fields{
				"component":  "security",
				"operation":  "rate_limit",
				"event_type": "rate_limit_exceeded",
				"client_ip":  clientIP,
				"method":     r.Method,
				"path":       r.URL.Path,
				"user_agent": r.UserAgent(),
			}).Warn("Rate limit exceeded")

			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// InputValidation validates and sanitizes input data
func (sm *SecurityMiddleware) InputValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for suspicious patterns in URL path
		if containsSuspiciousPatterns(r.URL.Path) {
			sm.logger.WithFields(log.Fields{
				"component":  "security",
				"operation":  "input_validation",
				"event_type": "suspicious_path",
				"client_ip":  getClientIP(r),
				"path":       r.URL.Path,
				"user_agent": r.UserAgent(),
				"method":     r.Method,
			}).Warn("Suspicious path detected")

			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Check for suspicious patterns in query parameters
		for key, values := range r.URL.Query() {
			for _, value := range values {
				if containsSuspiciousPatterns(key) || containsSuspiciousPatterns(value) {
					sm.logger.WithFields(log.Fields{
						"component":  "security",
						"operation":  "input_validation",
						"event_type": "suspicious_parameter",
						"client_ip":  getClientIP(r),
						"param":      key,
						"value":      value,
						"user_agent": r.UserAgent(),
						"method":     r.Method,
						"path":       r.URL.Path,
					}).Warn("Suspicious query parameter detected")

					http.Error(w, "Invalid request parameters", http.StatusBadRequest)
					return
				}
			}
		}

		// Limit request body size to prevent DoS
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit

		next.ServeHTTP(w, r)
	})
}

// CSRFProtection implements CSRF protection for state-changing operations
func (sm *SecurityMiddleware) CSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply CSRF protection to state-changing methods
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				// Limit request body size BEFORE parsing
				r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)

				// Try to get token from form data
				if err := r.ParseForm(); err == nil {
					token = r.FormValue("csrf_token")
				} else {
					http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
					return
				}
			}

			if !sm.validateCSRFToken(token) {
				sm.logger.WithFields(log.Fields{
					"component":  "security",
					"operation":  "csrf_protection",
					"event_type": "invalid_csrf_token",
					"client_ip":  getClientIP(r),
					"method":     r.Method,
					"path":       r.URL.Path,
					"user_agent": r.UserAgent(),
					"has_token":  token != "",
				}).Warn("Invalid or missing CSRF token")

				http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// GenerateCSRFToken generates a new CSRF token
func (sm *SecurityMiddleware) GenerateCSRFToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		sm.logger.WithError(err).WithFields(log.Fields{
			"component": "security",
			"operation": "csrf_token_generation",
		}).Error("Failed to generate CSRF token")
		return ""
	}

	token := base64.URLEncoding.EncodeToString(bytes)

	sm.csrfMutex.Lock()
	sm.csrfTokens[token] = time.Now().Add(24 * time.Hour) // Token expires in 24 hours
	sm.csrfMutex.Unlock()

	return token
}

// validateCSRFToken validates a CSRF token
func (sm *SecurityMiddleware) validateCSRFToken(token string) bool {
	if token == "" {
		return false
	}

	sm.csrfMutex.RLock()
	expiry, exists := sm.csrfTokens[token]
	sm.csrfMutex.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Token expired, remove it
		sm.csrfMutex.Lock()
		delete(sm.csrfTokens, token)
		sm.csrfMutex.Unlock()
		return false
	}

	return true
}

// cleanupExpiredTokens periodically removes expired CSRF tokens
func (sm *SecurityMiddleware) cleanupExpiredTokens() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		sm.csrfMutex.Lock()
		for token, expiry := range sm.csrfTokens {
			if now.After(expiry) {
				delete(sm.csrfTokens, token)
			}
		}
		sm.csrfMutex.Unlock()
	}
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// containsSuspiciousPatterns checks for common attack patterns
func containsSuspiciousPatterns(input string) bool {
	suspiciousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"onclick=",
		"../",
		"..\\",
		"SELECT",
		"UNION",
		"INSERT",
		"DELETE",
		"UPDATE",
		"DROP",
		"CREATE",
		"ALTER",
		"EXEC",
		"EXECUTE",
		"--",
		"/*",
		"*/",
		"xp_",
		"sp_",
		"'",
		"\"",
		";",
		"||",
		"&&",
		"|",
		"&",
		"`",
		"$(",
		"${",
		"<%",
		"%>",
		"<?",
		"?>",
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
