package checkrunner

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
)

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
