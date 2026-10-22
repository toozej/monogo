package obsidian

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

// Keep the state and lock names compatible with existing notes2ssg exports.
const (
	manifestName = ".notes2ssg-obsidian.json"
	lockName     = ".notes2ssg-obsidian.lock"
)

type manifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

// Export checks all conflicts before it writes notes or removes missing imports.
// Source notes control imported files. The manifest records the last exported hashes.
func (f *Formatter) Export(ctx context.Context, outputDir string, notes []backend.Note, deleteMissing bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating Obsidian export directory: %w", err)
	}
	lockPath := filepath.Join(outputDir, lockName)
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		return fmt.Errorf("cannot acquire Obsidian export lock %s; check for another exporter or a stale lock: %w", lockPath, err)
	}
	defer func() { _ = os.Remove(lockPath) }()

	state, err := readManifest(outputDir)
	if err != nil {
		return err
	}
	desired := make(map[string][]byte, len(notes))
	for _, n := range notes {
		name, err := noteFilename(n)
		if err != nil {
			return err
		}
		if _, exists := desired[name]; exists {
			return fmt.Errorf("duplicate %s note ID %q", n.Source, n.ID)
		}
		desired[name] = []byte(f.Format(n))
	}

	// Check every destination before the first write, including unchanged source notes.
	for name, content := range desired {
		if err := checkDestination(filepath.Join(outputDir, name), state.Files[name], content); err != nil {
			return err
		}
	}
	var removals []string
	if deleteMissing {
		for name, hash := range state.Files {
			if _, exists := desired[name]; exists {
				continue
			}
			if err := checkDestination(filepath.Join(outputDir, name), hash, nil); err != nil {
				return err
			}
			removals = append(removals, name)
		}
	}
	for name, content := range desired {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := checkDestination(filepath.Join(outputDir, name), state.Files[name], content); err != nil {
			return err
		}
		if err := writeIfChanged(filepath.Join(outputDir, name), content); err != nil {
			return err
		}
		state.Files[name] = digest(content)
	}
	for _, name := range removals {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := checkDestination(filepath.Join(outputDir, name), state.Files[name], nil); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing missing source note %s: %w", name, err)
		}
		delete(state.Files, name)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding Obsidian export state: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeIfChanged(filepath.Join(outputDir, manifestName), append(data, '\n')); err != nil {
		return err
	}
	fmt.Printf("Obsidian export: %d source notes, %d removed files.\n", len(notes), len(removals))
	return nil
}

// noteFilename preserves Simplenote filenames and hashes other source IDs.
// Memos IDs and Obsidian paths can contain separators and Unicode.
func noteFilename(n backend.Note) (string, error) {
	if n.Source == "simplenote" {
		return filename(n.ID)
	}
	if n.Source != "memos" && n.Source != "obsidian" {
		return "", fmt.Errorf("unsupported Obsidian export source %q", n.Source)
	}
	if n.ID == "" {
		return "", fmt.Errorf("the %s note ID is required", n.Source)
	}
	return n.Source + "-" + digest([]byte(n.ID)) + ".md", nil
}

func validFilename(name string) bool {
	for _, source := range []string{"memos", "obsidian"} {
		if id, ok := strings.CutPrefix(name, source+"-"); ok {
			id = strings.TrimSuffix(id, ".md")
			decoded, err := hex.DecodeString(id)
			return err == nil && len(decoded) == sha256.Size && source+"-"+id+".md" == name
		}
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, "simplenote-"), ".md")
	expected, err := filename(id)
	return err == nil && expected == name
}

func filename(id string) (string, error) {
	if len(id) == 0 || len(id) > 128 {
		return "", fmt.Errorf("the Simplenote note ID must contain 1 to 128 characters")
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return "", fmt.Errorf("the Simplenote note ID %q contains an unsupported character", id)
		}
	}
	return "simplenote-" + id + ".md", nil
}

func readManifest(outputDir string) (manifest, error) {
	state := manifest{Version: 1, Files: make(map[string]string)}
	data, err := readRegular(filepath.Join(outputDir, manifestName))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("reading Obsidian export state: %w", err)
	}
	state = manifest{}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("decoding Obsidian export state: %w", err)
	}
	if state.Version != 1 || state.Files == nil {
		return state, fmt.Errorf("unsupported or incomplete Obsidian export state")
	}
	for name, hash := range state.Files {
		if !validFilename(name) {
			return state, fmt.Errorf("invalid Obsidian export filename %q", name)
		}
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return state, fmt.Errorf("invalid Obsidian export hash for %q", name)
		}
	}
	return state, nil
}

func checkDestination(path, previousHash string, desired []byte) error {
	current, err := readRegular(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if desired != nil && bytes.Equal(current, desired) {
		return nil
	}
	if previousHash == "" {
		return fmt.Errorf("cannot replace an untracked Obsidian file: %s", path)
	}
	if digest(current) != previousHash {
		return fmt.Errorf("conflict in Obsidian export: %s changed in the vault. Preserve the local copy. Remove the imported file before you retry", path)
	}
	return nil
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("export to Obsidian requires a regular file: %s", path)
	}
	return os.ReadFile(path) // #nosec G304 -- Export validates manifest filenames and source IDs.
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// writeIfChanged replaces files atomically so sync clients do not read partial writes.
func writeIfChanged(path string, content []byte) error {
	existing, err := readRegular(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".notes2ssg-tmp-*")
	if err != nil {
		return fmt.Errorf("creating Obsidian export file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing Obsidian export file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing Obsidian export file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replacing Obsidian export file: %w", err)
	}
	return nil
}
