package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/github"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
)

func newOutdatedCmd() *cobra.Command {
	var update bool
	var pin bool
	var semver bool

	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Display outdated GitHub Actions",
		Long: `Scan workflow files and display GitHub Actions that are outdated compared to the latest release.
By default, updates are pinned to SHAs with semver comments. Use --semver for version strings instead of SHAs.
Use --pin to swap from semver version strings to SHAs.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			useSemver := semver
			if pin {
				useSemver = false
			}
			runOutdated(update, useSemver)
		},
	}

	cmd.Flags().BoolVar(&update, "update", false, "Write updated versions to affected workflow files")
	cmd.Flags().BoolVar(&pin, "pin", false, "Pin actions to SHAs instead of semver version strings (default format when --update is used)")
	cmd.Flags().BoolVar(&semver, "semver", false, "Use semver version strings instead of SHAs when updating")

	return cmd
}

func runOutdated(update bool, useSemver bool) {
	ctx := context.Background()
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	token := resolveToken()
	parser := workflow.NewParser()
	ghClient := github.NewClient(token)

	if debug {
		ghClient.LogRateLimits(ctx)
	}

	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, workDir)
		runReposModeOutdated(ctx, parser, ghClient, reposDir, workDir, update, useSemver)
		return
	}

	workflowFiles, allActionRefs := resolveWorkflowFiles(parser, workDir)
	processOutdated(ctx, parser, ghClient, workflowFiles, allActionRefs, workDir, update, useSemver)
}

func runReposModeOutdated(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, reposDir, workDir string, update bool, useSemver bool) {
	repos, err := parser.FindReposWithWorkflows(reposDir)
	if err != nil {
		log.Fatalf("Failed to find repos with workflows: %v", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories with .github/workflows found in the specified directory")
		return
	}

	if verbose {
		fmt.Printf("Found %d repositories with workflow files\n", len(repos))
	}

	hasAnyIssues := false
	for _, repoPath := range repos {
		fmt.Printf("\n%sScanning: %s\n", actioninfo.Emoji("📁 ", "[SCAN] "), repoPath)
		fmt.Println(strings.Repeat("-", len(repoPath)+10))

		actionRefs, workflows, err := parser.GetAllUsesFromRepoWithVersions(repoPath)
		if err != nil {
			log.Errorf("Failed to find workflow files in %s: %v", repoPath, err)
			continue
		}

		if len(actionRefs) == 0 {
			fmt.Println("No GitHub Actions found in workflows")
			continue
		}

		hasIssues := processOutdated(ctx, parser, ghClient, workflows, actionRefs, repoPath, update, useSemver)
		if hasIssues {
			hasAnyIssues = true
		}
	}

	if hasAnyIssues {
		os.Exit(1)
	}
}

func processOutdated(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, update bool, useSemver bool) bool {
	actioninfo.LogWorkflowInfo(verbose, workflowFiles, allActionRefs)

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	ownerRepos := actioninfo.GetOwnerRepos(allActionRefs)

	fmt.Printf("Checking %d action repositories for archived status...\n", len(ownerRepos))

	archived, errors := ghClient.CheckMultipleRepos(ctx, ownerRepos)

	if debug {
		ghClient.LogRateLimits(ctx)
	}

	if verbose && len(errors) > 0 {
		fmt.Printf("API errors encountered:\n")
		for repo, err := range errors {
			fmt.Printf(" - %s: %v\n", repo, err)
		}
	}

	nonArchivedRepos := actioninfo.GetNonArchivedRepos(allActionRefs, archived)

	var outdatedActions []actioninfo.OutdatedActionInfo
	var releases map[string]*github.ReleaseInfo

	if len(nonArchivedRepos) > 0 {
		fmt.Printf("Checking %d non-archived action repositories for latest versions...\n", len(nonArchivedRepos))
		var releaseErrors map[string]error
		releases, releaseErrors = ghClient.CheckMultipleReleases(ctx, nonArchivedRepos)

		if debug {
			ghClient.LogRateLimits(ctx)
		}

		if verbose && len(releaseErrors) > 0 {
			fmt.Printf("Release API errors encountered:\n")
			for repo, err := range releaseErrors {
				fmt.Printf(" - %s: %v\n", repo, err)
			}
		}

		outdatedActions = actioninfo.CheckOutdatedActions(ctx, ghClient, workflowFiles, archived, releases, verbose)
	}

	hasIssues := len(outdatedActions) > 0

	actioninfo.WriteActionOutput("outdated-count", fmt.Sprintf("%d", len(outdatedActions)))
	actioninfo.WriteActionOutput("has-outdated", fmt.Sprintf("%v", len(outdatedActions) > 0))

	if !hasIssues {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No outdated GitHub Actions found!")
		return false
	}

	uniqueOutdated := make(map[string]bool)
	for _, action := range outdatedActions {
		uniqueOutdated[action.OwnerRepo] = true
	}

	fmt.Printf("\n%sFound %d outdated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⚠️ ", "[WARN] "), len(uniqueOutdated), len(outdatedActions))

	outdatedWorkflowMap := make(map[string][]actioninfo.OutdatedActionInfo)
	for _, action := range outdatedActions {
		outdatedWorkflowMap[action.Workflow] = append(outdatedWorkflowMap[action.Workflow], action)
	}

	for wf, actions := range outdatedWorkflowMap {
		fmt.Printf("%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			fmt.Printf(" %s%s@%s (latest: %s)\n", actioninfo.Emoji("⚠️ ", "[WARN] "), action.OwnerRepo, action.CurrentRef, action.LatestTag)
		}
		fmt.Println()
	}

	if update && len(outdatedActions) > 0 {
		if err := actioninfo.WriteOutdatedActions(ctx, ghClient, workflowFiles, outdatedActions, releases, useSemver, verbose); err != nil {
			log.Errorf("Failed to write outdated action updates: %v", err)
		}
	}

	fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Outdated actions detected. Consider updating to the latest versions.")
	return true
}
