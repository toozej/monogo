package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
)

// forgeHTTP sends credentials only to its configured API origin.
type forgeHTTP struct {
	base    string
	client  *http.Client
	header  string
	token   string
	retries int
	wait    func(context.Context, time.Duration) error
}

func newForgeHTTP(base, header, token string, retries int) *forgeHTTP {
	f := &forgeHTTP{base: base, header: header, token: token, retries: retries, wait: waitContext}
	f.client = &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many asset redirects")
		}
		if err := f.validateURL(req.URL.String()); err != nil {
			return err
		}
		req.Header.Del("Authorization")
		req.Header.Del("Private-Token")
		req.Header.Del("Job-Token")
		return nil
	}}
	return f
}

func (f *forgeHTTP) validateURL(raw string) error {
	u, err := url.Parse(raw)
	// HTTP is available only for injected local test APIs. Production bases use HTTPS.
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && (!strings.HasPrefix(f.base, "http://") || u.Scheme != "http")) {
		return errors.New("asset URL must use HTTPS without credentials or a fragment")
	}
	return nil
}

func (f *forgeHTTP) request(ctx context.Context, raw string) (*http.Response, error) {
	if err := f.validateURL(raw); err != nil {
		return nil, err
	}
	target, _ := url.Parse(raw)
	base, _ := url.Parse(f.base)
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if target.Scheme == base.Scheme && target.Host == base.Host && f.token != "" {
			req.Header.Set(f.header, f.token)
		}
		response, err := f.client.Do(req)
		if err == nil && response.StatusCode == http.StatusOK {
			return response, nil
		}
		if err == nil {
			_ = response.Body.Close()
			err = &github.ErrorResponse{Response: response, Message: http.StatusText(response.StatusCode)}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt >= f.retries || !retryable(err) {
			return nil, err
		}
		if err := f.wait(ctx, retryDelay(err, attempt)); err != nil {
			return nil, err
		}
	}
}

func (f *forgeHTTP) json(ctx context.Context, endpoint string, output any) (http.Header, error) {
	response, err := f.request(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	// Limit metadata. A second decode rejects truncated or additional JSON documents.
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16*1024*1024+1))
	if err := decoder.Decode(output); err != nil {
		return nil, fmt.Errorf("read forge metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("forge metadata must contain one JSON document")
	}
	return response.Header, nil
}

func endpoint(base, path string, query url.Values) string {
	result := strings.TrimRight(base, "/") + path
	if len(query) != 0 {
		result += "?" + query.Encode()
	}
	return result
}

// nextPage reads only the page number. It does not follow a server-supplied URL.
func nextPage(header http.Header, current int) (int, error) {
	next := header.Get("X-Next-Page")
	if next == "" {
		for _, link := range strings.Split(header.Get("Link"), ",") {
			parts := strings.Split(link, ";")
			if len(parts) < 2 {
				continue
			}
			for _, relation := range parts[1:] {
				if strings.TrimSpace(relation) == `rel="next"` {
					u, err := url.Parse(strings.Trim(strings.TrimSpace(parts[0]), "<>"))
					if err != nil {
						return 0, errors.New("invalid forge pagination link")
					}
					next = u.Query().Get("page")
					if next == "" {
						return 0, errors.New("forge pagination link has no page")
					}
				}
			}
		}
	}
	if next == "" {
		return 0, nil
	}
	page, err := strconv.Atoi(next)
	if err != nil || page <= current {
		return 0, errors.New("forge pagination must advance")
	}
	return page, nil
}

func environmentToken(getenv func(string) string, keys ...string) string {
	for _, key := range keys {
		if token := strings.TrimSpace(getenv(key)); token != "" {
			return token
		}
	}
	return ""
}

// NewSources creates only the clients needed for this configuration.
func NewSources(ctx context.Context, c Config, getenv func(string) string, githubToken func(context.Context) string) (Sources, error) {
	sources := Sources{}
	for _, repo := range c.Repositories {
		host, _, err := repositoryParts(repo.Name)
		if err != nil {
			return nil, err
		}
		if sources[host] != nil {
			continue
		}
		switch host {
		case "github.com":
			source, err := NewGitHub(githubToken(ctx), c.Retries)
			if err != nil {
				return nil, err
			}
			sources[host] = source
		case "gitlab.com":
			token := environmentToken(getenv, "GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN")
			header := "PRIVATE-TOKEN"
			if token == "" {
				token = environmentToken(getenv, "OAUTH_TOKEN")
				if token != "" {
					header = "Authorization"
					token = "Bearer " + token
				}
			}
			sources[host] = newGitLab(newForgeHTTP("https://gitlab.com/api/v4", header, token, c.Retries))
		case "codeberg.org":
			token := environmentToken(getenv, "FORGEJO_TOKEN", "CODEBERG_TOKEN")
			if token != "" {
				token = "token " + token
			}
			sources[host] = newForgejo(newForgeHTTP("https://codeberg.org/api/v1", "Authorization", token, c.Retries))
		case "tangled.sh":
			// Tangled's public read API and artifact endpoints require no authentication.
			sources[host] = newTangled(newForgeHTTP("https://api.tangled.org", "", "", c.Retries))
		}
	}
	return sources, nil
}
