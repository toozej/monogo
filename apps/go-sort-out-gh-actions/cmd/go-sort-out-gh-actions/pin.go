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

func newPinCmd() *cobra.Command {
	var write bool

	cmd := &cobra.Command{
		Use:   "pin",
		Short: "Display GitHub Actions that can be pinned to commit SHAs",
		Long: `Scan workflow files and display GitHub Actions using version tags that can be pinned to commit SHAs.
Pinning actions to SHAs improves supply-chain security by ensuring immutable action references.
Use --write/-w to write the pinned SHA references to affected workflow files.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runPin(write)
		},
	}

	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write pinned SHA references to affected workflow files")

	return cmd
}

func runPin(writeFlag bool) {
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
		return processPin(rc, workflowFiles, allActionRefs, workDir, writeFlag)
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

func processPin(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, writeFlag bool) bool {
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
	pinnableActions := actioninfo.DetectPinnableActions(workflowFiles, result.Archived)

	hasPinnable := len(pinnableActions) > 0

	actioninfo.WriteActionOutput("pin-count", fmt.Sprintf("%d", len(pinnableActions)))
	actioninfo.WriteActionOutput("has-pinnable", fmt.Sprintf("%v", hasPinnable))

	if !hasPinnable {
		checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, nil, nil, false, "", actioninfo.Emoji("✅ ", "[OK] ")+"All GitHub Actions are already pinned to commit SHAs!")
		return false
	}

	summary := "\n" + actioninfo.Emoji("📌 ", "[PIN] ") + "Actions using version tags detected. Consider pinning them to commit SHAs for improved supply-chain security."
	if writeFlag {
		pinReport := actioninfo.PinActions(rc.Ctx, rc.GHClient, workflowFiles, pinnableActions, rc.Verbose)
		actioninfo.PrintPinUpdateReport(os.Stdout, pinReport)
		summary = actioninfo.BuildPinUpdateSummary(pinReport)
	}

	checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, nil, pinnableActions, hasPinnable, summary, "")

	return hasPinnable
}
