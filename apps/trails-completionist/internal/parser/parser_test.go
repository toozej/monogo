package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseChecklistIncludesFinalTrail(t *testing.T) {
	checklist := `# Trails
## Forest Park
- First Trail
    - Trail
    - 1.0 miles
- Final Trail
    - Connector
    - 2.5 miles
    - Completed 07/10/2026
`
	path := filepath.Join(t.TempDir(), "checklist.md")
	if err := os.WriteFile(path, []byte(checklist), 0o600); err != nil {
		t.Fatal(err)
	}
	trails, err := ParseTrailsFromChecklist(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(trails) != 2 || trails[1].Name != "Final Trail" || !trails[1].Completed {
		t.Fatalf("parsed trails = %#v", trails)
	}
}

func TestParseTracksRequiresOSMData(t *testing.T) {
	if _, err := ParseTrailsFromTrackFiles(t.TempDir(), true, nil); err == nil {
		t.Fatal("expected a clear error when OSM data is missing")
	}
}

func TestMissingInputReturnsError(t *testing.T) {
	if _, err := ParseTrailsFromRawInputFile(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("missing input must return an error instead of terminating the process")
	}
}
