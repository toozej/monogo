package simplenote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

// exportedNote follows sncli's JSON export, including Simplenote server metadata.
type exportedNote struct {
	Key              string          `json:"key"`
	Content          *string         `json:"content"`
	Tags             []string        `json:"tags"`
	SystemTags       []string        `json:"systemtags"`
	CreationDate     json.Number     `json:"creationDate"`
	ModificationDate json.Number     `json:"modificationDate"`
	Deleted          json.RawMessage `json:"deleted"`
}

func parseJSONExport(data []byte, tag string) ([]backend.Note, error) {
	// A first sncli run can print sync status before the JSON array.
	// Remove only known status lines before JSON. Preserve note content exactly.
	data = bytes.TrimSpace(data)
	for len(data) > 0 && data[0] != '[' {
		line, rest, _ := bytes.Cut(data, []byte("\n"))
		if strings.TrimSpace(discardSNCLILogLines(string(line))) != "" {
			return nil, fmt.Errorf("sncli export contains unexpected output before JSON")
		}
		data = bytes.TrimSpace(rest)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("sncli export is empty; expected a JSON array")
	}
	var exported []exportedNote
	if err := json.Unmarshal(data, &exported); err != nil {
		return nil, fmt.Errorf("decoding sncli export: %w", err)
	}
	var notes []backend.Note
	seen := make(map[string]bool)
	for _, n := range exported {
		switch string(n.Deleted) {
		case "1", "true":
			continue
		case "", "0", "false", "null":
		default:
			return nil, fmt.Errorf("sncli export has an invalid deleted flag for note %q", n.Key)
		}
		if n.Key == "" || n.Content == nil || seen[n.Key] {
			return nil, fmt.Errorf("sncli export has a missing key, missing content, or duplicate key for note %q", n.Key)
		}
		seen[n.Key] = true
		created, err := exportTime(n.CreationDate)
		if err != nil {
			return nil, fmt.Errorf("note %q creationDate: %w", n.Key, err)
		}
		modified, err := exportTime(n.ModificationDate)
		if err != nil {
			return nil, fmt.Errorf("note %q modificationDate: %w", n.Key, err)
		}
		if tag != "" && !hasExactTag(n.Tags, tag) {
			continue
		}
		notes = append(notes, backend.Note{
			ID: n.Key, Source: "simplenote", Title: exportTitle(*n.Content),
			Date: created, Modified: modified, Tags: n.Tags,
			SystemTags: n.SystemTags, Content: *n.Content,
		})
	}
	return notes, nil
}

func exportTime(value json.Number) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	seconds, err := value.Float64()
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds >= 253402300800 {
		return time.Time{}, fmt.Errorf("invalid Unix timestamp %q", value)
	}
	whole, fraction := math.Modf(seconds)
	return time.Unix(int64(whole), int64(math.Round(fraction*1e9))).UTC(), nil
}

func exportTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if strings.HasPrefix(line, "# ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			return line
		}
	}
	return "Untitled"
}

func hasExactTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
