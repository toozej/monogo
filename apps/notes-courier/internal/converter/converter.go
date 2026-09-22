// Package converter orchestrates the notes-courier workflow: fetch notes from a
// backend, format them for the selected static site generator, and write them
// to disk.
package converter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"github.com/toozej/monogo/apps/notes-courier/internal/config"
	"github.com/toozej/monogo/apps/notes-courier/internal/gotify"
	"github.com/toozej/monogo/apps/notes-courier/internal/hugo"
	"github.com/toozej/monogo/apps/notes-courier/internal/note"
	"github.com/toozej/monogo/apps/notes-courier/internal/obsidian"
	"github.com/toozej/monogo/apps/notes-courier/internal/simplenote"
	"github.com/toozej/monogo/apps/notes-courier/internal/ssg"
	"github.com/toozej/monogo/apps/notes-courier/internal/usememos"
	"github.com/toozej/monogo/apps/notes-courier/internal/vite"
)

const outputManifestName = ".notes-courier-manifest.json"
const legacyOutputManifestName = ".notes2ssg-manifest.json"

type outputManifest struct {
	Files []string `json:"files"`
}

// App holds the compiled application state and dependencies.
type App struct {
	config      config.Config
	backend     backend.Backend
	destination backend.Destination
	formatter   ssg.Formatter
	notifier    *gotify.Client
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
	case "memos":
		b = usememos.NewClient(cfg.MemosURL, cfg.MemosToken, nil)
	case "obsidian":
		client, err := obsidian.NewClient(cfg.ObsidianVaultPath, cfg.ObsidianFolder)
		if err != nil {
			return nil, err
		}
		b = client
	default:
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	app := newAppWithDeps(cfg, b)
	switch destinationType(cfg) {
	case "hugo", "vite":
	case "memos", "usememos":
		client := usememos.NewClient(cfg.MemosURL, cfg.MemosToken, nil)
		visibility := cfg.MemosVisibility
		if visibility == "" {
			visibility = "PRIVATE"
		}
		client.SetVisibility(strings.ToUpper(visibility))
		app.destination = client
	case "obsidian":
		client, err := obsidian.NewClient(cfg.ObsidianVaultPath, cfg.ObsidianFolder)
		if err != nil {
			return nil, err
		}
		app.destination = client
	default:
		return nil, fmt.Errorf("unsupported destination: %s", destinationType(cfg))
	}
	return app, nil
}

func destinationType(cfg config.Config) string {
	if cfg.Destination != "" {
		return strings.ToLower(cfg.Destination)
	}
	if cfg.SSGType != "" {
		return strings.ToLower(cfg.SSGType)
	}
	return "hugo"
}

