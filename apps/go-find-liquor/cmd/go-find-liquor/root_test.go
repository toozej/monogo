package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
)

func TestRootCmdRunReturnsConfigurationError(t *testing.T) {
	config.SetConfigFile(filepath.Join(t.TempDir(), "missing.yaml"))
	t.Cleanup(func() { config.SetConfigFile("") })

	err := rootCmdRun(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("rootCmdRun() error = nil, want missing-config error")
	}
}
