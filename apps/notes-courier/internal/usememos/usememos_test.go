package usememos

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

type mockHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func makeResponse(body string, status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://memos.local", "token123", nil)
	if c.baseURL != "http://memos.local" {
		t.Errorf("expected baseURL 'http://memos.local', got %q", c.baseURL)
	}
	if c.token != "token123" {
		t.Errorf("expected token 'token123', got %q", c.token)
	}
}

func TestFetchNotesSuccess(t *testing.T) {
	body := `{"memos":[{"id":1,"content":"# Hello\nWorld","createdTs":1693520000,"updatedTs":1693520000,"tags":["blog"]}]}`
	client := &mockHTTPClient{resp: makeResponse(body, 200)}
	c := NewClient("http://memos.local", "token", client)

	notes, err := c.FetchNotes(context.Background(), "blog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Title != "Hello" {
		t.Errorf("expected title 'Hello', got %q", notes[0].Title)
	}
	if notes[0].Content != "# Hello\nWorld" {
		t.Errorf("expected content, got %q", notes[0].Content)
	}
}

func TestFetchNotesEmpty(t *testing.T) {
	body := `{"memos":[]}`
	client := &mockHTTPClient{resp: makeResponse(body, 200)}
	c := NewClient("http://memos.local", "token", client)

	notes, err := c.FetchNotes(context.Background(), "blog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(notes))
	}
}

func TestFetchNotesBadStatus(t *testing.T) {
	client := &mockHTTPClient{resp: makeResponse("", 500)}
	c := NewClient("http://memos.local", "token", client)

	_, err := c.FetchNotes(context.Background(), "blog")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestFetchNotesInvalidJSON(t *testing.T) {
	client := &mockHTTPClient{resp: makeResponse("not json", 200)}
	c := NewClient("http://memos.local", "token", client)

	_, err := c.FetchNotes(context.Background(), "blog")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"# Title\nbody", "Title"},
		{"No markdown title", "No markdown title"},
		{"", ""},
	}
	for _, tc := range cases {
		got := extractTitle(tc.input)
		if got != tc.expected {
			t.Errorf("extractTitle(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestWriteNotesCreatesAndUpdatesByMarker(t *testing.T) {
	var stored []memo
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing token")
			w.WriteHeader(401)
			return
		}
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("pageSize") != "100" {
				t.Error("missing page size")
			}
			_ = json.NewEncoder(w).Encode(memosResponse{Memos: stored})
		case http.MethodPost:
			var body memo
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			body.Name = "memos/one"
			stored = append(stored, body)
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			if r.URL.Path != "/api/v1/memos/one" || r.URL.Query().Get("updateMask") != "content" {
				t.Errorf("wrong update request: %s", r.URL)
			}
			var body memo
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			stored[0].Content = body.Content
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "secret", server.Client())
	notes := []backend.Note{{ID: "simplenote:key", Title: "Title", Content: "first", Tags: []string{"work"}}}
	if err := client.WriteNotes(context.Background(), notes); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || !strings.Contains(stored[0].Content, "#work") {
		t.Fatalf("bad created memo: %+v", stored)
	}
	if err := client.WriteNotes(context.Background(), notes); err != nil {
		t.Fatal(err)
	}
	notes[0].Content = "changed"
	if err := client.WriteNotes(context.Background(), notes); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "GET,POST,GET,GET,PATCH" {
		t.Fatalf("wrong requests: %v", methods)
	}
	if !strings.Contains(stored[0].Content, "changed") {
		t.Fatal("memo did not update")
	}
}

func TestFetchNotesPagesAndTagFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") == "next" {
			_, _ = w.Write([]byte(`{"memos":[{"name":"memos/2","content":"# Second","createTime":"2024-01-01T00:00:00Z","tags":["work"]}]}`))
		} else {
			_, _ = w.Write([]byte(`{"memos":[{"name":"memos/1","content":"# First","tags":["other"]}],"nextPageToken":"next"}`))
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "secret", server.Client())
	notes, err := client.FetchNotes(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != "memos/2" || notes[0].Date.IsZero() {
		t.Fatalf("wrong notes: %+v", notes)
	}
}
