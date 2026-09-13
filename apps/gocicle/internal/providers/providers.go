// Package providers separates login, repository discovery, and Git transport metadata.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/toozej/monogo/pkg/urlsafe"
	"golang.org/x/oauth2"
)

type Config struct {
	BaseURL     string `json:"baseURL"`
	ClientID    string `json:"clientID"`
	SecretID    string `json:"secretID"`
	APIURL      string `json:"apiURL,omitempty"`
	RedirectURL string `json:"redirectURL,omitempty"`
}
type Profile struct {
	Subject string `json:"subject"`
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
}
type Repository struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SSHURL string `json:"sshURL"`
	Branch string `json:"branch"`
}
type State struct {
	Verifier string `json:"verifier"`
	DPoPKey  string `json:"dpopKey,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	PDS      string `json:"pds,omitempty"`
	Subject  string `json:"subject,omitempty"`
	TokenURL string `json:"tokenURL,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
}
type Login interface {
	Begin(context.Context, string, string) (string, State, error)
	Exchange(context.Context, string, string, State) (Profile, *oauth2.Token, error)
}
type Metadata interface {
	Repositories(context.Context, *oauth2.Token, string) ([]Repository, error)
}
type GitTransport interface {
	CloneURL(Repository, bool) (string, error)
}
type Adapter struct {
	Kind   string
	Config Config
	Secret string
	HTTP   *http.Client
}

