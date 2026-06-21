package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/trails-completionist/pkg/osm"
)

var OsmExportCmd = &cobra.Command{
	Use:   "osm-export",
	Short: "Load OSM XML and export parsed map to binary file",
	RunE: func(cmd *cobra.Command, args []string) error {
		osmFile := conf.OSMRegionFile
		if osmFile == "" {
			return fmt.Errorf("osmRegionFile must be specified via flag or env var")
		}
		_, err := osm.LoadOSMData(osmFile, false)
		if err != nil {
			return err
		}
		return nil
	},
}
