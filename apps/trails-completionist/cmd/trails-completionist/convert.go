package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/trails-completionist/pkg/tcx2gpx"
)

var ConvertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert TCX files to GPX format",
	RunE: func(cmd *cobra.Command, args []string) error {
		trackFiles := conf.TrackFiles
		if trackFiles == "" {
			return fmt.Errorf("trackFiles must be specified via flag or env var")
		}
		return tcx2gpx.ConvertAllTCXToGPX(trackFiles)
	},
}
