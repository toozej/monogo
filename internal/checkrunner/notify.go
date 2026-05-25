package checkrunner

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/go-sort-out-gh-actions/internal/notification"
)

func SendArchivedNotifications(rc *RunContext, actions []issue.ArchivedActionInfo) {
	if rc.Notifier == nil || len(actions) == 0 {
		return
	}

	repoName := actioninfo.GetRepoName(rc.WorkDir)

	var notificationActions []notification.ArchivedActionInfo
	for _, action := range actions {
		notificationActions = append(notificationActions, notification.ArchivedActionInfo{
			Repo:     action.Repo,
			Workflow: action.Workflow,
			Uses:     action.Uses,
		})
	}
	if err := rc.Notifier.NotifyArchivedActions(rc.Ctx, notificationActions, repoName); err != nil {
		log.Errorf("Failed to send notifications: %v", err)
	}
}

func CreateArchivedIssues(rc *RunContext, actions []issue.ArchivedActionInfo) {
	if rc.IssueCreator == nil || len(actions) == 0 {
		return
	}

	repoName := actioninfo.GetRepoName(rc.WorkDir)
	if repoName == "" {
		return
	}

	parts := strings.Split(repoName, "/")
	if len(parts) == 2 {
		owner, repo := parts[0], parts[1]
		if err := rc.IssueCreator.CreateArchivedActionIssue(rc.Ctx, owner, repo, actions); err != nil {
			log.Errorf("Failed to create GitHub issue: %v", err)
		}
	}
}
