package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"github.com/toozej/go-sort-out-gh-actions/internal/cache"
	"github.com/toozej/go-sort-out-gh-actions/internal/runtime"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
)

const maxConcurrency = 10

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithCache enables or disables the persistent disk cache and refresh mode.
func WithCache(enabled, refresh bool, ttl time.Duration) ClientOption {
	return func(c *Client) {
		c.cacheEnabled = enabled
		c.refreshCache = refresh
		c.cacheTTL = ttl
	}
}

func (c *Client) SetEOLClientForTest(baseURL string, httpClient *http.Client) {
	c.eolClient = runtime.NewEOLClientWithHTTP(baseURL, httpClient)
}

func NewClientWithHTTP(baseURL string, httpClient *http.Client, opts ...ClientOption) *Client {
	client := &Client{
		httpClient:    httpClient,
		token:         "",
		baseURL:       baseURL,
		eolClient:     runtime.NewEOLClient(httpClient),
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheTTL:      24 * time.Hour,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.cacheEnabled {
		var err error
		client.cacheStore, err = cache.NewCacheStore("go-sort-out-gh-actions")
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("Failed to initialize disk cache, falling back to in-memory only")
			client.cacheEnabled = false
		}
	}

	if client.cacheEnabled && client.cacheStore != nil && client.refreshCache {
		if err := client.cacheStore.ClearAll(); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("Failed to clear disk cache")
		}
	}

	if client.cacheEnabled && client.cacheStore != nil && !client.refreshCache {
		client.loadAllCaches()
	}

	return client
}

func NewClient(token string, opts ...ClientOption) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := &Client{
		httpClient:    tc,
		token:         token,
		baseURL:       "https://api.github.com",
		eolClient:     runtime.NewEOLClient(tc),
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheTTL:      24 * time.Hour,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.cacheEnabled {
		var err error
		client.cacheStore, err = cache.NewCacheStore("go-sort-out-gh-actions")
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("Failed to initialize disk cache, falling back to in-memory only")
			client.cacheEnabled = false
		}
	}

	if client.cacheEnabled && client.cacheStore != nil && client.refreshCache {
		if err := client.cacheStore.ClearAll(); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("Failed to clear disk cache")
		}
	}

	if client.cacheEnabled && client.cacheStore != nil && !client.refreshCache {
		client.loadAllCaches()
	}

	return client
}

func (c *Client) loadAllCaches() {
	if c.cacheStore == nil {
		return
	}

	var archivedCache map[string]bool
	var releaseCache map[string]*ReleaseInfo
	var refSHACache map[string]string
	var repoInfoCache map[string]*RepoInfo

	if err := c.cacheStore.Load("archived", &archivedCache); err == nil {
		c.archivedCache = archivedCache
	}
	if err := c.cacheStore.Load("releases", &releaseCache); err == nil {
		c.releaseCache = releaseCache
	}
	if err := c.cacheStore.Load("refsha", &refSHACache); err == nil {
		c.refSHACache = refSHACache
	}
	if err := c.cacheStore.Load("repoinfo", &repoInfoCache); err == nil {
		c.repoInfoCache = repoInfoCache
	}
}

func (c *Client) logRateLimits(resp *http.Response) {
	if resp == nil {
		return
	}
	limit := resp.Header.Get("X-RateLimit-Limit")
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	used := resp.Header.Get("X-RateLimit-Used")
	reset := resp.Header.Get("X-RateLimit-Reset")
	resource := resp.Header.Get("X-RateLimit-Resource")

	resetTime := ""
	if reset != "" {
		if resetUnix, err := strconv.ParseInt(reset, 10, 64); err == nil {
			resetTime = time.Unix(resetUnix, 0).Format(time.RFC3339)
		}
	}

	log.Debugf("Rate limit info: resource=%s limit=%s remaining=%s used=%s reset=%s",
		resource, limit, remaining, used, resetTime)
}

