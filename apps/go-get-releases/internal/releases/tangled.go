package releases

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// tangledSource reads public AT Protocol records and annotated Git tags.
// Tangled's read endpoints use no API token.
type tangledSource struct {
	http      *forgeHTTP
	identity  string
	appview   string
	repos     map[string]tangledRepo
	releases  map[string]map[int64][]*Asset
	downloads map[string]map[int64]string
}

type tangledRepo struct {
	URI   string `json:"uri"`
	Value struct {
		Name    string `json:"name"`
		Knot    string `json:"knot"`
		RepoDID string `json:"repoDid"`
	} `json:"value"`
}

type tangledArtifact struct {
	URI   string `json:"uri"`
	Value struct {
		Name string `json:"name"`
		Tag  struct {
			Bytes string `json:"$bytes"`
		} `json:"tag"`
		CreatedAt time.Time `json:"createdAt"`
		Artifact  struct {
			Size *int `json:"size"`
			Ref  struct {
				Link string `json:"$link"`
			} `json:"ref"`
		} `json:"artifact"`
	} `json:"value"`
}

type tangledTag struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
	Tag  *struct {
		Tagger struct {
			When time.Time `json:"When"`
		} `json:"Tagger"`
	} `json:"tag"`
}

func newTangled(client *forgeHTTP) *tangledSource {
	return &tangledSource{http: client, identity: "https://public.api.bsky.app", appview: "https://tangled.org", repos: map[string]tangledRepo{}, releases: map[string]map[int64][]*Asset{}, downloads: map[string]map[int64]string{}}
}

