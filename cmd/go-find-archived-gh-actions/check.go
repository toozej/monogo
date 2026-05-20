package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/github"
	"github.com/toozej/go-find-archived-gh-actions/internal/issue"
	"github.com/toozej/go-find-archived-gh-actions/internal/notification"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
)

func newCheckCmd() *cobra.Command {
	var write bool
	var staleDays int

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run all checks: archived, eol, and outdated",
		Long: `Run archived, eol, and outdated checks in order.
Use --write/-w to automatically apply updates: runs archived check, then eol with --update, then outdated with --update.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runCheck(write, staleDays)
		},
	}

	cmd.Flags().BoolVarP(&write, "write", "w", false, "Run eol and outdated with --update after archived check passes")
	cmd.Flags().IntVar(&staleDays, "stale-days", actioninfo.DefaultStaleDays, "Number of days after which an action is considered stale (default 365)")

	return cmd
}

func runCheck(writeFlag bool, staleDays int) {
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

	var notifier *notification.NotificationManager
	var issueCreator *issue.IssueCreator

	if notify {
		manager, nerr := notification.NewNotificationManager(conf.Notification)
		if nerr != nil {
			log.Fatalf("Failed to initialize notification manager: %v", nerr)
		}
		notifier = manager
	}

	if createIssue {
		issueCreator = issue.NewIssueCreator(token)
	}

	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, workDir)
		runReposModeCheck(ctx, parser, ghClient, notifier, issueCreator, reposDir, workDir, writeFlag, staleDays)
		return
	}

	workflowFiles, allActionRefs := resolveWorkflowFiles(parser, workDir)
	processCheck(ctx, parser, ghClient, notifier, issueCreator, workflowFiles, allActionRefs, workDir, writeFlag, staleDays)
}

func runReposModeCheck(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, issueCreator *issue.IssueCreator, reposDir, workDir string, writeFlag bool, staleDays int) {
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

		hasIssues := processCheck(ctx, parser, ghClient, notifier, issueCreator, workflows, actionRefs, repoPath, writeFlag, staleDays)
		if hasIssues {
			hasAnyIssues = true
		}
	}

	if hasAnyIssues {
		os.Exit(1)
	}
}

func processCheck(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, issueCreator *issue.IssueCreator, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, writeFlag bool, staleDays int) bool {
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

	var archivedActions []issue.ArchivedActionInfo
	var archivedRepos []string

	for _, wf := range workflowFiles {
		for _, ref := range wf.UsesWithVersions {
			if isArchived, exists := archived[ref.OwnerRepo]; exists && isArchived {
				archivedActions = append(archivedActions, issue.ArchivedActionInfo{
					Repo:     ref.OwnerRepo,
					Workflow: filepath.Base(wf.Path),
					Uses:     ref.FullRef,
				})
				archivedRepos = append(archivedRepos, ref.OwnerRepo)
			}
		}
	}

	archivedRepos = actioninfo.RemoveDuplicates(archivedRepos)

	nonArchivedRepos := actioninfo.GetNonArchivedRepos(allActionRefs, archived)

	var staleActions []actioninfo.StaleActionInfo
	if len(nonArchivedRepos) > 0 {
		days := actioninfo.SanitizeStaleDays(staleDays)
		staleThreshold := time.Duration(days) * 24 * time.Hour
		fmt.Printf("Checking %d non-archived action repositories for stale/deprecated status...\n", len(nonArchivedRepos))
		staleResults, staleErrors := ghClient.CheckMultipleStale(ctx, nonArchivedRepos, staleThreshold)

		if verbose && len(staleErrors) > 0 {
			fmt.Printf("Stale check errors encountered:\n")
			for repo, err := range staleErrors {
				fmt.Printf(" - %s: %v\n", repo, err)
			}
		}

		for _, wf := range workflowFiles {
			for _, ref := range wf.UsesWithVersions {
				if staleInfo, exists := staleResults[ref.OwnerRepo]; exists {
					staleActions = append(staleActions, actioninfo.StaleActionInfo{
						OwnerRepo:          ref.OwnerRepo,
						FullRef:            ref.FullRef,
						Workflow:           filepath.Base(wf.Path),
						Deprecated:         staleInfo.Deprecated,
						DeprecationMessage: staleInfo.DeprecationMessage,
						LastUpdated:        staleInfo.LastUpdated,
						StaleByAge:         staleInfo.StaleByAge,
					})
				}
			}
		}
	}

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

	hasIssues := len(archivedActions) > 0 || len(outdatedActions) > 0 || len(staleActions) > 0

	actioninfo.WriteActionOutput("archived-count", fmt.Sprintf("%d", len(archivedActions)))
	actioninfo.WriteActionOutput("has-archived", fmt.Sprintf("%v", len(archivedActions) > 0))
	actioninfo.WriteActionOutput("outdated-count", fmt.Sprintf("%d", len(outdatedActions)))
	actioninfo.WriteActionOutput("has-outdated", fmt.Sprintf("%v", len(outdatedActions) > 0))
	actioninfo.WriteActionOutput("eol-count", fmt.Sprintf("%d", len(staleActions)))
	actioninfo.WriteActionOutput("has-eol", fmt.Sprintf("%v", len(staleActions) > 0))

	if !hasIssues {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No archived, outdated, or stale GitHub Actions found!")
		return false
	}

	if len(archivedActions) > 0 {
		fmt.Printf("\n%sFound %d archived GitHub Actions in %d uses:\n\n", actioninfo.Emoji("🚨 ", "[ARCHIVED] "), len(archivedRepos), len(archivedActions))

		workflowMap := make(map[string][]issue.ArchivedActionInfo)
		for _, action := range archivedActions {
			workflowMap[action.Workflow] = append(workflowMap[action.Workflow], action)
		}

		for wf, actions := range workflowMap {
			fmt.Printf("%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
			for _, action := range actions {
				fmt.Printf(" %s%s\n", actioninfo.Emoji("❌ ", "[X] "), action.Uses)
			}
			fmt.Println()
		}
	}

	if len(staleActions) > 0 {
		uniqueStale := make(map[string]bool)
		for _, action := range staleActions {
			uniqueStale[action.OwnerRepo] = true
		}

		fmt.Printf("\n%sFound %d stale/deprecated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⏳ ", "[STALE] "), len(uniqueStale), len(staleActions))

		staleWorkflowMap := make(map[string][]actioninfo.StaleActionInfo)
		for _, action := range staleActions {
			staleWorkflowMap[action.Workflow] = append(staleWorkflowMap[action.Workflow], action)
		}

		for wf, actions := range staleWorkflowMap {
			fmt.Printf("%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
			for _, action := range actions {
				if action.Deprecated {
					msg := action.DeprecationMessage
					if msg != "" {
						msg = ": " + msg
					}
					fmt.Printf(" %s%s (DEPRECATED%s)\n", actioninfo.Emoji("⏳ ", "[STALE] "), action.FullRef, msg)
				} else if action.StaleByAge {
					fmt.Printf(" %s%s (not updated since %s)\n", actioninfo.Emoji("⏳ ", "[STALE] "), action.FullRef, action.LastUpdated.Format("2006-01-02"))
				}
			}
			fmt.Println()
		}
	}

	if len(outdatedActions) > 0 {
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
	}

	repoName := actioninfo.GetRepoName(workDir)

	if notifier != nil && len(archivedActions) > 0 {
		var notificationActions []notification.ArchivedActionInfo
		for _, action := range archivedActions {
			notificationActions = append(notificationActions, notification.ArchivedActionInfo{
				Repo:     action.Repo,
				Workflow: action.Workflow,
				Uses:     action.Uses,
			})
		}
		if err := notifier.NotifyArchivedActions(ctx, notificationActions, repoName); err != nil {
			log.Errorf("Failed to send notifications: %v", err)
		}
	}

	if writeFlag && len(archivedActions) == 0 && len(outdatedActions) > 0 {
		if err := actioninfo.WriteOutdatedActions(ctx, ghClient, workflowFiles, outdatedActions, releases, false, verbose); err != nil {
			log.Errorf("Failed to write action updates: %v", err)
		}
	} else if writeFlag && len(archivedActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Cannot auto-update because archived actions were found. Please replace archived actions first.")
	}

	if issueCreator != nil && repoName != "" && len(archivedActions) > 0 {
		parts := strings.Split(repoName, "/")
		if len(parts) == 2 {
			owner, repo := parts[0], parts[1]
			if err := issueCreator.CreateArchivedActionIssue(ctx, owner, repo, archivedActions); err != nil {
				log.Errorf("Failed to create GitHub issue: %v", err)
			}
		}
	}

	switch {
	case len(archivedActions) > 0:
		fmt.Println("\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives.")
		return true
	case len(outdatedActions) > 0:
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Outdated actions detected. Consider updating to the latest versions.")
		return true
	case len(staleActions) > 0:
		fmt.Println("\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "Stale or deprecated actions detected. Consider replacing them with actively maintained alternatives.")
		return true
	}

	return false
}
