package releases

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func restFixture(t *testing.T, gitlab bool) *restSource {
	t.Helper()
	body := "package bytes"
	checksum := hashText(t, "sha256", body) + "  App-amd64.AppImage\n"
	client := testForgeHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "token test" {
			t.Error("request omitted API credentials")
		}
		path := r.URL.EscapedPath()
		base := "/repos/Owner/Repo"
		if gitlab {
			base = "/projects/Group%2FSubgroup%2FRepo"
		}
		assetJSON := func(id int, name, content string) string {
			raw := "http://" + r.Host + "/download/" + strconv.Itoa(id)
			if gitlab {
				return fmt.Sprintf(`{"id":%d,"name":%q,"url":"https://unused.test/file","direct_asset_url":%q}`, id, name, raw)
			}
			return fmt.Sprintf(`{"id":%d,"name":%q,"size":%d,"browser_download_url":%q,"created_at":"2025-01-01T00:00:00Z"}`, id, name, len(content), raw)
		}
		switch path {
		case base:
			_, _ = fmt.Fprint(w, `{"id":1}`)
		case base + "/releases":
			page := r.URL.Query().Get("page")
			key := "limit"
			if gitlab {
				key = "per_page"
			}
			if r.URL.Query().Get(key) != "100" {
				t.Error("incorrect page limit")
			}
			if page == "1" {
				w.Header().Set("Link", `<https://evil.test/?page=2>; rel="next"`)
			}
			if gitlab {
				if page == "1" {
					_, _ = fmt.Fprintf(w, `[{"tag_name":"v2","name":"App v2","released_at":"2025-02-02T00:00:00Z","assets":{"links":[%s,%s]}}]`, assetJSON(1, "App-amd64.AppImage", body), assetJSON(2, "checksums.txt", checksum))
				} else {
					_, _ = fmt.Fprint(w, `[{"tag_name":"v1","released_at":"2025-01-01T00:00:00Z","assets":{"links":[]}},{"tag_name":"v9-beta","released_at":"2025-03-01T00:00:00Z","assets":{"links":[]}},{"tag_name":"v10","released_at":"2999-01-01T00:00:00Z","assets":{"links":[]}}]`)
				}
			} else {
				if page == "1" {
					_, _ = fmt.Fprint(w, `[{"id":2,"tag_name":"v2","name":"App v2","published_at":"2025-02-02T00:00:00Z","assets":[]}]`)
				} else {
					_, _ = fmt.Fprint(w, `[{"id":1,"tag_name":"v1","published_at":"2025-01-01T00:00:00Z"},{"id":3,"tag_name":"v9-beta","published_at":"2025-03-01T00:00:00Z"},{"id":4,"tag_name":"v10","draft":true,"published_at":"2025-04-01T00:00:00Z"},{"id":5,"tag_name":"v11","prerelease":true,"published_at":"2025-04-01T00:00:00Z"},{"id":6,"tag_name":"unpublished"}]`)
				}
			}
		case base + "/releases/2/assets":
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `<https://evil.test/?page=2>; rel="next"`)
				_, _ = fmt.Fprintf(w, `[%s]`, assetJSON(1, "App-amd64.AppImage", body))
			} else {
				_, _ = fmt.Fprintf(w, `[%s]`, assetJSON(2, "checksums.txt", checksum))
			}
		case "/download/1":
			_, _ = fmt.Fprint(w, body)
		case "/download/2":
			_, _ = fmt.Fprint(w, checksum)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	})
	return newREST(client, gitlab)
}

func TestRESTForgeRunner(t *testing.T) {
	for _, gitlab := range []bool{true, false} {
		name, repo, path := "codeberg.org", "Owner/Repo", filepath.Join("Owner", "Repo")
		if gitlab {
			name = "gitlab.com"
			repo = "Group/Subgroup/Repo"
			path = filepath.Join("Group", "Subgroup", "Repo")
		}
		t.Run(name, func(t *testing.T) {
			source := restFixture(t, gitlab)
			c := Defaults()
			c.Directory = t.TempDir()
			c.Keep = 1
			c.AllAssets = true
			c.Repositories = []Repository{{Name: name + "/" + repo}}
			runner := &Runner{Source: Sources{name: source}}
			for i := 0; i < 2; i++ {
				if err := runner.Run(context.Background(), c); err != nil {
					t.Fatal(err)
				}
				filename := filepath.Join(c.Directory, name, path, "v2", "App-amd64.AppImage")
				data, err := os.ReadFile(filename)
				if err != nil || string(data) != "package bytes" {
					t.Fatal(string(data), err)
				}
			}
			list, err := source.Releases(context.Background(), repo)
			if err != nil || len(latest(list, 3)) != 2 {
				t.Fatal(list, err)
			}
			assets, err := source.Assets(context.Background(), repo, list[0].ID)
			if err != nil || len(assets) != 2 || assets[0].GetState() != "uploaded" {
				t.Fatal(assets, err)
			}
			if gitlab && assets[0].Size != nil {
				t.Fatal("GitLab fabricated an asset size")
			}
			if !gitlab && assets[0].UpdatedAt == nil {
				t.Fatal("Forgejo asset omitted its timestamp")
			}
			if _, err := source.Download(context.Background(), "other/repo", assets[0].GetID()); err == nil {
				t.Fatal("asset ID crossed repositories")
			}
			if _, err := source.Download(context.Background(), repo, -1); err == nil {
				t.Fatal("accepted unknown asset")
			}
			if gitlab {
				if _, err := source.Assets(context.Background(), repo, -1); err == nil {
					t.Fatal("accepted unknown release")
				}
			}
		})
	}
}

