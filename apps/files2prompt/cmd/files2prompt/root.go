// Package cmd provides command-line interface functionality for the files2prompt application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the files2prompt tool.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal/files2prompt
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/files2prompt/cmd/files2prompt"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/files2prompt/internal/files2prompt"
	"github.com/toozej/files2prompt/pkg/config"
	"github.com/toozej/files2prompt/pkg/man"
	"github.com/toozej/files2prompt/pkg/version"
)

// conf holds the application configuration loaded from environment variables.
// It is populated during package initialization and can be modified by command-line flags.
var (
	conf config.Config
	// debug controls the logging level for the application.
	// When true, debug-level logging is enabled through logrus.
	debug bool
)

// rootCmd defines the base command for the files2prompt CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts file paths as arguments and can also read paths from stdin.
// It supports various filtering and formatting options for preparing files for AI prompts.
var rootCmd = &cobra.Command{
	Use:   "files2prompt [paths...]",
	Short: "Crawl and output file contents with various filtering options for AI prompting",
	Long: `files2prompt helps prepare files for AI prompts by crawling directories
and outputting file contents with optional filtering and formatting.`,
	Args:             cobra.ArbitraryArgs,
	PersistentPreRun: rootCmdPreRun,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read paths from stdin if available
		stdinPaths := readPathsFromStdin(conf.Null)
		// Combine args and stdin paths
		conf.Paths = append(args, stdinPaths...)
		if len(conf.Paths) == 0 {
			return fmt.Errorf("no paths provided via arguments or stdin")
		}
		return files2prompt.Run(conf)
	},
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

// readPathsFromStdin reads file paths from standard input when available.
//
// This function checks if stdin contains data and reads it as a list of file paths.
// It supports two input formats based on the useNull parameter:
//   - When useNull is true: paths are separated by null characters (\x00)
//   - When useNull is false: paths are separated by whitespace
//
// The function performs the following operations:
//  1. Checks if stdin has available data
//  2. Reads all content from stdin
//  3. Splits content based on the specified separator
//  4. Filters out empty path entries
//  5. Returns the list of valid paths
//
// Parameters:
//   - useNull: If true, use null character as separator; otherwise use whitespace
//
// Returns:
//   - []string: List of file paths read from stdin, or nil if no input available
//
// Example:
//
//	// Read whitespace-separated paths
//	paths := readPathsFromStdin(false)
//
//	// Read null-separated paths (useful with find -print0)
//	paths := readPathsFromStdin(true)
func readPathsFromStdin(useNull bool) []string {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil
	}
	// Check if stdin has data
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil // No input
	}
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil
	}
	var paths []string
	if useNull {
		paths = strings.Split(string(content), "\x00")
	} else {
		paths = strings.Fields(string(content))
	}
	// Filter empty
	var filtered []string
	for _, p := range paths {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
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
// meaning it's inherited by all subcommands. Other flags allow overriding
// configuration values from environment variables or .env files.
func init() {
	// get configuration from environment variables
	conf = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// override .env configurations with flags+args
	if len(conf.Extensions) == 0 {
		rootCmd.Flags().StringSliceVarP(&conf.Extensions, "extension", "e", []string{}, "File extensions to include")
	}
	if !conf.IncludeHidden {
		rootCmd.Flags().BoolVarP(&conf.IncludeHidden, "include-hidden", "", false, "Include hidden files and folders")
	}
	if !conf.IgnoreGitignore {
		rootCmd.Flags().BoolVarP(&conf.IgnoreGitignore, "ignore-gitignore", "", false, "Ignore .gitignore files")
	}
	if len(conf.IgnorePatterns) == 0 {
		rootCmd.Flags().StringSliceVarP(&conf.IgnorePatterns, "ignore", "", []string{},
			"Patterns to ignore (can be comma-separated or specified multiple times). "+
				"Use '/' suffix to match directories only. Examples: "+
				"'*.test.js', 'test/', 'path/to/ignore/, 'dir1/,dir2/'")
	}
	if conf.OutputFile == "" {
		rootCmd.Flags().StringVarP(&conf.OutputFile, "output", "o", "", "Output file path")
	}
	if !conf.ClaudeXML {
		rootCmd.Flags().BoolVarP(&conf.ClaudeXML, "cxml", "c", false, "Output in XML format for Claude")
	}
	if !conf.LineNumbers {
		rootCmd.Flags().BoolVarP(&conf.LineNumbers, "line-numbers", "n", false, "Display line numbers in output")
	}
	if !conf.Markdown {
		rootCmd.Flags().BoolVarP(&conf.Markdown, "markdown", "m", false, "Output in Markdown format with fenced code blocks")
	}
	if !conf.Null {
		rootCmd.Flags().BoolVarP(&conf.Null, "null", "0", false, "Use NUL character as separator when reading from stdin")
	}

	// add sub-commands
	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}
