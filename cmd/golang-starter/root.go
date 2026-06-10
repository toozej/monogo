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
//	import "github.com/toozej/golang-starter/cmd/golang-starter"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/golang-starter/internal/starter"
	"github.com/toozej/golang-starter/pkg/config"
	"github.com/toozej/golang-starter/pkg/man"
	"github.com/toozej/golang-starter/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf config.Config
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through logrus.
	debug bool
)

// rootCmd defines the base command for the golang-starter CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and delegates its main functionality
// to the starter package. It supports persistent flags that are inherited by
// all subcommands.
var rootCmd = &cobra.Command{
	Use:              "golang-starter",
	Short:            "golang starter template",
	Long:             `Golang starter template using cobra, logrus, dotenv and env modules`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	Run:              rootCmdRun,
}

// rootCmdRun is the main execution function for the root command.
// It calls the starter package's Run function with the configured username.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments (unused, as root command takes no args)
func rootCmdRun(cmd *cobra.Command, args []string) {
	starter.Run(conf.Username)
}

// rootCmdPreRun performs setup operations before executing the root command.
// This function is called before both the root command and any subcommands.
//
// It configures the logging level based on the debug flag. When debug mode
// is enabled, logrus is set to DebugLevel for detailed logging output.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments
func rootCmdPreRun(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}
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
//   - Loads configuration from environment variables using config.GetEnvVars()
//   - Defines persistent flags that are available to all commands
//   - Sets up command-specific flags for the root command
//   - Registers subcommands (man pages and version information)
//
// The debug flag (-d, --debug) enables debug-level logging and is persistent,
// meaning it's inherited by all subcommands. The username flag (-u, --username)
// allows overriding the username from environment variables.
func init() {
	// get configuration from environment variables
	conf = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// optional flag for username, overrides env var
	rootCmd.Flags().StringVarP(&conf.Username, "username", "u", conf.Username, "Username")

	// add sub-commands
	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}
