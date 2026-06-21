// Package cmd provides command-line interface functionality for the ghreleases2rss application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for subscribing to GitHub repository release feeds in Miniflux RSS reader.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal/ghreleases2rss
//   - GitHub API integration through internal/github
//   - Miniflux RSS integration through internal/miniflux
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/ghreleases2rss/cmd/ghreleases2rss"
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

	"github.com/toozej/ghreleases2rss/internal/ghreleases2rss"
	"github.com/toozej/ghreleases2rss/pkg/config"
	"github.com/toozej/ghreleases2rss/pkg/man"
	"github.com/toozej/ghreleases2rss/pkg/version"
)

// debug controls the logging level for the application.
// When true, debug-level logging is enabled through logrus.
var debug bool

// conf holds the application configuration loaded from environment variables.
var conf config.Config

// rootCmd defines the base command for the ghreleases2rss CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and delegates its main functionality
// to the ghreleases2rss package. It supports persistent flags that are inherited by
// all subcommands for debug logging and category feed management.
var rootCmd = &cobra.Command{
	Use:              "ghreleases2rss",
	Short:            "Subscribe to GitHub projects' releases in RSS reader",
	Long:             `Subscribe to GitHub repo release feeds in Miniflux`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	Run:              rootCmdRun,
}

// rootCmdRun is a wrapper function that validates configuration and passes
// the loaded configuration to the ghreleases2rss.Run function.
func rootCmdRun(cmd *cobra.Command, args []string) {
	// Validate required configuration only when main command runs
	if err := config.ValidateRequired(conf); err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	ghreleases2rss.Run(cmd, args, conf)
}

// rootCmdPreRun performs setup operations before executing the root command.
// This function is called before both the root command and any subcommands.
//
// It loads configuration from environment variables and configures the logging
// level based on the debug flag. When debug mode is enabled, logrus is set to
// DebugLevel for detailed logging output.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments
func rootCmdPreRun(cmd *cobra.Command, args []string) {
	// Load configuration from environment variables
	conf = config.GetEnvVars()

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
//   - Defines persistent flags that are available to all commands
//   - Sets up command-specific flags for the root command
//   - Registers subcommands (man pages and version information)
//
// The debug flag (-d, --debug) enables debug-level logging and is persistent,
// meaning it's inherited by all subcommands. The clearCategoryFeeds flag (-r)
// allows clearing existing feeds in a category before adding new ones. The file
// flag (-f) specifies the input file containing GitHub repository URLs, and the
// category flag (-c) allows organizing feeds into categories.
func init() {
	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")
	rootCmd.PersistentFlags().BoolP("clearCategoryFeeds", "r", false, "Delete all feeds within category before subscribing to new feeds")
	rootCmd.Flags().StringP("file", "f", "", "Input file with GitHub repo URLs or names (required)")
	rootCmd.Flags().StringP("category", "c", "", "RSS feed category name (optional)")
	_ = rootCmd.MarkFlagRequired("file")

	// add sub-commands
	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}
