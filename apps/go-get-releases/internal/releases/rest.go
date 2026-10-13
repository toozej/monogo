package releases

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// restSource adapts the GitLab and Forgejo release APIs.
// The runner calls each source sequentially.
type restSource struct {
	http      *forgeHTTP
	gitlab    bool
	releases  map[string]map[int64][]restAsset
	downloads map[string]map[int64]string
}

func newGitLab(client *forgeHTTP) *restSource  { return newREST(client, true) }
func newForgejo(client *forgeHTTP) *restSource { return newREST(client, false) }
func newREST(client *forgeHTTP, gitlab bool) *restSource {
	return &restSource{http: client, gitlab: gitlab, releases: map[string]map[int64][]restAsset{}, downloads: map[string]map[int64]string{}}
}
func (s *restSource) repositoryPath(repo string) string {
	if s.gitlab {
		return "/projects/" + url.PathEscape(repo)
	}
	parts := strings.Split(repo, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return "/repos/" + strings.Join(parts, "/")
}
func (s *restSource) VerifyRepository(ctx context.Context, repo string) error {
	var data struct {
		ID int64 `json:"id"`
	}
	_, err := s.http.json(ctx, endpoint(s.http.base, s.repositoryPath(repo), nil), &data)
	return err
}

type restAsset struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Size       *int      `json:"size"`
	URL        string    `json:"url"`
	DirectURL  string    `json:"direct_asset_url"`
	BrowserURL string    `json:"browser_download_url"`
	Digest     string    `json:"digest"`
	CreatedAt  time.Time `json:"created_at"`
}

type restRelease struct {
	Tag        string     `json:"tag_name"`
	Name       string     `json:"name"`
	ReleasedAt *Timestamp `json:"released_at"`
	Assets     struct {
		Links []restAsset `json:"links"`
	} `json:"assets"`
}

func (s *restSource) Releases(ctx context.Context, repo string) ([]*Release, error) {
	result := []*Release{}
	s.releases[repo] = map[int64][]restAsset{}
	s.downloads[repo] = map[int64]string{}
	for page := 1; page != 0; {
		query := url.Values{"page": {strconv.Itoa(page)}}
		key := "limit"
		if s.gitlab {
			key = "per_page"
		}
		query.Set(key, "100")
		var header http.Header
		if s.gitlab {
			var data []restRelease
			h, err := s.http.json(ctx, endpoint(s.http.base, s.repositoryPath(repo)+"/releases", query), &data)
			if err != nil {
				return nil, err
			}
			header = h
			for _, item := range data {
				id := stableID(item.Tag)
				result = append(result, &Release{ID: id, TagName: item.Tag, Name: &item.Name, PublishedAt: item.ReleasedAt, Draft: item.ReleasedAt != nil && item.ReleasedAt.After(time.Now())})
				s.releases[repo][id] = item.Assets.Links
			}
		} else {
			// Forgejo embeds an assets array, unlike GitLab's assets object.
			var data []struct {
				ID          int64      `json:"id"`
				Tag         string     `json:"tag_name"`
				Name        string     `json:"name"`
				Draft       bool       `json:"draft"`
				Prerelease  bool       `json:"prerelease"`
				PublishedAt *Timestamp `json:"published_at"`
			}
			h, err := s.http.json(ctx, endpoint(s.http.base, s.repositoryPath(repo)+"/releases", query), &data)
			if err != nil {
				return nil, err
			}
			header = h
			for _, item := range data {
				result = append(result, &Release{ID: item.ID, TagName: item.Tag, Name: &item.Name, Draft: item.Draft, Prerelease: item.Prerelease, PublishedAt: item.PublishedAt})
			}
		}
		next, err := nextPage(header, page)
		if err != nil {
			return nil, err
		}
		page = next
	}
	return result, nil
}

func (s *restSource) convertAssets(repo string, data []restAsset, gitlab bool) ([]*Asset, error) {
	result := []*Asset{}
	if s.downloads[repo] == nil {
		s.downloads[repo] = map[int64]string{}
	}
	for _, item := range data {
		raw := item.BrowserURL
		size := item.Size
		id := item.ID
		if gitlab {
			raw = item.DirectURL
			if raw == "" {
				raw = item.URL
			}
			// GitLab release links omit sizes. The downloader checks checksums and streams the body.
			size = nil
			id = stableID(strconv.FormatInt(item.ID, 10) + ":" + raw)
		}
		if err := s.http.validateURL(raw); err != nil {
			return nil, err
		}
		state := "uploaded"
		asset := &Asset{ID: &id, Name: &item.Name, Size: size, State: &state, Digest: &item.Digest}
		if !item.CreatedAt.IsZero() {
			asset.UpdatedAt = &Timestamp{item.CreatedAt}
		}
		result = append(result, asset)
		s.downloads[repo][id] = raw
	}
	return result, nil
}

func (s *restSource) Assets(ctx context.Context, repo string, id int64) ([]*Asset, error) {
	if s.gitlab {
		result, ok := s.releases[repo][id]
		if !ok {
			return nil, errors.New("unknown GitLab release ID")
		}
		return s.convertAssets(repo, result, true)
	}
	result := []*Asset{}
	for page := 1; page != 0; {
		query := url.Values{"page": {strconv.Itoa(page)}, "limit": {"100"}}
		var data []restAsset
		header, err := s.http.json(ctx, endpoint(s.http.base, s.repositoryPath(repo)+"/releases/"+strconv.FormatInt(id, 10)+"/assets", query), &data)
		if err != nil {
			return nil, err
		}
		assets, err := s.convertAssets(repo, data, false)
		if err != nil {
			return nil, err
		}
		result = append(result, assets...)
		next, err := nextPage(header, page)
		if err != nil {
			return nil, err
		}
		page = next
	}
	return result, nil
}
func (s *restSource) Download(ctx context.Context, repo string, id int64) (io.ReadCloser, error) {
	raw, ok := s.downloads[repo][id]
	if !ok {
		return nil, errors.New("unknown forge asset ID")
	}
	response, err := s.http.request(ctx, raw)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
