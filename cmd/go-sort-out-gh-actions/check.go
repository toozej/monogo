package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/checkrunner"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
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
	token := resolveToken()
	of := resolveOutputFormat()
	rc := checkrunner.NewRunContext(token, conf, notify, createIssue, of)
	rc.Verbose = verbose
	rc.Debug = debug

	if debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	processFunc := func(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return processCheck(rc, workflowFiles, allActionRefs, workDir, writeFlag, staleDays)
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

func processCheck(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, writeFlag bool, staleDays int) bool {
	actioninfo.LogWorkflowInfo(os.Stdout, rc.Verbose, workflowFiles, allActionRefs)

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	result, _ := checkrunner.DetectArchived(rc, workflowFiles, allActionRefs)
	staleActions := checkrunner.DetectStale(rc, workflowFiles, allActionRefs, result.Archived, staleDays)
	runtimeEOLActions := checkrunner.DetectRuntimeEOL(rc, workflowFiles, result.Archived, result.NonArchivedRepos)
	outdatedActions, releases := checkrunner.DetectOutdated(rc, workflowFiles, result.Archived, result.NonArchivedRepos)

	hasIssues := len(result.ArchivedActions) > 0 || len(outdatedActions) > 0 || len(staleActions) > 0 || len(runtimeEOLActions) > 0

	actioninfo.WriteActionOutput("archived-count", fmt.Sprintf("%d", len(result.ArchivedActions)))
	actioninfo.WriteActionOutput("has-archived", fmt.Sprintf("%v", len(result.ArchivedActions) > 0))
	actioninfo.WriteActionOutput("outdated-count", fmt.Sprintf("%d", len(outdatedActions)))
	actioninfo.WriteActionOutput("has-outdated", fmt.Sprintf("%v", len(outdatedActions) > 0))
	totalEOL := len(staleActions) + len(runtimeEOLActions)
	actioninfo.WriteActionOutput("eol-count", fmt.Sprintf("%d", totalEOL))
	actioninfo.WriteActionOutput("has-eol", fmt.Sprintf("%v", totalEOL > 0))

	if !hasIssues {
		checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, nil, false, "", actioninfo.Emoji("✅ ", "[OK] ")+"No archived, outdated, or stale GitHub Actions found!")
		return false
	}

	checkrunner.SendArchivedNotifications(rc, result.ArchivedActions)

	if writeFlag && len(result.ArchivedActions) == 0 && len(outdatedActions) > 0 {
		if err := actioninfo.WriteOutdatedActions(rc.Ctx, rc.GHClient, workflowFiles, outdatedActions, releases, false, rc.Verbose); err != nil {
			log.Errorf("Failed to write action updates: %v", err)
		}
	} else if writeFlag && len(result.ArchivedActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Cannot auto-update because archived actions were found. Please replace archived actions first.")
	}

	checkrunner.CreateArchivedIssues(rc, result.ArchivedActions)

	var summary string
	switch {
	case len(result.ArchivedActions) > 0:
		summary = "\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives."
	case len(outdatedActions) > 0:
		summary = "\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Outdated actions detected. Consider updating to the latest versions."
	case len(staleActions) > 0:
		summary = "\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "Stale or deprecated actions detected. Consider replacing them with actively maintained alternatives."
	case len(runtimeEOLActions) > 0:
		summary = "\n" + actioninfo.Emoji("🖥️ ", "[RUNTIME] ") + "Actions using EOL runtimes detected. Consider updating to actions that use supported runtime versions."
	}

	checkrunner.WriteResult(rc.OutputWriter, result.ArchivedActions, result.ArchivedRepos, staleActions, runtimeEOLActions, outdatedActions, hasIssues, summary, "")

	return hasIssues
}
