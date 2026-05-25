package checkrunner

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
)

func DetectArchived(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef) (*CheckResult, error) {
	ownerRepos := actioninfo.GetOwnerRepos(allActionRefs)

	fmt.Printf("Checking %d action repositories for archived status...\n", len(ownerRepos))

	archived, errors := rc.GHClient.CheckMultipleRepos(rc.Ctx, ownerRepos)

	if rc.Debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	if rc.Verbose && len(errors) > 0 {
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

	return &CheckResult{
		ArchivedActions:  archivedActions,
		ArchivedRepos:    archivedRepos,
		Archived:         archived,
		NonArchivedRepos: nonArchivedRepos,
	}, nil
}

func DetectStale(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, archived map[string]bool, staleDays int) []actioninfo.StaleActionInfo {
	nonArchivedRepos := actioninfo.GetNonArchivedRepos(allActionRefs, archived)

	if len(nonArchivedRepos) == 0 {
		return nil
	}

	days := actioninfo.SanitizeStaleDays(staleDays)
	staleThreshold := time.Duration(days) * 24 * time.Hour
	fmt.Printf("Checking %d non-archived action repositories for stale/deprecated status...\n", len(nonArchivedRepos))
	staleResults, staleErrors := rc.GHClient.CheckMultipleStale(rc.Ctx, nonArchivedRepos, staleThreshold)

	if rc.Verbose && len(staleErrors) > 0 {
		fmt.Printf("Stale check errors encountered:\n")
		for repo, err := range staleErrors {
			fmt.Printf(" - %s: %v\n", repo, err)
		}
	}

	var staleActions []actioninfo.StaleActionInfo
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

	return staleActions
}

func DetectRuntimeEOL(rc *RunContext, workflowFiles []*workflow.WorkflowFile, archived map[string]bool, nonArchivedRepos []string) []actioninfo.RuntimeEOLActionInfo {
	if len(nonArchivedRepos) == 0 {
		return nil
	}

	var runtimeRefs []workflow.ActionRef
	seen := make(map[string]bool)
	for _, wf := range workflowFiles {
		for _, ref := range wf.UsesWithVersions {
			if isArchived, exists := archived[ref.OwnerRepo]; exists && isArchived {
				continue
			}
			key := ref.OwnerRepo + "@" + ref.Version
			if !seen[key] {
				seen[key] = true
				runtimeRefs = append(runtimeRefs, workflow.ActionRef{
					OwnerRepo: ref.OwnerRepo,
					Version:   ref.Version,
					FullRef:   ref.FullRef,
				})
			}
		}
	}

	if len(runtimeRefs) == 0 {
		return nil
	}

	fmt.Printf("Checking %d action references for EOL runtime versions...\n", len(runtimeRefs))
	runtimeResults, runtimeErrors := rc.GHClient.CheckMultipleRuntimeEOL(rc.Ctx, runtimeRefs)

	if rc.Verbose && len(runtimeErrors) > 0 {
		fmt.Printf("Runtime EOL check errors encountered:\n")
		for ref, err := range runtimeErrors {
			fmt.Printf(" - %s: %v\n", ref, err)
		}
	}

	var runtimeEOLActions []actioninfo.RuntimeEOLActionInfo
	for _, wf := range workflowFiles {
		for _, ref := range wf.UsesWithVersions {
			key := ref.OwnerRepo + "@" + ref.Version
			if eolResult, exists := runtimeResults[key]; exists {
				runtimeEOLActions = append(runtimeEOLActions, actioninfo.RuntimeEOLActionInfo{
					OwnerRepo: ref.OwnerRepo,
					FullRef:   ref.FullRef,
					Workflow:  filepath.Base(wf.Path),
					Runtime:   eolResult.Runtime,
					Version:   eolResult.Version,
					EOLDate:   eolResult.EOLDate,
				})
			}
		}
	}

	return runtimeEOLActions
}

func DetectOutdated(rc *RunContext, workflowFiles []*workflow.WorkflowFile, archived map[string]bool, nonArchivedRepos []string) ([]actioninfo.OutdatedActionInfo, map[string]*github.ReleaseInfo) {
	if len(nonArchivedRepos) == 0 {
		return nil, nil
	}

	fmt.Printf("Checking %d non-archived action repositories for latest versions...\n", len(nonArchivedRepos))
	releases, releaseErrors := rc.GHClient.CheckMultipleReleases(rc.Ctx, nonArchivedRepos)

	if rc.Debug {
		rc.GHClient.LogRateLimits(rc.Ctx)
	}

	if rc.Verbose && len(releaseErrors) > 0 {
		fmt.Printf("Release API errors encountered:\n")
		for repo, err := range releaseErrors {
			fmt.Printf(" - %s: %v\n", repo, err)
		}
	}

	outdatedActions := actioninfo.CheckOutdatedActions(rc.Ctx, rc.GHClient, workflowFiles, archived, releases, rc.Verbose)
	return outdatedActions, releases
}