func (c *Client) handleRateLimit(resp *http.Response) error {
	c.logRateLimits(resp)

	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "0" || resp.StatusCode == 403 || resp.StatusCode == 429 {
		resetTime := resp.Header.Get("X-RateLimit-Reset")
		if resetTime != "" {
			if resetUnix, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
				reset := time.Unix(resetUnix, 0)
				return fmt.Errorf("rate limited, resets at %s", reset.Format(time.RFC3339))
			}
		}
		return fmt.Errorf("rate limited by GitHub API")
	}
	return nil
}

func (c *Client) getCachedArchived(ownerRepo string) (bool, *RepoInfo, bool) {
	if !c.cacheEnabled {
		return false, nil, false
	}
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	if archived, ok := c.archivedCache[cleanRepo]; ok {
		info := c.repoInfoCache[cleanRepo]
		return archived, info, true
	}
	return false, nil, false
}

func (c *Client) setCachedArchived(ownerRepo string, archived bool, info *RepoInfo) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	c.archivedCache[cleanRepo] = archived
	if info != nil {
		c.repoInfoCache[cleanRepo] = info
	}
}

func (c *Client) getCachedRelease(ownerRepo string) (*ReleaseInfo, bool) {
	if !c.cacheEnabled {
		return nil, false
	}
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	if release, ok := c.releaseCache[cleanRepo]; ok {
		return release, true
	}
	return nil, false
}

func (c *Client) setCachedRelease(ownerRepo string, release *ReleaseInfo) {
	if !c.cacheEnabled {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	c.releaseCache[cleanRepo] = release
}

func (c *Client) getCachedRefSHA(ownerRepo, ref string) (string, bool) {
	if !c.cacheEnabled {
		return "", false
	}
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	key := cleanOwnerRepo(ownerRepo) + "@" + ref
	if sha, ok := c.refSHACache[key]; ok {
		return sha, true
	}
	return "", false
}

func (c *Client) setCachedRefSHA(ownerRepo, ref, sha string) {
	if !c.cacheEnabled {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	key := cleanOwnerRepo(ownerRepo) + "@" + ref
	c.refSHACache[key] = sha
}

func cleanOwnerRepo(ownerRepo string) string {
	ownerRepo = strings.TrimSpace(ownerRepo)
	ownerRepo = strings.TrimPrefix(ownerRepo, "https://github.com/")
	if idx := strings.Index(ownerRepo, "@"); idx != -1 {
		ownerRepo = ownerRepo[:idx]
	}
	return ownerRepo
}

func parseOwnerRepo(ownerRepo string) (string, string, error) {
	ownerRepo = cleanOwnerRepo(ownerRepo)
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid owner/repo format: %s", ownerRepo)
	}
	return parts[0], parts[1], nil
}

func (c *Client) IsRepoArchived(ctx context.Context, ownerRepo string) (bool, *RepoInfo, error) {
	if archived, info, ok := c.getCachedArchived(ownerRepo); ok {
		return archived, info, nil
	}

	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return false, nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if err := c.handleRateLimit(resp); err != nil {
		return false, nil, err
	}

	if resp.StatusCode == 404 {
		return false, nil, fmt.Errorf("repository %s/%s not found", owner, repo)
	}

	if resp.StatusCode != 200 {
		return false, nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var repoInfo RepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return false, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.setCachedArchived(ownerRepo, repoInfo.Archived, &repoInfo)
	return repoInfo.Archived, &repoInfo, nil
}

func (c *Client) CheckMultipleRepos(ctx context.Context, repos []string) (map[string]bool, map[string]error) {
	archived := make(map[string]bool)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			isArchived, _, err := c.IsRepoArchived(ctx, repo)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors[repo] = err
				return
			}
			archived[repo] = isArchived
		}(repo)
	}

	wg.Wait()
	return archived, errors
}

func (c *Client) GetLatestRelease(ctx context.Context, ownerRepo string) (*ReleaseInfo, error) {
	if release, ok := c.getCachedRelease(ownerRepo); ok {
		return release, nil
	}

	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found for %s/%s", owner, repo)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !isSemverTag(release.TagName) {
		fallbackTag, fallbackErr := c.GetLatestSemverTag(ctx, ownerRepo)
		if fallbackErr == nil {
			release.TagName = fallbackTag
			release.HTMLURL = fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, fallbackTag)
		} else {
			log.WithFields(log.Fields{
				"repo":          ownerRepo,
				"release_tag":   release.TagName,
				"fallbackError": fallbackErr,
			}).Debug("latest release tag is not semver and semver fallback failed")
		}
	}

	c.setCachedRelease(ownerRepo, &release)
	return &release, nil
}

