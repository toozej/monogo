// Package cmd provides command-line interface functionality for the go-listen application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the go-listen Spotify playlist management application.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/go-listen/cmd/go-listen"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/logging"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf config.Config
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through slog.
	debug bool
)

// rootCmd defines the base command for the go-listen CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and shows help when no subcommand
// is provided. It supports persistent flags that are inherited by all subcommands.
var rootCmd = &cobra.Command{
	Use:               "go-listen",
	Short:             "Spotify playlist management tool",
	Long:              `go-listen is a web application that allows users to search for artists and automatically add their top 5 songs to designated "incoming" playlists on Spotify.`,
	Args:              cobra.ExactArgs(0),
	PersistentPreRunE: rootCmdPreRun,
	Run:               rootCmdRun,
}

// rootCmdRun is the main execution function for the root command.
// It shows help when no subcommand is provided, as go-listen is primarily
// used through its subcommands (serve, etc.).
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments (unused, as root command takes no args)
func rootCmdRun(cmd *cobra.Command, args []string) {
	// Show help when no subcommand is provided
	if err := cmd.Help(); err != nil {
		slog.Error("Failed to show help", "error", err)
	}
}

// rootCmdPreRun performs setup operations before executing the root command.
// This function is called before both the root command and any subcommands.
//
// It configures the logging level based on the debug flag and loads the
// application configuration from environment variables. When debug mode
// is enabled, slog is set to debug level for detailed logging output.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments
func rootCmdPreRun(cmd *cobra.Command, args []string) error {
	level := "info"
	if debug {
		level = "debug"
	}
	slog.SetDefault(logging.NewLogger(logging.Config{Level: level, Format: "json", Output: "stdout"}))

	// Load configuration
	loaded, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	conf = loaded
	return nil
}

// Execute starts the command-line interface execution.
// This is the main entry point called from main.go to begin command processing.
//
// If command execution fails, it prints the error message to stdout and
// exits the program with status code 1. This follows standard Unix conventions
// for command-line tool error handling.
//
// Example:
//
//	func main() {
//		cmd.Execute()
//	}
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

// init initializes the command-line interface during package loading.
//
// This function performs the following setup operations:
//   - Defines persistent flags that are available to all commands
//   - Registers subcommands (man pages and version information)
//
// The debug flag (-d, --debug) enables debug-level logging and is persistent,
// meaning it's inherited by all subcommands. Configuration is loaded in
// rootCmdPreRun so parsing failures can be returned through Cobra normally.
func init() {
	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// add sub-commands
	rootCmd.AddCommand(
		avatar.NewCommand("go-listen"),
		man.NewManCmd(),
		version.Command(),
	)
}
