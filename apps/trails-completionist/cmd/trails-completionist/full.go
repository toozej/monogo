package cmd

import (
	"github.com/spf13/cobra"
	trailscompletionist "github.com/toozej/monogo/apps/trails-completionist/internal/trails-completionist"
)

var FullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run the full trails-completionist workflow",
	RunE: func(cmd *cobra.Command, args []string) error {
		return trailscompletionist.RunTrailsCompletionist(conf, debug)
	},
}
