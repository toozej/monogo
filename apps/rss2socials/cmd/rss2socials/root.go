// Package cmd provides command-line interface functionality for the rss2socials application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the rss2socials application.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal/rss2socials
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/rss2socials/cmd/rss2socials"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/rss2socials/internal/config"
	rss2socials "github.com/toozej/monogo/apps/rss2socials/internal/rss2socials"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf          config.Config
	configLoadErr error
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through logrus.
	debug bool
)

// rootCmd defines the base command for the rss2socials CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and delegates its main functionality
// to the rss2socials package. It supports persistent flags that are inherited by
// all subcommands.
var rootCmd = &cobra.Command{
	Use:              "rss2socials",
	Short:            "Watches a RSS feed for new posts, then announces them on various social media sites",
	Long:             `Watches a RSS feed for new posts, then announces them on various social media sites`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	RunE:             rootCmdRun,
}

// rootCmdRun is the main execution function for the root command.
// It calls the rss2socials package's Run function with the loaded configuration.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments (unused, as root command takes no args)
func rootCmdRun(cmd *cobra.Command, args []string) error {
	// Validate required configuration only when the main command runs, so that
	// subcommands like completion, version, and man work without a fully
	// configured environment.
	if configLoadErr != nil {
		return fmt.Errorf("load configuration: %w", configLoadErr)
	}
	return rss2socials.RunContext(cmd.Context(), conf)
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
// It installs a signal handler so that SIGINT/SIGTERM cancel the run context
// and allow an in-flight cycle to unwind gracefully. If command execution fails
// for any reason other than that graceful cancellation, it prints the error
// message to stderr and exits the program with status code 1. This follows
// standard Unix conventions for command-line tool error handling.
//
// Example:
//
//	func main() {
//		cmd.Execute()
//	}
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rootCmd.SetContext(ctx)
	err := rootCmd.Execute()
	stop()
	if err != nil && !errors.Is(err, context.Canceled) {
		// context.Canceled is the expected result of a SIGINT/SIGTERM-driven
		// graceful shutdown, so treat it as a clean (exit 0) termination.
		fmt.Fprintln(os.Stderr, err.Error())
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
// meaning it's inherited by all subcommands. Other flags allow overriding
// configuration values from environment variables.
func init() {
	// get configuration from environment variables
	conf, configLoadErr = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// optional flags for configuration, overrides env vars
	rootCmd.Flags().StringVarP(&conf.FeedURL, "feed-url", "f", conf.FeedURL, "RSS feed URL to watch")
	rootCmd.Flags().IntVarP(&conf.Interval, "interval", "i", conf.Interval, "Interval in minutes to check the RSS feed")
	rootCmd.Flags().StringVarP(&conf.Category, "category", "c", conf.Category, "Category to filter URL last segment")
	rootCmd.Flags().StringSliceVar(&conf.SkipPrefixCategories, "skip-prefix-categories", conf.SkipPrefixCategories, "List of categories to skip the 'New blog post:' prefix")

	// Mastodon flags
	rootCmd.Flags().StringVar(&conf.MastodonURL, "mastodon-url", conf.MastodonURL, "Mastodon URL")
	rootCmd.Flags().StringVar(&conf.MastodonClientKey, "mastodon-client-key", conf.MastodonClientKey, "Mastodon Client Key")
	rootCmd.Flags().StringVar(&conf.MastodonClientSecret, "mastodon-client-secret", conf.MastodonClientSecret, "Mastodon Client Secret")
	rootCmd.Flags().StringVar(&conf.MastodonAccessToken, "mastodon-access-token", conf.MastodonAccessToken, "Mastodon Access Token")

	// Bluesky flags
	rootCmd.Flags().StringVar(&conf.BlueskyHandle, "bluesky-handle", conf.BlueskyHandle, "Bluesky handle")
	rootCmd.Flags().StringVar(&conf.BlueskyAppKey, "bluesky-appkey", conf.BlueskyAppKey, "Bluesky app key/password")

	// Threads flags
	rootCmd.Flags().StringVar(&conf.ThreadsUserID, "threads-user-id", conf.ThreadsUserID, "Threads User ID")
	rootCmd.Flags().StringVar(&conf.ThreadsToken, "threads-access-token", conf.ThreadsToken, "Threads Access Token")
	rootCmd.Flags().StringVar(&conf.ThreadsClientID, "threads-client-id", conf.ThreadsClientID, "Threads Client ID")
	rootCmd.Flags().StringVar(&conf.ThreadsClientSecret, "threads-client-secret", conf.ThreadsClientSecret, "Threads Client Secret")
	rootCmd.Flags().StringVar(&conf.ThreadsRedirectURI, "threads-redirect-uri", conf.ThreadsRedirectURI, "Threads Redirect URI")

	// Social sites filter flag
	rootCmd.Flags().StringSliceVar(&conf.SocialSites, "social-sites", conf.SocialSites, "Social media sites to post to (mastodon,bluesky,threads). Defaults to all sites with credentials configured.")

	// Gotify flags
	rootCmd.Flags().BoolVar(&conf.GotifyNotifyOnSuccess, "gotify-notify-on-success", conf.GotifyNotifyOnSuccess, "Send Gotify notifications on successful posts")

	// Dedup flags
	rootCmd.Flags().BoolVar(&conf.PostNewEntriesOnly, "post-new-entries-only", conf.PostNewEntriesOnly, "Only post entries that appear after first startup (skip existing feed entries)")
	rootCmd.Flags().BoolVar(&conf.ShortRun, "short-run", conf.ShortRun, "Short run mode: only process the 3 most recent RSS feed items")
	rootCmd.Flags().StringVar(&conf.DBPath, "db-path", conf.DBPath, "Path to SQLite database file")

	// add sub-commands
	rootCmd.AddCommand(
		avatar.NewCommand("rss2socials"),
		man.NewManCmd(),
		version.Command(),
	)
}
