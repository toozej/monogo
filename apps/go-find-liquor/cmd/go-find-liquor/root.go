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
//	import "github.com/toozej/go-find-liquor/cmd/go-find-liquor"
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
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-liquor/internal/runner"
	"github.com/toozej/go-find-liquor/pkg/config"
	"github.com/toozej/go-find-liquor/pkg/man"
	"github.com/toozej/go-find-liquor/pkg/version"
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
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration summary for multi-user scenarios
	logConfigurationSummary(conf)

	// Create runner (supports both single and multi-user configurations)
	r, err := runner.NewRunner(conf)
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Info("Received termination signal, shutting down...")
		r.Stop()
		cancel()
	}()

	// Run once or continuously
	if once {
		log.Info("Running single search for all configured users")
		if err := r.RunOnce(ctx); err != nil {
			log.Errorf("Failed to run single search: %v", err)
			return err
		}
		log.Info("Single search completed successfully")
	} else {
		userCount := len(conf.Users)
		if userCount == 1 {
			log.Infof("Starting continuous search for user '%s' with interval %.0f hours",
				conf.Users[0].Name, conf.Interval.Hours())
		} else {
			log.Infof("Starting continuous search for %d users with interval %.0f hours",
				userCount, conf.Interval.Hours())
		}

		if err := r.Start(ctx); err != nil {
			log.Errorf("Failed to run continuous search: %v", err)
			return err
		}
		log.Info("Continuous search completed")
	}

	return nil
}

func logConfigurationSummary(conf config.Config) {
	userCount := len(conf.Users)

	if userCount == 1 {
		user := conf.Users[0]
		log.Infof("Configuration loaded: Single user '%s'", user.Name)
		log.Infof("  - Items: %d", len(user.Items))
		log.Infof("  - Location: %s (within %d miles)", user.Zipcode, user.Distance)
		log.Infof("  - Notifications: %d configured", len(user.Notifications))

		// Log condensing status for notifications
		for i, notif := range user.Notifications {
			condenseStatus := "individual"
			if notif.Condense {
				condenseStatus = "condensed"
			}
			log.Infof("  - Notification %d (%s): %s messages", i+1, notif.Type, condenseStatus)
		}
	} else {
		log.Infof("Configuration loaded: Multi-user setup with %d users", userCount)
		for i, user := range conf.Users {
			log.Infof("  User %d: '%s' - %d items, %s (%d miles), %d notifications",
				i+1, user.Name, len(user.Items), user.Zipcode, user.Distance, len(user.Notifications))
		}
	}

	log.Infof("Global settings: interval=%.0fh, verbose=%t", conf.Interval.Hours(), conf.Verbose)
	if conf.UserAgent != "" {
		log.Infof("Using custom user agent: %s", conf.UserAgent)
	}
}

func rootCmdPreRun(cmd *cobra.Command, args []string) {
	// Set custom config file if specified
	if configFile != "" {
		config.SetConfigFile(configFile)
		log.Infof("Using config file: %s", configFile)
	}

	// Set log level based on debug flag or config verbose setting
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled via command line flag")
	} else {
		// Load config to check verbose setting
		if conf, err := config.GetConfig(); err == nil && conf.Verbose {
			log.SetLevel(log.DebugLevel)
			log.Debug("Debug logging enabled via configuration")
		}
	}
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
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
		man.NewManCmd(),
		version.Command(),
	)
}
