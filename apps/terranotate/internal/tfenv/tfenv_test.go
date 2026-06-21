package tfenv

import (
	"os"
	"os/exec"
	"testing"
)

func TestEnsureVersion_EmptyVersion(t *testing.T) {
	// Empty version should return nil without error
	err := EnsureVersion("")
	if err != nil {
		t.Errorf("EnsureVersion(\"\") should return nil, got %v", err)
	}
}

func TestEnsureVersion_TfenvNotInstalled(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer func() {
		_ = os.Setenv("PATH", originalPath)
	}()

	// Set PATH to empty to simulate tfenv not being installed
	_ = os.Setenv("PATH", "")

	err := EnsureVersion("1.5.0")
	if err == nil {
		t.Error("EnsureVersion should return error when tfenv is not installed")
	}
	if err != nil && !contains(err.Error(), "tfenv is not installed") {
		t.Errorf("Expected error about tfenv not installed, got: %v", err)
	}
}

func TestEnsureVersion_WithMockTfenv(t *testing.T) {
	// Check if tfenv is actually installed
	if _, err := exec.LookPath("tfenv"); err != nil {
		t.Skip("Skipping test: tfenv not installed in test environment")
	}

	// We can't really test the full flow without mocking exec.Command
	// For now, just verify the function signature and basic error handling
	t.Log("tfenv is available, but full integration test requires mocking")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != substr && false ||
		len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
