package checkrunner

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/version"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

// DetectOutdatedFiltered applies optional release-age and Dependabot PR filters.
// API errors stop the filtered check instead of reporting a successful result.
func DetectOutdatedFiltered(rc *RunContext, files []*workflow.WorkflowFile, archived map[string]bool, repos []string) ([]actioninfo.OutdatedActionInfo, map[string]*github.ReleaseInfo, error) {
	opts := rc.OutdatedOptions
	if opts.MinReleaseAgeDays == 0 && !opts.IgnoreDependabotPRs {
		actions, releases := DetectOutdated(rc, files, archived, repos)
		return actions, releases, nil
	}
	var releases map[string]*github.ReleaseInfo
	known := make(map[string][]github.ReleaseInfo)
	if opts.MinReleaseAgeDays > 0 {
		releases = make(map[string]*github.ReleaseInfo)
		cutoff := time.Now().UTC().AddDate(0, 0, -opts.MinReleaseAgeDays)
		for _, repo := range repos {
			all, err := rc.GHClient.ListReleases(rc.Ctx, repo)
			if err != nil {
				return nil, nil, fmt.Errorf("read releases for %s: %w", repo, err)
			}
			known[repo] = all
			eligible, err := latestEligibleRelease(all, cutoff)
			if err != nil {
				return nil, nil, fmt.Errorf("check release age for %s: %w", repo, err)
			}
			if eligible != nil {
				releases[repo] = eligible
			} else if rc.Verbose {
				fmt.Printf("Skipping %s: no stable SemVer release meets the %d-day minimum age.\n", repo, opts.MinReleaseAgeDays)
			}
		}
	} else {
		var apiErrors map[string]error
		releases, apiErrors = rc.GHClient.CheckMultipleReleases(rc.Ctx, repos)
		for repo, err := range apiErrors {
			return nil, nil, fmt.Errorf("read latest release for %s: %w", repo, err)
		}
	}
	// The original detector logs reference errors and omits the affected findings.
	// Resolve required refs first so a filtered check cannot silently pass on an API failure.
	for _, file := range files {
		for _, ref := range file.UsesWithVersions {
			release := releases[ref.OwnerRepo]
			if archived[ref.OwnerRepo] || release == nil {
				continue
			}
			if version.IsCommitSHA(ref.Version) || (version.IsMajorVersionTag(ref.Version) && version.SameMajorVersion(ref.Version, release.TagName)) {
				if _, err := rc.GHClient.GetRefSHA(rc.Ctx, ref.OwnerRepo, release.TagName); err != nil {
					return nil, nil, fmt.Errorf("resolve eligible release for %s: %w", ref.OwnerRepo, err)
				}
				if version.IsMajorVersionTag(ref.Version) {
					if _, err := rc.GHClient.GetRefSHA(rc.Ctx, ref.OwnerRepo, ref.Version); err != nil {
						return nil, nil, fmt.Errorf("resolve current major tag for %s: %w", ref.OwnerRepo, err)
					}
				}
			}
		}
	}
	actions := actioninfo.CheckOutdatedActions(rc.Ctx, rc.GHClient, files, archived, releases, rc.Verbose)
	// A pin to a newer release must not become a downgrade to an older eligible release.
	var remaining []actioninfo.OutdatedActionInfo
	for _, action := range actions {
		if version.IsCommitSHA(action.CurrentRef) && opts.MinReleaseAgeDays > 0 {
			current, err := refCoversRelease(rc, action.OwnerRepo, action.CurrentRef, action.LatestTag, known[action.OwnerRepo])
			if err != nil {
				return nil, nil, err
			}
			if current {
				continue
			}
		}
		remaining = append(remaining, action)
	}
	if opts.IgnoreDependabotPRs && len(remaining) > 0 {
		var err error
		remaining, err = excludeDependabotUpdates(rc, remaining, known)
		if err != nil {
			return nil, nil, err
		}
	}
	return remaining, releases, nil
}

func latestEligibleRelease(releases []github.ReleaseInfo, cutoff time.Time) (*github.ReleaseInfo, error) {
	var latest *github.ReleaseInfo
	var latestVersion *semver.Version
	for _, release := range releases {
		v, err := semver.NewVersion(release.TagName)
		if release.Draft || release.Prerelease || err != nil || v.Prerelease() != "" {
			continue
		}
		published, err := time.Parse(time.RFC3339, release.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("read publication date for %s: %w", release.TagName, err)
		}
		if published.After(cutoff) {
			continue
		}
		if latest == nil || v.GreaterThan(latestVersion) {
			copy := release
			latest, latestVersion = &copy, v
		}
	}
	return latest, nil
}

