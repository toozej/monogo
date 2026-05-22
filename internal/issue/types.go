package issue

import (
	"context"

	"github.com/google/go-github/v85/github"
)

type IssueCreator struct {
	token    string
	client   *github.Client
	isTest   bool
	testImpl func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error
}

type ArchivedActionInfo struct {
	Repo     string
	Workflow string
	Uses     string
}
