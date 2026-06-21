package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/trails-completionist/internal/generator"
	"github.com/toozej/trails-completionist/internal/parser"
)

var GenerateHTMLCmd = &cobra.Command{
	Use:   "generate-html",
	Short: "Generate HTML page from template and trails checklist file",
	RunE: func(cmd *cobra.Command, args []string) error {
		checklistFile := conf.ChecklistFile
		htmlFile := conf.HTMLFile
		if checklistFile == "" || htmlFile == "" {
			return fmt.Errorf("checklistFile and htmlFile must be specified via flag or env var")
		}
		trails, err := parser.ParseTrailsFromChecklist(checklistFile)
		if err != nil {
			return err
		}
		return generator.GenerateHTMLOutput(htmlFile, trails)
	},
}
