package notification

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mockTelegramTransport() {
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "mock_error") {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok": false}`)),
				Header:     make(http.Header),
			}, nil
		}
		body := `{"ok": true, "result": {"id": 123, "is_bot": true, "first_name": "bot", "username": "bot"}}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	})
}
