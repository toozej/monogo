package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v85/github"
	"golang.org/x/oauth2"

	log "github.com/sirupsen/logrus"
)

type TestIssueFunc func(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error

type testIssueCreator struct {
	fn TestIssueFunc
}

func (t *testIssueCreator) CreateArchivedActionIssue(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
	return t.fn(ctx, owner, repo, archivedActions)
}

func NewTestIssueCreator(fn TestIssueFunc) IssueCreatorIface {
	return &testIssueCreator{fn: fn}
}

func NewIssueCreator(token string) *IssueCreator {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &IssueCreator{
		token:  token,
		client: github.NewClient(tc),
	}
}

func (ic *IssueCreator) CreateArchivedActionIssue(ctx context.Context, owner, repo string, archivedActions []ArchivedActionInfo) error {
	if len(archivedActions) == 0 {
		return nil
	}

	title := "Replace archived GitHub Actions"
	body := BuildIssueBody(archivedActions)
	labels := []string{"maintenance", "github-actions", "security"}

	issueReq := &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}

	issue, resp, err := ic.client.Issues.Create(ctx, owner, repo, issueReq)
	if err != nil {
		if resp != nil && resp.StatusCode == 422 {
			log.Warnf("Issue may already exist in %s/%s", owner, repo)
			return nil
		}
		return fmt.Errorf("failed to create issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned status %d when creating issue", resp.StatusCode)
	}

	log.Infof("Successfully created GitHub issue #%d in %s/%s", issue.GetNumber(), owner, repo)
	return nil
}

func BuildIssueBody(actions []ArchivedActionInfo) string {
	var body strings.Builder

	body.WriteString("## Archived GitHub Actions Detected\n\n")
	body.WriteString("This repository uses the following GitHub Actions that have been archived by their maintainers:\n\n")

	for _, action := range actions {
		_, _ = fmt.Fprintf(&body, "- `%s` (used in `%s`)\n", action.Uses, action.Workflow)
	}

	body.WriteString("\n## What does this mean?\n\n")
	body.WriteString("Archived actions are no longer maintained and may:\n")
	body.WriteString("- Contain security vulnerabilities\n")
	body.WriteString("- Stop working in future GitHub updates\n")
	body.WriteString("- Not receive bug fixes\n\n")

	body.WriteString("## Recommended Actions\n\n")
	body.WriteString("1. **Review each archived action** and find actively maintained alternatives\n")
	body.WriteString("2. **Test thoroughly** after replacing actions\n")
	body.WriteString("3. **Update your workflows** to use the new actions\n\n")

	body.WriteString("## Resources\n\n")
	body.WriteString("- [GitHub Actions Marketplace](https://github.com/marketplace?type=actions)\n")
	body.WriteString("- [Awesome Actions](https://github.com/sdras/awesome-actions)\n\n")

	body.WriteString("---\n\n")
	body.WriteString("*This issue was automatically created by [go-sort-out-gh-actions](https://github.com/toozej/monogo/apps/go-sort-out-gh-actions)*\n")

	return body.String()
}
