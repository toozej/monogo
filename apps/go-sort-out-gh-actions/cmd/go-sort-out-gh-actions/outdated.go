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

func newOutdatedCmd() *cobra.Command {
	var update bool
	var pin bool
	var semver bool
	var options checkrunner.OutdatedOptions

	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Display outdated GitHub Actions",
		Long: `Scan workflow files and display GitHub Actions that are outdated compared to the latest release.
By default, updates are pinned to SHAs with semver comments. Use --semver for version strings instead of SHAs.
Use --pin to swap from semver version strings to SHAs.`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if options.MinReleaseAgeDays < 0 || options.MinReleaseAgeDays > 36500 {
				return fmt.Errorf("--min-release-age-days must be between 0 and 36500")
			}
			if options.IgnoreDependabotPRs {
				if options.Repository == "" {
					return fmt.Errorf("--repository is required with --ignore-dependabot-prs")
				}
				if remoteRepo != "" || orgName != "" || userName != "" || reposDir != "" {
					return fmt.Errorf("--ignore-dependabot-prs requires a local scan of one repository")
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			useSemver := semver
			if pin {
				useSemver = false
			}
			runOutdated(update, useSemver, options)
		},
	}

	cmd.Flags().BoolVar(&update, "update", false, "Write updated versions to affected workflow files")
	cmd.Flags().BoolVar(&pin, "pin", false, "Pin actions to SHAs instead of semver version strings (default format when --update is used)")
	cmd.Flags().BoolVar(&semver, "semver", false, "Use semver version strings instead of SHAs when updating")
	cmd.Flags().IntVar(&options.MinReleaseAgeDays, "min-release-age-days", 0, "Minimum release age in days (0 disables the filter)")
	cmd.Flags().BoolVar(&options.IgnoreDependabotPRs, "ignore-dependabot-prs", false, "Exclude updates covered by open Dependabot PRs")
	cmd.Flags().StringVar(&options.Repository, "repository", "", "GitHub repository for the Dependabot PR filter (owner/repo)")
	cmd.Flags().StringVar(&options.BaseBranch, "base-branch", "", "Target branch for the Dependabot PR filter (default: repository default branch)")

	return cmd
}

func runOutdated(update bool, useSemver bool, options ...checkrunner.OutdatedOptions) {
	token := resolveToken()
	of := resolveOutputFormat()
	rc := newRunContextFromFlags(token, of)
	defer func() { _ = rc.Close() }()
	rc.Verbose = verbose
	rc.Debug = debug
	if len(options) > 0 {
		rc.OutdatedOptions = options[0]
	}

	if debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	processFunc := func(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return processOutdated(rc, workflowFiles, allActionRefs, workDir, update, useSemver)
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

func processOutdated(rc *checkrunner.RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string, update bool, useSemver bool) bool {
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
	outdatedActions, releases, err := checkrunner.DetectOutdatedFiltered(rc, workflowFiles, result.Archived, result.NonArchivedRepos)
	if err != nil {
		log.Errorf("Outdated-action check did not complete: %v", err)
		return true
	}

	hasIssues := len(outdatedActions) > 0

	actioninfo.WriteActionOutput("outdated-count", fmt.Sprintf("%d", len(outdatedActions)))
	actioninfo.WriteActionOutput("has-outdated", fmt.Sprintf("%v", len(outdatedActions) > 0))

	if !hasIssues {
		checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, nil, nil, false, "", actioninfo.Emoji("✅ ", "[OK] ")+"No outdated GitHub Actions found!")
		return false
	}

	summary := "\n" + actioninfo.Emoji("⚠️ ", "[WARN] ") + "Outdated actions detected. Consider updating to the latest versions."
	if update {
		updateReport := actioninfo.WriteOutdatedActions(rc.Ctx, rc.GHClient, workflowFiles, outdatedActions, releases, useSemver, rc.Verbose)
		actioninfo.PrintOutdatedUpdateReport(os.Stdout, updateReport)
		summary = actioninfo.BuildOutdatedUpdateSummary(updateReport)
	}

	checkrunner.WriteResult(rc.OutputWriter, nil, nil, nil, nil, outdatedActions, nil, hasIssues, summary, "")

	return hasIssues
}
