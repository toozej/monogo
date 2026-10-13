package releases

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
)

type fakeSource struct {
	releases  []*Release
	assets    map[int64][]*Asset
	data      map[int64]string
	failures  map[int64]int
	calls     map[int64]int
	verifyErr map[string]error
	listErr   error
	assetsErr error
}

func (f *fakeSource) VerifyRepository(_ context.Context, repo string) error { return f.verifyErr[repo] }
func (f *fakeSource) Releases(context.Context, string) ([]*Release, error) {
	return f.releases, f.listErr
}
func (f *fakeSource) Assets(_ context.Context, _ string, id int64) ([]*Asset, error) {
	return f.assets[id], f.assetsErr
}
func (f *fakeSource) Download(_ context.Context, _ string, id int64) (io.ReadCloser, error) {
	f.calls[id]++
	if f.failures[id] > 0 {
		f.failures[id]--
		return nil, errors.New("download failed")
	}
	return io.NopCloser(strings.NewReader(f.data[id])), nil
}

func assetFixture(id int64, name, body string) *Asset {
	return &Asset{ID: github.Ptr(id), Name: github.Ptr(name), Size: github.Ptr(len(body)), State: github.Ptr("uploaded")}
}

func setupRunner(t *testing.T) (*Runner, *fakeSource, Config) {
	t.Helper()
	c := Defaults()
	c.Directory = t.TempDir()
	c.Keep = 2
	c.Targets = []Target{{OS: "linux", Arch: "amd64", Formats: []string{"appimage"}}}
	c.Repositories = []Repository{{Name: "owner/repo"}}
	f := &fakeSource{assets: map[int64][]*Asset{}, data: map[int64]string{}, failures: map[int64]int{}, calls: map[int64]int{}, verifyErr: map[string]error{}}
	for id := int64(1); id <= 3; id++ {
		body := fmt.Sprintf("package %d", id)
		a := assetFixture(id, "App-amd64.AppImage", body)
		a.Digest = github.Ptr("sha256:" + hashText(t, "sha256", body))
		f.releases = append(f.releases, releaseFixture(id, fmt.Sprintf("v%d", id), id))
		f.assets[id] = []*Asset{a}
		f.data[id] = body
	}
	return &Runner{Source: f, Log: &bytes.Buffer{}, Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}, f, c
}

func storedPath(c Config, tag string) string {
	return filepath.Join(c.Directory, filepath.Join("owner", "repo"), component(tag))
}
func requireExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
func requireMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path remains: %s (%v)", path, err)
	}
}

func TestRunRetentionAndCache(t *testing.T) {
	r, f, c := setupRunner(t)
	c.Keep = 3
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"v1", "v2", "v3"} {
		requireExists(t, filepath.Join(storedPath(c, tag), manifestName))
	}
	unrelated := filepath.Join(c.Directory, filepath.Join("owner", "repo"), "personal-notes")
	if err := os.Mkdir(unrelated, 0700); err != nil {
		t.Fatal(err)
	}
	c.Keep = 2
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	requireMissing(t, storedPath(c, "v1"))
	requireExists(t, unrelated)
	if f.calls[2] != 1 || f.calls[3] != 1 {
		t.Fatal("cache downloaded valid assets again", f.calls)
	}
	if err := os.WriteFile(filepath.Join(storedPath(c, "v3"), "App-amd64.AppImage"), []byte("bad bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if f.calls[3] != 2 {
		t.Fatal("cache did not repair corrupt bytes")
	}
	requireMissing(t, filepath.Join(c.Directory, ".go-get-releases.lock"))
}

