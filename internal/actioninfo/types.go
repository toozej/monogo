package actioninfo

import "time"

type OutdatedActionInfo struct {
	OwnerRepo  string
	ActionPath string
	CurrentRef string
	LatestTag  string
	LatestURL  string
	Workflow   string
	FullRef    string
}

type StaleActionInfo struct {
	OwnerRepo          string
	FullRef            string
	Workflow           string
	Deprecated         bool
	DeprecationMessage string
	LastUpdated        time.Time
	StaleByAge         bool
}

type RuntimeEOLActionInfo struct {
	OwnerRepo string
	FullRef   string
	Workflow  string
	Runtime   string
	Version   string
	EOLDate   time.Time
}

type FileUpdate struct {
	OldUse string
	NewUse string
}

type OutdatedUpdateFailure struct {
	WorkflowFile string
	OldUse       string
	NewUse       string
	Reason       string
}

type OutdatedUpdateReport struct {
	UpdatedByFile map[string][]FileUpdate
	FailedUpdates []OutdatedUpdateFailure
}

const DefaultStaleDays = 365
const MaxStaleDays = 3650
