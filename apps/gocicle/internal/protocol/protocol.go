// Package protocol defines the runner API payloads.
package protocol

import (
	"encoding/json"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
)

const HeartbeatInterval = 15 * time.Second
const LeaseDuration = 60 * time.Second
const MaxLogBytes = 10 << 20

type Assignment struct {
	InspectionID  string                `json:"inspectionID,omitempty"`
	RunID         string                `json:"runID"`
	LeaseToken    string                `json:"leaseToken"`
	ExpiresAt     time.Time             `json:"expiresAt"`
	Deadline      time.Time             `json:"deadline"`
	Spec          jobs.Spec             `json:"spec"`
	Source        jobs.Source           `json:"source"`
	Preferences   jobs.Preferences      `json:"preferences"`
	Credentials   map[string]Credential `json:"credentials"`
	ApprovedRoots []string              `json:"approvedRoots"`
}
type Credential struct {
	Value      string `json:"value,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	KnownHosts string `json:"knownHosts,omitempty"`
	Repository string `json:"repository,omitempty"`
}
type Event struct {
	Sequence int64           `json:"sequence"`
	Kind     string          `json:"kind"`
	Data     json.RawMessage `json:"data"`
}
type Completion struct {
	Status        string  `json:"status"`
	ExitCode      int     `json:"exitCode"`
	SourceCommit  string  `json:"sourceCommit"`
	ImageIdentity string  `json:"imageIdentity"`
	PushResult    string  `json:"pushResult"`
	Duration      float64 `json:"duration"`
	Error         string  `json:"error,omitempty"`
}
type Metrics struct {
	Available bool      `json:"available"`
	CPU       *float64  `json:"cpu,omitempty"`
	Memory    *uint64   `json:"memory,omitempty"`
	NetworkRX *uint64   `json:"networkRX,omitempty"`
	NetworkTX *uint64   `json:"networkTX,omitempty"`
	At        time.Time `json:"at"`
}