// refCoversRelease accepts the target version or a newer stable release.
// SHA pins and moving major tags require a commit match.
func refCoversRelease(rc *RunContext, repo, ref, target string, known []github.ReleaseInfo) (bool, error) {
	if ref == target {
		return true, nil
	}
	targetVersion, err := semver.NewVersion(target)
	if err != nil {
		return false, fmt.Errorf("parse target release %s: %w", target, err)
	}
	if !version.IsCommitSHA(ref) {
		if v, err := semver.NewVersion(ref); err == nil && v.Prerelease() == "" {
			if !version.IsMajorVersionTag(ref) {
				return !v.LessThan(targetVersion), nil
			}
			if v.Major() > targetVersion.Major() {
				return true, nil
			}
		}
	}
	refSHA := ref
	if !version.IsCommitSHA(ref) {
		refSHA, err = rc.GHClient.GetRefSHA(rc.Ctx, repo, ref)
		if err != nil {
			return false, fmt.Errorf("resolve %s@%s: %w", repo, ref, err)
		}
	}
	tags := []string{target}
	for _, release := range known {
		v, err := semver.NewVersion(release.TagName)
		if err == nil && !release.Draft && !release.Prerelease && v.Prerelease() == "" && v.GreaterThan(targetVersion) {
			tags = append(tags, release.TagName)
		}
	}
	for _, tag := range tags {
		sha, err := rc.GHClient.GetRefSHA(rc.Ctx, repo, tag)
		if err != nil {
			return false, fmt.Errorf("resolve %s@%s: %w", repo, tag, err)
		}
		if strings.HasPrefix(strings.ToLower(sha), strings.ToLower(refSHA)) {
			return true, nil
		}
	}
	return false, nil
}

func excludeDependabotUpdates(rc *RunContext, actions []actioninfo.OutdatedActionInfo, known map[string][]github.ReleaseInfo) ([]actioninfo.OutdatedActionInfo, error) {
	opts := rc.OutdatedOptions
	prs, err := rc.GHClient.ListDependabotPullRequests(rc.Ctx, opts.Repository, opts.BaseBranch)
	if err != nil {
		return nil, fmt.Errorf("read open Dependabot PRs: %w", err)
	}
	covered := make(map[int]bool)
	for _, pr := range prs {
		files, err := rc.GHClient.ListPullRequestFiles(rc.Ctx, opts.Repository, pr.Number)
		if err != nil {
			return nil, fmt.Errorf("read files for PR #%d: %w", pr.Number, err)
		}
		for _, file := range files {
			if file.Status != "modified" {
				continue
			}
			var candidates []int
			for i, action := range actions {
				path, err := repositoryPath(rc.WorkDir, action.Workflow)
				if err != nil {
					return nil, err
				}
				if !covered[i] && path == file.Filename {
					candidates = append(candidates, i)
				}
			}
			if len(candidates) == 0 {
				continue
			}
			if pr.Head.SHA == "" || pr.Base.SHA == "" || pr.Head.Repo.FullName == "" {
				return nil, fmt.Errorf("PR #%d has incomplete commit metadata", pr.Number)
			}
			base, err := readPRWorkflow(rc, opts.Repository, file.Filename, pr.Base.SHA)
			if err != nil {
				return nil, fmt.Errorf("read base workflow for PR #%d: %w", pr.Number, err)
			}
			head, err := readPRWorkflow(rc, pr.Head.Repo.FullName, file.Filename, pr.Head.SHA)
			if err != nil {
				return nil, fmt.Errorf("read head workflow for PR #%d: %w", pr.Number, err)
			}
			for _, i := range candidates {
				action := actions[i]
				if !hasFullRef(base, action.FullRef) || hasFullRef(head, action.FullRef) {
					continue
				}
				for _, ref := range head.UsesWithVersions {
					if ref.OwnerRepo != action.OwnerRepo || ref.ActionPath != action.ActionPath {
						continue
					}
					covers, err := refCoversRelease(rc, action.OwnerRepo, ref.Version, action.LatestTag, known[action.OwnerRepo])
					if err != nil {
						return nil, fmt.Errorf("check update in PR #%d: %w", pr.Number, err)
					}
					if covers {
						covered[i] = true
						if rc.Verbose {
							fmt.Printf("Skipping %s in %s: Dependabot PR #%d covers %s.\n", action.FullRef, file.Filename, pr.Number, action.LatestTag)
						}
						break
					}
				}
			}
		}
	}
	var remaining []actioninfo.OutdatedActionInfo
	for i, action := range actions {
		if !covered[i] {
			remaining = append(remaining, action)
		}
	}
	return remaining, nil
}

func repositoryPath(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("read repository path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workflow %s is outside repository %s", path, root)
	}
	return filepath.ToSlash(rel), nil
}

func readPRWorkflow(rc *RunContext, repo, path, sha string) (*workflow.WorkflowFile, error) {
	content, err := rc.GHClient.GetFileContent(rc.Ctx, repo, path, sha)
	if err != nil {
		return nil, err
	}
	return rc.Parser.ParseWorkflowContent(path, content)
}

func hasFullRef(file *workflow.WorkflowFile, fullRef string) bool {
	for _, ref := range file.UsesWithVersions {
		if ref.FullRef == fullRef {
			return true
		}
	}
	return false
}
