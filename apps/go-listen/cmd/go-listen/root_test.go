package cmd

import (
	"strings"
	"testing"
)

func TestRootCmdPreRunReturnsConfigurationErrors(t *testing.T) {
	t.Setenv("SERVER_PORT", "not-a-number")
	err := rootCmdPreRun(rootCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "load configuration") {
		t.Fatalf("rootCmdPreRun() error = %v", err)
	}
}
