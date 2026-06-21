package middleware

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, 2) // 2 requests per second, burst of 2

	ip := "192.168.1.1"

	// First two requests should be allowed (burst)
	if !rl.Allow(ip) {
		t.Error("First request should be allowed")
	}

	if !rl.Allow(ip) {
		t.Error("Second request should be allowed")
	}

	// Third request should be denied (rate limit exceeded)
	if rl.Allow(ip) {
		t.Error("Third request should be denied")
	}

	// Wait for rate limiter to refill
	time.Sleep(time.Second)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("Request after waiting should be allowed")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 request per second, burst of 1

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// Each IP should have its own rate limit
	if !rl.Allow(ip1) {
		t.Error("First IP should be allowed")
	}

	if !rl.Allow(ip2) {
		t.Error("Second IP should be allowed")
	}

	// Both IPs should now be rate limited
	if rl.Allow(ip1) {
		t.Error("First IP should be rate limited")
	}

	if rl.Allow(ip2) {
		t.Error("Second IP should be rate limited")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 request per second, burst of 1

	ip := "192.168.1.1"

	// Use up the rate limit
	if !rl.Allow(ip) {
		t.Error("First request should be allowed")
	}

	if rl.Allow(ip) {
		t.Error("Second request should be denied")
	}

	// Reset the rate limiter for this IP
	rl.Reset(ip)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("Request after reset should be allowed")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	ip := "192.168.1.1"
	var allowed, denied int32

	// Make many concurrent requests
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			if rl.Allow(ip) {
				atomic.AddInt32(&allowed, 1)
			} else {
				atomic.AddInt32(&denied, 1)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should have allowed exactly 10 (burst size) and denied 10
	allowedCount := atomic.LoadInt32(&allowed)
	deniedCount := atomic.LoadInt32(&denied)

	if allowedCount != 10 {
		t.Errorf("Expected 10 allowed requests, got %d", allowedCount)
	}

	if deniedCount != 10 {
		t.Errorf("Expected 10 denied requests, got %d", deniedCount)
	}
}
