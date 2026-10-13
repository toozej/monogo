package releases

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
)

// Runner downloads releases. A failure prevents retention cleanup for that repository.
type Runner struct {
	Source Source
	Log    io.Writer
	Wait   func(context.Context, time.Duration) error
}

func (r *Runner) log(format string, args ...any) {
	if r.Log != nil {
		_, _ = fmt.Fprintf(r.Log, format+"\n", args...)
	}
}

// Run processes repositories independently and returns all repository errors.
func (r *Runner) Run(ctx context.Context, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if r.Source == nil {
		return errors.New("release source must not be nil")
	}
	if r.Wait == nil {
		r.Wait = waitContext
	}
	if err := os.MkdirAll(c.Directory, 0750); err != nil {
		return err
	}
	root, err := os.OpenRoot(c.Directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	const lock = ".go-get-releases.lock"
	if err := root.Mkdir(lock, 0700); err != nil {
		return fmt.Errorf("cannot acquire directory lock (remove a stale lock after the process stops): %w", err)
	}
	defer func() { _ = root.Remove(lock) }()
	failures := []error{}
	for _, repo := range c.Repositories {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := r.repository(ctx, root, c, repo); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", repo.Name, err))
			r.log("%s: %v", repo.Name, err)
		}
	}
	return errors.Join(failures...)
}

func latest(releases []*github.RepositoryRelease, n int) []*github.RepositoryRelease {
	result := []*github.RepositoryRelease{}
	for _, release := range releases {
		if release != nil && stable(release) {
			result = append(result, release)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.PublishedAt.Time.Equal(b.PublishedAt.Time) {
			return a.GetID() > b.GetID()
		}
		return a.PublishedAt.After(b.PublishedAt.Time)
	})
	// Duplicate tags must not occupy two retention slots.
	unique := result[:0]
	seen := map[string]bool{}
	for _, release := range result {
		if !seen[release.GetTagName()] {
			unique = append(unique, release)
			seen[release.GetTagName()] = true
		}
	}
	return unique[:min(n, len(unique))]
}

func (r *Runner) repository(ctx context.Context, root *os.Root, c Config, repo Repository) error {
	if err := r.Source.VerifyRepository(ctx, repo.Name); err != nil {
		return fmt.Errorf("verify repository: %w", err)
	}
	releases, err := r.Source.Releases(ctx, repo.Name)
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}
	pattern := c.ReleaseMatch
	if repo.ReleaseMatch != "" {
		pattern = repo.ReleaseMatch
	}
	filter := regexp.MustCompile(pattern)
	matching := make([]*github.RepositoryRelease, 0, len(releases))
	for _, release := range releases {
		if release != nil && filter.MatchString(release.GetTagName()) {
			matching = append(matching, release)
		}
	}
	releases = latest(matching, c.Keep)
	if len(releases) == 0 {
		return errors.New("repository has no stable published releases")
	}
	targets := repo.Targets
	if len(targets) == 0 {
		targets = c.Targets
	}
	owner, name := splitRepository(repo.Name)
	ownerDir := component(owner)
	if err := directory(root, ownerDir); err != nil {
		return err
	}
	repoDir := filepath.Join(ownerDir, component(name))
	if err := directory(root, repoDir); err != nil {
		return err
	}
	keep := map[string]bool{}
	failures := []error{}
	for _, release := range releases {
		tag := release.GetTagName()
		keep[component(tag)] = true
		if err := recoverSwap(root, repoDir, repo.Name, tag); err != nil {
			return err
		}
		assets, err := r.Source.Assets(ctx, repo.Name, release.GetID())
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: list assets: %w", tag, err))
			continue
		}
		selected, missing := selectAssets(assets, targets)
		if c.AllAssets {
			selected = nil
			missing = nil
			for _, asset := range assets {
				if asset.GetState() == "uploaded" && !checksumAsset(asset.GetName()) {
					selected = append(selected, asset)
				}
			}
		}
		for _, t := range missing {
			r.log("%s %s: no asset for %s/%s", repo.Name, tag, t.OS, t.Arch)
		}
		if len(selected) == 0 || (c.RequireAll && len(missing) > 0) {
			failures = append(failures, fmt.Errorf("%s: release does not contain the required assets", tag))
			continue
		}
		if err := r.release(ctx, root, c, repo.Name, repoDir, release, assets, selected); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", tag, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	if len(releases) < c.Keep {
		r.log("%s: only %d stable releases exist", repo.Name, len(releases))
	}
	return prune(root, repoDir, repo.Name, keep, filter)
}

func (r *Runner) retry(ctx context.Context, retries int, fn func() error) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}
		if attempt >= retries || ctx.Err() != nil {
			return err
		}
		r.log("Download failed. Retry %d of %d", attempt+1, retries)
		if err := r.Wait(ctx, retryDelay(err, attempt)); err != nil {
			return err
		}
	}
}

