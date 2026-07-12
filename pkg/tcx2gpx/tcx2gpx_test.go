package tcx2gpx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const validTCX = `<TrainingCenterDatabase><Activities><Activity Sport="Other"><Id>walk</Id><Lap StartTime="2026-07-10T00:00:00Z"><Track><Trackpoint><Time>2026-07-10T00:00:00Z</Time><Position><LatitudeDegrees>45.5</LatitudeDegrees><LongitudeDegrees>-122.6</LongitudeDegrees></Position></Trackpoint></Track></Lap></Activity></Activities></TrainingCenterDatabase>`

func TestConvertAllTCXToGPXPublishesAtomically(t *testing.T) {
	dir := t.TempDir()
	tcxPath := filepath.Join(dir, "walk.tcx")
	if err := os.WriteFile(tcxPath, []byte(validTCX), 0o600); err != nil {
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

func TestConvertAllTCXToGPXRejectsTrailingXML(t *testing.T) {
	dir := t.TempDir()
	tcxPath := filepath.Join(dir, "trailing.tcx")
	if err := os.WriteFile(tcxPath, []byte(validTCX+`<unexpected/>`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ConvertAllTCXToGPX(dir); err == nil {
		t.Fatal("TCX with trailing XML must fail")
	}
	if _, err := os.Stat(tcxPath); err != nil {
		t.Fatalf("rejected TCX source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "trailing.gpx")); !os.IsNotExist(err) {
		t.Fatalf("rejected TCX published GPX, stat error = %v", err)
	}
}

func TestConvertAllTCXToGPXPreservesOutputModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	dir := t.TempDir()
	reference, err := os.Create(filepath.Join(dir, "reference"))
	if err != nil {
		t.Fatal(err)
	}
	if err := reference.Close(); err != nil {
		t.Fatal(err)
	}
	referenceInfo, err := os.Stat(reference.Name())
	if err != nil {
		t.Fatal(err)
	}

	tcxPath := filepath.Join(dir, "walk.tcx")
	gpxPath := filepath.Join(dir, "walk.gpx")
	if err := os.WriteFile(tcxPath, []byte(validTCX), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConvertAllTCXToGPX(dir); err != nil {
		t.Fatal(err)
	}
	gpxInfo, err := os.Stat(gpxPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gpxInfo.Mode().Perm(), referenceInfo.Mode().Perm(); got != want {
		t.Fatalf("new GPX mode = %o, want os.Create mode %o", got, want)
	}

	if err := os.Chmod(gpxPath, 0o660); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tcxPath, []byte(validTCX), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConvertAllTCXToGPX(dir); err != nil {
		t.Fatal(err)
	}
	gpxInfo, err = os.Stat(gpxPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gpxInfo.Mode().Perm(), os.FileMode(0o660); got != want {
		t.Fatalf("replacement GPX mode = %o, want preserved %o", got, want)
	}
}

func TestConvertAllTCXToGPXContinuesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.tcx")
	brokenPath := filepath.Join(dir, "broken.tcx")
	if err := os.WriteFile(validPath, []byte(validTCX), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brokenPath, []byte("<broken>"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ConvertAllTCXToGPX(dir)
	if err == nil || !strings.Contains(err.Error(), "broken.tcx") {
		t.Fatalf("ConvertAllTCXToGPX() error = %v, want broken file context", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "valid.gpx")); err != nil {
		t.Fatalf("valid file was not converted: %v", err)
	}
	if _, err := os.Stat(validPath); !os.IsNotExist(err) {
		t.Fatalf("valid converted source remains, stat error = %v", err)
	}
	if _, err := os.Stat(brokenPath); err != nil {
		t.Fatalf("failed source was not preserved: %v", err)
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
