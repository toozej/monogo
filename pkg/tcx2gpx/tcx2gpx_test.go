package tcx2gpx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertAllTCXToGPXPublishesAtomically(t *testing.T) {
	dir := t.TempDir()
	tcxPath := filepath.Join(dir, "walk.tcx")
	tcx := `<TrainingCenterDatabase><Activities><Activity Sport="Other"><Id>walk</Id><Lap StartTime="2026-07-10T00:00:00Z"><Track><Trackpoint><Time>2026-07-10T00:00:00Z</Time><Position><LatitudeDegrees>45.5</LatitudeDegrees><LongitudeDegrees>-122.6</LongitudeDegrees></Position></Trackpoint></Track></Lap></Activity></Activities></TrainingCenterDatabase>`
	if err := os.WriteFile(tcxPath, []byte(tcx), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConvertAllTCXToGPX(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tcxPath); !os.IsNotExist(err) {
		t.Fatalf("converted source should be removed, stat error = %v", err)
	}
	gpx, err := os.ReadFile(filepath.Join(dir, "walk.gpx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gpx), "<trkpt lat=\"45.5\" lon=\"-122.6\">") {
		t.Fatalf("unexpected GPX output: %s", gpx)
	}
}

func TestConvertAllTCXToGPXPreservesSourceOnFailure(t *testing.T) {
	dir := t.TempDir()
	tcxPath := filepath.Join(dir, "broken.tcx")
	if err := os.WriteFile(tcxPath, []byte("<broken>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConvertAllTCXToGPX(dir); err == nil {
		t.Fatal("invalid TCX should return an error")
	}
	if _, err := os.Stat(tcxPath); err != nil {
		t.Fatalf("failed conversion must preserve source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "broken.gpx")); !os.IsNotExist(err) {
		t.Fatalf("failed conversion must not publish GPX, stat error = %v", err)
	}
}
