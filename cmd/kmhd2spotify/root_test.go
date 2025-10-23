package cmd

import (
	"bytes"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Set required environment variables for config validation during init()
	os.Setenv("KMHD_BASE_URL", "https://example.com")
	os.Setenv("KMHD_PLAYLIST_PATH", "/playlist")
	os.Exit(m.Run())
}

// TestExecute is difficult to unit test due to os.Exit calls, so we skip it

func TestRootCmdRun(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Create a test command
	cmd := &cobra.Command{}
	args := []string{}

	// Call the function
	rootCmdRun(cmd, args)

	// Check that log contains expected messages
	output := buf.String()
	assert.Contains(t, output, "Use 'kmhd2spotify sync' to sync KMHD playlist to Spotify")
	assert.Contains(t, output, "Use 'kmhd2spotify search <query>' to search for songs")
}

func TestRootCmdPreRun(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		expected log.Level
	}{
		{
			name:     "debug false",
			debug:    false,
			expected: log.InfoLevel, // default level
		},
		{
			name:     "debug true",
			debug:    true,
			expected: log.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original debug and restore after test
			origDebug := debug
			defer func() { debug = origDebug }()

			// Set test debug value
			debug = tt.debug

			// Create a test command
			cmd := &cobra.Command{}
			args := []string{}

			// Call the function
			rootCmdPreRun(cmd, args)

			// Check log level
			assert.Equal(t, tt.expected, log.GetLevel())

			// Check that conf is loaded (basic check)
			require.NotNil(t, conf)
		})
	}
}

func TestInit(t *testing.T) {
	// Test that init has been called by checking rootCmd has expected flags
	flag := rootCmd.PersistentFlags().Lookup("debug")
	require.NotNil(t, flag)
	assert.Equal(t, "d", flag.Shorthand)
	assert.Equal(t, "Enable debug-level logging", flag.Usage)

	// Check that subcommands are added
	subcommands := rootCmd.Commands()
	require.Greater(t, len(subcommands), 0)

	// Debug: print all subcommands
	for _, subcmd := range subcommands {
		t.Logf("Found subcommand: %s", subcmd.Use)
	}

	// Check for expected subcommands (at least sync and search)
	foundSync := false
	foundSearch := false
	for _, subcmd := range subcommands {
		if subcmd.Use == "sync" {
			foundSync = true
		}
		if subcmd.Use == "search [query]" {
			foundSearch = true
		}
	}
	assert.True(t, foundSync, "sync subcommand should be present")
	assert.True(t, foundSearch, "search subcommand should be present")
}
