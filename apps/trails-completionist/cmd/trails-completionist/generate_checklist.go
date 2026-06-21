package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/trails-completionist/internal/generator"
	"github.com/toozej/trails-completionist/internal/matcher"
	"github.com/toozej/trails-completionist/internal/parser"
	"github.com/toozej/trails-completionist/internal/types"
)

var GenerateChecklistCmd = &cobra.Command{
	Use:   "generate-checklist",
	Short: "Generate trails checklist from raw input and GPX files",
	RunE: func(cmd *cobra.Command, args []string) error {
		trackFiles := conf.TrackFiles
		inputFile := conf.InputFile
		checklistFile := conf.ChecklistFile
		if inputFile == "" || checklistFile == "" {
			return fmt.Errorf("inputFile and checklistFile must be specified via flag or env var")
		}
		var foundGPXTrails []types.Trail
		var err error
		if trackFiles != "" {
			foundGPXTrails, err = parser.ParseTrailsFromTrackFiles(trackFiles, true, nil)
			if err != nil {
				return err
			}
		}
		rawTrails, err := parser.ParseTrailsFromRawInputFile(inputFile)
		if err != nil {
			return err
		}
		combined, err := matcher.MatchTrails(foundGPXTrails, rawTrails)
		if err != nil {
			return err
		}
		return generator.GenerateChecklist(checklistFile, combined)
	},
}
