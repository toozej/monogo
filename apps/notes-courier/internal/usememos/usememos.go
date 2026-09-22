// Package usememos reads and writes notes through the Memos HTTP API.
package usememos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

const defaultTimeout = 30 * time.Second
const markerPrefix = "<!-- notes-courier:"

// HTTPClient performs HTTP requests.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client reads and writes Memos notes.
type Client struct {
	baseURL    string
	token      string
	client     HTTPClient
	visibility string
}

// NewClient creates a Memos client.
func NewClient(baseURL, token string, client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: client, visibility: "PRIVATE"}
}

// SetVisibility sets the visibility of new notes.
func (c *Client) SetVisibility(visibility string) { c.visibility = visibility }

type memo struct {
	Name       string      `json:"name"`
	ID         json.Number `json:"id"`
	Content    string      `json:"content"`
	CreateTime string      `json:"createTime"`
	CreatedTs  int64       `json:"createdTs"`
	Tags       []string    `json:"tags"`
}

type memosResponse struct {
	Memos         []memo `json:"memos"`
	Data          []memo `json:"data"`
	NextPageToken string `json:"nextPageToken"`
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body any, result any) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("MEMOS_URL and MEMOS_TOKEN are required")
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parsing Memos URL: %w", err)
	}
	u.RawQuery = query.Encode()
	var input *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding Memos request: %w", err)
		}
		input = bytes.NewReader(data)
	} else {
		input = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), input)
	if err != nil {
		return fmt.Errorf("creating Memos request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("memos API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("memos API returned status %d", resp.StatusCode)
	}
	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding Memos response: %w", err)
		}
	}
	return nil
}

func (c *Client) list(ctx context.Context) ([]memo, error) {
	var all []memo
	pageToken := ""
	for {
		query := url.Values{"pageSize": {"100"}}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		var response memosResponse
		if err := c.request(ctx, http.MethodGet, "/api/v1/memos", query, nil, &response); err != nil {
			return nil, err
		}
		if response.Memos != nil {
			all = append(all, response.Memos...)
		} else {
			all = append(all, response.Data...)
		}
		if response.NextPageToken == "" {
			return all, nil
		}
		if response.NextPageToken == pageToken {
			return nil, errors.New("memos returned a repeated page token")
		}
		pageToken = response.NextPageToken
	}
}

// FetchNotes lists Memos notes. The API filter and a local check select the requested tag.
func (c *Client) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	items, err := c.list(ctx)
	if err != nil {
		return nil, err
	}
	notes := make([]backend.Note, 0, len(items))
	for _, m := range items {
		if tag != "" && !hasTag(m.Tags, tag) {
			continue
		}
		date, _ := time.Parse(time.RFC3339Nano, m.CreateTime)
		if date.IsZero() && m.CreatedTs != 0 {
			date = time.Unix(m.CreatedTs, 0)
		}
		id := m.Name
		if id == "" {
			id = m.ID.String()
		}
		notes = append(notes, backend.Note{ID: id, Title: extractTitle(m.Content), Date: date, Tags: m.Tags, Content: m.Content})
	}
	return notes, nil
}

func hasTag(tags []string, tag string) bool {
	for _, value := range tags {
		if value == tag {
			return true
		}
	}
	return false
}

// WriteNotes creates or updates notes by their source marker.
func (c *Client) WriteNotes(ctx context.Context, notes []backend.Note) error {
	if c.visibility != "PRIVATE" && c.visibility != "PROTECTED" && c.visibility != "PUBLIC" {
		return fmt.Errorf("invalid MEMOS_VISIBILITY %q", c.visibility)
	}
	items, err := c.list(ctx)
	if err != nil {
		return err
	}
	byMarker := map[string]memo{}
	for _, m := range items {
		if marker := findMarker(m.Content); marker != "" {
			if _, exists := byMarker[marker]; exists {
				return fmt.Errorf("duplicate Memos marker %q", marker)
			}
			byMarker[marker] = m
		}
	}
	for _, n := range notes {
		marker := noteMarker(n)
		content := memoContent(n) + "\n" + marker + "\n"
		if existing, ok := byMarker[marker]; ok {
			if existing.Content == content {
				continue
			}
			name := existing.Name
			if name == "" {
				name = "memos/" + existing.ID.String()
			}
			if name == "memos/" {
				return errors.New("memos returned a note without an ID")
			}
			body := map[string]any{"name": name, "content": content}
			query := url.Values{"updateMask": {"content"}}
			if err := c.request(ctx, http.MethodPatch, "/api/v1/"+name, query, body, nil); err != nil {
				return fmt.Errorf("updating %s: %w", name, err)
			}
			continue
		}
		memoBody := map[string]any{"content": content, "visibility": c.visibility, "state": "NORMAL"}
		if !n.Date.IsZero() {
			memoBody["createTime"] = n.Date.Format(time.RFC3339)
		}
		if err := c.request(ctx, http.MethodPost, "/api/v1/memos", nil, memoBody, nil); err != nil {
			return fmt.Errorf("creating Memos note %q: %w", n.Title, err)
		}
	}
	return nil
}

func noteMarker(n backend.Note) string {
	id := n.ID
	if id == "" {
		id = n.Title + "\x00" + n.Date.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%s%x -->", markerPrefix, sum[:12])
}

func findMarker(content string) string {
	start := strings.Index(content, markerPrefix)
	if start < 0 {
		return ""
	}
	end := strings.Index(content[start:], " -->")
	if end < 0 {
		return ""
	}
	return content[start : start+end+4]
}

func memoContent(n backend.Note) string {
	content := strings.TrimSpace(n.Content)
	if n.Title != "" && !strings.HasPrefix(content, "# ") {
		content = "# " + n.Title + "\n\n" + content
	}
	for _, tag := range n.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !strings.Contains(content, "#"+tag) {
			content += "\n#" + strings.ReplaceAll(tag, " ", "-")
		}
	}
	return strings.TrimSpace(content)
}

func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "# "))
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
