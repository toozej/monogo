// Package cmd provides command-line interface functionality for the go-find-liquor application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the Oregon Liquor Search Notification Service.
//
// The package integrates with several components:
//   - Configuration management through pkg/config (YAML/env/godotenv)
//   - Core functionality through internal/runner (multi-user search orchestration)
//   - Search functionality through internal/search (OLCC website scraping)
//   - Notification system through internal/notification (multi-channel alerts)
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Key features:
//   - Multi-user configuration support
//   - Continuous and single-run search modes
//   - Signal handling for graceful shutdown
//   - Debug logging configuration
//   - Custom config file support
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/go-find-liquor/cmd/go-find-liquor"
//
//	func main() {
//		if err := cmd.Execute(); err != nil {
//			os.Exit(1)
//		}
//	}
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/runner"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/logging"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

var (
	configFile string
	once       bool
	debug      bool
)

var rootCmd = &cobra.Command{
	Use:              "go-find-liquor",
	Short:            "Oregon Liquor Search Notification Service",
	Long:             `Oregon Liquor Search Notification Service using the OLCC Liquor Search website, Go, and the nikoksr/notify library`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	RunE:             rootCmdRun,
}

func rootCmdRun(cmd *cobra.Command, args []string) error {
	// Get configuration
	conf, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// Log configuration summary for multi-user scenarios
	logConfigurationSummary(conf)

	// Create runner (supports both single and multi-user configurations)
	r, err := runner.NewRunner(conf)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	ctx := cmd.Context()

	// Run once or continuously
	if once {
		slog.Info("Running single search for all configured users")
		if err := r.RunOnce(ctx); err != nil {
			slog.Error(fmt.Sprintf("Failed to run single search: %v", err))
			return err
		}
		slog.Info("Single search completed successfully")
	} else {
		userCount := len(conf.Users)
		if userCount == 1 {
			slog.Info(fmt.Sprintf("Starting continuous search for user '%s' with interval %.0f hours",
				conf.Users[0].Name, conf.Interval.Hours()))
		} else {
			slog.Info(fmt.Sprintf("Starting continuous search for %d users with interval %.0f hours",
				userCount, conf.Interval.Hours()))
		}

		if err := r.Start(ctx); err != nil {
			slog.Error(fmt.Sprintf("Failed to run continuous search: %v", err))
			return err
		}
		slog.Info("Continuous search completed")
	}

	return nil
}

func logConfigurationSummary(conf config.Config) {
	userCount := len(conf.Users)

	if userCount == 1 {
		user := conf.Users[0]
		slog.Info(fmt.Sprintf("Configuration loaded: Single user '%s'", user.Name))
		slog.Info(fmt.Sprintf("  - Items: %d", len(user.Items)))
		slog.Info(fmt.Sprintf("  - Location: %s (within %d miles)", user.Zipcode, user.Distance))
		slog.Info(fmt.Sprintf("  - Notifications: %d configured", len(user.Notifications)))

		// Log condensing status for notifications
		for i, notif := range user.Notifications {
			condenseStatus := "individual"
			if notif.Condense {
				condenseStatus = "condensed"
			}
			slog.Info(fmt.Sprintf("  - Notification %d (%s): %s messages", i+1, notif.Type, condenseStatus))
		}
	} else {
		slog.Info(fmt.Sprintf("Configuration loaded: Multi-user setup with %d users", userCount))
		for i, user := range conf.Users {
			slog.Info(fmt.Sprintf("  User %d: '%s' - %d items, %s (%d miles), %d notifications",
				i+1, user.Name, len(user.Items), user.Zipcode, user.Distance, len(user.Notifications)))
		}
	}

	slog.Info(fmt.Sprintf("Global settings: interval=%.0fh, verbose=%t", conf.Interval.Hours(), conf.Verbose))
	if conf.UserAgent != "" {
		slog.Info(fmt.Sprintf("Using custom user agent: %s", conf.UserAgent))
	}
}

func rootCmdPreRun(cmd *cobra.Command, args []string) {
	// Set custom config file if specified
	if configFile != "" {
		config.SetConfigFile(configFile)
	}

	// Set log level based on debug flag or config verbose setting.
	level := "info"
	debugVia := ""
	if debug {
		level = "debug"
		debugVia = "command line flag"
	} else if conf, err := config.GetConfig(); err == nil && conf.Verbose {
		// Load config to check verbose setting
		level = "debug"
		debugVia = "configuration"
	}

	// Configure the process-wide structured logger before emitting any logs so
	// they use the configured (stderr, JSON) handler.
	slog.SetDefault(logging.NewLogger(logging.Config{Level: level, Format: "json", Output: "stderr"}))

	if configFile != "" {
		slog.Info(fmt.Sprintf("Using config file: %s", configFile))
	}
	if debugVia != "" {
		slog.Debug("Debug logging enabled via " + debugVia)
	}
}

func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rootCmd.SetContext(ctx)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	return nil
}

func init() {
	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file path")
	rootCmd.Flags().BoolVarP(&once, "once", "o", false, "Run search once and exit")

	// add sub-commands
	rootCmd.AddCommand(
		avatar.NewCommand("go-find-liquor"),
		man.NewManCmd(),
		version.Command(),
	)
}
