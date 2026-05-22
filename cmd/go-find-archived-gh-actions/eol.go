package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/checkrunner"
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
	token := resolveToken()
	rc := checkrunner.NewRunContext(token, conf, notify, createIssue)
	rc.Verbose = verbose
	rc.Debug = debug

	if debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	processFunc := func(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return processEOL(rc, workflowFiles, allActionRefs, workDir, update, staleDays)
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

func processEOL(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, update bool, staleDays int) bool {
	actioninfo.LogWorkflowInfo(rc.Verbose, workflowFiles, allActionRefs)

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	result, _ := checkrunner.DetectArchived(rc, workflowFiles, allActionRefs)
	staleActions := checkrunner.DetectStale(rc, workflowFiles, allActionRefs, result.Archived, staleDays)
	runtimeEOLActions := checkrunner.DetectRuntimeEOL(rc, workflowFiles, result.Archived, result.NonArchivedRepos)

	hasIssues := len(result.ArchivedActions) > 0 || len(staleActions) > 0 || len(runtimeEOLActions) > 0

	totalEOL := len(staleActions) + len(runtimeEOLActions)
	actioninfo.WriteActionOutput("eol-count", fmt.Sprintf("%d", totalEOL))
	actioninfo.WriteActionOutput("has-eol", fmt.Sprintf("%v", totalEOL > 0))

	if !hasIssues {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No EOL GitHub Actions found!")
		return false
	}

	checkrunner.PrintArchived(result.ArchivedActions, result.ArchivedRepos)
	checkrunner.PrintStale(staleActions)
	checkrunner.PrintRuntimeEOL(runtimeEOLActions)

	checkrunner.SendArchivedNotifications(rc, result.ArchivedActions)

	if update && len(staleActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "EOL actions detected. Writing updates for stale/deprecated actions...")
		eolRepos := make([]string, 0)
		for _, action := range staleActions {
			eolRepos = append(eolRepos, action.OwnerRepo)
		}
		eolRepos = actioninfo.RemoveDuplicates(eolRepos)

		if len(eolRepos) > 0 && len(result.NonArchivedRepos) > 0 {
			outdatedActions, releases := checkrunner.DetectOutdated(rc, workflowFiles, result.Archived, result.NonArchivedRepos)
			if len(outdatedActions) > 0 {
				if err := actioninfo.WriteOutdatedActions(rc.Ctx, rc.GHClient, workflowFiles, outdatedActions, releases, false, rc.Verbose); err != nil {
					log.Errorf("Failed to write EOL action updates: %v", err)
				}
			}
		}
	}

	if len(result.ArchivedActions) > 0 {
		fmt.Println("\n" + actioninfo.Emoji("❌ ", "[X] ") + "Archived actions detected. Please replace them with actively maintained alternatives.")
		return true
	}

	fmt.Println("\n" + actioninfo.Emoji("⏳ ", "[STALE] ") + "EOL actions detected. Consider replacing them with actively maintained alternatives.")
	return true
}
