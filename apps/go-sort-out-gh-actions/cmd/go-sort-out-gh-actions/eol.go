package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/checkrunner"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

// exitCode is set by run functions to communicate the desired os.Exit value
// back to main after deferred cleanups have run.
var exitCode int

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
	token := resolveToken()
	of := resolveOutputFormat()
	rc := newRunContextFromFlags(token, of)
	defer func() { _ = rc.Close() }()
	rc.Verbose = verbose
	rc.Debug = debug

	if debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	processFunc := func(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return processEOL(rc, workflowFiles, allActionRefs, workDir, update, staleDays)
	}

	if orgName != "" {
		hasIssues, err := checkrunner.RunOrgMode(rc, orgName, includeForks, processFunc)
		if err != nil {
			log.Errorf("Failed to run org mode: %v", err)
			exitCode = 1
			return
		}
		if hasIssues {
			exitCode = 1
		}
		return
	}
	if userName != "" {
		hasIssues, err := checkrunner.RunUserMode(rc, userName, includeForks, processFunc)
		if err != nil {
			log.Errorf("Failed to run user mode: %v", err)
			exitCode = 1
			return
		}
		if hasIssues {
			exitCode = 1
		}
		return
	}

	if remoteRepo != "" {
		if checkrunner.RunRemoteRepoMode(rc, remoteRepo, remoteRef, processFunc) {
			exitCode = 1
		}
		return
	}

	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, rc.WorkDir)
		if checkrunner.RunReposMode(rc, reposDir, processFunc) {
			exitCode = 1
		}
		return
	}

	workflowFiles, allActionRefs := resolveWorkflowFiles(rc.Parser, rc.WorkDir)
	if processFunc(rc, workflowFiles, allActionRefs, rc.WorkDir) {
		exitCode = 1
	}
}

func processEOL(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, update bool, staleDays int) bool {
	actioninfo.LogWorkflowInfo(os.Stdout, rc.Verbose, workflowFiles, allActionRefs)

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	result, err := checkrunner.DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		log.Errorf("Archived-action precheck did not complete: %v", err)
		return true
	}
	staleActions := checkrunner.DetectStale(rc, workflowFiles, allActionRefs, result.Archived, staleDays)
	runtimeEOLActions := checkrunner.DetectRuntimeEOL(rc, workflowFiles, result.Archived, result.NonArchivedRepos)

	hasIssues := len(result.ArchivedActions) > 0 || len(staleActions) > 0 || len(runtimeEOLActions) > 0

	totalEOL := len(staleActions) + len(runtimeEOLActions)
	actioninfo.WriteActionOutput("eol-count", fmt.Sprintf("%d", totalEOL))
	actioninfo.WriteActionOutput("has-eol", fmt.Sprintf("%v", totalEOL > 0))

	if !hasIssues {
		checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, nil, nil, false, "", actioninfo.Emoji("✅ ", "[OK] ")+"No EOL GitHub Actions found!")
		return false
	}

	checkrunner.SendArchivedNotifications(rc, result.ArchivedActions)
	checkrunner.CreateArchivedIssues(rc, result.ArchivedActions)

	summary := "\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "EOL actions detected. Consider replacing them with actively maintained alternatives."
	if len(result.ArchivedActions) > 0 {
		summary = "\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives."
	}

	if update && len(staleActions) > 0 && len(result.ArchivedActions) == 0 {
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "EOL actions detected. Writing updates for stale/deprecated actions...")
		eolRepos := make([]string, 0, len(staleActions))
		for _, action := range staleActions {
			eolRepos = append(eolRepos, action.OwnerRepo)
		}
		eolRepos = actioninfo.RemoveDuplicates(eolRepos)

		didUpdate := false
		if len(eolRepos) > 0 && len(result.NonArchivedRepos) > 0 {
			outdatedActions, releases := checkrunner.DetectOutdated(rc, workflowFiles, result.Archived, result.NonArchivedRepos)
			if len(outdatedActions) > 0 {
				updateReport := actioninfo.WriteOutdatedActions(rc.Ctx, rc.GHClient, workflowFiles, outdatedActions, releases, false, rc.Verbose)
				actioninfo.PrintOutdatedUpdateReport(os.Stdout, updateReport)
				summary = actioninfo.BuildOutdatedUpdateSummary(updateReport)
				didUpdate = true
			}
		}

		if !didUpdate {
			summary = "\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "EOL actions were detected, but no automatic version updates were available."
		}
	}

	checkrunner.WriteResult(rc.OutputWriter, result.ArchivedActions, result.ArchivedRepos, staleActions, runtimeEOLActions, nil, nil, hasIssues, summary, "")

	return hasIssues
}
