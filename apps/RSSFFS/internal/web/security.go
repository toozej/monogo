package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// GenerateCSRFToken generates a new, random CSRF token.
func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// RateLimiter implements basic rate limiting
type RateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	limiter := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Start cleanup goroutine
	go limiter.cleanupOldRequests()

	return limiter
}

// IsAllowed checks if a request from the given IP is allowed
func (rl *RateLimiter) IsAllowed(ip string) bool {
	now := time.Now()

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Get existing requests for this IP
	requests, exists := rl.requests[ip]
	if !exists {
		requests = make([]time.Time, 0)
	}

	// Remove requests outside the window
	validRequests := make([]time.Time, 0)
	for _, reqTime := range requests {
		if now.Sub(reqTime) < rl.window {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if limit is exceeded
	if len(validRequests) >= rl.limit {
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[ip] = validRequests

	return true
}

// cleanupOldRequests periodically removes old request records
func (rl *RateLimiter) cleanupOldRequests() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		rl.mutex.Lock()
		for ip, requests := range rl.requests {
			validRequests := make([]time.Time, 0)
			for _, reqTime := range requests {
				if now.Sub(reqTime) < rl.window {
					validRequests = append(validRequests, reqTime)
				}
			}
			if len(validRequests) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = validRequests
			}
		}
		rl.mutex.Unlock()
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Forwarded headers are intentionally ignored unless trusted-proxy support
	// is explicitly configured in the future; accepting them from direct clients
	// makes rate limits trivial to bypass.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	// If SplitHostPort fails, return RemoteAddr as is (it might be just an IP).
	return r.RemoteAddr
}
