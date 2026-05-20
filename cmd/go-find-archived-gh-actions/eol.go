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

func newEOLCmd() *cobra.Command {
	var update bool
	var staleDays int

	cmd := &cobra.Command{
		Use:   "eol",
		Short: "Display GitHub Actions using end-of-life languages and runtimes",
		Long:  `Scan workflow files and display GitHub Actions that rely on or use end-of-life languages and runtimes. With --update, writes updated versions to affected workflow files.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runEOL(update, staleDays)
		},
	}

	cmd.Flags().BoolVar(&update, "update", false, "Write updated versions to affected workflow files")
	cmd.Flags().IntVar(&staleDays, "stale-days", actioninfo.DefaultStaleDays, "Number of days after which an action is considered stale (default 365)")

	return cmd
}

func runEOL(update bool, staleDays int) {
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
	if notify {
		manager, nerr := notification.NewNotificationManager(conf.Notification)
		if nerr != nil {
			log.Fatalf("Failed to initialize notification manager: %v", nerr)
		}
		notifier = manager
	}

	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, workDir)
		runReposModeEOL(ctx, parser, ghClient, notifier, reposDir, workDir, update, staleDays)
		return
	}

	workflowFiles, allActionRefs := resolveWorkflowFiles(parser, workDir)
	processEOL(ctx, parser, ghClient, notifier, workflowFiles, allActionRefs, workDir, update, staleDays)
}

func runReposModeEOL(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, reposDir, workDir string, update bool, staleDays int) {
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

		hasIssues := processEOL(ctx, parser, ghClient, notifier, workflows, actionRefs, repoPath, update, staleDays)
		if hasIssues {
			hasAnyIssues = true
		}
	}

	if hasAnyIssues {
		os.Exit(1)
	}
}

func processEOL(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, update bool, staleDays int) bool {
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

	var staleActions []actioninfo.StaleActionInfo
	nonArchivedRepos := actioninfo.GetNonArchivedRepos(allActionRefs, archived)

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

	hasIssues := len(archivedActions) > 0 || len(staleActions) > 0

	actioninfo.WriteActionOutput("eol-count", fmt.Sprintf("%d", len(staleActions)))
	actioninfo.WriteActionOutput("has-eol", fmt.Sprintf("%v", len(staleActions) > 0))

	if !hasIssues {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No EOL GitHub Actions found!")
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

	if update && len(staleActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "EOL actions detected. Writing updates for stale/deprecated actions...")
		eolRepos := make([]string, 0)
		for _, action := range staleActions {
			eolRepos = append(eolRepos, action.OwnerRepo)
		}
		eolRepos = actioninfo.RemoveDuplicates(eolRepos)

		if len(eolRepos) > 0 && len(nonArchivedRepos) > 0 {
			releases, releaseErrors := ghClient.CheckMultipleReleases(ctx, nonArchivedRepos)
			if verbose && len(releaseErrors) > 0 {
				fmt.Printf("Release API errors encountered:\n")
				for repo, err := range releaseErrors {
					fmt.Printf(" - %s: %v\n", repo, err)
				}
			}

			outdatedActions := actioninfo.CheckOutdatedActions(ctx, ghClient, workflowFiles, archived, releases, verbose)
			if len(outdatedActions) > 0 {
				if err := actioninfo.WriteOutdatedActions(ctx, ghClient, workflowFiles, outdatedActions, releases, false, verbose); err != nil {
					log.Errorf("Failed to write EOL action updates: %v", err)
				}
			}
		}
	}

	if len(archivedActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives.")
		return true
	}

	fmt.Println("\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "EOL actions detected. Consider replacing them with actively maintained alternatives.")
	return true
}
