package obsidian

import (
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"gopkg.in/yaml.v3"
)

func TestFormatObsidianProperties(t *testing.T) {
	n := backend.Note{
		ID: "abc123", Source: "simplenote", Title: "C#: \"旅行\"",
		Date:       time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("offset", 3600)),
		Modified:   time.Date(2025, 2, 3, 4, 5, 6, 250000000, time.UTC),
		Tags:       []string{"blog:go", "two words", "123", "#Go", "go", "旅行", "😀", "!!!", "C++", "a=b"},
		SystemTags: []string{"markdown", "pinned"},
		Content:    "# C#: \"旅行\"\n\n| a | b |\n|---|---|\n---\n\n  code\n",
	}
	out := NewFormatter().Format(n)
	parts := strings.SplitN(out, "---\n\n", 2)
	if len(parts) != 2 || parts[1] != n.Content {
		t.Fatalf("original Markdown changed: %q", out)
	}
	var properties map[string]any
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---\n")), &properties); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if properties["title"] != n.Title || properties["simplenote_id"] != n.ID || properties["source"] != n.Source {
		t.Fatalf("metadata changed: %#v", properties)
	}
	if properties["created"] != n.Date.UTC() || properties["modified"] != n.Modified {
		t.Fatalf("unexpected date properties: %#v", properties)
	}
	tags := properties["tags"].([]any)
	want := []string{"blog/go", "two-words", "tag-123", "Go", "旅行", "😀", "untagged", "C", "a-b"}
	if len(tags) != len(want) {
		t.Fatalf("unexpected normalized tags: %v", tags)
	}
	for i, value := range want {
		if tags[i] != value {
			t.Errorf("tag %d = %q, want %q", i, tags[i], value)
		}
	}
	if len(properties["simplenote_tags"].([]any)) != len(n.Tags) {
		t.Fatal("original tags were lost")
	}
	if len(properties["aliases"].([]any)) != 1 || len(properties["simplenote_system_tags"].([]any)) != 2 {
		t.Fatalf("aliases or system tags changed: %#v", properties)
	}
	for _, key := range []string{"url", "categories", "unlisted", "type", "summary"} {
		if _, exists := properties[key]; exists {
			t.Errorf("unexpected site property: %s", key)
		}
	}
}

func TestFormatOmitsUnknownDatesAndUsesEmptyLists(t *testing.T) {
	out := NewFormatter().Format(backend.Note{ID: "abc", Source: "simplenote", Title: "Untitled"})
	for _, value := range []string{"created:", "modified:", "0001-"} {
		if strings.Contains(out, value) {
			t.Errorf("unknown timestamp in output: %s", out)
		}
	}
	for _, key := range []string{"tags", "simplenote_tags", "simplenote_system_tags"} {
		if !strings.Contains(out, key+": []\n") {
			t.Errorf("missing empty list for %s", key)
		}
	}
}
