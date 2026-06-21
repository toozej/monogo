package workflow

import "context"

type RemoteContentFetcher interface {
	GetRemoteWorkflowContents(ctx context.Context, ownerRepo, ref string) (map[string]string, error)
}

type ActionRef struct {
	OwnerRepo  string
	ActionPath string
	Version    string
	FullRef    string
}

func (a ActionRef) Key() string {
	if a.ActionPath == "" {
		return a.OwnerRepo + "@" + a.Version
	}
	return a.OwnerRepo + "/" + a.ActionPath + "@" + a.Version
}

type WorkflowFile struct {
	Path             string
	Uses             []string
	UsesWithVersions []ActionRef
	Error            error
}

type WorkflowParser struct{}