func (c *Client) GetLatestSemverTag(ctx context.Context, ownerRepo string) (string, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if err := c.handleRateLimit(resp); err != nil {
		return "", err
	}

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("repository %s/%s not found", owner, repo)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	for _, tag := range tags {
		if isSemverTag(tag.Name) {
			return tag.Name, nil
		}
	}

	return "", fmt.Errorf("no semver tags found for %s/%s", owner, repo)
}

func isSemverTag(tag string) bool {
	tag = strings.TrimSpace(strings.TrimPrefix(tag, "v"))
	_, err := semver.NewVersion(tag)
	return err == nil
}

func (c *Client) CheckMultipleReleases(ctx context.Context, repos []string) (map[string]*ReleaseInfo, map[string]error) {
	releases := make(map[string]*ReleaseInfo)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			release, err := c.GetLatestRelease(ctx, repo)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors[repo] = err
				return
			}
			releases[repo] = release
		}(repo)
	}

	wg.Wait()
	return releases, errors
}

func (c *Client) GetRefSHA(ctx context.Context, ownerRepo, ref string) (string, error) {
	if sha, ok := c.getCachedRefSHA(ownerRepo, ref); ok {
		return sha, nil
	}

	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return "", err
	}

	sha, err := c.getRefSHA(ctx, owner, repo, "tags", ref)
	if err == nil {
		return sha, nil
	}

	sha, err = c.getRefSHA(ctx, owner, repo, "heads", ref)
	if err == nil {
		return sha, nil
	}

	return "", fmt.Errorf("ref %s not found in %s/%s", ref, owner, repo)
}

func (c *Client) getRefSHA(ctx context.Context, owner, repo, refType, ref string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/%s/%s", c.baseURL, owner, repo, refType, ref)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if resp.StatusCode != 200 {
		if err := c.handleRateLimit(resp); err != nil {
			return "", err
		}
		return "", fmt.Errorf("ref %s/%s not found in %s/%s", refType, ref, owner, repo)
	}

	var refInfo RefInfo
	if err := json.NewDecoder(resp.Body).Decode(&refInfo); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	sha := refInfo.Object.SHA
	if refInfo.Object.Type == "tag" {
		sha, err = c.dereferenceTag(ctx, owner, repo, sha)
		if err != nil {
			return "", fmt.Errorf("failed to dereference tag object %s: %w", refInfo.Object.SHA, err)
		}
	}

	c.setCachedRefSHA(owner+"/"+repo, ref, sha)
	return sha, nil
}

func (c *Client) dereferenceTag(ctx context.Context, owner, repo, tagObjectSHA string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", c.baseURL, owner, repo, tagObjectSHA)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tag object %s not found in %s/%s", tagObjectSHA, owner, repo)
	}

	var tagObj struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagObj); err != nil {
		return "", fmt.Errorf("failed to decode tag object response: %w", err)
	}

	return tagObj.Object.SHA, nil
}

func (c *Client) CompareRefSHAs(ctx context.Context, ownerRepo, ref1, ref2 string) (bool, string, string, error) {
	sha1, err := c.GetRefSHA(ctx, ownerRepo, ref1)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to get SHA for ref %s: %w", ref1, err)
	}

	sha2, err := c.GetRefSHA(ctx, ownerRepo, ref2)
	if err != nil {
		return false, sha1, "", fmt.Errorf("failed to get SHA for ref %s: %w", ref2, err)
	}

	return sha1 == sha2, sha1, sha2, nil
}

