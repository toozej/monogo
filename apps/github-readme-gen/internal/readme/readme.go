// Package readme renders the README template using data fetched from GitHub.
package readme

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/v89/github"
	"golang.org/x/oauth2"
)

// Repo is a minimal view of a GitHub repository used in the README template.
type Repo struct {
	Name        string
	URL         string
	Description string
}

// Contribution is a recent activity entry on a repository.
type Contribution struct {
	Repo       Repo
	OccurredAt time.Time
}

// Data is the data passed to the README template.
type Data struct {
	Username            string
	RecentCreatedRepos  []Repo
	RecentContributions []Contribution
	RecentStarredRepos  []Repo
}

// Client is a thin wrapper around the GitHub API client.
type Client struct {
	gh *github.Client
}

// NewClient returns a Client authenticated with the given token (if non-empty).
func NewClient(ctx context.Context, token string) *Client {
	_ = ctx
	var opts []github.ClientOptionsFunc
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient := oauth2.NewClient(ctx, ts)
		opts = append(opts, github.WithHTTPClient(httpClient))
	}
	c, err := github.NewClient(opts...)
	if err != nil {
		log.Fatalf("github.NewClient: %v", err)
	}
	return &Client{gh: c}
}

// Fetch collects the data needed to render the README for the given user.
func (c *Client) Fetch(ctx context.Context, user string) (*Data, error) {
	starredRepos, starredRepoNames, err := c.recentStarredRepos(ctx, user, 5)
	if err != nil {
		return nil, err
	}
	createdRepos, err := c.recentCreatedRepos(ctx, user, 5, starredRepoNames)
	if err != nil {
		return nil, err
	}
	contributions, err := c.recentContributions(ctx, user, 10, starredRepoNames)
	if err != nil {
		return nil, err
	}
	return &Data{
		Username:            user,
		RecentCreatedRepos:  createdRepos,
		RecentContributions: contributions,
		RecentStarredRepos:  starredRepos,
	}, nil
}

// recentCreatedRepos returns the n most recently created public repositories
// owned by user.
func (c *Client) recentCreatedRepos(ctx context.Context, user string, n int, excluded map[string]struct{}) ([]Repo, error) {
	opt := &github.RepositoryListByUserOptions{
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: max(n*4, 100)},
		Type:        "public",
	}
	repos, _, err := c.gh.Repositories.ListByUser(ctx, user, opt)
	if err != nil {
		return nil, fmt.Errorf("list user repos: %w", err)
	}
	out := make([]Repo, 0, len(repos))
	for _, r := range repos {
		if r == nil {
			continue
		}
		if _, ok := excluded[r.GetFullName()]; ok {
			continue
		}
		out = append(out, Repo{
			Name:        r.GetName(),
			URL:         r.GetHTMLURL(),
			Description: r.GetDescription(),
		})
		if n > 0 && len(out) == n {
			break
		}
	}
	return out, nil
}

// recentStarredRepos returns the n most recently starred public repositories
// and the complete set of starred repository names. The set keeps starred
// repositories out of every other README section.
func (c *Client) recentStarredRepos(ctx context.Context, user string, n int) ([]Repo, map[string]struct{}, error) {
	starred := make([]Repo, 0, n)
	names := make(map[string]struct{})
	opt := &github.ActivityListStarredOptions{
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		repos, resp, err := c.gh.Activity.ListStarred(ctx, user, opt)
		if err != nil {
			return nil, nil, fmt.Errorf("list starred repos: %w", err)
		}
		for _, starredRepo := range repos {
			if starredRepo == nil || starredRepo.Repository == nil {
				continue
			}
			repo := starredRepo.Repository
			names[repo.GetFullName()] = struct{}{}
			if n <= 0 || len(starred) < n {
				starred = append(starred, Repo{
					Name:        repo.GetFullName(),
					URL:         repo.GetHTMLURL(),
					Description: repo.GetDescription(),
				})
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return starred, names, nil
}

// watchEventType is the GitHub event type emitted when a user stars a
// repository. Starred repos are not contributions and should not appear in
// the "What I've been working on" section.
const watchEventType = "WatchEvent"

// recentContributions returns up to n recent public activity events for user,
// deduplicated by repository (most recent occurrence per repo wins), with
// star events (WatchEvent) excluded.
func (c *Client) recentContributions(ctx context.Context, user string, n int, excluded map[string]struct{}) ([]Contribution, error) {
	events, _, err := c.gh.Activity.ListEventsPerformedByUser(ctx, user, false, &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("list user events: %w", err)
	}
	out := filterAndDedupeEventsExcluding(events, excluded)
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// filterAndDedupeEvents converts a slice of GitHub events into a slice of
// Contributions, filtered to exclude non-contribution event types (e.g.
// WatchEvent for starring repos) and deduplicated by repository name,
// keeping the most recent occurrence per repo. The result is sorted by
// OccurredAt in descending order.
func filterAndDedupeEvents(events []*github.Event) []Contribution {
	return filterAndDedupeEventsExcluding(events, nil)
}

// filterAndDedupeEventsExcluding also omits repositories in excluded, which
// ensures repositories in the starred section do not appear elsewhere.
func filterAndDedupeEventsExcluding(events []*github.Event, excluded map[string]struct{}) []Contribution {
	byRepo := make(map[string]Contribution, len(events))
	for _, ev := range events {
		if ev == nil || ev.Repo == nil {
			continue
		}
		if ev.GetType() == watchEventType {
			continue
		}
		name := ev.Repo.GetName()
		if _, ok := excluded[name]; ok {
			continue
		}
		if _, seen := byRepo[name]; seen {
			continue
		}
		htmlURL := ev.Repo.GetHTMLURL()
		if htmlURL == "" && name != "" {
			htmlURL = "https://github.com/" + name
		}
		byRepo[name] = Contribution{
			Repo: Repo{
				Name:        name,
				URL:         htmlURL,
				Description: ev.Repo.GetDescription(),
			},
			OccurredAt: ev.GetCreatedAt().Time,
		}
	}
	out := make([]Contribution, 0, len(byRepo))
	for _, c := range byRepo {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	return out
}

// humanize returns a short human-readable string for the time delta, like
// "just now", "5 minutes ago", "1 day ago", "2 weeks ago".
func humanize(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// Render reads the template at templatePath, executes it with data, and
// writes the result to outputPath.
func Render(templatePath, outputPath string, data *Data) error {
	// The template path is an explicit CLI argument, which lets the generator
	// render profile-specific templates outside this package.
	tplBytes, err := os.ReadFile(templatePath) // #nosec G304 -- intentionally caller-selected template
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	funcs := template.FuncMap{"humanize": humanize}
	tpl, err := template.New("readme").Funcs(funcs).Parse(string(tplBytes))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
