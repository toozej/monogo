// Package gotify provides a lightweight client for sending push notifications
// via a Gotify server.
package gotify

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// HTTPDoer is the minimal interface used by Client to send HTTP requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
	PostForm(url string, data url.Values) (*http.Response, error)
}

// Client sends notifications to a Gotify instance.
type Client struct {
	baseURL string
	token   string
	client  HTTPDoer
}

// NewClient creates a new Gotify notification client.
// If baseURL or token are empty, Send becomes a no-op.
func NewClient(baseURL, token string, client HTTPDoer) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client:  client,
	}
}

// IsConfigured returns true if the client has both a URL and token.
func (c *Client) IsConfigured() bool {
	return c.baseURL != "" && c.token != ""
}

// Send dispatches a notification with the given title and message.
func (c *Client) Send(title, message string) error {
	if !c.IsConfigured() {
		fmt.Println("Gotify URL and/or token not specified in environment. No notification was sent")
		return nil
	}

	postURL := c.baseURL + "/message?token=" + c.token
	data := url.Values{}
	data.Set("title", title)
	data.Set("message", message)

	resp, err := c.client.PostForm(postURL, data)
	if err != nil {
		return fmt.Errorf("sending gotify notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gotify returned status %d", resp.StatusCode)
	}

	return nil
}
