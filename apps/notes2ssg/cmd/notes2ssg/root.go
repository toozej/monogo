// Package cmd provides command-line interface functionality for the notes2ssg application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the notes2ssg application.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/notes2ssg/internal/config"
	"github.com/toozej/monogo/apps/notes2ssg/internal/converter"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf config.Config
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through logrus.
	debug bool
)

// rootCmd defines the base command for the notes2ssg CLI application.
var rootCmd = &cobra.Command{
	Use:              "notes2ssg",
	Short:            "Convert notes to Hugo-formatted Markdown",
	Long:             `Notes2ssg fetches notes from Simplenote or Usememos and writes them as Hugo Markdown files.`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	RunE:             rootCmdRun,
}

// runConverter creates the app and runs exports until a signal cancels the
// context. A zero polling cycle runs one export pass. Tests can replace it.
var runConverter = func(cfg config.Config) error {
	app, err := converter.NewApp(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.RunPolling(ctx)
}

// rootCmdRun is the main execution function for the root command.
func rootCmdRun(cmd *cobra.Command, args []string) error {
	return runConverter(confEffective())
}

// confEffective returns the current effective configuration. It is extracted
// to a variable so tests can override it.
var confEffective = func() config.Config {
	return conf
}

// rootCmdPreRun performs setup operations before executing the root command.
func rootCmdPreRun(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}
}

// Execute starts the command-line interface execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	// get configuration from environment variables
	conf = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// add sub-commands
	rootCmd.AddCommand(
		avatar.NewCommand("notes2ssg"),
		man.NewManCmd(),
		version.Command(),
	)
}
