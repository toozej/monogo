package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// getPage reads one API page. It accepts pagination links only from the API host.
func (c *Client) getPage(ctx context.Context, apiURL string, target any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create API request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-sort-out-gh-actions")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("read API page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	c.logRateLimits(resp)
	if err := c.handleRateLimit(resp); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return "", fmt.Errorf("decode API page: %w", err)
	}
	return parseNextLink(resp.Header.Get("Link"), c.baseURLHost()), nil
}

// listPages reads all pages without accepting repeated pagination links.
func listPages[T any](ctx context.Context, c *Client, apiURL string) ([]T, error) {
	var result []T
	seen := make(map[string]bool)
	for apiURL != "" {
		if seen[apiURL] {
			return nil, fmt.Errorf("GitHub API repeated a pagination link")
		}
		seen[apiURL] = true
		var page []T
		next, err := c.getPage(ctx, apiURL, &page)
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
		apiURL = next
	}
	return result, nil
}

// ListReleases reads release dates without using the latest-release cache.
func (c *Client) ListReleases(ctx context.Context, ownerRepo string) ([]ReleaseInfo, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return nil, err
	}
	return listPages[ReleaseInfo](ctx, c, fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo))
}

// ListDependabotPullRequests reads open Dependabot PRs for one target branch.
func (c *Client) ListDependabotPullRequests(ctx context.Context, ownerRepo, baseBranch string) ([]PullRequest, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return nil, err
	}
	if baseBranch == "" {
		info, err := c.GetRepoInfo(ctx, ownerRepo)
		if err != nil {
			return nil, fmt.Errorf("read default branch: %w", err)
		}
		baseBranch = info.DefaultBranch
		if baseBranch == "" {
			return nil, fmt.Errorf("GitHub API did not return a default branch for %s", ownerRepo)
		}
	}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100&base=%s", c.baseURL, owner, repo, url.QueryEscape(baseBranch))
	prs, err := listPages[PullRequest](ctx, c, apiURL)
	if err != nil {
		return nil, err
	}
	var result []PullRequest
	for _, pr := range prs {
		if pr.State == "open" && pr.User.Login == "dependabot[bot]" && pr.Base.Ref == baseBranch {
			result = append(result, pr)
		}
	}
	return result, nil
}

func (c *Client) ListPullRequestFiles(ctx context.Context, ownerRepo string, number int) ([]PullRequestFile, error) {
	owner, repo, err := parseOwnerRepo(ownerRepo)
	if err != nil {
		return nil, err
	}
	return listPages[PullRequestFile](ctx, c, fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?per_page=100", c.baseURL, owner, repo, number))
}
