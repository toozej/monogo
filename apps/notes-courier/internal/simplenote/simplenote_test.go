package simplenote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("/usr/bin/sncli")
	if c.cliPath != "/usr/bin/sncli" {
		t.Errorf("expected cliPath '/usr/bin/sncli', got %q", c.cliPath)
	}
	if c.maxRetries != defaultMaxRetries {
		t.Errorf("expected maxRetries %d, got %d", defaultMaxRetries, c.maxRetries)
	}
}

func TestFetchNotesSuccess(t *testing.T) {
	c := NewClient("sncli")
	c.maxRetries = 2
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		data := `+-----+
| Title: Test Note |
| Date: Fri, 01 Sep 2023 02:33:35 |
| Tags: blog |
+-----+
Body here.`
		return []byte(data), nil
	}

	notes, err := c.FetchNotes(context.Background(), "blog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %q", notes[0].Title)
	}
}

func TestFetchNotesRetries(t *testing.T) {
	c := NewClient("sncli")
	c.maxRetries = 3
	attempts := 0
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("simulated failure")
		}
		return []byte(`+-----+
| Title: Retry Note |
| Date: Fri, 01 Sep 2023 02:33:35 |
| Tags: blog |
+-----+
Body.`), nil
	}
	c.sleepFunc = func(d time.Duration) {} // no-op sleep

	notes, err := c.FetchNotes(context.Background(), "blog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestFetchNotesFailure(t *testing.T) {
	c := NewClient("sncli")
	c.maxRetries = 2
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		return nil, errors.New("always fails")
	}
	c.sleepFunc = func(d time.Duration) {}

	_, err := c.FetchNotes(context.Background(), "blog")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExponentialBackoffDelay(t *testing.T) {
	c := NewClient("sncli")
	d1 := c.exponentialBackoffDelay(0)
	d2 := c.exponentialBackoffDelay(1)
	d3 := c.exponentialBackoffDelay(2)

	if d1 <= 0 {
		t.Errorf("expected positive delay, got %v", d1)
	}
	if d2 <= d1 {
		t.Errorf("expected d2 > d1, got d2=%v d1=%v", d2, d1)
	}
	if d3 <= d2 {
		t.Errorf("expected d3 > d2, got d3=%v d2=%v", d3, d2)
	}
}

func TestFetchNotesEmptyUnfilteredResult(t *testing.T) {
	c := NewClient("sncli")
	c.maxRetries = 1
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		return []byte(""), nil
	}
	c.sleepFunc = func(d time.Duration) {}

	notes, err := c.FetchNotes(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error for empty unfiltered result: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected no notes, got %d", len(notes))
	}
}

func TestWithMaxRetries(t *testing.T) {
	c := NewClient("sncli", WithMaxRetries(10))
	if c.maxRetries != 10 {
		t.Errorf("expected maxRetries 10, got %d", c.maxRetries)
	}
}

func TestDefaultDumpRunner(t *testing.T) {
	_, err := defaultDumpRunner(context.Background(), "/nonexistent/sncli", "", "", "tag")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestDefaultDumpRunnerPassesCredentialsAndOmitsEmptyTag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper is a POSIX shell script")
	}

	path := filepath.Join(t.TempDir(), "sncli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s|%s|%s|%s|%s|%s' \"$SN_USERNAME\" \"$SN_PASSWORD\" \"$#\" \"$1\" \"$2\" \"$3\"\n"), 0o700); err != nil {
		t.Fatalf("writing sncli helper: %v", err)
	}

	data, err := defaultDumpRunner(context.Background(), path, "me@example.com", "secret", "")
	if err != nil {
		t.Fatalf("running sncli helper: %v", err)
	}
	if got, want := string(data), "me@example.com|secret|3|--config=/dev/null|-r|dump"; got != want {
		t.Errorf("unexpected unfiltered command: got %q, want %q", got, want)
	}
}

func TestFetchNotesRejectsDumpWithoutRequestedTag(t *testing.T) {
	c := NewClient("sncli")
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		return []byte(`+-----+
| Title: Other Note |
| Tags: other |
+-----+
Body.`), nil
	}

	_, err := c.FetchNotes(context.Background(), "blog")
	if err == nil {
		t.Fatal("expected tag validation error")
	}
}

func TestFetchNotesDiscardsSNCLILogLines(t *testing.T) {
	c := NewClient("sncli")
	c.dumpFunc = func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
		return []byte(`Starting full sync
+-----+
| Title: Test Note |
| Tags: blog |
+-----+
Body.
Full sync completed`), nil
	}

	notes, err := c.FetchNotes(context.Background(), "blog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 || notes[0].Title != "Test Note" {
		t.Fatalf("unexpected parsed notes: %#v", notes)
	}
}

func TestWithCredentials(t *testing.T) {
	c := NewClient("sncli", WithCredentials("me@example.com", "secret"))
	if c.username != "me@example.com" || c.password != "secret" {
		t.Fatal("expected configured Simplenote credentials")
	}
}

func TestRandomFloat(t *testing.T) {
	f := randomFloat()
	if f < 0 || f >= 1 {
		t.Errorf("expected float in [0,1), got %f", f)
	}
}
