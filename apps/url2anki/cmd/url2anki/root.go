// Package cmd provides command-line interface functionality for the url2anki application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the url2anki flashcard generation application.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal/url2anki
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/url2anki/cmd/url2anki"
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

	"github.com/toozej/url2anki/internal/url2anki"
	"github.com/toozej/url2anki/pkg/config"
	"github.com/toozej/url2anki/pkg/man"
	"github.com/toozej/url2anki/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf config.Config
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through logrus.
	debug bool
)

var rootCmd = &cobra.Command{
	Use:              "url2anki",
	Short:            "Generate Anki flashcards from a URL",
	Long:             `Generate Anki-formatted flashcards from a given URL and export them to a file to be imported into Anki`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	Run:              rootCmdRun,
}

// rootCmdRun is the main execution function for the root command.
// It calls the url2anki package's Run function with the current configuration.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments (unused, as root command takes no args)
func rootCmdRun(cmd *cobra.Command, args []string) {
	url2anki.Run(cmd, args)
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
//   - Marks required flags for proper validation
//
// The debug flag (-d, --debug) enables debug-level logging and is persistent,
// meaning it's inherited by all subcommands. Other flags allow overriding
// configuration values from environment variables or .env files.
//
// Required flags:
//   - url: The URL to scrape for flashcards
//   - question-selector: HTML selector for questions
//   - answer-selector: HTML selector for answers
func init() {
	// get configuration from environment variables
	conf = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", conf.Debug, "Enable debug-level logging")

	// CLI flags that can override environment variables
	rootCmd.Flags().StringVarP(&conf.URL, "url", "u", conf.URL, "The URL to scrape for flashcards (EX: https://kubernetes.io/docs/reference/glossary/?all=true)")
	rootCmd.Flags().StringVarP(&conf.QuestionSelector, "question-selector", "q", conf.QuestionSelector, "The HTML selector for the questions (EX: div.term-name)")
	rootCmd.Flags().StringVarP(&conf.AnswerSelector, "answer-selector", "a", conf.AnswerSelector, "The HTML selector for the answers (EX: div.term-definition)")
	rootCmd.Flags().StringVarP(&conf.OutputFile, "output-file", "o", conf.OutputFile, "The filename (including extension) to export flashcards to")
	rootCmd.Flags().BoolVarP(&conf.Preview, "preview", "p", conf.Preview, "Preview the flashcards before exporting")

	// Mark required flags
	_ = cobra.MarkFlagRequired(rootCmd.Flags(), "url")
	_ = cobra.MarkFlagRequired(rootCmd.Flags(), "question-selector")
	_ = cobra.MarkFlagRequired(rootCmd.Flags(), "answer-selector")

	// add sub-commands
	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}
