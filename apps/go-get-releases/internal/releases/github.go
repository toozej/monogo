package releases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
)

// Token uses environment credentials before it checks the GitHub CLI.
func Token(ctx context.Context, getenv func(string) string, command func(context.Context) ([]byte, error)) string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_API_TOKEN"} {
		if token := strings.TrimSpace(getenv(key)); token != "" {
			return token
		}
	}
	data, err := command(ctx)
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// DiscoverToken limits the time spent on the optional GitHub CLI command.
func DiscoverToken(ctx context.Context) string {
	return Token(ctx, os.Getenv, func(ctx context.Context) ([]byte, error) {
		if _, err := exec.LookPath("gh"); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com").Output()
	})
}

// GitHub uses google/go-github for the official GitHub REST API.
// Calls are sequential. The client waits for primary and secondary rate limits.
type GitHub struct {
	client    *github.Client
	downloads *http.Client
	retries   int
	wait      func(context.Context, time.Duration) error
	next      time.Time
}

// NewGitHub creates an API client and a separate client for asset redirects.
func NewGitHub(token string, retries int) (*GitHub, error) {
	opts := []github.ClientOptionsFunc{github.WithHTTPClient(&http.Client{Timeout: 15 * time.Minute})}
	if token != "" {
		opts = append(opts, github.WithAuthToken(token))
	}
	c, err := github.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &GitHub{client: c, downloads: &http.Client{Timeout: 15 * time.Minute}, retries: retries, wait: waitContext}, nil
}

func waitContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(err error, attempt int) time.Duration {
	delay := time.Second * time.Duration(1<<min(attempt, 8))
	var primary *github.RateLimitError
	if errors.As(err, &primary) {
		return max(delay, time.Until(primary.Rate.Reset.Time)+time.Second)
	}
	var secondary *github.AbuseRateLimitError
	if errors.As(err, &secondary) {
		if secondary.RetryAfter != nil {
			return max(delay, *secondary.RetryAfter)
		}
		return time.Minute * time.Duration(1<<min(attempt, 6))
	}
	var response *github.ErrorResponse
	if errors.As(err, &response) && response.Response != nil {
		if value := response.Response.Header.Get("Retry-After"); value != "" {
			if d, err := time.ParseDuration(value + "s"); err == nil {
				return max(delay, d)
			}
			if at, err := http.ParseTime(value); err == nil {
				return max(delay, time.Until(at))
			}
		}
	}
	return delay
}

func retryable(err error) bool {
	var primary *github.RateLimitError
	var secondary *github.AbuseRateLimitError
	if errors.As(err, &primary) || errors.As(err, &secondary) {
		return true
	}
	var response *github.ErrorResponse
	if errors.As(err, &response) && response.Response != nil {
		status := response.Response.StatusCode
		return status == http.StatusTooManyRequests || status >= 500
	}
	return !errors.Is(err, context.Canceled)
}

func (g *GitHub) call(ctx context.Context, fn func() (*github.Response, error)) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := g.wait(ctx, time.Until(g.next)); err != nil {
			return err
		}
		response, err := fn()
		if response != nil && response.Rate.Limit > 0 && response.Rate.Remaining == 0 {
			g.next = response.Rate.Reset.Add(time.Second)
		}
		if err == nil {
			return nil
		}
		if attempt >= g.retries || !retryable(err) {
			return err
		}
		if err := g.wait(ctx, retryDelay(err, attempt)); err != nil {
			return err
		}
	}
}

func splitRepository(repo string) (string, string) {
	owner, name, _ := strings.Cut(repo, "/")
	return owner, name
}

// VerifyRepository checks that the repository exists and is accessible.
func (g *GitHub) VerifyRepository(ctx context.Context, repo string) error {
	owner, name := splitRepository(repo)
	return g.call(ctx, func() (*github.Response, error) {
		_, response, err := g.client.Repositories.Get(ctx, owner, name)
		return response, err
	})
}

// Releases reads all pages because the API order uses creation dates, not publication dates.
func (g *GitHub) Releases(ctx context.Context, repo string) ([]*Release, error) {
	owner, name := splitRepository(repo)
	result := []*Release{}
	for page := 1; page != 0; {
		var releases []*github.RepositoryRelease
		var response *github.Response
		err := g.call(ctx, func() (*github.Response, error) {
			var err error
			releases, response, err = g.client.Repositories.ListReleases(ctx, owner, name, &github.ListOptions{Page: page, PerPage: 100})
			return response, err
		})
		if err != nil {
			return nil, err
		}
		for _, release := range releases {
			result = append(result, githubRelease(release))
		}
		page = response.NextPage
	}
	return result, nil
}

// Assets reads all asset pages instead of relying on embedded release assets.
func (g *GitHub) Assets(ctx context.Context, repo string, id int64) ([]*Asset, error) {
	owner, name := splitRepository(repo)
	result := []*Asset{}
	for page := 1; page != 0; {
		var assets []*github.ReleaseAsset
		var response *github.Response
		err := g.call(ctx, func() (*github.Response, error) {
			var err error
			assets, response, err = g.client.Repositories.ListReleaseAssets(ctx, owner, name, id, &github.ListOptions{Page: page, PerPage: 100})
			return response, err
		})
		if err != nil {
			return nil, err
		}
		for _, asset := range assets {
			result = append(result, githubAsset(asset))
		}
		page = response.NextPage
	}
	return result, nil
}

// Download sends credentials only to the API. Redirect downloads use no token.
func (g *GitHub) Download(ctx context.Context, repo string, id int64) (io.ReadCloser, error) {
	owner, name := splitRepository(repo)
	var body io.ReadCloser
	var location string
	err := g.call(ctx, func() (*github.Response, error) {
		var err error
		body, location, err = g.client.Repositories.DownloadReleaseAsset(ctx, owner, name, id, nil)
		return nil, err
	})
	if err != nil {
		return nil, err
	}
	if body != nil {
		return body, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	response, err := g.downloads.Do(req)
	if err != nil {
		return nil, err
	}
	if err := github.CheckResponse(response); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("asset download returned status %d", response.StatusCode)
	}
	return response.Body, nil
}

func githubRelease(r *github.RepositoryRelease) *Release {
	if r == nil {
		return nil
	}
	result := &Release{ID: r.GetID(), TagName: r.GetTagName(), Name: r.Name, Draft: r.GetDraft(), Prerelease: r.GetPrerelease()}
	if r.PublishedAt != nil {
		result.PublishedAt = &Timestamp{r.PublishedAt.Time}
	}
	return result
}

func githubAsset(a *github.ReleaseAsset) *Asset {
	if a == nil {
		return nil
	}
	result := &Asset{ID: a.ID, Name: a.Name, Size: a.Size, State: a.State, Digest: a.Digest}
	if a.UpdatedAt != nil {
		result.UpdatedAt = &Timestamp{a.UpdatedAt.Time}
	}
	return result
}
