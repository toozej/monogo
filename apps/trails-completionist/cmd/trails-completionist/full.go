package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	trailscompletionist "github.com/toozej/monogo/apps/trails-completionist/internal/trails-completionist"
)

var FullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run the full trails-completionist workflow",
	Run: func(cmd *cobra.Command, args []string) {
		if err := trailscompletionist.RunTrailsCompletionist(conf, debug); err != nil {
			log.Fatal(err)
		}
	},
}
