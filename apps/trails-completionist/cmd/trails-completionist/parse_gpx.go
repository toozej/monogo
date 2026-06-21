package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/trails-completionist/internal/parser"
)

var ParseGPXCmd = &cobra.Command{
	Use:   "parse-gpx",
	Short: "Parse trails out of GPX files",
	RunE: func(cmd *cobra.Command, args []string) error {
		trackFiles := conf.TrackFiles
		if trackFiles == "" {
			return fmt.Errorf("trackFiles must be specified via flag or env var")
		}
		trails, err := parser.ParseTrailsFromTrackFiles(trackFiles, true, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Parsed trails: %v\n", trails)
		return nil
	},
}