func TestFailuresPreserveOlderReleases(t *testing.T) {
	cases := []string{"download", "digest", "checksum", "size", "missing", "require all", "list", "assets", "verify", "no stable", "unmanaged", "unsafe name", "duplicate name", "checksum too large", "unsupported digest"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			r, f, c := setupRunner(t)
			c.Keep = 3
			if err := r.Run(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			c.Keep = 2
			a := f.assets[3][0]
			switch name {
			case "download":
				f.failures[3] = 10
				a.UpdatedAt = &Timestamp{Time: time.Now()}
			case "digest":
				a.Digest = github.Ptr("sha256:" + strings.Repeat("0", 64))
			case "unsupported digest":
				a.Digest = github.Ptr("sha999:bad")
			case "checksum":
				body := strings.Repeat("0", 64) + "  App-amd64.AppImage\n"
				f.assets[3] = append(f.assets[3], assetFixture(30, "checksums.txt", body))
				f.data[30] = body
			case "size":
				a.Size = github.Ptr(100)
			case "missing":
				f.assets[3] = nil
			case "require all":
				c.RequireAll = true
				c.Targets = append(c.Targets, Defaults().Targets[0])
			case "list":
				f.listErr = errors.New("list failed")
			case "assets":
				f.assetsErr = errors.New("assets failed")
			case "verify":
				f.verifyErr["owner/repo"] = errors.New("not found")
			case "no stable":
				for _, release := range f.releases {
					release.Prerelease = true
				}
			case "unmanaged":
				if err := os.Remove(filepath.Join(storedPath(c, "v3"), manifestName)); err != nil {
					t.Fatal(err)
				}
			case "unsafe name":
				a.Name = github.Ptr("../App-amd64.AppImage")
			case "duplicate name":
				f.assets[3] = append(f.assets[3], a)
			case "checksum too large":
				checks := assetFixture(30, "checksums.txt", "")
				checks.Size = github.Ptr(9 * 1024 * 1024)
				f.assets[3] = append(f.assets[3], checks)
			}
			if err := r.Run(context.Background(), c); err == nil {
				t.Fatal("expected failure")
			}
			requireExists(t, storedPath(c, "v1"))
			if name == "download" && f.calls[3] != 4 {
				t.Fatal("incorrect retries", f.calls)
			}
			entries, err := os.ReadDir(filepath.Join(c.Directory, filepath.Join("owner", "repo")))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".download-") {
					t.Fatal("temporary download remains")
				}
			}
		})
	}
}

func TestRetryAndChecksums(t *testing.T) {
	r, f, c := setupRunner(t)
	f.failures[3] = 1
	body := hashText(t, "sha512", f.data[3]) + "  App-amd64.AppImage\n"
	f.assets[3] = append(f.assets[3], assetFixture(30, "SHA512SUMS", body))
	f.data[30] = body
	f.failures[30] = 1
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if f.calls[3] != 2 || f.calls[30] != 2 {
		t.Fatal("failed download did not retry", f.calls)
	}
	requireExists(t, filepath.Join(storedPath(c, "v3"), "SHA512SUMS"))
}

func TestNoUpstreamChecksumAndMissingTargets(t *testing.T) {
	r, f, c := setupRunner(t)
	f.assets[3][0].Digest = nil
	c.Targets = append(c.Targets, Defaults().Targets[0])
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	text := r.Log.(*bytes.Buffer).String()
	if !strings.Contains(text, "no upstream checksum") || !strings.Contains(text, "no asset for windows/amd64") {
		t.Fatal(text)
	}
}

func TestRepositoryIsolation(t *testing.T) {
	r, f, c := setupRunner(t)
	c.Repositories = append([]Repository{{Name: "bad/repo"}}, c.Repositories...)
	f.verifyErr["bad/repo"] = errors.New("not found")
	if err := r.Run(context.Background(), c); err == nil {
		t.Fatal("expected failure")
	}
	requireExists(t, storedPath(c, "v3"))
}

func TestRepositoryTargetOverride(t *testing.T) {
	r, _, c := setupRunner(t)
	c.Repositories[0].Targets = c.Targets
	c.Targets = Defaults().Targets[:1]
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	requireExists(t, storedPath(c, "v3"))
}

