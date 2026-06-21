package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/toozej/terranotate/internal/app"
)

var generateOutput string

var generateCmd = &cobra.Command{
	Use:   "generate [path] [schema-file]",
	Short: "Generate markdown documentation from Terraform resources and their annotations",
	Long: `Generate markdown documentation tables from Terraform resources.

Creates a markdown document with a table per module showing:
  - Resource type and name
  - All required metadata fields from schema
  - Actual values from resource annotations

Output is written to stdout by default, or to a file with --output flag.`,
	Args: cobra.ExactArgs(2),
	Run:  runGenerateCommand,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output file (default: stdout)")
}

func runGenerateCommand(cmd *cobra.Command, args []string) {
	path := args[0]
	schemaFile := args[1]

	if err := app.Generate(afero.NewOsFs(), path, schemaFile, generateOutput); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
