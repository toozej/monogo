package checkrunner

import (
	"fmt"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/issue"
)

func PrintArchived(actions []issue.ArchivedActionInfo, repos []string) {
	if len(actions) == 0 {
		return
	}

	fmt.Printf("\n%sFound %d archived GitHub Actions in %d uses:\n\n", actioninfo.Emoji("🚨 ", "[ARCHIVED] "), len(repos), len(actions))

	workflowMap := make(map[string][]issue.ArchivedActionInfo)
	for _, action := range actions {
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

func PrintStale(actions []actioninfo.StaleActionInfo) {
	if len(actions) == 0 {
		return
	}

	uniqueStale := make(map[string]bool)
	for _, action := range actions {
		uniqueStale[action.OwnerRepo] = true
	}

	fmt.Printf("\n%sFound %d stale/deprecated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⏳ ", "[STALE] "), len(uniqueStale), len(actions))

	staleWorkflowMap := make(map[string][]actioninfo.StaleActionInfo)
	for _, action := range actions {
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

func PrintRuntimeEOL(actions []actioninfo.RuntimeEOLActionInfo) {
	if len(actions) == 0 {
		return
	}

	uniqueRuntimeEOL := make(map[string]bool)
	for _, action := range actions {
		uniqueRuntimeEOL[action.OwnerRepo] = true
	}

	fmt.Printf("\n%sFound %d actions using EOL runtimes in %d uses:\n\n", actioninfo.Emoji("🖥️ ", "[RUNTIME] "), len(uniqueRuntimeEOL), len(actions))

	runtimeWorkflowMap := make(map[string][]actioninfo.RuntimeEOLActionInfo)
	for _, action := range actions {
		runtimeWorkflowMap[action.Workflow] = append(runtimeWorkflowMap[action.Workflow], action)
	}

	for wf, actions := range runtimeWorkflowMap {
		fmt.Printf("%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			eolDateStr := action.EOLDate.Format("2006-01-02")
			if action.EOLDate.IsZero() {
				eolDateStr = "unknown"
			}
			fmt.Printf(" %s%s (uses %s%s, EOL since %s)\n", actioninfo.Emoji("🖥️ ", "[RUNTIME] "), action.FullRef, action.Runtime, action.Version, eolDateStr)
		}
		fmt.Println()
	}
}

func PrintOutdated(actions []actioninfo.OutdatedActionInfo) {
	if len(actions) == 0 {
		return
	}

	uniqueOutdated := make(map[string]bool)
	for _, action := range actions {
		uniqueOutdated[action.OwnerRepo] = true
	}

	fmt.Printf("\n%sFound %d outdated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⚠️ ", "[WARN] "), len(uniqueOutdated), len(actions))

	outdatedWorkflowMap := make(map[string][]actioninfo.OutdatedActionInfo)
	for _, action := range actions {
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
