package middleware

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements token bucket rate limiting per IP address
type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}

	// Start cleanup goroutine for inactive visitors
	go rl.cleanupInactiveVisitors()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}
	rl.mu.Unlock()

	return limiter.Allow()
}

// Reset removes the rate limiter for a specific IP
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	delete(rl.visitors, ip)
	rl.mu.Unlock()
}

// cleanupInactiveVisitors removes rate limiters for IPs that haven't been seen recently
func (rl *RateLimiter) cleanupInactiveVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// Remove visitors that haven't made a request in the last 10 minutes
		// This is a simple cleanup - in production you might want more sophisticated tracking
		for ip, limiter := range rl.visitors {
			// If the limiter has full tokens, it hasn't been used recently
			if limiter.Tokens() == float64(rl.burst) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}
