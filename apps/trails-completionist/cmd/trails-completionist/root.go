// Package cmd provides command-line interface functionality for the trails-completionist application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the trails-completionist application.
//
// The package integrates with several components:
//   - Configuration management through pkg/config
//   - Core functionality through internal packages
//   - Manual pages through pkg/man
//   - Version information through pkg/version
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/trails-completionist/cmd/trails-completionist"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd

import (
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
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

// rootCmd defines the base command for the trails-completionist CLI application.
// It serves as the entry point for all command-line operations and establishes
// the application's structure, flags, and subcommands.
//
// The command accepts no positional arguments and provides tools for tracking
// completion of trails through various subcommands.
var rootCmd = &cobra.Command{
	Use:               "trails-completionist",
	Short:             "tools for tracking completion of trails",
	Long:              `A Golang application to parse a list of trails from a directory of track files, then display the found trails in a searchable HTML table for ease of tracking completion.`,
	PersistentPreRunE: rootCmdPreRun,
}

// rootCmdPreRun performs setup operations before executing any command.
// This function is called before both the root command and any subcommands.
//
// It configures the logging level based on the debug flag. When debug mode
// is enabled, logrus is set to DebugLevel for detailed logging output.
//
// Parameters:
//   - cmd: The cobra command being executed
//   - args: Command-line arguments
func rootCmdPreRun(cmd *cobra.Command, args []string) error {
	if debug {
		log.SetLevel(log.DebugLevel)
	}
	if math.IsNaN(conf.MatchDistance) || math.IsInf(conf.MatchDistance, 0) || conf.MatchDistance <= 0 || conf.MatchDistance > 1000 || math.IsNaN(conf.MinimumCoverage) || conf.MinimumCoverage <= 0 || conf.MinimumCoverage > 1 {
		return fmt.Errorf("matchDistance must be greater than zero and at most 1000. minimumCoverage must be greater than zero and at most one")
	}
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

// init initializes the command-line interface during package loading.
//
// This function performs the following setup operations:
//   - Loads configuration from environment variables using config.GetEnvVars()
//   - Defines persistent flags that are available to all commands
//   - Sets up command-specific flags for configuration options
//   - Registers subcommands for various trail processing operations
//
// The debug flag (-d, --debug) enables debug-level logging and is persistent,
// meaning it's inherited by all subcommands. Configuration flags allow
// overriding environment variables with command-line options.
func init() {
	// get configuration from environment variables
	conf = config.GetEnvVars()

	// create rootCmd-level flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	// optional flags for configuration, overrides env vars
	rootCmd.PersistentFlags().StringVarP(&conf.TrackFiles, "trackFiles", "t", conf.TrackFiles, "Track files directory")
	rootCmd.PersistentFlags().StringVarP(&conf.OSMRegionFile, "osmRegionFile", "r", conf.OSMRegionFile, "OSM region file")
	rootCmd.PersistentFlags().StringVarP(&conf.InputFile, "inputFile", "i", conf.InputFile, "Input file")
	rootCmd.PersistentFlags().StringSliceVar(&conf.InventoryFiles, "inventoryFile", conf.InventoryFiles, "GeoJSON inventory files. Repeat this flag for additional files")
	rootCmd.PersistentFlags().StringVar(&conf.BoundaryFile, "boundaryFile", conf.BoundaryFile, "GeoJSON polygon that selects and clips inventory and OSM geometry")
	rootCmd.PersistentFlags().StringVar(&conf.ReportFile, "reportFile", conf.ReportFile, "JSON audit output file")
	rootCmd.PersistentFlags().Float64Var(&conf.MatchDistance, "matchDistance", conf.MatchDistance, "Maximum distance in metres for geometry coverage")
	rootCmd.PersistentFlags().Float64Var(&conf.MinimumCoverage, "minimumCoverage", conf.MinimumCoverage, "Required length fraction from zero to one")
	rootCmd.PersistentFlags().StringVarP(&conf.ChecklistFile, "checklistFile", "c", conf.ChecklistFile, "Checklist file")
	rootCmd.PersistentFlags().StringVarP(&conf.HTMLFile, "htmlFile", "o", conf.HTMLFile, "HTML file")
	rootCmd.PersistentFlags().BoolVarP(&conf.Serve, "serve", "s", conf.Serve, "Serve the generated HTML file")
	rootCmd.PersistentFlags().StringVar(&conf.ServeAddress, "serveAddress", conf.ServeAddress, "HTTP listen address (host:port). Use 0.0.0.0:3000 for LAN access")

	// add sub-commands from separate files
	rootCmd.AddCommand(
		avatar.NewCommand("trails-completionist"),
		man.NewManCmd(),
		version.Command(),
		ConvertCmd,
		OsmExportCmd,
		AuditOSMCmd,
		ParseGPXCmd,
		GenerateChecklistCmd,
		GenerateHTMLCmd,
		ServeCmd,
		FullCmd,
	)
}
