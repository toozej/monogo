// Package config defines the notes2ssg application configuration.
//
// The struct lives with the app, while the loading mechanics (.env discovery,
// path-traversal protection, and environment parsing) are provided by the
// shared github.com/toozej/monogo/pkg/config package.
package config

import sharedconfig "github.com/toozej/monogo/pkg/config"

// Config represents the notes2ssg application configuration. Fields are tagged
// with the environment variable names used to populate them.
type Config struct {
	// Backend specifies the notes backend to use. Supported values are
	// "simplenote" and "usememos". Defaults to "simplenote".
	Backend string `env:"BACKEND" envDefault:"simplenote"`

	// SNUsername and SNPassword are credentials for the Simplenote backend.
	SNUsername string `env:"SN_USERNAME"`
	SNPassword string `env:"SN_PASSWORD"`
	// SNCliPath is the path to the sncli binary. Defaults to "sncli".
	SNCliPath string `env:"SN_CLI_PATH" envDefault:"sncli"`

	// MemosURL is the base URL of the Usememos instance.
	MemosURL string `env:"MEMOS_URL"`
	// MemosToken is the API token for the Usememos instance.
	MemosToken string `env:"MEMOS_TOKEN"`

	// TagToDownload is the tag used to filter notes for downloading/exporting.
	TagToDownload string `env:"TAG_TO_DOWNLOAD"`
	// ContinuousNoteTag is the tag that identifies a continuous note which
	// should be split into individual notes per line.
	ContinuousNoteTag string `env:"CONTINUOUS_NOTE_TAG"`
	// UnlistedTags is a comma-separated list of tags that should cause notes
	// to be marked as unlisted in the generated front matter.
	UnlistedTags string `env:"UNLISTED_TAGS"`
	// TitleSubstitutions is a comma-separated list of find:replace pairs used
	// to compute the summary field.
	TitleSubstitutions string `env:"TITLE_SUBSTITUTIONS"`

	// ViteSubtitle is used to populate the subtitle field in go-vite templates.
	ViteSubtitle string `env:"VITE_SUBTITLE"`

	// SSGType is the static site generator type. Defaults to "hugo".
	SSGType string `env:"SSG_TYPE" envDefault:"hugo"`
	// InputDir is the directory for temporary/raw input files.
	InputDir string `env:"INPUT_DIR"`
	// OutputDir is the directory where generated Markdown files are written.
	OutputDir string `env:"OUTPUT_DIR"`
	// Author is the author name used in front matter. Defaults to "root".
	Author string `env:"AUTHOR" envDefault:"root"`
	// PollingCycle is the sleep duration between runs in seconds. Defaults to 3600.
	PollingCycle int `env:"POLLING_CYCLE" envDefault:"3600"`
	// Debug enables debug-level logging when true.
	Debug bool `env:"DEBUG" envDefault:"false"`

	// GotifyURL and GotifyToken are used to send push notifications.
	GotifyURL   string `env:"GOTIFY_URL"`
	GotifyToken string `env:"GOTIFY_TOKEN"`
}

// GetEnvVars loads and returns theificent, returning any error to the caller.
func GetEnvVars() Config {
	return sharedconfig.MustLoad[Config]()
}

// Load loads and returns the application configuration, returning any error to
// the caller instead of exiting.
func Load() (Config, error) {
	return sharedconfig.Load[Config]()
}
