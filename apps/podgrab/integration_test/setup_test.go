//go:build integration
// +build integration

package integration_test

import (
	"os"
	"testing"
)

// TestMain lets the integration suite reach the httptest servers it binds to
// 127.0.0.1. makeQuery's SSRF guard blocks private/internal targets by default
// in production; the tests opt in via ALLOW_PRIVATE_NETWORK.
func TestMain(m *testing.M) {
	if err := os.Setenv("ALLOW_PRIVATE_NETWORK", "true"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
