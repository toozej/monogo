package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	trailscompletionist "github.com/toozej/monogo/apps/trails-completionist/internal/trails-completionist"
)

var ParseGPXCmd = &cobra.Command{
	Use:   "parse-gpx",
	Short: "Parse trails out of GPX files",
	RunE: func(cmd *cobra.Command, args []string) error {
		trackFiles := conf.TrackFiles
		if trackFiles == "" {
			return fmt.Errorf("trackFiles must be specified via flag or env var")
		}
		trails, err := trailscompletionist.CollectTrails(conf)
		if err != nil {
			return err
		}
		for _, trail := range trails {
			if !trail.Completed {
				continue
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s miles\t%s\n", trail.ID, trail.Name, trail.Length, trail.CompletionDate); err != nil {
				return err
			}
		}
		return nil
	},
}
