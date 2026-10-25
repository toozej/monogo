package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	trailscompletionist "github.com/toozej/monogo/apps/trails-completionist/internal/trails-completionist"
)

var AuditOSMCmd = &cobra.Command{
	Use:   "audit-osm",
	Short: "List OSM trail candidates and audit input inventories",
	RunE: func(cmd *cobra.Command, args []string) error {
		if conf.OSMRegionFile == "" {
			return fmt.Errorf("osmRegionFile is required for an OSM audit")
		}
		cfg := conf
		if cfg.ReportFile == "" {
			cfg.ReportFile = "osm-audit.json"
		}
		cfg.TrackFiles = ""
		_, err := trailscompletionist.CollectTrails(cfg)
		return err
	},
}
