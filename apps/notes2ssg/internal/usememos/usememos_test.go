package usememos

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
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
