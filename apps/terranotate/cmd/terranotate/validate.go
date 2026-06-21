package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/toozej/terranotate/internal/app"
)

var validateCmd = &cobra.Command{
	Use:   "validate [path] [schema-file]",
	Short: "Validate Terraform files, modules, or workspaces against schema",
	Long: `Validate Terraform files against a schema.
Auto-detects whether the path is a single file, module, or workspace.

A module is detected if:
  - The path contains a modules/ subdirectory
  - The path itself is inside a modules/ directory

A workspace is detected if:
  - Multiple subdirectories or environment directories are found
  - Or if it's explicitly a large multi-module setup

Otherwise, it validates as a single file or directory.`,
	Args: cobra.ExactArgs(2),
	Run:  runValidateCommand,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidateCommand(cmd *cobra.Command, args []string) {
	path := args[0]
	schemaFile := args[1]

	if err := app.ValidateAuto(afero.NewOsFs(), path, schemaFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
