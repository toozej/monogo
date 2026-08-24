// Package usememos implements the backend.Backend interface for Usememos.
package usememos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
)

const defaultTimeout = 30 * time.Second

// HTTPClient is the interface used to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements backend.Backend for Usememos.
type Client struct {
	baseURL string
	token   string
	client  HTTPClient
}

// NewClient creates a new Usememos backend client.
func NewClient(baseURL, token string, client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		client:  client,
	}
}

// FetchNotes retrieves notes from the Usememos API filtered by tag.
func (c *Client) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/memos")
	if err != nil {
		return nil, fmt.Errorf("parsing memos URL: %w", err)
	}

	q := u.Query()
	q.Set("tag", tag)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memos API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("memos API returned status %d", resp.StatusCode)
	}

	var result memosResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding memos response: %w", err)
	}

	notes := make([]backend.Note, 0, len(result.Memos))
	for _, m := range result.Memos {
		notes = append(notes, backend.Note{
			Title:   extractTitle(m.Content),
			Date:    time.Unix(m.CreatedTs, 0),
			Tags:    m.Tags,
			Content: m.Content,
		})
	}

	return notes, nil
}

// memosResponse represents the JSON structure returned by the Memos API.
type memosResponse struct {
	Memos []memo `json:"memos"`
}

// memo represents a single memo in the API response.
type memo struct {
	ID        int64    `json:"id"`
	Content   string   `json:"content"`
	CreatedTs int64    `json:"createdTs"`
	UpdatedTs int64    `json:"updatedTs"`
	Tags      []string `json:"tags"`
}

// extractTitle extracts the first markdown H1 title from content.
// If no H1 is found, it returns the first line.
func extractTitle(content string) string {
	lines := splitLines(content)
	for _, line := range lines {
		line = cleanLine(line)
		if len(line) > 2 && line[:2] == "# " {
			return line[2:]
		}
	}
	if len(lines) > 0 {
		return cleanLine(lines[0])
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func cleanLine(s string) string {
	return removeCR(s)
}

func removeCR(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\r' {
			b = append(b, s[i])
		}
	}
	return string(b)
}
