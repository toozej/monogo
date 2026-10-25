package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/trails-completionist/internal/generator"
	trailscompletionist "github.com/toozej/monogo/apps/trails-completionist/internal/trails-completionist"
)

var GenerateChecklistCmd = &cobra.Command{
	Use:   "generate-checklist",
	Short: "Generate a checklist from trail inventories, OSM, and GPX tracks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if conf.ChecklistFile == "" {
			return fmt.Errorf("checklistFile must be specified via flag or env var")
		}
		trails, err := trailscompletionist.CollectTrails(conf)
		if err != nil {
			return err
		}
		return generator.GenerateChecklist(conf.ChecklistFile, trails)
	},
}
