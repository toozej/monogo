package releases

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testTangledDID = "did:plc:wshs7t2adsemcrrd4snkeqli"
const testRepoDID = "did:plc:j5hmlfdrwkvtxm7cjmu7j2is"

func tangledFixture(t *testing.T, override func(http.ResponseWriter, *http.Request) bool) *tangledSource {
	t.Helper()
	body := "package bytes"
	checksum := hashText(t, "sha256", body) + "  App-amd64.AppImage\n"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Private-Token") != "" {
			t.Error("Tangled public request contained credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		if override != nil && override(w, r) {
			return
		}
		switch r.URL.Path {
		case "/xrpc/com.atproto.identity.resolveHandle":
			if r.URL.Query().Get("handle") != "user.tngl.sh" {
				t.Error(r.URL)
			}
			_, _ = fmt.Fprintf(w, `{"did":%q}`, testTangledDID)
		case "/xrpc/sh.tangled.repo.listRepos":
			if r.URL.Query().Get("subject") != testTangledDID {
				t.Error(r.URL)
			}
			if r.URL.Query().Get("cursor") == "" {
				_, _ = fmt.Fprint(w, `{"items":[{"uri":"ignored","value":{"name":"other"}}],"cursor":"next"}`)
			} else {
				_, _ = fmt.Fprintf(w, `{"items":[{"uri":%q,"value":{"name":"Repo","knot":%q,"repoDid":%q}}]}`, "at://"+testTangledDID+"/sh.tangled.repo/rkey", r.Host, testRepoDID)
			}
		case "/xrpc/sh.tangled.repo.listArtifacts":
			if subject := r.URL.Query().Get("subject"); subject != testRepoDID && subject != "at://"+testTangledDID+"/sh.tangled.repo/rkey" {
				t.Error(r.URL)
			}
			items := []tangledArtifact{}
			for _, version := range []string{"1", "2", "3"} {
				name, content := "App-amd64.AppImage", body
				if r.URL.Query().Get("cursor") != "" {
					name, content = "checksums.txt", checksum
				}
				hash, _ := hex.DecodeString(strings.Repeat(version, 40))
				var item tangledArtifact
				item.URI = "at://" + testTangledDID + "/sh.tangled.repo.artifact/" + version + name
				item.Value.Name = name
				item.Value.Tag.Bytes = base64.StdEncoding.EncodeToString(hash)
				item.Value.CreatedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				size := len(content)
				item.Value.Artifact.Size = &size
				item.Value.Artifact.Ref.Link = "blob" + version + name
				items = append(items, item)
			}
			cursor := ""
			if r.URL.Query().Get("cursor") == "" {
				cursor = "artifacts-next"
			}
			_ = json.NewEncoder(w).Encode(struct {
				Items  []tangledArtifact `json:"items"`
				Cursor string            `json:"cursor,omitempty"`
			}{items, cursor})
		case "/xrpc/sh.tangled.repo.tags":
			if r.URL.Query().Get("repo") != testTangledDID+"/rkey" || r.URL.Query().Get("limit") != "100" {
				t.Error(r.URL)
			}
			var tags []map[string]any
			if r.URL.Query().Get("cursor") == "0" {
				tags = append(tags, map[string]any{"name": "v1", "hash": strings.Repeat("1", 40), "tag": map[string]any{"Tagger": map[string]any{"When": "2025-01-01T00:00:00Z"}}})
				for i := 1; i < 100; i++ {
					tags = append(tags, map[string]any{"name": fmt.Sprintf("lightweight-%d", i), "hash": strings.Repeat("0", 40)})
				}
			} else {
				tags = append(tags, map[string]any{"name": "v2", "hash": strings.Repeat("2", 40), "tag": map[string]any{"Tagger": map[string]any{"When": "2025-02-01T00:00:00Z"}}})
				tags = append(tags, map[string]any{"name": "v3-beta", "hash": strings.Repeat("3", 40), "tag": map[string]any{"Tagger": map[string]any{"When": "2025-03-01T00:00:00Z"}}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tags": tags})
		default:
			if strings.Contains(r.URL.Path, "/tags/v2/download/") {
				if strings.HasSuffix(r.URL.Path, "checksums.txt") {
					_, _ = fmt.Fprint(w, checksum)
				} else {
					_, _ = fmt.Fprint(w, body)
				}
			} else {
				t.Errorf("unexpected request %s", r.URL)
				http.NotFound(w, r)
			}
		}
	}))
	t.Cleanup(server.Close)
	client := newForgeHTTP(server.URL, "", "", 1)
	client.client.Transport = server.Client().Transport
	client.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	source := newTangled(client)
	source.identity = server.URL
	source.appview = server.URL
	return source
}

