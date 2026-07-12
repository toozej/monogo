package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
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

func TestParsersPropagateScannerErrors(t *testing.T) {
	longLine := strings.Repeat("x", 70*1024)
	tests := []struct {
		name    string
		content string
		parse   func(string) ([]types.Trail, error)
	}{
		{name: "raw input", content: longLine + "\n", parse: ParseTrailsFromRawInputFile},
		{name: "checklist", content: "- " + longLine + "\n", parse: ParseTrailsFromChecklist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.txt")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := tt.parse(path); err == nil {
				t.Fatal("oversized scanner token must return an error")
			}
		})
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

func TestRawInputRejectsIncompleteFinalRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.txt")
	if err := os.WriteFile(path, []byte("Trail Name\nTrail 1.0 miles\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ParseTrailsFromRawInputFile(path); err == nil {
		t.Fatal("partial final raw-input record must return an error")
	}
}