func (r *Runner) download(ctx context.Context, root *os.Root, path, repo string, a *github.ReleaseAsset, expected []checksum, limit int64) (string, error) {
	var sum string
	body, err := r.Source.Download(ctx, repo, a.GetID())
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	f, err := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	size, copyErr := io.Copy(f, io.LimitReader(body, limit+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		return "", errors.Join(copyErr, closeErr)
	}
	if size > limit || size != int64(a.GetSize()) {
		return "", fmt.Errorf("asset size does not match for %s: received %d, expected %d", a.GetName(), size, a.GetSize())
	}
	input, err := root.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = input.Close() }()
	sum, err = verify(input, expected)
	return sum, err
}

func (r *Runner) release(ctx context.Context, root *os.Root, c Config, repo, repoDir string, release *github.RepositoryRelease, assets, selected []*github.ReleaseAsset) error {
	// Reject duplicate or unsafe filenames before the tool creates any asset files.
	names := map[string]bool{}
	for _, a := range assets {
		if !validAssetName(a.GetName()) || names[a.GetName()] {
			return fmt.Errorf("unsafe or duplicate asset filename: %q", a.GetName())
		}
		if a.GetSize() < 0 {
			return errors.New("asset size must not be negative")
		}
		names[a.GetName()] = true
	}
	stage := filepath.Join(repoDir, ".download-"+rand.Text())
	if err := root.Mkdir(stage, 0700); err != nil {
		return err
	}
	defer func() { _ = root.RemoveAll(stage) }()
	dest := filepath.Join(repoDir, component(release.GetTagName()))
	old, _ := readManifest(root, dest)
	expected := checksums{}
	records := []storedAsset{}
	for _, a := range assets {
		if a.GetState() != "uploaded" || !checksumAsset(a.GetName()) {
			continue
		}
		if a.GetSize() > 8*1024*1024 {
			return fmt.Errorf("checksum file exceeds 8 MiB: %s", a.GetName())
		}
		path := filepath.Join(stage, a.GetName())
		var sum string
		err := r.retry(ctx, c.Retries, func() error {
			_ = root.Remove(path)
			var err error
			digests, err := assetDigests(a)
			if err != nil {
				return err
			}
			sum, err = r.download(ctx, root, path, repo, a, digests, 8*1024*1024)
			return err
		})
		if err != nil {
			return fmt.Errorf("download checksum %s: %w", a.GetName(), err)
		}
		data, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := parseChecksums(a.GetName(), data)
		if err != nil {
			return err
		}
		for name, checks := range parsed {
			expected[name] = append(expected[name], checks...)
		}
		record := descriptor(a)
		record.SHA256 = sum
		records = append(records, record)
	}
	for _, a := range selected {
		digests, err := assetDigests(a)
		if err != nil {
			return err
		}
		digests = append(digests, expected[a.GetName()]...)
		if len(digests) == 0 {
			r.log("%s %s: no upstream checksum for %s; check size and store a local SHA-256", repo, release.GetTagName(), a.GetName())
		}
		path := filepath.Join(stage, a.GetName())
		var sum string
		err = r.retry(ctx, c.Retries, func() error {
			_ = root.Remove(path)
			if old.Repository == repo && old.ReleaseID == release.GetID() {
				for _, record := range old.Assets {
					if sameDescriptor(record, descriptor(a)) {
						cachedSum, err := copyCached(root, filepath.Join(dest, a.GetName()), path, a, digests, record.SHA256)
						if err == nil {
							sum = cachedSum
							return nil
						}
						_ = root.Remove(path)
					}
				}
			}
			var err error
			sum, err = r.download(ctx, root, path, repo, a, digests, int64(a.GetSize()))
			return err
		})
		if err != nil {
			return fmt.Errorf("download asset %s: %w", a.GetName(), err)
		}
		record := descriptor(a)
		record.SHA256 = sum
		records = append(records, record)
	}
	slices.SortFunc(records, func(a, b storedAsset) int { return strings.Compare(a.Name, b.Name) })
	m := manifest{Schema: 1, Repository: repo, ReleaseID: release.GetID(), Tag: release.GetTagName(), Assets: records}
	if err := writeManifest(root, stage, m); err != nil {
		return err
	}
	if err := publish(root, stage, dest, repo, release.GetTagName()); err != nil {
		return err
	}
	r.log("%s %s: stored %d assets", repo, release.GetTagName(), len(selected))
	return nil
}

func assetDigests(a *github.ReleaseAsset) ([]checksum, error) {
	if a.GetDigest() == "" {
		return nil, nil
	}
	c, err := parseDigest(a.GetDigest())
	if err != nil {
		return nil, err
	}
	return []checksum{c}, nil
}

func copyCached(root *os.Root, source, dest string, a *github.ReleaseAsset, expected []checksum, sum string) (string, error) {
	info, err := root.Lstat(source)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() != int64(a.GetSize()) || len(sum) != 64 {
		return "", fs.ErrInvalid
	}
	f, err := root.Open(source)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	expected = append(expected, checksum{"sha256", sum})
	// Check the cache before the tool copies it into the staged release.
	actual, err := verify(f, expected)
	if err != nil {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	out, err := root.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, f)
	closeErr := out.Close()
	return actual, errors.Join(copyErr, closeErr)
}
