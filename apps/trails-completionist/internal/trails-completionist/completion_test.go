package trailscompletionist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
	"github.com/toozej/monogo/apps/trails-completionist/internal/generator"
	"github.com/toozej/monogo/apps/trails-completionist/internal/parser"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

func completionFixture(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	write := func(name, contents string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	feature := func(id int, name, park, status string, lines [][][]float64) map[string]any {
		return map[string]any{"type": "Feature", "properties": map[string]any{"OBJECTID": id, "TrailName": name, "ParksProperty": park, "TrailStatus": status, "TrailType": "Trail/Pathway", "url": "https://example.test/trail"}, "geometry": map[string]any{"type": "MultiLineString", "coordinates": lines}}
	}
	line := func(lat, west, east float64) [][][]float64 { return [][][]float64{{{west, lat}, {east, lat}}} }
	features := []map[string]any{
		feature(1, "Municipal & Trail", "Park A", "existing", line(45, -122, -121.999)),
		feature(2, "", "Park A", "existing", line(45, -121.999, -121.998)),
		feature(3, "Municipal & Trail", "Park B", "existing", line(45.01, -122, -121.999)),
		feature(4, "Multipart Trail", "Park A", "existing", [][][]float64{{{-121.996, 45}, {-121.995, 45}}, {{-121.992, 45}, {-121.991, 45}}}),
		feature(5, "Closed Trail", "Park A", "closed", line(45, -121.998, -121.997)),
	}
	data, err := json.Marshal(map[string]any{"type": "FeatureCollection", "name": "test-inventory", "features": features})
	if err != nil {
		t.Fatal(err)
	}
	inventory := write("inventory.geojson", string(data))
	text := write("input.txt", "Text Trail\nTrail 0.1 miles\nRegion > Park A\n")
	osm := write("region.osm", `<osm version="0.6">
<node id="1" lat="45" lon="-122"/><node id="2" lat="45" lon="-121.999"/><node id="3" lat="45" lon="-121.998"/><node id="4" lat="45" lon="-121.997"/>
<node id="5" lat="45" lon="-121.987"/><node id="6" lat="45" lon="-121.986"/>
<node id="7" lat="45" lon="-121.98"/><node id="8" lat="45" lon="-121.979"/>
<way id="10"><nd ref="1"/><nd ref="2"/><tag k="highway" v="path"/><tag k="name" v="Different OSM Name"/></way>
<way id="11"><nd ref="2"/><nd ref="3"/><tag k="highway" v="path"/></way>
<way id="12"><nd ref="3"/><nd ref="4"/><tag k="highway" v="path"/><tag k="name" v="OSM Closure Name"/></way>
<way id="13"><nd ref="5"/><nd ref="6"/><tag k="highway" v="path"/><tag k="name" v="Text Trail"/></way>
<way id="14"><nd ref="7"/><nd ref="8"/><tag k="highway" v="path"/><tag k="name" v="Unlisted Trail"/></way>
</osm>`)
	tracks := filepath.Join(dir, "tracks")
	if err := os.Mkdir(tracks, 0o700); err != nil {
		t.Fatal(err)
	}
	write("tracks/a-later.gpx", `<gpx version="1.1"><trk>
<trkseg><trkpt lat="45" lon="-122"><time>2026-08-10T12:00:00Z</time></trkpt><trkpt lat="45" lon="-121.997"/></trkseg>
<trkseg><trkpt lat="45" lon="-121.996"/><trkpt lat="45" lon="-121.995"/></trkseg>
<trkseg><trkpt lat="45" lon="-121.987"/><trkpt lat="45" lon="-121.986"/></trkseg>
<trkseg><trkpt lat="45" lon="-121.98"/><trkpt lat="45" lon="-121.979"/></trkseg>
</trk></gpx>`)
	write("tracks/z-earlier.gpx", `<gpx version="1.1"><trk><trkseg><trkpt lat="45" lon="-122"><time>2026-07-05T12:00:00Z</time></trkpt><trkpt lat="45" lon="-121.999"/></trkseg></trk></gpx>`)
	return config.Config{OSMRegionFile: osm, InputFile: text, InventoryFiles: []string{inventory}, TrackFiles: tracks, ChecklistFile: filepath.Join(dir, "checklist.md"), HTMLFile: filepath.Join(dir, "trails.html")}
}

func TestGPXCompletionSurvivesInventoryMergeAndOutput(t *testing.T) {
	cfg := completionFixture(t)
	if err := RunTrailsCompletionist(cfg, false); err != nil {
		t.Fatal(err)
	}
	trails, err := parser.ParseTrailsFromChecklist(cfg.ChecklistFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(trails) != 7 {
		t.Fatalf("want five inventory records, one text record, and one discovery, got %#v", trails)
	}
	byID := map[string]types.Trail{}
	for _, trail := range trails {
		byID[trail.ID] = trail
	}
	first := byID["test-inventory/1"]
	if !first.Completed || first.CompletionDate != "07/05/2026" || first.Name != "Municipal & Trail" || first.Park != "Park A" || first.Source != "test-inventory" || first.Type != "Trail/Pathway" || first.URL != "https://example.test/trail" {
		t.Fatalf("completion or inventory metadata was lost: %#v", first)
	}
	if !byID["test-inventory/2"].Completed {
		t.Fatal("unnamed inventory segment did not receive completion")
	}
	for _, id := range []string{"test-inventory/3", "test-inventory/4", "test-inventory/5"} {
		if byID[id].Completed {
			t.Fatalf("distant, partial, or closed record %s became complete", id)
		}
	}
	if !byID["osm/way/14"].Completed {
		t.Fatal("GPX discovery absent from the inventory was lost")
	}
	completed := 0
	for _, trail := range trails {
		if trail.Completed {
			completed++
		}
		if trail.ID == "osm/way/10" || trail.ID == "osm/way/11" || trail.ID == "osm/way/12" || trail.ID == "osm/way/13" {
			t.Fatalf("OSM duplicate remained in the output: %#v", trail)
		}
	}
	if completed != 4 {
		t.Fatalf("got %d completed rows, want four", completed)
	}
	for _, path := range []string{cfg.HTMLFile, filepath.Join(filepath.Dir(cfg.HTMLFile), "from-checklist.html")} {
		if path != cfg.HTMLFile {
			if err := generator.GenerateHTMLOutput(path, trails); err != nil {
				t.Fatal(err)
			}
		}
		page, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(page), `type="checkbox" checked`) != 4 {
			t.Fatalf("HTML lost completed checkboxes: %s", path)
		}
		_, encoded, ok := strings.Cut(string(page), `<script id="trail-data" type="application/json">`)
		if !ok {
			t.Fatal("HTML omitted its full trail collection")
		}
		encoded, _, _ = strings.Cut(encoded, "</script>")
		var data struct {
			Records []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Completed bool   `json:"completed"`
				Date      string `json:"date"`
			} `json:"records"`
		}
		if err := json.Unmarshal([]byte(encoded), &data); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, record := range data.Records {
			if record.ID == "test-inventory/1" {
				found = true
				if !record.Completed || record.Date != "07/05/2026" || record.Name != "Municipal & Trail" {
					t.Fatalf("HTML lost completion or metadata: %+v", record)
				}
			}
		}
		if !found {
			t.Fatal("HTML omitted the completed inventory record")
		}
	}
}

func TestGPXMatchesInventoryWithoutOSM(t *testing.T) {
	cfg := completionFixture(t)
	cfg.OSMRegionFile, cfg.InputFile = "", ""
	trails, err := CollectTrails(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(trails) != 5 {
		t.Fatalf("inventory records were lost or duplicated: %#v", trails)
	}
	completed := 0
	for _, trail := range trails {
		if trail.Completed {
			completed++
		}
		if trail.ID == "test-inventory/1" && trail.CompletionDate != "07/05/2026" {
			t.Fatalf("earliest activity date was lost: %#v", trail)
		}
	}
	if completed != 2 {
		t.Fatalf("got %d completed inventory records, want two", completed)
	}
}
