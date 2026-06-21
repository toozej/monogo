package checkrunner

import (
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
)

func WriteResult(w *output.Writer, archivedActions []issue.ArchivedActionInfo, archivedRepos []string, staleActions []actioninfo.StaleActionInfo, runtimeEOLActions []actioninfo.RuntimeEOLActionInfo, outdatedActions []actioninfo.OutdatedActionInfo, pinnableActions []actioninfo.PinActionInfo, hasIssues bool, summary string, noIssuesMessage string) {
	co := &output.CheckOutput{
		ArchivedActions: archivedActions,
		StaleActions:    staleActions,
		RuntimeEOL:      runtimeEOLActions,
		OutdatedActions: outdatedActions,
		PinnableActions: pinnableActions,
		ArchivedRepos:   archivedRepos,
		HasIssues:       hasIssues,
		Summary:         summary,
		NoIssuesMessage: noIssuesMessage,
	}
	w.WriteCheckResult(co)
}
