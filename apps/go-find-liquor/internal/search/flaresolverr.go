package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FlareSolverrTransport adapts ordinary HTTP requests to FlareSolverr's /v1
// API. It also returns the browser cookies as Set-Cookie headers so the
// enclosing http.Client cookie jar carries them into subsequent requests.
type FlareSolverrTransport struct {
	endpoint string
	client   *http.Client
}

type flareSolverrRequest struct {
	Cmd        string               `json:"cmd"`
	URL        string               `json:"url"`
	PostData   string               `json:"postData,omitempty"`
	Headers    map[string]string    `json:"headers,omitempty"`
	Cookies    []flareSolverrCookie `json:"cookies,omitempty"`
	MaxTimeout int                  `json:"maxTimeout"`
}

type flareSolverrCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
	Path   string `json:"path,omitempty"`
}

type flareSolverrResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution struct {
		URL      string               `json:"url"`
		Status   int                  `json:"status"`
		Headers  map[string]string    `json:"headers"`
		Response string               `json:"response"`
		Cookies  []flareSolverrCookie `json:"cookies"`
	} `json:"solution"`
}

func NewFlareSolverrTransport(endpoint string) (*FlareSolverrTransport, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("flaresolverr endpoint must be an absolute HTTP(S) URL")
	}
	return &FlareSolverrTransport{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		client:   &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

func (t *FlareSolverrTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, maxHTMLBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}
	if len(body) > maxHTMLBytes {
		return nil, fmt.Errorf("request body exceeds %d-byte limit", maxHTMLBytes)
	}

	cmd := "request.get"
	if req.Method == http.MethodPost {
		cmd = "request.post"
	} else if req.Method != http.MethodGet {
		return nil, fmt.Errorf("flaresolverr does not support %s requests", req.Method)
	}

	payload := flareSolverrRequest{Cmd: cmd, URL: req.URL.String(), MaxTimeout: 60000}
	if req.Method == http.MethodPost {
		payload.PostData = string(body)
	}
	if userAgent := req.Header.Get("User-Agent"); userAgent != "" {
		payload.Headers = map[string]string{"User-Agent": userAgent}
	}
	for _, cookie := range req.Cookies() {
		payload.Cookies = append(payload.Cookies, flareSolverrCookie{Name: cookie.Name, Value: cookie.Value, Domain: req.URL.Hostname(), Path: "/"})
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal flaresolverr request: %w", err)
	}
	apiReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, t.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create flaresolverr request: %w", err)
	}
	apiReq.Header.Set("Content-Type", "application/json")
	apiResp, err := t.client.Do(apiReq) // #nosec G704 -- endpoint is validated application config
	if err != nil {
		return nil, fmt.Errorf("request flaresolverr: %w", err)
	}
	defer func() { _ = apiResp.Body.Close() }()
	if apiResp.StatusCode < http.StatusOK || apiResp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("flaresolverr returned status: %s", apiResp.Status)
	}

	var result flareSolverrResponse
	if err := json.NewDecoder(io.LimitReader(apiResp.Body, maxHTMLBytes+1)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode flaresolverr response: %w", err)
	}
	if result.Status != "ok" {
		return nil, fmt.Errorf("flaresolverr failed: %s", result.Message)
	}

	headers := make(http.Header, len(result.Solution.Headers)+len(result.Solution.Cookies))
	for name, value := range result.Solution.Headers {
		headers.Set(name, value)
	}
	for _, cookie := range result.Solution.Cookies {
		httpCookie := &http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}
		if httpCookie.Domain == "" {
			httpCookie.Domain = req.URL.Hostname()
		}
		if httpCookie.Path == "" {
			httpCookie.Path = "/"
		}
		headers.Add("Set-Cookie", httpCookie.String())
	}
	status := result.Solution.Status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(result.Solution.Response)),
		Request:    req,
	}, nil
}