func TestTangledRunner(t *testing.T) {
	for _, owner := range []string{"user.tngl.sh", testTangledDID} {
		t.Run(owner, func(t *testing.T) {
			source := tangledFixture(t, nil)
			repo := owner + "/Repo"
			c := Defaults()
			c.Directory = t.TempDir()
			c.Keep = 1
			c.AllAssets = true
			c.Repositories = []Repository{{Name: "tangled.sh/" + repo}}
			runner := Runner{Source: Sources{"tangled.sh": source}}
			if err := runner.Run(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(c.Directory, "tangled.sh", component(owner), "Repo", "v2", "App-amd64.AppImage"))
			if err != nil || string(data) != "package bytes" {
				t.Fatal(string(data), err)
			}
			list, err := source.Releases(context.Background(), repo)
			if err != nil || len(list) != 3 || len(latest(list, 3)) != 2 {
				t.Fatal(list, err)
			}
			ids := map[string]int64{}
			for _, r := range list {
				ids[r.TagName] = r.ID
			}
			again, err := source.Releases(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range again {
				if ids[r.TagName] != r.ID {
					t.Fatal("release IDs changed")
				}
			}
			assets, err := source.Assets(context.Background(), repo, ids["v2"])
			if err != nil || len(assets) != 2 {
				t.Fatal(assets, err)
			}
			stream, err := source.Download(context.Background(), repo, assets[0].GetID())
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, stream)
			_ = stream.Close()
			if _, err := source.Assets(context.Background(), repo, -1); err == nil {
				t.Fatal("accepted unknown release")
			}
			if _, err := source.Download(context.Background(), "other/repo", assets[0].GetID()); err == nil {
				t.Fatal("asset ID crossed repositories")
			}
			if _, err := source.Download(context.Background(), repo, -1); err == nil {
				t.Fatal("accepted unknown asset")
			}
			record := source.repos[repo]
			record.Value.RepoDID = ""
			source.repos[repo] = record
			if _, err := source.Releases(context.Background(), repo); err != nil {
				t.Fatal("legacy repository failed", err)
			}
		})
	}
}

func TestTangledVerificationErrors(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
	}{
		{"identity error", "/xrpc/com.atproto.identity.resolveHandle", "", 404},
		{"identity JSON", "/xrpc/com.atproto.identity.resolveHandle", "{", 200},
		{"invalid DID", "/xrpc/com.atproto.identity.resolveHandle", `{"did":"not-a-did"}`, 200},
		{"repo error", "/xrpc/sh.tangled.repo.listRepos", "", 404},
		{"repo not found", "/xrpc/sh.tangled.repo.listRepos", `{"items":[]}`, 200},
		{"repo cursor", "/xrpc/sh.tangled.repo.listRepos", `{"items":[],"cursor":"same"}`, 200},
		{"bad URI", "/xrpc/sh.tangled.repo.listRepos", `{"items":[{"uri":"https://evil.test/","value":{"name":"Repo","knot":"knot.tangled.sh"}}]}`, 200},
		{"wrong owner", "/xrpc/sh.tangled.repo.listRepos", `{"items":[{"uri":"at://did:plc:wrong/sh.tangled.repo/key","value":{"name":"Repo","knot":"knot.tangled.sh"}}]}`, 200},
		{"bad knot", "/xrpc/sh.tangled.repo.listRepos", `{"items":[{"uri":"at://` + testTangledDID + `/sh.tangled.repo/key","value":{"name":"Repo","knot":"user@knot.tangled.sh"}}]}`, 200},
		{"knot path", "/xrpc/sh.tangled.repo.listRepos", `{"items":[{"uri":"at://` + testTangledDID + `/sh.tangled.repo/key","value":{"name":"Repo","knot":"knot.tangled.sh/path"}}]}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tangledFixture(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != tc.path {
					return false
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
				return true
			})
			if err := source.VerifyRepository(context.Background(), "user.tngl.sh/Repo"); err == nil {
				t.Fatal("accepted invalid Tangled repository")
			}
		})
	}
}

func TestTangledReleaseErrors(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
	}{
		{"artifact status", "/xrpc/sh.tangled.repo.listArtifacts", "", 404},
		{"artifact JSON", "/xrpc/sh.tangled.repo.listArtifacts", "{", 200},
		{"artifact cursor", "/xrpc/sh.tangled.repo.listArtifacts", `{"items":[],"cursor":"same"}`, 200},
		{"invalid base64", "/xrpc/sh.tangled.repo.listArtifacts", `{"items":[{"value":{"tag":{"$bytes":"%"}}}]}`, 200},
		{"invalid hash length", "/xrpc/sh.tangled.repo.listArtifacts", `{"items":[{"value":{"tag":{"$bytes":"YWJj"}}}]}`, 200},
		{"tag status", "/xrpc/sh.tangled.repo.tags", "", 404},
		{"tag JSON", "/xrpc/sh.tangled.repo.tags", "{", 200},
		{"duplicate tags", "/xrpc/sh.tangled.repo.tags", `{"tags":[{"name":"v1","hash":"duplicate"},{"name":"v1","hash":"duplicate"}]}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tangledFixture(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != tc.path {
					return false
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
				return true
			})
			if _, err := source.Releases(context.Background(), "user.tngl.sh/Repo"); err == nil {
				t.Fatal("accepted unverified repository")
			}
			if err := source.VerifyRepository(context.Background(), "user.tngl.sh/Repo"); err != nil {
				t.Fatal(err)
			}
			if _, err := source.Releases(context.Background(), "user.tngl.sh/Repo"); err == nil {
				t.Fatal("accepted invalid Tangled releases")
			}
		})
	}
	source := tangledFixture(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/download/") {
			w.WriteHeader(404)
			return true
		}
		return false
	})
	repo := "user.tngl.sh/Repo"
	if err := source.VerifyRepository(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	releases, err := source.Releases(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := source.Assets(context.Background(), repo, releases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Download(context.Background(), repo, assets[0].GetID()); err == nil {
		t.Fatal("accepted failed artifact download")
	}
}
