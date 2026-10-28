// Package photos reads public Google Photos albums without account credentials.
package photos

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxResponseBytes = 32 << 20

// Client uses anonymous HTTP requests. It never loads or stores account cookies.
type Client struct {
	Timeout   time.Duration
	Logger    *slog.Logger
	transport http.RoundTripper
}

type publicSession struct {
	client     *http.Client
	endpoint   string
	sourcePath string
	sessionID  string
	build      string
	logger     *slog.Logger
}

var globalDataPattern = regexp.MustCompile(`window\.WIZ_global_data\s*=\s*`)

// LikedFilenames reads a public sharing link without a Google account.
func (c *Client) LikedFilenames(ctx context.Context, album string) ([]string, error) {
	if err := ValidateAlbumURL(album); err != nil {
		return nil, err
	}
	if c.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than zero")
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	logger := c.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	session := &publicSession{client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: checkRedirect, Transport: c.transport}, logger: logger}
	defer session.client.CloseIdleConnections()
	return read(ctx, session, album, logger)
}

func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("too many Google Photos redirects")
	}
	if err := allowedURL(request.URL); err != nil {
		return err
	}
	return nil
}

func allowedURL(u *url.URL) error {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Host != "photos.google.com" && u.Host != "photos.app.goo.gl") {
		return fmt.Errorf("the sharing link must open a public Google Photos album")
	}
	return nil
}

func (s *publicSession) Navigate(ctx context.Context, target string) (string, error) {
	started := time.Now()
	if s.logger != nil {
		s.logger.DebugContext(ctx, "Open public album")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("open public album: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := readResponse(response)
	if err != nil {
		return "", err
	}
	if err := allowedURL(response.Request.URL); err != nil {
		return "", err
	}
	if err := s.bootstrap(body); err != nil {
		return "", err
	}
	s.sourcePath = response.Request.URL.Path
	if s.logger != nil {
		s.logger.DebugContext(ctx, "Opened public album", "host", response.Request.URL.Host, "status", response.StatusCode, "bytes", len(body), "duration", time.Since(started))
	}
	return response.Request.URL.String(), nil
}

func (s *publicSession) bootstrap(body []byte) error {
	location := globalDataPattern.FindIndex(body)
	if location == nil {
		return fmt.Errorf("cannot read this public album; check that link sharing is enabled")
	}
	var data struct {
		Endpoint  string `json:"Im6cmf"`
		SessionID string `json:"FdrFJe"`
		Build     string `json:"cfb2h"`
	}
	if err := json.NewDecoder(strings.NewReader(string(body[location[1]:]))).Decode(&data); err != nil {
		return fmt.Errorf("the Google Photos page contains invalid session metadata")
	}
	endpoint, err := url.Parse("https://photos.google.com" + data.Endpoint + "/data/batchexecute")
	if err != nil || data.Endpoint == "" || !strings.HasPrefix(data.Endpoint, "/_/") || endpoint.Host != "photos.google.com" || endpoint.RawQuery != "" || endpoint.Fragment != "" || data.SessionID == "" || data.Build == "" {
		return fmt.Errorf("the Google Photos page contains invalid session metadata")
	}
	s.endpoint = endpoint.String()
	s.sessionID = data.SessionID
	s.build = data.Build
	return nil
}

func (s *publicSession) RPC(ctx context.Context, id string, payload []any) ([]any, error) {
	started := time.Now()
	if s.logger != nil {
		s.logger.DebugContext(ctx, "Request Google Photos metadata", "rpc", id)
	}
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	envelope, err := json.Marshal([]any{[]any{[]any{id, string(encodedPayload), nil, "1"}}})
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(s.endpoint)
	if err != nil || s.endpoint == "" {
		return nil, fmt.Errorf("open the public album before requesting metadata")
	}
	query := endpoint.Query()
	query.Set("rpcids", id)
	query.Set("source-path", s.sourcePath)
	query.Set("f.sid", s.sessionID)
	query.Set("bl", s.build)
	query.Set("rt", "c")
	endpoint.RawQuery = query.Encode()
	// Public albums require no account token. The sharing key remains in payload.
	form := url.Values{"f.req": {string(envelope)}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://photos.google.com")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request public album metadata: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := readResponse(response)
	if err != nil {
		return nil, err
	}
	data, err := decodeRPC(string(body), id)
	if s.logger != nil {
		s.logger.DebugContext(ctx, "Received Google Photos metadata", "rpc", id, "status", response.StatusCode, "bytes", len(body), "duration", time.Since(started), "decoded", err == nil)
	}
	return data, err
}

func readResponse(response *http.Response) ([]byte, error) {
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the Google Photos interface returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Google response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("the Google Photos response exceeds 32 MiB")
	}
	return body, nil
}

// decodeRPC accepts Google's length-prefixed response frames and rejects RPC errors.
func decodeRPC(body, id string) ([]any, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 4096), maxResponseBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var rows [][]json.RawMessage
		if err := json.Unmarshal([]byte(line), &rows); err != nil {
			return nil, fmt.Errorf("the Google Photos interface returned a malformed response frame")
		}
		for _, row := range rows {
			if len(row) < 3 {
				continue
			}
			var kind, rpcID string
			_ = json.Unmarshal(row[0], &kind)
			_ = json.Unmarshal(row[1], &rpcID)
			if kind != "wrb.fr" || rpcID != id {
				continue
			}
			var encoded string
			if err := json.Unmarshal(row[2], &encoded); err != nil || encoded == "" {
				return nil, fmt.Errorf("the Google Photos interface denied the request; check that link sharing is enabled")
			}
			var data []any
			if err := json.Unmarshal([]byte(encoded), &data); err != nil || data == nil {
				return nil, fmt.Errorf("the Google Photos interface returned invalid album data")
			}
			return data, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Google response: %w", err)
	}
	return nil, errors.New("the Google Photos interface did not return the requested data; check the sharing link or the Google Photos web interface")
}
