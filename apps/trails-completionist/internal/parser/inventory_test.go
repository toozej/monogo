package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

func inputFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.geojson")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInventoryRetainsUnnamedAndMultipartRecords(t *testing.T) {
	path := inputFile(t, `{"type":"FeatureCollection","features":[
{"type":"Feature","properties":{"OBJECTID":1,"TrailName":null,"ParksProperty":"Forest Park","TrailStatus":"proposed","UserPedestrian":"no"},"geometry":{"type":"MultiLineString","coordinates":[[[-122,45],[-121.999,45]],[[-122,45.001],[-121.999,45.001]]]}},
{"type":"Feature","properties":{"OBJECTID":2,"TrailName":null},"geometry":null}
]}`)
	trails, err := ParseInventory(path, geo.Boundary{})
	if err != nil {
		t.Fatal(err)
	}
	if len(trails) != 2 || trails[0].ID == trails[1].ID || len(trails[0].Geometry) != 2 || !strings.Contains(trails[0].Status, "UserPedestrian=no") || !trails[1].Incomplete {
		t.Fatalf("inventory lost source data: %#v", trails)
	}
}

func TestInventoryRejectsIncompleteOrUnsupportedData(t *testing.T) {
	for _, contents := range []string{
		`{"type":"FeatureCollection","exceededTransferLimit":true,"features":[]}`,
		`{"type":"FeatureCollection","source":{"feature_count":20},"features":[]}`,
		`{"type":"FeatureCollection","crs":{"properties":{"name":"EPSG:3857"}},"features":[]}`,
		`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"id":"a"},"geometry":null},{"type":"Feature","properties":{"id":"a"},"geometry":null}]}`,
		`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"id":"a"},"geometry":{"type":"Point","coordinates":[0,0]}}]}`,
	} {
		if _, err := ParseInventory(inputFile(t, contents), geo.Boundary{}); err == nil {
			t.Fatalf("accepted invalid inventory %s", contents)
		}
	}
}

func TestGPXMatchesEveryCoveredTargetAndRejectsCrossing(t *testing.T) {
	dir := t.TempDir()
	contents := `<gpx version="1.1"><trk><trkseg><trkpt lat="45" lon="-122"><time>2026-01-01T12:00:00Z</time></trkpt><trkpt lat="45" lon="-121.98"/></trkseg></trk></gpx>`
	if err := os.WriteFile(filepath.Join(dir, "walk.GPX"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	var targets []types.Trail
	for i := range 8 {
		lon := -122 + float64(i)*.002
		targets = append(targets, types.Trail{ID: fmt.Sprint(i), Name: fmt.Sprintf("Trail %d", i), Geometry: [][]types.Point{{{Lat: 45, Lon: lon}, {Lat: 45, Lon: lon + .001}}}})
	}
	targets = append(targets, types.Trail{ID: "crossing", Name: "Crossing", Geometry: [][]types.Point{{{Lat: 44.998, Lon: -121.99}, {Lat: 45.002, Lon: -121.99}}}})
	trails, err := ParseTracksWithTargets(dir, true, targets, 25, .9)
	if err != nil {
		t.Fatal(err)
	}
	if len(trails) != 8 {
		t.Fatalf("got %d trails, want all eight covered trails: %#v", len(trails), trails)
	}
	for _, trail := range trails {
		if !trail.Completed || trail.CompletionDate != "01/01/2026" {
			t.Fatalf("invalid completion: %#v", trail)
		}
	}
}

func TestGPXDoesNotBridgeSegmentsOrIgnoreErrors(t *testing.T) {
	dir := t.TempDir()
	contents := `<gpx version="1.1"><trk><trkseg><trkpt lat="45" lon="-122"/><trkpt lat="45" lon="-121.999"/></trkseg><trkseg><trkpt lat="45" lon="-121.991"/><trkpt lat="45" lon="-121.99"/></trkseg></trk></gpx>`
	path := filepath.Join(dir, "walk.gpx")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	targets := []types.Trail{{ID: "gap", Geometry: [][]types.Point{{{Lat: 45, Lon: -121.996}, {Lat: 45, Lon: -121.994}}}}}
	trails, err := ParseTracksWithTargets(dir, true, targets, 25, .9)
	if err != nil || len(trails) != 0 {
		t.Fatalf("GPX gap matched: %#v, %v", trails, err)
	}
	if err := os.WriteFile(path, []byte("invalid GPX"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTracksWithTargets(dir, true, targets, 25, .9); err == nil {
		t.Fatal("invalid GPX was silently ignored")
	}
}

func TestTextInputRetainsDifferentLengthsWithEqualNames(t *testing.T) {
	path := inputFile(t, "Connector Trail\nConnector 0.1 miles\nRegion > Park\nConnector Trail\nConnector 0.2 miles\nRegion > Park\nConnector Trail\nConnector 0.1 miles\nRegion > Park\n")
	trails, err := ParseTrailsFromRawInputFile(path)
	if err != nil || len(trails) != 2 {
		t.Fatalf("distinct connectors were lost: %#v, %v", trails, err)
	}
}
