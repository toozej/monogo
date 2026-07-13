package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/trails-completionist/internal/parser"
	"github.com/toozej/monogo/pkg/osm"
)

var ParseGPXCmd = &cobra.Command{
	Use:   "parse-gpx",
	Short: "Parse trails out of GPX files",
	RunE: func(cmd *cobra.Command, args []string) error {
		trackFiles := conf.TrackFiles
		if trackFiles == "" {
			return fmt.Errorf("trackFiles must be specified via flag or env var")
		}
		osmData, err := loadConfiguredOSM()
		if err != nil {
			return err
		}
		trails, err := parser.ParseTrailsFromTrackFiles(trackFiles, true, osmData)
		if err != nil {
			return err
		}
		fmt.Printf("Parsed trails: %v\n", trails)
		return nil
	},
}

func loadConfiguredOSM() (*osm.OSMData, error) {
	if conf.OSMRegionFile == "" {
		return nil, fmt.Errorf("osmRegionFile must be specified when parsing GPX tracks")
	}
	data, err := osm.LoadOSMData(conf.OSMRegionFile, false)
	if err != nil {
		return nil, fmt.Errorf("loading OSM region file: %w", err)
	}
	return data, nil
}
