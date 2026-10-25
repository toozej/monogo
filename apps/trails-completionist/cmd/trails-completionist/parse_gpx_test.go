package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
)

func TestParseGPXUsesInventoryAndPrintsOnlyCompletedRecords(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	inventory := write("inventory.geojson", `{"type":"FeatureCollection","name":"fixture","features":[
{"type":"Feature","properties":{"id":"1","name":"Inventory Trail"},"geometry":{"type":"LineString","coordinates":[[-122,45],[-121.999,45]]}},
{"type":"Feature","properties":{"id":"2","name":"Unwalked Trail"},"geometry":{"type":"LineString","coordinates":[[-122,45.01],[-121.999,45.01]]}}
]}`)
	write("walk.gpx", `<gpx version="1.1"><trk><trkseg><trkpt lat="45" lon="-122"><time>2026-10-01T12:00:00Z</time></trkpt><trkpt lat="45" lon="-121.999"/></trkseg></trk></gpx>`)
	previousConfig, previousOutput := conf, ParseGPXCmd.OutOrStdout()
	t.Cleanup(func() { conf = previousConfig; ParseGPXCmd.SetOut(previousOutput) })
	conf = config.Config{InventoryFiles: []string{inventory}, TrackFiles: dir}
	var output bytes.Buffer
	ParseGPXCmd.SetOut(&output)
	if err := ParseGPXCmd.RunE(ParseGPXCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "fixture/1\tInventory Trail\t0.049 miles\t10/01/2026") || strings.Contains(output.String(), "Unwalked Trail") {
		t.Fatalf("parse-gpx did not report completed inventory records: %s", output.String())
	}
}
