// Package converter orchestrates the notes2ssg workflow: fetch notes from a
// backend, format them for the selected static site generator, and write them
// to disk.
package converter

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/config"
	"github.com/toozej/monogo/apps/notes2ssg/internal/gotify"
	"github.com/toozej/monogo/apps/notes2ssg/internal/hugo"
	"github.com/toozej/monogo/apps/notes2ssg/internal/note"
	"github.com/toozej/monogo/apps/notes2ssg/internal/simplenote"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
	"github.com/toozej/monogo/apps/notes2ssg/internal/usememos"
	"github.com/toozej/monogo/apps/notes2ssg/internal/vite"
)

// App holds the compiled application state and dependencies.
type App struct {
	config    config.Config
	backend   backend.Backend
	formatter ssg.Formatter
	notifier  *gotify.Client
}

// NewApp builds an App from configuration, wiring up the appropriate backend
// and support services.
func NewApp(cfg config.Config) (*App, error) {
	var b backend.Backend
	switch strings.ToLower(cfg.Backend) {
	case "simplenote":
		b = simplenote.NewClient(
			cfg.SNCliPath,
			simplenote.WithCredentials(cfg.SNUsername, cfg.SNPassword),
		)
	case "usememos":
		b = usememos.NewClient(cfg.MemosURL, cfg.MemosToken, nil)
	default:
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	return newAppWithDeps(cfg, b), nil
}

// newAppWithDeps creates an App with explicit dependencies. Used by tests.
func newFormatter(cfg config.Config) ssg.Formatter {
	switch strings.ToLower(cfg.SSGType) {
	case "vite":
		return vite.NewFormatter(cfg.ViteSubtitle)
	case "hugo", "":
		return hugo.NewFormatter(cfg.Author, parseList(cfg.UnlistedTags), parseSubstitutions(cfg.TitleSubstitutions))
	default:
		return hugo.NewFormatter(cfg.Author, parseList(cfg.UnlistedTags), parseSubstitutions(cfg.TitleSubstitutions))
	}
}

func newAppWithDeps(cfg config.Config, b backend.Backend) *App {
	return &App{
		config:    cfg,
		backend:   b,
		formatter: newFormatter(cfg),
		notifier:  gotify.NewClient(cfg.GotifyURL, cfg.GotifyToken, nil),
	}
}

// Run executes a single pass of the notes2ssg workflow.
func (a *App) Run(ctx context.Context) error {
	if err := a.ensureDirs(); err != nil {
		return err
	}

	startCount := a.countOutputFiles()

	notes, err := a.backend.FetchNotes(ctx, a.config.TagToDownload)
	if err != nil {
		a.notify("notes2ssg FATAL error", fmt.Sprintf("FATAL: %v", err))
		return fmt.Errorf("fetching notes: %w", err)
	}

	processedNotes := a.processNotes(notes)

	for _, n := range processedNotes {
		if err := a.formatter.WriteFile(n, a.config.OutputDir); err != nil {
			a.notify("notes2ssg FATAL error", fmt.Sprintf("FATAL: %v", err))
			return fmt.Errorf("writing note file: %w", err)
		}
	}

	endCount := a.countOutputFiles()
	if len(processedNotes) != endCount-startCount {
		msg := fmt.Sprintf("FATAL: number of notes (%d) does not match output files written", len(processedNotes))
		a.notify("notes2ssg FATAL error", msg)
		return fmt.Errorf("parsed/output mismatch: %d notes vs %d new files", len(processedNotes), endCount-startCount)
	}

	a.notify("notes2ssg success", fmt.Sprintf("processed %d notes", len(processedNotes)))
	return nil
}

// processNotes applies business rules: continuous-note splitting and
// unlisted-tag detection.
func (a *App) processNotes(notes []backend.Note) []backend.Note {
	var result []backend.Note
	for _, n := range notes {
		// Handle continuous notes
		if a.config.ContinuousNoteTag != "" && contains(n.Tags, a.config.ContinuousNoteTag) {
			replacement := ""
			parts := strings.Split(a.config.ContinuousNoteTag, ":")
			if len(parts) > 1 {
				replacement = parts[1]
			}
			split := note.ContinuousNote(n, a.config.ContinuousNoteTag, replacement)
			for i := range split {
				split[i] = a.applyUnlisted(split[i])
			}
			result = append(result, split...)
			continue
		}
		result = append(result, a.applyUnlisted(n))
	}
	return result
}

// applyUnlisted marks a note as unlisted if any of its tags match configured unlisted tags.
func (a *App) applyUnlisted(n backend.Note) backend.Note {
	for _, ut := range parseList(a.config.UnlistedTags) {
		if contains(n.Tags, ut) {
			n.Unlisted = true
			break
		}
	}
	return n
}

// ensureDirs creates input and output directories when needed.
func (a *App) ensureDirs() error {
	if a.config.InputDir != "" {
		if err := os.MkdirAll(a.config.InputDir, 0o750); err != nil {
			return fmt.Errorf("creating input directory: %w", err)
		}
	}
	if a.config.OutputDir != "" {
		if err := os.MkdirAll(a.config.OutputDir, 0o750); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}
	return nil
}

// countOutputFiles returns the number of files in the output directory.
func (a *App) countOutputFiles() int {
	if a.config.OutputDir == "" {
		return 0
	}
	entries, err := os.ReadDir(a.config.OutputDir)
	if err != nil {
		return 0
	}
	return len(entries)
}

// notify is a convenience wrapper for Gotify notifications.
func (a *App) notify(title, message string) {
	_ = a.notifier.Send(title, message)
}

// parseList splits a comma-separated string into a slice of trimmed values.
func parseList(s string) []string {
	var result []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseSubstitutions parses a comma-separated list of find:replace pairs.
func parseSubstitutions(s string) map[string]string {
	m := make(map[string]string)
	for _, p := range strings.Split(s, ",") {
		parts := strings.SplitN(p, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

// contains returns true if slice contains item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SleepFunc matches the signature of time.Sleep.
type SleepFunc func(time.Duration)