func TestLockAndCancellation(t *testing.T) {
	r, _, c := setupRunner(t)
	lock := filepath.Join(c.Directory, ".go-get-releases.lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		t.Fatal(err)
	}
	if err := r.Run(context.Background(), c); err == nil {
		t.Fatal("accepted overlapping run")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Run(ctx, c); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	requireMissing(t, lock)
	if err := r.retry(ctx, 1, func() error { t.Fatal("called after cancellation"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	r.Wait = func(context.Context, time.Duration) error { return context.Canceled }
	if err := r.retry(context.Background(), 1, func() error { return errors.New("failure") }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSafePaths(t *testing.T) {
	for _, name := range []string{"", "..", ".", "../file", "path/file", "path\\file", manifestName, "file\x00"} {
		if validAssetName(name) {
			t.Errorf("accepted %q", name)
		}
	}
	seen := map[string]bool{}
	for _, tag := range []string{"v1", "apps/tool/v1", "apps%2Ftool%2Fv1", "..", ".", ".download-x", "v1\\v2", "v1:build", "CON", "con.txt", "v1."} {
		got := component(tag)
		if strings.ContainsAny(got, "/\\:") || strings.HasPrefix(got, ".") || seen[got] {
			t.Fatalf("unsafe or colliding component: %s", got)
		}
		seen[got] = true
	}
}

func TestSymlinks(t *testing.T) {
	for _, level := range []string{"owner", "repository"} {
		t.Run(level, func(t *testing.T) {
			r, _, c := setupRunner(t)
			outside := t.TempDir()
			link := filepath.Join(c.Directory, "owner")
			if level == "repository" {
				if err := os.Mkdir(link, 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(link, "repo")
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			if err := r.Run(context.Background(), c); err == nil {
				t.Fatal("accepted a directory symlink")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal(entries, err)
			}
		})
	}
}

func TestStoreRecovery(t *testing.T) {
	r, _, c := setupRunner(t)
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	repoDir := filepath.Join("owner", "repo")
	backup := filepath.Join(c.Directory, repoDir, ".backup-v3")
	if err := os.Rename(storedPath(c, "v3"), backup); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(c.Directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := recoverSwap(root, repoDir, "owner/repo", "v3"); err != nil {
		t.Fatal(err)
	}
	requireExists(t, storedPath(c, "v3"))
	requireMissing(t, backup)
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	if err := recoverSwap(root, repoDir, "owner/repo", "v3"); err == nil {
		t.Fatal("accepted unmanaged backup")
	}
}

func TestInvalidRunner(t *testing.T) {
	c := Defaults()
	r := Runner{}
	if err := r.Run(context.Background(), c); err == nil {
		t.Fatal("accepted empty repos")
	}
	c.Repositories = []Repository{{Name: "owner/repo"}}
	if err := r.Run(context.Background(), c); err == nil {
		t.Fatal("accepted nil source")
	}
}

func TestReleaseFilterAndRetentionScope(t *testing.T) {
	r, f, c := setupRunner(t)
	c.Keep = 3
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	c.Keep = 1
	c.ReleaseMatch = `^v[12]$`
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	requireExists(t, storedPath(c, "v2"))
	requireExists(t, storedPath(c, "v3"))
	requireMissing(t, storedPath(c, "v1"))
	if f.calls[3] != 1 {
		t.Fatal("release outside the filter was downloaded")
	}
	c.ReleaseMatch = `^no-such-app/`
	if err := r.Run(context.Background(), c); err == nil {
		t.Fatal("expected no matching release error")
	}
	requireExists(t, storedPath(c, "v2"))
}

func TestRepositoryReleaseFilterOverride(t *testing.T) {
	r, _, c := setupRunner(t)
	c.Keep = 1
	c.ReleaseMatch = `^v3$`
	c.Repositories[0].ReleaseMatch = `^v1$`
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	requireExists(t, storedPath(c, "v1"))
	requireMissing(t, storedPath(c, "v3"))
}

func TestAllAssets(t *testing.T) {
	r, f, c := setupRunner(t)
	c.AllAssets = true
	c.Targets = nil
	c.Keep = 1
	for i, name := range []string{"pebble-darkroom.tar.gz", "package.aarch64.apk", "checksums.txt.sigstore.json", "package.sbom.json"} {
		id := int64(30 + i)
		body := "asset bytes"
		f.assets[3] = append(f.assets[3], assetFixture(id, name, body))
		f.data[id] = body
	}
	pending := assetFixture(40, "pending.zip", "pending")
	pending.State = github.Ptr("new")
	f.assets[3] = append(f.assets[3], pending)
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pebble-darkroom.tar.gz", "package.aarch64.apk", "checksums.txt.sigstore.json", "package.sbom.json"} {
		requireExists(t, filepath.Join(storedPath(c, "v3"), name))
	}
	if f.calls[40] != 0 {
		t.Fatal("downloaded an incomplete asset")
	}
}

func TestOwnerRepositoryLayout(t *testing.T) {
	r, _, c := setupRunner(t)
	c.Repositories = []Repository{{Name: "first/repo"}, {Name: "second/repo"}, {Name: "first/other"}}
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	for _, repo := range c.Repositories {
		owner, name := splitRepository(repo.Name)
		requireExists(t, filepath.Join(c.Directory, owner, name, "v3", manifestName))
		requireMissing(t, filepath.Join(c.Directory, component(repo.Name)))
	}
}
