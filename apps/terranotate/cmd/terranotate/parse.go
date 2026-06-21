package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/toozej/terranotate/internal/app"
)

var parseCmd = &cobra.Command{
	Use:   "parse [terraform-file]",
	Short: "Parse and display Terraform file comments",
	Args:  cobra.ExactArgs(1),
	Run:   runParseCommand,
}

func init() {
	rootCmd.AddCommand(parseCmd)
}

func runParseCommand(cmd *cobra.Command, args []string) {
	filename := args[0]

	if err := app.Parse(afero.NewOsFs(), filename); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
