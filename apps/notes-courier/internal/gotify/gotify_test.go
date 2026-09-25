package gotify

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
)

type mockHTTPClient struct {
	resp *http.Response
	err  error
	req  *http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.req = req
	return m.resp, m.err
}

func (m *mockHTTPClient) PostForm(url string, data url.Values) (*http.Response, error) {
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
	c := NewClient("http://gotify.local", "token", nil)
	if c.baseURL != "http://gotify.local" {
		t.Errorf("expected baseURL 'http://gotify.local', got %q", c.baseURL)
	}
	if c.token != "token" {
		t.Errorf("expected token 'token', got %q", c.token)
	}
}

func TestIsConfigured(t *testing.T) {
	c1 := NewClient("", "", nil)
	if c1.IsConfigured() {
		t.Error("expected IsConfigured to be false when empty")
	}
	c2 := NewClient("http://gotify.local", "token", nil)
	if !c2.IsConfigured() {
		t.Error("expected IsConfigured to be true when set")
	}
}

func TestSendSuccess(t *testing.T) {
	client := &mockHTTPClient{resp: makeResponse("{}", 200)}
	c := NewClient("http://gotify.local", "token", client)

	err := c.Send("Test", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendNotConfigured(t *testing.T) {
	c := NewClient("", "", nil)
	err := c.Send("Test", "Hello")
	if err != nil {
		t.Fatalf("expected no error when not configured, got %v", err)
	}
}

func TestSendBadStatus(t *testing.T) {
	client := &mockHTTPClient{resp: makeResponse("", 403)}
	c := NewClient("http://gotify.local", "token", client)

	err := c.Send("Test", "Hello")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
