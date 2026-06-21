package checkrunner

import (
	"context"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/notification"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

type CheckResult struct {
	ArchivedActions   []issue.ArchivedActionInfo
	ArchivedRepos     []string
	StaleActions      []actioninfo.StaleActionInfo
	RuntimeEOLActions []actioninfo.RuntimeEOLActionInfo
	OutdatedActions   []actioninfo.OutdatedActionInfo
	Releases          map[string]*github.ReleaseInfo
	Archived          map[string]bool
	NonArchivedRepos  []string
}

type RunContext struct {
	Ctx          context.Context
	WorkDir      string
	Parser       *workflow.WorkflowParser
	GHClient     *github.Client
	Notifier     *notification.NotificationManager
	IssueCreator issue.IssueCreatorIface
	OutputWriter *output.Writer
	Verbose      bool
	Debug        bool
}

type ProcessFunc func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool
