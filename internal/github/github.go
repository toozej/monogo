package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const maxConcurrency = 10

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string

	archivedCache map[string]bool
	releaseCache  map[string]*ReleaseInfo
	refSHACache   map[string]string
	repoInfoCache map[string]*RepoInfo
	cacheMu       sync.RWMutex
}

type RepoInfo struct {
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	Archived       bool   `json:"archived"`
	Private        bool   `json:"private"`
	HTMLURL        string `json:"html_url"`
	Owner          Owner  `json:"owner"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	PushedAt       string `json:"pushed_at"`
	Deprecated     bool   `json:"deprecated"`
	DeprecationMsg string `json:"deprecation_warning_message"`
}

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type Owner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type RateLimitInfo struct {
	Limit     int
	Remaining int
	Used      int
	Reset     time.Time
	Resource  string
}

func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient:    httpClient,
		token:         "",
		baseURL:       baseURL,
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
	}
}

func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		httpClient:    tc,
		token:         token,
		baseURL:       "https://api.github.com",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
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
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	if release, ok := c.releaseCache[cleanRepo]; ok {
		return release, true
	}
	return nil, false
}

func (c *Client) setCachedRelease(ownerRepo string, release *ReleaseInfo) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	cleanRepo := cleanOwnerRepo(ownerRepo)
	c.releaseCache[cleanRepo] = release
}

func (c *Client) getCachedRefSHA(ownerRepo, ref string) (string, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	key := cleanOwnerRepo(ownerRepo) + "@" + ref
	if sha, ok := c.refSHACache[key]; ok {
		return sha, true
	}
	return "", false
}

func (c *Client) setCachedRefSHA(ownerRepo, ref, sha string) {
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
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

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
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

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

	c.setCachedRelease(ownerRepo, &release)
	return &release, nil
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

type RefInfo struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		URL  string `json:"url"`
		Type string `json:"type"`
	} `json:"object"`
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
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

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
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

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

type StaleInfo struct {
	Repo               string
	Deprecated         bool
	DeprecationMessage string
	LastUpdated        time.Time
	StaleByAge         bool
}

func (c *Client) GetRateLimits(ctx context.Context) (*RateLimitInfo, error) {
	url := fmt.Sprintf("%s/rate_limit", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-find-archived-gh-actions")

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