func TestRESTErrors(t *testing.T) {
	for _, gitlab := range []bool{true, false} {
		for _, mode := range []string{"status", "json", "pagination", "asset URL"} {
			t.Run(fmt.Sprintf("%t/%s", gitlab, mode), func(t *testing.T) {
				client := testForgeHTTP(t, func(w http.ResponseWriter, r *http.Request) {
					switch mode {
					case "status":
						w.WriteHeader(404)
					case "json":
						_, _ = fmt.Fprint(w, `{`)
					case "pagination":
						w.Header().Set("X-Next-Page", "1")
						_, _ = fmt.Fprint(w, `[]`)
					case "asset URL":
						if gitlab {
							_, _ = fmt.Fprint(w, `[{"tag_name":"v1","released_at":"2025-01-01T00:00:00Z","assets":{"links":[{"name":"asset","url":"http://user:pass@evil.test/file"}]}}]`)
						} else {
							_, _ = fmt.Fprint(w, `[{"id":1,"name":"asset","browser_download_url":"file:///tmp/secret"}]`)
						}
					}
				})
				source := newREST(client, gitlab)
				list, err := source.Releases(context.Background(), "owner/repo")
				if mode != "asset URL" {
					if err == nil {
						t.Fatal("accepted invalid releases")
					}
				} else if gitlab {
					if err != nil {
						t.Fatal(err)
					}
					if _, err := source.Assets(context.Background(), "owner/repo", list[0].ID); err == nil {
						t.Fatal("accepted invalid asset URL")
					}
				}
				if !gitlab {
					if _, err := source.Assets(context.Background(), "owner/repo", 1); err == nil {
						t.Fatal("accepted invalid assets")
					}
				}
				if mode == "status" || mode == "json" {
					if err := source.VerifyRepository(context.Background(), "owner/repo"); err == nil {
						t.Fatal("accepted invalid repository response")
					}
				}
			})
		}
	}
	client := testForgeHTTP(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	source := newGitLab(client)
	source.downloads["owner/repo"] = map[int64]string{1: client.base + "/missing"}
	if _, err := source.Download(context.Background(), "owner/repo", 1); err == nil {
		t.Fatal("accepted failed download")
	}
	for _, gitlab := range []bool{true, false} {
		source := newREST(client, gitlab)
		assets, err := source.convertAssets("owner/repo", []restAsset{{ID: 1, Name: "asset", URL: client.base + "/file", BrowserURL: client.base + "/file"}}, gitlab)
		if err != nil || len(assets) != 1 {
			t.Fatal(assets, err)
		}
	}
}

func TestUnknownAssetSize(t *testing.T) {
	runner, source, c := setupRunner(t)
	for _, assets := range source.assets {
		for _, asset := range assets {
			asset.Size = nil
		}
	}
	if err := runner.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(c.Directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	manifest, err := readManifest(root, filepath.Join("owner", "repo", "v3"))
	if err != nil || manifest.Assets[0].Size != -1 {
		t.Fatal(manifest, err)
	}
	before := source.calls[3]
	if err := runner.Run(context.Background(), c); err != nil || source.calls[3] != before {
		t.Fatal(source.calls, err)
	}
	if err := os.WriteFile(filepath.Join(storedPath(c, "v3"), "App-amd64.AppImage"), []byte("corrupt cache with different size"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(context.Background(), c); err != nil || source.calls[3] != before+1 {
		t.Fatal(source.calls, err)
	}
	body := strings.Repeat("a", 8*1024*1024+1)
	checksum := assetFixture(9, "checksums.txt", body)
	checksum.Size = nil
	source.assets[3] = append(source.assets[3], checksum)
	source.data[9] = body
	if err := runner.Run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "asset exceeds download limit") {
		t.Fatal("accepted oversized checksum", err)
	}
}

func TestSourceRouting(t *testing.T) {
	source := &fakeSource{data: map[int64]string{1: "data"}, calls: map[int64]int{}}
	sources := Sources{"gitlab.com": source, "github.com": source, "codeberg.org": source, "tangled.sh": source}
	for _, repo := range []string{"owner/repo", "gitlab.com/a/b/c", "codeberg.org/a/b", "tangled.org/a/b"} {
		if err := sources.VerifyRepository(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		if _, err := sources.Releases(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		if _, err := sources.Assets(context.Background(), repo, 1); err != nil {
			t.Fatal(err)
		}
		body, err := sources.Download(context.Background(), repo, 1)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, body)
		_ = body.Close()
	}
	for _, repo := range []string{"invalid", "https://evil.test/a/b", "gitlab.com/a/b"} {
		empty := Sources{}
		if err := empty.VerifyRepository(context.Background(), repo); err == nil {
			t.Fatal("accepted missing source")
		}
		if _, err := empty.Releases(context.Background(), repo); err == nil {
			t.Fatal("accepted missing source")
		}
		if _, err := empty.Assets(context.Background(), repo, 1); err == nil {
			t.Fatal("accepted missing source")
		}
		if _, err := empty.Download(context.Background(), repo, 1); err == nil {
			t.Fatal("accepted missing source")
		}
	}
}

func TestMixedForgeNamespacesAndFailures(t *testing.T) {
	sources := Sources{}
	repositories := []Repository{{Name: "Owner/Repo"}, {Name: "gitlab.com/Owner/Repo"}, {Name: "codeberg.org/Owner/Repo"}, {Name: "tangled.sh/Owner/Repo"}}
	c := Defaults()
	c.Directory = t.TempDir()
	c.Keep = 1
	c.AllAssets = true
	c.Repositories = repositories
	for _, repo := range repositories {
		_, f, _ := setupRunner(t)
		host, _, err := repositoryParts(repo.Name)
		if err != nil {
			t.Fatal(err)
		}
		body := host + " bytes"
		f.data[3] = body
		f.assets[3] = []*Asset{assetFixture(3, "App-amd64.AppImage", body)}
		sources[host] = f
	}
	runner := Runner{Source: sources}
	if err := runner.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	for _, repo := range repositories {
		normalized, err := NormalizeRepository(repo.Name)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(strings.Split(normalized, "/")...)
		host, _, _ := repositoryParts(repo.Name)
		data, err := os.ReadFile(filepath.Join(c.Directory, path, "v3", "App-amd64.AppImage"))
		if err != nil || string(data) != host+" bytes" {
			t.Fatal(string(data), err)
		}
	}
	sources["gitlab.com"].(*fakeSource).listErr = fmt.Errorf("GitLab is unavailable")
	for host, source := range sources {
		f := source.(*fakeSource)
		body := host + " newer bytes"
		f.releases = append(f.releases, releaseFixture(4, "v4", 4))
		f.assets[4] = []*Asset{assetFixture(4, "App-amd64.AppImage", body)}
		f.data[4] = body
	}
	if err := runner.Run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "GitLab is unavailable") {
		t.Fatal(err)
	}
	for _, host := range []string{"github.com", "gitlab.com", "codeberg.org", "tangled.sh"} {
		path := filepath.Join(c.Directory, host, "Owner", "Repo")
		if host == "github.com" {
			path = filepath.Join(c.Directory, "owner", "repo")
		}
		_, oldErr := os.Stat(filepath.Join(path, "v3"))
		_, newErr := os.Stat(filepath.Join(path, "v4"))
		if host == "gitlab.com" {
			if oldErr != nil || !os.IsNotExist(newErr) {
				t.Fatal(oldErr, newErr)
			}
		} else if !os.IsNotExist(oldErr) || newErr != nil {
			t.Fatal(oldErr, newErr)
		}
	}
}

func TestUnknownSizeWithoutChecksumRefreshes(t *testing.T) {
	runner, source, c := setupRunner(t)
	for _, assets := range source.assets {
		for _, asset := range assets {
			asset.Size = nil
			asset.Digest = nil
		}
	}
	if err := runner.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	before := source.calls[3]
	source.data[3] = "changed upstream package bytes"
	if err := runner.Run(context.Background(), c); err != nil || source.calls[3] != before+1 {
		t.Fatal(source.calls, err)
	}
	data, err := os.ReadFile(filepath.Join(storedPath(c, "v3"), "App-amd64.AppImage"))
	if err != nil || string(data) != source.data[3] {
		t.Fatal(string(data), err)
	}
}