// newAppWithDeps creates an App with explicit dependencies. Used by tests.
func newFormatter(cfg config.Config) ssg.Formatter {
	switch destinationType(cfg) {
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

// Run executes a single pass of the notes-courier workflow.
func (a *App) Run(ctx context.Context) error {
	if a.destination != nil {
		notes, err := a.backend.FetchNotes(ctx, a.config.TagToDownload)
		if err != nil {
			return fmt.Errorf("fetching notes: %w", err)
		}
		if len(notes) == 0 {
			return fmt.Errorf("source returned no notes; destination was not changed")
		}
		processed := assignOutputSlugs(a.processNotes(notes))
		for i := range processed {
			id := processed[i].ID
			if id == "" {
				id = processed[i].Title + "\x00" + processed[i].Date.UTC().Format(time.RFC3339Nano)
			}
			processed[i].ID = strings.ToLower(a.config.Backend) + ":" + id
		}
		if err := a.destination.WriteNotes(ctx, processed); err != nil {
			return fmt.Errorf("writing notes: %w", err)
		}
		a.notify("notes-courier success", fmt.Sprintf("processed %d notes", len(processed)))
		return nil
	}
	if strings.TrimSpace(a.config.OutputDir) == "" {
		return fmt.Errorf("output directory is required")
	}
	if err := a.ensureDirs(); err != nil {
		return err
	}

	previousFiles, err := readOutputManifest(a.config.OutputDir)
	if err != nil {
		return err
	}

	notes, err := a.backend.FetchNotes(ctx, a.config.TagToDownload)
	if err != nil {
		a.notify("notes-courier FATAL error", fmt.Sprintf("FATAL: %v", err))
		return fmt.Errorf("fetching notes: %w", err)
	}

	processedNotes := assignOutputSlugs(a.processNotes(notes))
	currentFiles := outputFiles(processedNotes)

	for _, n := range processedNotes {
		if err := a.formatter.WriteFile(n, a.config.OutputDir); err != nil {
			a.notify("notes-courier FATAL error", fmt.Sprintf("FATAL: %v", err))
			return fmt.Errorf("writing note file: %w", err)
		}
	}

	if err := removeStaleOutputFiles(a.config.OutputDir, previousFiles, currentFiles); err != nil {
		a.notify("notes-courier FATAL error", fmt.Sprintf("FATAL: %v", err))
		return err
	}
	if err := writeOutputManifest(a.config.OutputDir, currentFiles); err != nil {
		a.notify("notes-courier FATAL error", fmt.Sprintf("FATAL: %v", err))
		return err
	}
	if err := os.Remove(filepath.Join(a.config.OutputDir, legacyOutputManifestName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing old output manifest: %w", err)
	}

	a.notify("notes-courier success", fmt.Sprintf("processed %d notes", len(processedNotes)))
	return nil
}

// RunPolling exports notes until ctx is canceled. A zero or negative polling
// cycle runs one export pass and returns.
func (a *App) RunPolling(ctx context.Context) error {
	for {
		if err := a.Run(ctx); err != nil {
			return err
		}
		if a.config.PollingCycle <= 0 {
			return nil
		}

		timer := time.NewTimer(time.Duration(a.config.PollingCycle) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// processNotes applies business rules: continuous-note splitting and
// unlisted-tag detection.
func (a *App) processNotes(notes []backend.Note) []backend.Note {
	var result []backend.Note
	continuousTags := parseList(a.config.ContinuousNoteTag)
	for _, n := range notes {
		continuousTag, replacement, ok := matchingContinuousTag(n.Tags, continuousTags)
		if ok {
			split := note.ContinuousNote(n, continuousTag, replacement)
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

// matchingContinuousTag returns the first configured continuous-note tag in
// tags. The text after the first colon becomes the generated category.
func matchingContinuousTag(tags, continuousTags []string) (string, string, bool) {
	for _, continuousTag := range continuousTags {
		if !contains(tags, continuousTag) {
			continue
		}
		parts := strings.SplitN(continuousTag, ":", 2)
		replacement := ""
		if len(parts) == 2 {
			replacement = parts[1]
		}
		return continuousTag, replacement, true
	}
	return "", "", false
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

// parseSubstitutions parses ordered find:replace pairs.
func parseSubstitutions(s string) []hugo.Substitution {
	var substitutions []hugo.Substitution
	for _, p := range strings.Split(s, ",") {
		parts := strings.SplitN(p, ":", 2)
		if len(parts) != 2 {
			continue
		}
		find := strings.TrimSpace(parts[0])
		if find != "" {
			substitutions = append(substitutions, hugo.Substitution{
				Find:    find,
				Replace: strings.TrimSpace(parts[1]),
			})
		}
	}
	return substitutions
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

// assignOutputSlugs gives every note a non-empty, collision-free output slug.
func assignOutputSlugs(notes []backend.Note) []backend.Note {
	result := append([]backend.Note(nil), notes...)
	groups := make(map[string][]int)
	for i := range result {
		base := ssg.Slugify(result[i].Title)
		if base == "" {
			base = "note-" + noteHash(result[i])
		}
		groups[base] = append(groups[base], i)
	}

	for base, indexes := range groups {
		if len(indexes) == 1 {
			result[indexes[0]].Slug = base
			continue
		}
		sort.Slice(indexes, func(i, j int) bool {
			left := noteHash(result[indexes[i]])
			right := noteHash(result[indexes[j]])
			if left == right {
				return indexes[i] < indexes[j]
			}
			return left < right
		})
		seen := make(map[string]int)
		for _, index := range indexes {
			slug := base + "-" + noteHash(result[index])
			seen[slug]++
			if seen[slug] > 1 {
				slug = fmt.Sprintf("%s-%d", slug, seen[slug])
			}
			result[index].Slug = slug
		}
	}
	return result
}

func noteHash(note backend.Note) string {
	value := note.Title + "\x00" + note.Date.UTC().Format(time.RFC3339Nano) + "\x00" + strings.Join(note.Tags, "\x00") + "\x00" + note.Content
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:4])
}

func outputFiles(notes []backend.Note) map[string]struct{} {
	files := make(map[string]struct{}, len(notes))
	for _, note := range notes {
		files[ssg.SlugFor(note)+".md"] = struct{}{}
	}
	return files
}

func readOutputManifest(outputDir string) (map[string]struct{}, error) {
	path := filepath.Join(outputDir, outputManifestName)
	data, err := os.ReadFile(path) // #nosec G304 -- the path combines OUTPUT_DIR with a constant manifest name.
	if errors.Is(err, os.ErrNotExist) {
		path = filepath.Join(outputDir, legacyOutputManifestName)
		data, err = os.ReadFile(path) // #nosec G304 -- the path combines OUTPUT_DIR with a constant legacy manifest name.
	}
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]struct{}), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading output manifest: %w", err)
	}

	var manifest outputManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing output manifest: %w", err)
	}
	files := make(map[string]struct{}, len(manifest.Files))
	for _, file := range manifest.Files {
		if !isGeneratedFile(file) {
			return nil, fmt.Errorf("invalid output manifest file: %q", file)
		}
		files[file] = struct{}{}
	}
	return files, nil
}

func removeStaleOutputFiles(outputDir string, previous, current map[string]struct{}) error {
	for file := range previous {
		if _, ok := current[file]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(outputDir, file)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing stale output file %q: %w", file, err)
		}
	}
	return nil
}

func writeOutputManifest(outputDir string, files map[string]struct{}) error {
	names := make([]string, 0, len(files))
	for file := range files {
		names = append(names, file)
	}
	sort.Strings(names)
	data, err := json.Marshal(outputManifest{Files: names})
	if err != nil {
		return fmt.Errorf("encoding output manifest: %w", err)
	}

	path := filepath.Join(outputDir, outputManifestName)
	temporary, err := os.CreateTemp(outputDir, outputManifestName+"-*")
	if err != nil {
		return fmt.Errorf("creating output manifest: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing output manifest: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("setting output manifest permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing output manifest: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replacing output manifest: %w", err)
	}
	return nil
}

func isGeneratedFile(file string) bool {
	return file != ".md" && filepath.Base(file) == file && filepath.Ext(file) == ".md"
}
