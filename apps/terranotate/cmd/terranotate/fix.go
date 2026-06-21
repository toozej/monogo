package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/toozej/terranotate/internal/app"
)

var fixRevert bool

var fixCmd = &cobra.Command{
	Use:   "fix [terraform-file-or-dir] [schema-file]",
	Short: "Auto-fix validation issues by adding missing comments",
	Args:  cobra.RangeArgs(1, 2),
	Run:   runFixCommand,
}

func init() {
	rootCmd.AddCommand(fixCmd)
	fixCmd.Flags().BoolVar(&fixRevert, "revert", false, "Revert to backup files (restore .bak files)")
}

func runFixCommand(cmd *cobra.Command, args []string) {
	path := args[0]

	// Handle revert mode
	if fixRevert {
		if err := app.RevertFix(afero.NewOsFs(), path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	// Normal fix mode requires schema file
	if len(args) < 2 {
		fmt.Println("Error: schema-file argument is required for fix mode")
		fmt.Println("Usage: terranotate fix [terraform-file-or-dir] [schema-file]")
		fmt.Println("   or: terranotate fix --revert [terraform-file-or-dir]")
		os.Exit(1)
	}

	schemaFile := args[1]

	if err := app.Fix(afero.NewOsFs(), path, schemaFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
