package issue

import (
	"context"

	"github.com/google/go-github/v85/github"
)

type IssueCreatorIface interface {
	CreateArchivedActionIssue(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error
}

type IssueCreator struct {
	token  string
	client *github.Client
}

type ArchivedActionInfo struct {
	Repo     string
	Workflow string
	Uses     string
}
