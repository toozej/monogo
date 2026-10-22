// Package simplenote implements the backend.Backend interface for Simplenote
// by invoking the sncli command-line tool.
package simplenote

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/note"
)

// Default max retries and delays for exponential backoff.
const (
	defaultMaxRetries = 5
	defaultBaseDelay  = 1.0
	defaultMaxDelay   = 300.0
)

// dumpRunner matches the signature used to execute a sncli dump.
type dumpRunner func(ctx context.Context, cliPath, username, password, tag string) ([]byte, error)

// Client implements backend.Backend for Simplenote.
type Client struct {
	cliPath    string
	username   string
	password   string
	maxRetries int
	baseDelay  float64
	maxDelay   float64
	sleepFunc  func(time.Duration)
	dumpFunc   dumpRunner
	jsonExport bool
}

// Option customizes the Simplenote client.
type Option func(*Client)

// WithJSONExport preserves source IDs, timestamps, and complete note content.
func WithJSONExport() Option {
	return func(c *Client) {
		c.jsonExport = true
		c.dumpFunc = defaultExportRunner
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithCredentials supplies the Simplenote credentials used by sncli. They are
// passed only to the child process, so the configured values work even when
// the parent environment does not already contain SN_USERNAME and SN_PASSWORD.
func WithCredentials(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// NewClient creates a new Simplenote backend client.
func NewClient(cliPath string, opts ...Option) *Client {
	c := &Client{
		cliPath:    cliPath,
		maxRetries: defaultMaxRetries,
		baseDelay:  defaultBaseDelay,
		maxDelay:   defaultMaxDelay,
		dumpFunc:   defaultDumpRunner,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// defaultDumpRunner executes sncli dump and returns its stdout bytes.
func defaultDumpRunner(ctx context.Context, cliPath, username, password, tag string) ([]byte, error) {
	args := []string{"--config=/dev/null", "-r", "dump"}
	if tag != "" {
		args = append(args, tag)
	}
	return runSNCLI(ctx, cliPath, username, password, args)
}

// defaultExportRunner fetches all active notes. The parser applies exact tag filtering.
func defaultExportRunner(ctx context.Context, cliPath, username, password, _ string) ([]byte, error) {
	return runSNCLI(ctx, cliPath, username, password, []string{"--config=/dev/null", "export"})
}

func runSNCLI(ctx context.Context, cliPath, username, password string, args []string) ([]byte, error) {
	// #nosec G204: The configuration supplies the sncli binary path.
	cmd := exec.CommandContext(ctx, cliPath, args...)
	cmd.Env = os.Environ()
	if username != "" {
		cmd.Env = append(cmd.Env, "SN_USERNAME="+username)
	}
	if password != "" {
		cmd.Env = append(cmd.Env, "SN_PASSWORD="+password)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sncli command failed: %w", err)
	}
	return out, nil
}

// FetchNotes runs sncli to dump notes with the given tag and returns parsed notes.
// It retries with exponential backoff on failure.
func (c *Client) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			delay := c.exponentialBackoffDelay(attempt - 1)
			if err := c.waitForRetry(ctx, delay); err != nil {
				return nil, err
			}
		}

		data, err := c.dumpFunc(ctx, c.cliPath, c.username, c.password, tag)
		if err != nil {
			lastErr = err
			continue
		}

		if c.jsonExport {
			return parseJSONExport(data, tag)
		}

		parser := note.NewParser()
		notes := parser.ParseNotes(discardSNCLILogLines(string(data)))
		if tag != "" && !allNotesHaveTag(notes, tag) {
			return nil, fmt.Errorf("sncli dump validation failed: requested tag %q missing from dump", tag)
		}

		return notes, nil
	}

	return nil, fmt.Errorf("failed to fetch notes from Simplenote after %d attempts: %w", c.maxRetries, lastErr)
}

func (c *Client) waitForRetry(ctx context.Context, delay time.Duration) error {
	if c.sleepFunc != nil {
		c.sleepFunc(delay)
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// discardSNCLILogLines removes status lines sncli can include in dump output.
// Those lines are not note content and would otherwise confuse the dump parser.
func discardSNCLILogLines(dump string) string {
	logMessages := []string{
		"sncli database doesn't exist",
		"Starting full sync",
		"Synced new note from server",
		"Saved note to disk",
		"Full sync completed",
	}
	lines := strings.Split(dump, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		isLog := false
		for _, message := range logMessages {
			if strings.Contains(line, message) {
				isLog = true
				break
			}
		}
		if !isLog {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// allNotesHaveTag verifies that a filtered sncli dump did not silently return
// unfiltered data. An empty dump is invalid when a tag was requested.
func allNotesHaveTag(notes []backend.Note, tag string) bool {
	if len(notes) == 0 {
		return false
	}
	for _, n := range notes {
		found := false
		for _, noteTag := range n.Tags {
			if noteTag == tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// exponentialBackoffDelay calculates the delay for a given attempt index.
func (c *Client) exponentialBackoffDelay(attempt int) time.Duration {
	multiplier := 1.0
	for i := 0; i < attempt; i++ {
		multiplier *= 2
	}
	delay := c.baseDelay * multiplier
	if delay > c.maxDelay {
		delay = c.maxDelay
	}
	// Add jitter: ±25%
	jitter := delay * 0.25 * (2*randomFloat() - 1)
	delay += jitter
	if delay < 0.1 {
		delay = 0.1
	}
	return time.Duration(delay * float64(time.Second))
}

// randomFloat returns a float64 in the range [0.0, 1.0).
// It is shadowed in tests.
var randomFloat = func() float64 {
	return float64(time.Now().UnixNano()%1000) / 1000.0
}
