package github

import (
	"net/http"
	"sync"
	"time"

	"github.com/toozej/go-find-archived-gh-actions/internal/runtime"
)

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
	eolClient  *runtime.EOLClient

	archivedCache map[string]bool
	releaseCache  map[string]*ReleaseInfo
	refSHACache   map[string]string
	repoInfoCache map[string]*RepoInfo
	cacheMu       sync.RWMutex
}

type RepoInfo struct {
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	Archived       bool   `json:"archived"`
	Private        bool   `json:"private"`
	HTMLURL        string `json:"html_url"`
	Owner          Owner  `json:"owner"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	PushedAt       string `json:"pushed_at"`
	Deprecated     bool   `json:"deprecated"`
	DeprecationMsg string `json:"deprecation_warning_message"`
}

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type Owner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type RateLimitInfo struct {
	Limit     int
	Remaining int
	Used      int
	Reset     time.Time
	Resource  string
}

type RefInfo struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		URL  string `json:"url"`
		Type string `json:"type"`
	} `json:"object"`
}

type StaleInfo struct {
	Repo               string
	Deprecated         bool
	DeprecationMessage string
	LastUpdated        time.Time
	StaleByAge         bool
}

type RuntimeEOLResult struct {
	OwnerRepo string
	Runtime   string
	Version   string
	EOLDate   time.Time
	IsEOL     bool
}

type actionYML struct {
	Runs struct {
		Using string `yaml:"using"`
	} `yaml:"runs"`
}
