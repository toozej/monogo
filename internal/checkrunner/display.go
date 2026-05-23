package checkrunner

import (
	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/issue"
	"github.com/toozej/go-find-archived-gh-actions/internal/output"
)

func WriteResult(w *output.Writer, archivedActions []issue.ArchivedActionInfo, archivedRepos []string, staleActions []actioninfo.StaleActionInfo, runtimeEOLActions []actioninfo.RuntimeEOLActionInfo, outdatedActions []actioninfo.OutdatedActionInfo, hasIssues bool, summary string, noIssuesMessage string) {
	co := &output.CheckOutput{
		ArchivedActions: archivedActions,
		StaleActions:    staleActions,
		RuntimeEOL:      runtimeEOLActions,
		OutdatedActions: outdatedActions,
		ArchivedRepos:   archivedRepos,
		HasIssues:       hasIssues,
		Summary:         summary,
		NoIssuesMessage: noIssuesMessage,
	}
	w.WriteCheckResult(co)
}