func (c *Client) GetRepoInfo(ctx context.Context, ownerRepo string) (*RepoInfo, error) {
	if _, info, ok := c.getCachedArchived(ownerRepo); ok && info != nil {
		return info, nil
	}

	_, info, err := c.IsRepoArchived(ctx, ownerRepo)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *Client) CheckMultipleStale(ctx context.Context, repos []string, staleThreshold time.Duration) (map[string]*StaleInfo, map[string]error) {
	results := make(map[string]*StaleInfo)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := c.GetRepoInfo(ctx, repo)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors[repo] = err
				return
			}

			staleInfo := &StaleInfo{Repo: repo}

			if info.Deprecated {
				staleInfo.Deprecated = true
				staleInfo.DeprecationMessage = info.DeprecationMsg
			}

			if info.UpdatedAt != "" {
				updated, err := time.Parse(time.RFC3339, info.UpdatedAt)
				if err == nil {
					staleInfo.LastUpdated = updated
					if time.Since(updated) > staleThreshold {
						staleInfo.StaleByAge = true
					}
				}
			}

			if staleInfo.Deprecated || staleInfo.StaleByAge {
				results[repo] = staleInfo
			}
		}(repo)
	}

	wg.Wait()
	return results, errors
}

func actionManifestPaths(actionPath string) []string {
	paths := make([]string, 0, 4)
	if actionPath != "" {
		cleanPath := strings.Trim(actionPath, "/")
		paths = append(paths, cleanPath+"/action.yml", cleanPath+"/action.yaml")
	}
	paths = append(paths, "action.yml", "action.yaml")
	return paths
}

func (c *Client) GetActionYML(ctx context.Context, ownerRepo, actionPath, ref string) (string, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return "", err
	}

	for _, manifestPath := range actionManifestPaths(actionPath) {
		apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, manifestPath, url.QueryEscape(ref))
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("User-Agent", "go-sort-out-gh-actions")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to make request: %w", err)
		}

		c.logRateLimits(resp)

		if err := c.handleRateLimit(resp); err != nil {
			_ = resp.Body.Close()
			return "", err
		}

		if resp.StatusCode == 404 {
			_ = resp.Body.Close()
			continue
		}

		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			return "", fmt.Errorf("GitHub API returned status %d for %s", resp.StatusCode, manifestPath)
		}

		var contentResp struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&contentResp); err != nil {
			_ = resp.Body.Close()
			return "", fmt.Errorf("failed to decode response: %w", err)
		}
		_ = resp.Body.Close()

		decoded, err := base64.StdEncoding.DecodeString(contentResp.Content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 content: %w", err)
		}
		return string(decoded), nil
	}

	if actionPath != "" {
		return "", fmt.Errorf("no action.yml or action.yaml found in %s/%s@%s", ownerRepo, actionPath, ref)
	}

	return "", fmt.Errorf("no action.yml or action.yaml found in %s@%s", ownerRepo, ref)
}

func (c *Client) GetRawActionYML(ctx context.Context, ownerRepo, actionPath, ref string) (string, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return "", err
	}

	for _, manifestPath := range actionManifestPaths(actionPath) {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, manifestPath)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		if c.token != "" {
			req.Header.Set("Authorization", "token "+c.token)
		}
		req.Header.Set("User-Agent", "go-sort-out-gh-actions")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to make request: %w", err)
		}

		if resp.StatusCode == 404 {
			_ = resp.Body.Close()
			continue
		}

		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			return "", fmt.Errorf("raw content API returned status %d for %s", resp.StatusCode, manifestPath)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
		return string(body), nil
	}

	if actionPath != "" {
		return "", fmt.Errorf("no action.yml or action.yaml found in %s/%s@%s", ownerRepo, actionPath, ref)
	}

	return "", fmt.Errorf("no action.yml or action.yaml found in %s@%s", ownerRepo, ref)
}

func ParseActionYML(content string) (string, error) {
	var action actionYML
	if err := yaml.Unmarshal([]byte(content), &action); err != nil {
		return "", fmt.Errorf("failed to parse action.yml: %w", err)
	}
	return action.Runs.Using, nil
}

