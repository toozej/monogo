// Package cmd provides command-line interface functionality for the golang-starter application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the golang-starter template application.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal/starter
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/golang-starter/cmd/golang-starter"
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

	"github.com/toozej/monogo/apps/golang-starter/internal/config"
	"github.com/toozej/monogo/apps/golang-starter/internal/starter"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/logging"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

// rootCmd defines the base command for the golang-starter CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and delegates its main functionality
// to the starter package. It supports persistent flags that are inherited by
// all subcommands.
var rootCmd *cobra.Command

// newRootCommand constructs the root cobra command together with its flags and
// subcommands.
//
// SilenceErrors and SilenceUsage are enabled so that command failures are
// reported exactly once, by Execute, rather than also being printed by cobra.
//
// The debug flag (-d, --debug) is persistent and inherited by all subcommands;
// it enables debug-level logging. The username flag (-u, --username) is local to
// the root command and overrides the username loaded from configuration.
func newRootCommand() *cobra.Command {
	var debug bool
	var username string

	cmd := &cobra.Command{
		Use:           "golang-starter",
		Short:         "golang-starter starter template",
		Long:          `Golang starter template using cobra, slog, dotenv and env modules`,
		Args:          cobra.ExactArgs(0),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			rootCmdPreRun(debug)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmdRun(cmd, args, username)
		},
	}
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username")
	cmd.AddCommand(
		avatar.NewCommand("golang-starter"),
		man.NewManCmd(),
		version.Command(),
	)
	return cmd
}

// rootCmdRun is the main execution function for the root command.
// It calls the starter package's Run function with the configured username.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments (unused, as root command takes no args)
//   - username: The value of the root command's username flag
func rootCmdRun(cmd *cobra.Command, args []string, username string) error {
	loadedConf, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if cmd.Flags().Changed("username") {
		loadedConf.Username = username
	}
	starter.Run(loadedConf.Username)
	return nil
}

// rootCmdPreRun performs setup operations before executing the root command.
// This function is called before both the root command and any subcommands.
//
// It configures the default slog logger based on the debug flag. When debug
// mode is enabled, the logger is set to debug level for detailed logging
// output; otherwise it logs at info level.
//
// Parameters:
//   - debug: Whether debug logging was requested for this command tree
func rootCmdPreRun(debug bool) {
	level := "info"
	if debug {
		level = "debug"
	}
	slog.SetDefault(logging.NewLogger(logging.Config{Level: level, Format: "json", Output: "stdout"}))
}

// Execute starts the command-line interface execution.
// This is the main entry point called from main.go to begin command processing.
//
// If command execution fails, it prints the error message to stderr and
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
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// init initializes the command-line interface during package loading.
//
// It builds the root command, including its flags and subcommands, via
// newRootCommand. Application configuration is loaded later by rootCmdRun so
// utility subcommands such as "version" and "man" do not read .env or the
// application environment.
func init() {
	rootCmd = newRootCommand()
}