func (s *tangledSource) VerifyRepository(ctx context.Context, repo string) error {
	delete(s.repos, repo)
	owner, name := splitRepository(repo)
	did := owner
	if !strings.HasPrefix(did, "did:") {
		var identity struct {
			DID string `json:"did"`
		}
		_, err := s.http.json(ctx, endpoint(s.identity, "/xrpc/com.atproto.identity.resolveHandle", url.Values{"handle": {owner}}), &identity)
		if err != nil {
			return err
		}
		if !tangledDID.MatchString(identity.DID) {
			return errors.New("the Tangled handle has an invalid DID")
		}
		did = identity.DID
	}
	seen := map[string]bool{}
	for cursor := ""; ; {
		query := url.Values{"subject": {did}, "limit": {"100"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var data struct {
			Items  []tangledRepo `json:"items"`
			Cursor string        `json:"cursor"`
		}
		_, err := s.http.json(ctx, endpoint(s.http.base, "/xrpc/sh.tangled.repo.listRepos", query), &data)
		if err != nil {
			return err
		}
		for _, item := range data.Items {
			prefix := "at://" + did + "/sh.tangled.repo/"
			key := strings.TrimPrefix(item.URI, prefix)
			displayName := item.Value.Name
			if displayName == "" && strings.HasPrefix(item.URI, prefix) {
				displayName = key
			}
			if displayName != name {
				continue
			}
			if !strings.HasPrefix(item.URI, prefix) || !repoPart.MatchString(key) || key == "." || key == ".." {
				return errors.New("invalid Tangled repository URI")
			}
			if err := s.http.validateURL("https://" + item.Value.Knot); err != nil {
				return errors.New("invalid Tangled knot hostname")
			}
			// A knot field must be a hostname, not a URL or a path.
			if strings.ContainsAny(item.Value.Knot, "/?#@") {
				return errors.New("invalid Tangled knot hostname")
			}
			s.repos[repo] = item
			return nil
		}
		if data.Cursor == "" {
			return errors.New("the Tangled repository was not found")
		}
		if seen[data.Cursor] {
			return errors.New("the Tangled repository cursor did not advance")
		}
		seen[data.Cursor] = true
		cursor = data.Cursor
	}
}

func (s *tangledSource) artifacts(ctx context.Context, record tangledRepo) ([]tangledArtifact, error) {
	subject := record.Value.RepoDID
	if subject == "" {
		subject = record.URI
	}
	result := []tangledArtifact{}
	seen := map[string]bool{}
	for cursor := ""; ; {
		query := url.Values{"subject": {subject}, "limit": {"100"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var data struct {
			Items  []tangledArtifact `json:"items"`
			Cursor string            `json:"cursor"`
		}
		_, err := s.http.json(ctx, endpoint(s.http.base, "/xrpc/sh.tangled.repo.listArtifacts", query), &data)
		if err != nil {
			return nil, err
		}
		result = append(result, data.Items...)
		if data.Cursor == "" {
			return result, nil
		}
		if seen[data.Cursor] {
			return nil, errors.New("the Tangled artifact cursor did not advance")
		}
		seen[data.Cursor] = true
		cursor = data.Cursor
	}
}

func (s *tangledSource) Releases(ctx context.Context, repo string) ([]*Release, error) {
	record, ok := s.repos[repo]
	if !ok {
		return nil, errors.New("verify the Tangled repository before listing releases")
	}
	artifacts, err := s.artifacts(ctx, record)
	if err != nil {
		return nil, err
	}
	byHash := map[string][]tangledArtifact{}
	for _, artifact := range artifacts {
		hash, err := base64.StdEncoding.DecodeString(artifact.Value.Tag.Bytes)
		if err != nil || len(hash) != 20 {
			return nil, errors.New("invalid Tangled artifact tag hash")
		}
		byHash[hex.EncodeToString(hash)] = append(byHash[hex.EncodeToString(hash)], artifact)
	}
	s.releases[repo] = map[int64][]*Asset{}
	s.downloads[repo] = map[int64]string{}
	owner, key, _ := strings.Cut(strings.TrimPrefix(record.URI, "at://"), "/sh.tangled.repo/")
	knotRepo := owner + "/" + key
	result := []*Release{}
	seen := map[string]bool{}
	for offset := 0; ; offset += 100 {
		var data struct {
			Tags []tangledTag `json:"tags"`
		}
		query := url.Values{"repo": {knotRepo}, "limit": {"100"}, "cursor": {strconv.Itoa(offset)}}
		_, err := s.http.json(ctx, endpoint("https://"+record.Value.Knot, "/xrpc/sh.tangled.repo.tags", query), &data)
		if err != nil {
			return nil, err
		}
		for _, tag := range data.Tags {
			tagKey := tag.Name + ":" + tag.Hash
			if seen[tagKey] {
				return nil, errors.New("the Tangled tag page did not advance")
			}
			seen[tagKey] = true
			// Only annotated tags can have artifacts. Tags without artifacts are not releases.
			if tag.Tag == nil || len(byHash[tag.Hash]) == 0 {
				continue
			}
			id := stableID(tag.Hash + ":" + tag.Name)
			release := &Release{ID: id, TagName: tag.Name}
			if !tag.Tag.Tagger.When.IsZero() {
				release.PublishedAt = &Timestamp{tag.Tag.Tagger.When}
			}
			assets := []*Asset{}
			for _, artifact := range byHash[tag.Hash] {
				a := artifact.Value
				assetID := stableID(artifact.URI + ":" + a.Artifact.Ref.Link)
				state := "uploaded"
				asset := &Asset{ID: &assetID, Name: &a.Name, Size: a.Artifact.Size, State: &state, UpdatedAt: &Timestamp{a.CreatedAt}}
				assets = append(assets, asset)
				owner, name := splitRepository(repo)
				s.downloads[repo][assetID] = endpoint(s.appview, "/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/tags/"+url.PathEscape(tag.Name)+"/download/"+url.PathEscape(a.Name), nil)
			}
			s.releases[repo][id] = assets
			result = append(result, release)
		}
		if len(data.Tags) < 100 {
			return result, nil
		}
	}
}

func (s *tangledSource) Assets(_ context.Context, repo string, id int64) ([]*Asset, error) {
	assets, ok := s.releases[repo][id]
	if !ok {
		return nil, errors.New("unknown Tangled release ID")
	}
	return assets, nil
}
func (s *tangledSource) Download(ctx context.Context, repo string, id int64) (io.ReadCloser, error) {
	raw, ok := s.downloads[repo][id]
	if !ok {
		return nil, errors.New("unknown Tangled artifact ID")
	}
	response, err := s.http.request(ctx, raw)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
