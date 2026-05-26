package workflow

type ActionRef struct {
	OwnerRepo string
	Subpath   string
	Version   string
	FullRef   string
}

type WorkflowFile struct {
	Path             string
	Uses             []string
	UsesWithVersions []ActionRef
	Error            error
}

type WorkflowParser struct{}
