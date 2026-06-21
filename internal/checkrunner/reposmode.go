package checkrunner

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/github"
)

const maxRemoteConcurrency = 5

func RunReposMode(rc *RunContext, reposDir string, processFunc ProcessFunc) bool {
	repos, err := rc.Parser.FindReposWithWorkflows(reposDir)
	if err != nil {
		log.Fatalf("Failed to find repos with workflows: %v", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories with .github/workflows found in the specified directory")
		return false
	}

	if rc.Verbose {
		fmt.Printf("Found %d repositories with workflow files\n", len(repos))
	}

	hasAnyIssues := false
	for _, repoPath := range repos {
		fmt.Printf("\n%sScanning: %s\n", actioninfo.Emoji("📁 ", "[SCAN] "), repoPath)
		fmt.Println(strings.Repeat("-", len(repoPath)+10))

		actionRefs, workflows, err := rc.Parser.GetAllUsesFromRepoWithVersions(repoPath)
		if err != nil {
			log.Errorf("Failed to find workflow files in %s: %v", repoPath, err)
			continue
		}

		if len(actionRefs) == 0 {
			fmt.Println("No GitHub Actions found in workflows")
			continue
		}

		if processFunc(rc, workflows, actionRefs, repoPath) {
			hasAnyIssues = true
		}
	}

	return hasAnyIssues
}

func RunRemoteRepoMode(rc *RunContext, ownerRepo, ref string, processFunc ProcessFunc) bool {
	fmt.Printf("\n%sScanning: %s\n", actioninfo.Emoji("🌐 ", "[REMOTE] "), ownerRepo)
	fmt.Println(strings.Repeat("-", len(ownerRepo)+10))

	actionRefs, workflows, err := rc.Parser.GetRemoteUsesFromRepo(rc.Ctx, rc.GHClient, ownerRepo, ref)
	if err != nil {
		log.Errorf("Failed to get remote workflow contents from %s: %v", ownerRepo, err)
		return false
	}

	if len(actionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	return processFunc(rc, workflows, actionRefs, ownerRepo)
}

func RunOrgMode(rc *RunContext, org string, includeForks bool, processFunc ProcessFunc) (bool, error) {
	opts := &github.ListOrgReposOptions{
		IncludeForks: includeForks,
		OnlyActive:   true,
	}

	repos, err := rc.GHClient.ListOrgRepos(rc.Ctx, org, opts)
	if err != nil {
		return false, fmt.Errorf("failed to list org repos: %w", err)
	}

	return runRemoteRepos(rc, repos, processFunc), nil
}

func RunUserMode(rc *RunContext, username string, includeForks bool, processFunc ProcessFunc) (bool, error) {
	opts := &github.ListOrgReposOptions{
		IncludeForks: includeForks,
		OnlyActive:   true,
	}

	repos, err := rc.GHClient.ListUserRepos(rc.Ctx, username, opts)
	if err != nil {
		return false, fmt.Errorf("failed to list user repos: %w", err)
	}

	return runRemoteRepos(rc, repos, processFunc), nil
}

func runRemoteRepos(rc *RunContext, repos []github.RepoEntry, processFunc ProcessFunc) bool {
	if len(repos) == 0 {
		fmt.Println("No repositories found")
		return false
	}

	fmt.Printf("Found %d repositories to scan\n", len(repos))

	var hasAnyIssues bool
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxRemoteConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo github.RepoEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			fmt.Printf("\n%sScanning: %s\n", actioninfo.Emoji("📁 ", "[SCAN] "), repo.FullName)
			fmt.Println(strings.Repeat("-", len(repo.FullName)+10))
			mu.Unlock()

			actionRefs, workflows, err := rc.Parser.GetRemoteUsesFromRepo(rc.Ctx, rc.GHClient, repo.FullName, "")
			if err != nil {
				log.WithFields(log.Fields{
					"repo":  repo.FullName,
					"error": err,
				}).Warn("Failed to fetch remote workflow contents")
				return
			}

			if len(actionRefs) == 0 {
				return
			}

			if processFunc(rc, workflows, actionRefs, repo.FullName) {
				mu.Lock()
				hasAnyIssues = true
				mu.Unlock()
			}
		}(repo)
	}

	wg.Wait()
	return hasAnyIssues
}
