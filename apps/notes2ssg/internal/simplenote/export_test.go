package simplenote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestJSONExportPreservesMetadataAndBody(t *testing.T) {
	data := []byte(`Starting full sync
Saved note to disk
Full sync completed
[
  {"key":"abc123","content":"# C# notes\n\n| a | b |\n|---|---|\nStarting full sync\n+-----+\n","tags":["blog:go","two words"],"systemtags":["markdown","pinned"],"creationDate":"1693535615.25","modificationDate":1693535616.5},
  {"key":"other","content":"Other note","tags":["blog:gopher"]},
  {"key":"trash","content":"Deleted","deleted":1}
]`)
	client := NewClient("sncli", WithJSONExport())
	client.dumpFunc = func(context.Context, string, string, string, string) ([]byte, error) { return data, nil }
	notes, err := client.FetchNotes(context.Background(), "blog:go")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected one exact tag match, got %d", len(notes))
	}
	n := notes[0]
	if n.ID != "abc123" || n.Source != "simplenote" || n.Title != "C# notes" {
		t.Fatalf("source metadata changed: %+v", n)
	}
	if !n.Date.Equal(time.Unix(1693535615, 250000000)) || !n.Modified.Equal(time.Unix(1693535616, 500000000)) {
		t.Fatalf("timestamps changed: %v, %v", n.Date, n.Modified)
	}
	if n.Content != "# C# notes\n\n| a | b |\n|---|---|\nStarting full sync\n+-----+\n" {
		t.Fatalf("body changed: %q", n.Content)
	}
	if len(n.SystemTags) != 2 || n.SystemTags[1] != "pinned" {
		t.Fatalf("system tags changed: %v", n.SystemTags)
	}
}

func TestJSONExportRejectsInvalidSnapshots(t *testing.T) {
	for _, input := range []string{
		"", "null", "unexpected output\n[]", "[]\nunexpected output", "[{}]",
		`[{"key":"abc"}]`, `[{"key":"abc","content":"a"},{"key":"abc","content":"b"}]`,
		`[{"key":"abc","content":"a","creationDate":"invalid"}]`,
		`[{"key":"abc","content":"a","modificationDate":-1}]`,
		`[{"key":"abc","content":"a","creationDate":253402300800}]`,
		`[{"key":"abc","content":"a","deleted":"unknown"}]`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := parseJSONExport([]byte(input), ""); err == nil {
				t.Fatalf("accepted invalid snapshot %q", input)
			}
		})
	}
}

func TestJSONExportAllowsEmptySnapshotsAndMissingDates(t *testing.T) {
	for _, input := range []string{"[]", `[{"key":"abc","content":"Untitled body","tags":["other"]}]`} {
		notes, err := parseJSONExport([]byte(input), "missing")
		if err != nil || len(notes) != 0 {
			t.Fatalf("empty filtered snapshot: %v, %v", notes, err)
		}
	}
	notes, err := parseJSONExport([]byte(`[{"key":"abc","content":"\n"}]`), "")
	if err != nil || len(notes) != 1 || notes[0].Title != "Untitled" || !notes[0].Date.IsZero() {
		t.Fatalf("empty note or missing date changed: %v, %v", notes, err)
	}
}

func TestDefaultExportRunnerUsesJSONAndNoRemoteSearch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the helper requires a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "sncli")
	script := "#!/bin/sh\nprintf '%s|%s|%s|%s|%s' \"$SN_USERNAME\" \"$SN_PASSWORD\" \"$#\" \"$1\" \"$2\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := defaultExportRunner(context.Background(), path, "user", "secret", "tag:with regex.*")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "user|secret|2|--config=/dev/null|export" {
		t.Fatalf("unexpected export command: %s", got)
	}
}

func TestJSONExportCancelsRetryDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := NewClient("sncli", WithJSONExport())
	client.baseDelay = 3600
	client.maxDelay = 3600
	calls := 0
	client.dumpFunc = func(context.Context, string, string, string, string) ([]byte, error) {
		calls++
		cancel()
		return nil, errors.New("source failure")
	}
	if _, err := client.FetchNotes(ctx, ""); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("retry did not stop after cancellation: %d calls, %v", calls, err)
	}
}