func (c *Client) CheckMultipleRuntimeEOL(ctx context.Context, refs []workflow.ActionRef) (map[string]*RuntimeEOLResult, map[string]error) {
	results := make(map[string]*RuntimeEOLResult)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrency)

	for _, ref := range refs {
		wg.Add(1)
		go func(ref workflow.ActionRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			refKey := ref.Key()

			actionContent, err := c.GetActionYML(ctx, ref.OwnerRepo, ref.ActionPath, ref.Version)
			if err != nil {
				actionContent, err = c.GetRawActionYML(ctx, ref.OwnerRepo, ref.ActionPath, ref.Version)
				if err != nil {
					mu.Lock()
					errors[refKey] = err
					mu.Unlock()
					return
				}
			}

			using, err := ParseActionYML(actionContent)
			if err != nil {
				mu.Lock()
				errors[refKey] = err
				mu.Unlock()
				return
			}

			eolInfo, eolErr := c.eolClient.CheckRunsUsing(ctx, using)
			if eolErr != nil {
				mu.Lock()
				errors[refKey] = eolErr
				mu.Unlock()
				return
			}
			if eolInfo != nil && eolInfo.IsEOL {
				mu.Lock()
				results[refKey] = &RuntimeEOLResult{
					OwnerRepo: ref.OwnerRepo,
					Runtime:   eolInfo.Runtime,
					Version:   eolInfo.Version,
					EOLDate:   eolInfo.EOLDate,
					IsEOL:     eolInfo.IsEOL,
				}
				mu.Unlock()
			}
		}(ref)
	}

	wg.Wait()
	return results, errors
}