func New(kind string, cfg Config, secret string) (*Adapter, error) {
	switch kind {
	case "github", "gitlab", "codeberg", "forgejo", "sourcehut", "tangled", "git":
	default:
		return nil, errors.New("provider type is unsupported")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = map[string]string{"github": "https://github.com", "gitlab": "https://gitlab.com", "codeberg": "https://codeberg.org", "sourcehut": "https://meta.sr.ht", "tangled": "https://tangled.org"}[kind]
	}
	if kind != "git" {
		if err := httpsURL(cfg.BaseURL); err != nil {
			return nil, err
		}
	}
	if cfg.APIURL != "" {
		if err := httpsURL(cfg.APIURL); err != nil {
			return nil, err
		}
	}
	return &Adapter{Kind: kind, Config: cfg, Secret: secret, HTTP: PublicClient()}, nil
}
func httpsURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("provider URL must use HTTPS")
	}
	return nil
}
func PublicClient() *http.Client {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if urlsafe.IsInternalIP(ip.IP) {
				return nil, errors.New("provider address is not public")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("provider address is unavailable")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}}
}
func (a *Adapter) oauth() *oauth2.Config {
	base := strings.TrimRight(a.Config.BaseURL, "/")
	auth, token := base+"/login/oauth/authorize", base+"/login/oauth/access_token"
	scopes := []string{"read:user"}
	switch a.Kind {
	case "gitlab":
		auth = base + "/oauth/authorize"
		token = base + "/oauth/token"
		scopes = []string{"read_user", "read_api"}
	case "forgejo", "codeberg":
		scopes = []string{"read:user", "read:repository"}
	case "sourcehut":
		auth = base + "/oauth2/authorize"
		token = base + "/oauth2/access-token"
		scopes = []string{"meta.sr.ht/PROFILE:RO", "git.sr.ht/REPOSITORIES:RO"}
	}
	return &oauth2.Config{ClientID: a.Config.ClientID, ClientSecret: a.Secret, RedirectURL: a.Config.RedirectURL, Scopes: scopes, Endpoint: oauth2.Endpoint{AuthURL: auth, TokenURL: token, AuthStyle: oauth2.AuthStyleInParams}}
}
func (a *Adapter) Begin(ctx context.Context, state, loginHint string) (string, State, error) {
	if a.Kind == "git" {
		return "", State{}, errors.New("generic Git has no login provider")
	}
	if a.Kind == "tangled" {
		return a.beginAT(ctx, state, loginHint)
	}
	verifier := oauth2.GenerateVerifier()
	return a.oauth().AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), State{Verifier: verifier}, nil
}
func (a *Adapter) Exchange(ctx context.Context, code, issuer string, state State) (Profile, *oauth2.Token, error) {
	if a.Kind == "tangled" {
		return a.exchangeAT(ctx, code, issuer, state)
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.HTTP)
	token, err := a.oauth().Exchange(ctx, code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return Profile{}, nil, fmt.Errorf("%s login failed: reconnect the provider", a.Kind)
	}
	profile, err := a.profile(ctx, token)
	return profile, token, err
}
func (a *Adapter) request(ctx context.Context, method, endpoint, token string, body any, out any) error {
	if err := httpsURL(endpoint); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return errors.New("provider request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned HTTP %d: check the connection and credentials", a.Kind, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
}
func (a *Adapter) api() string {
	if a.Config.APIURL != "" {
		return strings.TrimRight(a.Config.APIURL, "/")
	}
	switch a.Kind {
	case "github":
		if a.Config.BaseURL == "https://github.com" {
			return "https://api.github.com"
		}
		return a.Config.BaseURL + "/api/v3"
	case "gitlab":
		return a.Config.BaseURL + "/api/v4"
	case "sourcehut":
		return a.Config.BaseURL
	default:
		return a.Config.BaseURL + "/api/v1"
	}
}
func (a *Adapter) profile(ctx context.Context, token *oauth2.Token) (Profile, error) {
	var raw struct {
		ID       json.Number `json:"id"`
		Login    string      `json:"login"`
		Name     string      `json:"name"`
		Username string      `json:"username"`
		Email    string      `json:"email"`
		Avatar   string      `json:"avatar_url"`
	}
	if a.Kind == "sourcehut" {
		var result struct {
			Data struct {
				Me struct {
					ID                      int
					Username, Email, Avatar string
				}
			}
			Errors []any
		}
		err := a.request(ctx, "POST", a.api()+"/query", token.AccessToken, map[string]string{"query": "{ me { id username email avatar } }"}, &result)
		if err != nil || len(result.Errors) > 0 || result.Data.Me.ID <= 0 {
			return Profile{}, errors.New("SourceHut profile query failed")
		}
		return Profile{Subject: strconv.Itoa(result.Data.Me.ID), Name: result.Data.Me.Username, Email: result.Data.Me.Email, Avatar: result.Data.Me.Avatar}, nil
	}
	if err := a.request(ctx, "GET", a.api()+"/user", token.AccessToken, nil, &raw); err != nil {
		return Profile{}, err
	}
	if raw.ID == "" {
		return Profile{}, errors.New("provider omitted its stable subject")
	}
	name := raw.Name
	if name == "" {
		name = raw.Login
	}
	if name == "" {
		name = raw.Username
	}
	return Profile{Subject: string(raw.ID), Name: name, Email: raw.Email, Avatar: raw.Avatar}, nil
}
func (a *Adapter) Repositories(ctx context.Context, token *oauth2.Token, cursor string) ([]Repository, error) {
	if token == nil {
		return nil, errors.New("repository discovery needs a provider connection")
	}
	if a.Kind == "tangled" {
		return a.repositoriesAT(ctx, token, cursor)
	}
	if a.Kind == "sourcehut" {
		endpoint := a.Config.APIURL
		if endpoint == "" {
			endpoint = "https://git.sr.ht"
		}
		var result struct {
			Data struct {
				Me           struct{ CanonicalName string }
				Repositories struct {
					Results []struct {
						Name  string
						Owner struct{ CanonicalName string }
					}
				}
			}
			Errors []any
		}
		body := map[string]any{"query": "query($cursor: Cursor) { repositories(cursor: $cursor) { results { name owner { canonicalName } } } }", "variables": map[string]any{"cursor": nil}}
		if cursor != "" {
			body["variables"] = map[string]any{"cursor": cursor}
		}
		if err := a.request(ctx, "POST", endpoint+"/query", token.AccessToken, body, &result); err != nil {
			return nil, err
		}
		if len(result.Errors) > 0 {
			return nil, errors.New("SourceHut repository query failed")
		}
		repos := []Repository{}
		host, _ := url.Parse(endpoint)
		for _, r := range result.Data.Repositories.Results {
			p := r.Owner.CanonicalName + "/" + r.Name
			repos = append(repos, Repository{Name: r.Name, URL: endpoint + "/" + p, SSHURL: "ssh://git@" + host.Host + "/" + p})
		}
		return repos, nil
	}
	page := 1
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 1 {
			return nil, errors.New("page must be a positive integer")
		}
		page = n
	}
	endpoint := a.api() + "/user/repos?per_page=100&page=" + strconv.Itoa(page)
	if a.Kind == "gitlab" {
		endpoint = a.api() + "/projects?membership=true&per_page=100&page=" + strconv.Itoa(page)
	}
	var rows []struct {
		Name     string
		CloneURL string `json:"clone_url"`
		HTTPURL  string `json:"http_url_to_repo"`
		SSHURL   string `json:"ssh_url"`
		SSHRepo  string `json:"ssh_url_to_repo"`
		Branch   string `json:"default_branch"`
	}
	if err := a.request(ctx, "GET", endpoint, token.AccessToken, nil, &rows); err != nil {
		return nil, err
	}
	result := make([]Repository, 0, len(rows))
	for _, row := range rows {
		if row.CloneURL == "" {
			row.CloneURL = row.HTTPURL
		}
		if row.SSHURL == "" {
			row.SSHURL = row.SSHRepo
		}
		result = append(result, Repository{Name: row.Name, URL: row.CloneURL, SSHURL: row.SSHURL, Branch: row.Branch})
	}
	return result, nil
}
func (a *Adapter) CloneURL(r Repository, write bool) (string, error) {
	value := r.URL
	if write && r.SSHURL != "" {
		value = r.SSHURL
	}
	if value == "" {
		return "", errors.New("repository clone URL is unavailable")
	}
	return value, nil
}
