package service

import (
	"os"
	"testing"
)

// TestMain allows the service test-suite to reach the httptest servers it spins
// up on 127.0.0.1. makeQuery's SSRF guard blocks private/internal targets by
// default in production; the tests opt in via ALLOW_PRIVATE_NETWORK, the same
// escape hatch documented for self-hosted setups that fetch feeds from a
// private network.
func TestMain(m *testing.M) {
	if err := os.Setenv("ALLOW_PRIVATE_NETWORK", "true"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
