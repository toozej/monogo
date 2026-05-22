package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/checkrunner"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
)

func newArchivedCmd() *cobra.Command {
	var staleDays int

	cmd := &cobra.Command{
		Use:   "archived",
		Short: "Display archived GitHub Actions",
		Long:  `Scan workflow files and display GitHub Actions that have been archived upstream. Also checks for stale/deprecated actions.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runArchived(staleDays)
		},
	}

	cmd.Flags().IntVar(&staleDays, "stale-days", actioninfo.DefaultStaleDays, "Number of days after which an action is considered stale (default 365)")

	return cmd
}

func runArchived(staleDays int) {
	token := resolveToken()
	rc := checkrunner.NewRunContext(token, conf, notify, createIssue)
	rc.Verbose = verbose
	rc.Debug = debug

	if debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	processFunc := func(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return processArchived(rc, workflowFiles, allActionRefs, workDir, staleDays)
	}

	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, rc.WorkDir)
		if checkrunner.RunReposMode(rc, reposDir, processFunc) {
			os.Exit(1)
		}
		return
	}

	workflowFiles, allActionRefs := resolveWorkflowFiles(rc.Parser, rc.WorkDir)
	processFunc(rc, workflowFiles, allActionRefs, rc.WorkDir)
}

func processArchived(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, staleDays int) bool {
	actioninfo.LogWorkflowInfo(rc.Verbose, workflowFiles, allActionRefs)

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	result, _ := checkrunner.DetectArchived(rc, workflowFiles, allActionRefs)
	staleActions := checkrunner.DetectStale(rc, workflowFiles, allActionRefs, result.Archived, staleDays)

	hasIssues := len(result.ArchivedActions) > 0 || len(staleActions) > 0

	actioninfo.WriteActionOutput("archived-count", fmt.Sprintf("%d", len(result.ArchivedActions)))
	actioninfo.WriteActionOutput("has-archived", fmt.Sprintf("%v", len(result.ArchivedActions) > 0))
	actioninfo.WriteActionOutput("stale-count", fmt.Sprintf("%d", len(staleActions)))
	actioninfo.WriteActionOutput("has-stale", fmt.Sprintf("%v", len(staleActions) > 0))

	if !hasIssues {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No archived or stale GitHub Actions found!")
		return false
	}

	checkrunner.PrintArchived(result.ArchivedActions, result.ArchivedRepos)
	checkrunner.PrintStale(staleActions)

	checkrunner.SendArchivedNotifications(rc, result.ArchivedActions)
	checkrunner.CreateArchivedIssues(rc, result.ArchivedActions)

	if len(result.ArchivedActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives.")
		return true
	}

	fmt.Println("\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "Stale or deprecated actions detected. Consider replacing them with actively maintained alternatives.")
	return true
}
