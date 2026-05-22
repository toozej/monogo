package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var nodeRuntimeRe = regexp.MustCompile(`^node(\d+)$`)

func ParseNodeRuntime(using string) (string, bool) {
	using = strings.TrimSpace(strings.ToLower(using))
	m := nodeRuntimeRe.FindStringSubmatch(using)
	if len(m) > 1 {
		return m[1], true
	}
	return "", false
}

func NewEOLClient(httpClient *http.Client) *EOLClient {
	return &EOLClient{
		httpClient: httpClient,
		baseURL:    "https://endoflife.date/api/v1",
		cache:      make(map[string]*RuntimeEOLInfo),
	}
}

func NewEOLClientWithHTTP(baseURL string, httpClient *http.Client) *EOLClient {
	return &EOLClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		cache:      make(map[string]*RuntimeEOLInfo),
	}
}

func (c *EOLClient) getCached(product, version string) (*RuntimeEOLInfo, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	key := product + "/" + version
	info, ok := c.cache[key]
	return info, ok
}

func (c *EOLClient) setCached(product, version string, info *RuntimeEOLInfo) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	key := product + "/" + version
	c.cache[key] = info
}

func (c *EOLClient) FetchReleaseEOL(ctx context.Context, product, version string) (*RuntimeEOLInfo, error) {
	if info, ok := c.getCached(product, version); ok {
		return info, nil
	}

	url := fmt.Sprintf("%s/products/%s/releases/%s", c.baseURL, product, version)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("endoflife.date API returned status %d for %s/%s", resp.StatusCode, product, version)
	}

	var releaseResp ProductReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&releaseResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	info := &RuntimeEOLInfo{
		Runtime: product,
		Version: version,
		IsEOL:   releaseResp.Result.IsEol,
	}

	if releaseResp.Result.EolFrom != nil && *releaseResp.Result.EolFrom != "" {
		eolDate, parseErr := time.Parse("2006-01-02", *releaseResp.Result.EolFrom)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse EolFrom date %q for %s/%s: %w", *releaseResp.Result.EolFrom, product, version, parseErr)
		}
		info.EOLDate = eolDate
	}

	c.setCached(product, version, info)
	return info, nil
}

func (c *EOLClient) CheckRunsUsing(ctx context.Context, using string) (*RuntimeEOLInfo, error) {
	version, ok := ParseNodeRuntime(using)
	if !ok {
		return nil, nil
	}
	return c.FetchReleaseEOL(ctx, "nodejs", version)
}

func formatEOLDate(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02")
}

func (i *RuntimeEOLInfo) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s%s (EOL since %s)", i.Runtime, i.Version, formatEOLDate(i.EOLDate))
}