func (c *Client) GetRateLimits(ctx context.Context) (*RateLimitInfo, error) {
	url := fmt.Sprintf("%s/rate_limit", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rateLimitResponse struct {
		Resources struct {
			Core struct {
				Limit     int    `json:"limit"`
				Remaining int    `json:"remaining"`
				Used      int    `json:"used"`
				Reset     int64  `json:"reset"`
				Resource  string `json:"resource"`
			} `json:"core"`
			Search struct {
				Limit     int    `json:"limit"`
				Remaining int    `json:"remaining"`
				Used      int    `json:"used"`
				Reset     int64  `json:"reset"`
				Resource  string `json:"resource"`
			} `json:"search"`
		} `json:"resources"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rateLimitResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	core := rateLimitResponse.Resources.Core
	return &RateLimitInfo{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		Used:      core.Used,
		Reset:     time.Unix(core.Reset, 0),
		Resource:  core.Resource,
	}, nil
}

func (c *Client) LogRateLimits(ctx context.Context) {
	info, err := c.GetRateLimits(ctx)
	if err != nil {
		log.Debugf("Failed to get rate limit info: %v", err)
		return
	}
	log.Debugf("GitHub API rate limit: limit=%d remaining=%d used=%d reset=%s resource=%s",
		info.Limit, info.Remaining, info.Used, info.Reset.Format(time.RFC3339), info.Resource)
}

// FlushCache persists all in-memory caches to disk.
func (c *Client) FlushCache() error {
	if c.cacheStore == nil || !c.cacheEnabled {
		return nil
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	if err := c.cacheStore.Save("archived", c.archivedCache); err != nil {
		return fmt.Errorf("failed to flush archived cache: %w", err)
	}
	if err := c.cacheStore.Save("releases", c.releaseCache); err != nil {
		return fmt.Errorf("failed to flush release cache: %w", err)
	}
	if err := c.cacheStore.Save("refsha", c.refSHACache); err != nil {
		return fmt.Errorf("failed to flush refsha cache: %w", err)
	}
	if err := c.cacheStore.Save("repoinfo", c.repoInfoCache); err != nil {
		return fmt.Errorf("failed to flush repoinfo cache: %w", err)
	}

	return nil
}

func (c *Client) ListOrgRepos(ctx context.Context, org string, opts *ListOrgReposOptions) ([]RepoEntry, error) {
	if opts == nil {
		opts = &ListOrgReposOptions{}
	}

	startURL := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&type=all&sort=full_name", c.baseURL, org)
	notFoundErr := fmt.Errorf("organization %s not found", org)

	return c.listEntityRepos(ctx, startURL, notFoundErr, opts)
}

func (c *Client) ListUserRepos(ctx context.Context, username string, opts *ListOrgReposOptions) ([]RepoEntry, error) {
	if opts == nil {
		opts = &ListOrgReposOptions{}
	}

	startURL := fmt.Sprintf("%s/users/%s/repos?per_page=100&type=all&sort=full_name", c.baseURL, username)
	notFoundErr := fmt.Errorf("user %s not found", username)

	return c.listEntityRepos(ctx, startURL, notFoundErr, opts)
}

func (c *Client) listEntityRepos(ctx context.Context, startURL string, notFoundErr error, opts *ListOrgReposOptions) ([]RepoEntry, error) {
	var allRepos []RepoEntry
	nextURL := startURL

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("User-Agent", "go-sort-out-gh-actions")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		c.logRateLimits(resp)

		if err := c.handleRateLimit(resp); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}

		if resp.StatusCode == 404 {
			_ = resp.Body.Close()
			return nil, notFoundErr
		}

		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}

		var page []RepoEntry
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		_ = resp.Body.Close()

		allRepos = append(allRepos, page...)

		nextURL = parseNextLink(resp.Header.Get("Link"), c.baseURLHost())
	}

	return filterRepos(allRepos, opts), nil
}

func parseNextLink(linkHeader, allowedHost string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasSuffix(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start != -1 && end != -1 && end > start {
				nextURL := part[start+1 : end]
				if parsed, err := url.Parse(nextURL); err == nil && parsed.Host == allowedHost {
					return nextURL
				}
				log.WithFields(log.Fields{
					"next_url":     nextURL,
					"allowed_host": allowedHost,
				}).Warn("Skipping pagination link with unexpected host")
				return ""
			}
		}
	}
	return ""
}

func filterRepos(repos []RepoEntry, opts *ListOrgReposOptions) []RepoEntry {
	var filtered []RepoEntry
	for _, repo := range repos {
		if opts.OnlyActive && repo.Archived {
			continue
		}
		if !opts.IncludeForks && repo.Fork {
			continue
		}
		filtered = append(filtered, repo)
	}
	return filtered
}

// Close flushes any in-memory cache to disk and releases resources.
func (c *Client) ListWorkflowFiles(ctx context.Context, ownerRepo, ref string) ([]ContentEntry, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/.github/workflows", c.baseURL, owner, repo)
	if ref != "" {
		apiURL += "?ref=" + url.QueryEscape(ref)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return []ContentEntry{}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var entries []ContentEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var workflowFiles []ContentEntry
	for _, entry := range entries {
		if entry.Type == "file" && (strings.HasSuffix(entry.Name, ".yml") || strings.HasSuffix(entry.Name, ".yaml")) {
			workflowFiles = append(workflowFiles, entry)
		}
	}

	return workflowFiles, nil
}

func (c *Client) GetFileContent(ctx context.Context, ownerRepo, path, ref string) (string, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	if ref != "" {
		apiURL += "?ref=" + url.QueryEscape(ref)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.logRateLimits(resp)

	if err := c.handleRateLimit(resp); err != nil {
		return "", err
	}

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("file %s not found in %s", path, ownerRepo)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var contentResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contentResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if contentResp.Content == "" {
		return "", fmt.Errorf("empty content returned for %s", path)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(contentResp.Content, "\n", ""))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 content: %w", err)
	}

	return string(decoded), nil
}

func (c *Client) GetRemoteWorkflowContents(ctx context.Context, ownerRepo, ref string) (map[string]string, error) {
	entries, err := c.ListWorkflowFiles(ctx, ownerRepo, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow files: %w", err)
	}

	if len(entries) == 0 {
		return map[string]string{}, nil
	}

	contents := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	for _, entry := range entries {
		wg.Add(1)
		go func(entry ContentEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			content, err := c.GetFileContent(ctx, ownerRepo, entry.Path, ref)
			if err != nil {
				log.WithFields(log.Fields{
					"path":  entry.Path,
					"error": err,
				}).Warn("Failed to fetch workflow file content")
				return
			}

			mu.Lock()
			contents[entry.Path] = content
			mu.Unlock()
		}(entry)
	}

	wg.Wait()
	return contents, nil
}

func (c *Client) Close() error {
	if err := c.FlushCache(); err != nil {
		return err
	}
	return nil
}

func (c *Client) baseURLHost() string {
	if parsed, err := url.Parse(c.baseURL); err == nil {
		return parsed.Host
	}
	return ""
}
