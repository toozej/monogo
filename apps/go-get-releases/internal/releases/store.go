package releases

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/v89/github"
)

const manifestName = ".go-get-releases.json"

type storedAsset struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Size    int    `json:"size"`
	Digest  string `json:"digest,omitempty"`
	Updated string `json:"updated,omitempty"`
	SHA256  string `json:"sha256"`
}

type manifest struct {
	Schema     int           `json:"schema"`
	Repository string        `json:"repository"`
	ReleaseID  int64         `json:"release_id"`
	Tag        string        `json:"tag"`
	Assets     []storedAsset `json:"assets"`
}

var reservedWindowsName = regexp.MustCompile(`(?i)^(?:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`)

// component keeps ordinary tags intact and encodes path separators and reserved names.
func component(value string) string {
	encoded := url.PathEscape(value)
	// Encode backslashes for portable paths, including Windows.
	encoded = strings.ReplaceAll(encoded, "\\", "%5C")
	encoded = strings.ReplaceAll(encoded, ":", "%3A")
	if strings.HasPrefix(encoded, ".") {
		encoded = "%2E" + encoded[1:]
	}
	for strings.HasSuffix(encoded, ".") {
		encoded = encoded[:len(encoded)-1] + "%2E"
	}
	if reservedWindowsName.MatchString(encoded) {
		encoded = fmt.Sprintf("%%%02X", encoded[0]) + encoded[1:]
	}
	return encoded
}

func validAssetName(name string) bool {
	return name != "" && name != "." && name != ".." && name != manifestName && !strings.ContainsAny(name, "/\\\x00") && filepath.Base(name) == name
}

func descriptor(a *github.ReleaseAsset) storedAsset {
	updated := ""
	if a.UpdatedAt != nil {
		updated = a.UpdatedAt.String()
	}
	return storedAsset{ID: a.GetID(), Name: a.GetName(), Size: a.GetSize(), Digest: a.GetDigest(), Updated: updated}
}

func sameDescriptor(a, b storedAsset) bool {
	a.SHA256 = ""
	b.SHA256 = ""
	return a == b
}

func readManifest(root *os.Root, dir string) (manifest, error) {
	var m manifest
	data, err := root.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(data, &m)
	if err == nil && (m.Schema != 1 || m.Repository == "" || m.Tag == "" || len(m.Assets) == 0) {
		err = errors.New("invalid release manifest")
	}
	return m, err
}

func writeManifest(root *os.Root, dir string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFile(filepath.Join(dir, manifestName), append(data, '\n'), 0600)
}

func directory(root *os.Root, path string) error {
	info, err := root.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return root.Mkdir(path, 0750)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path must be a real directory: %s", path)
	}
	return nil
}

// recoverSwap restores an interrupted replacement before the next download.
func recoverSwap(root *os.Root, repoDir, repo, tag string) error {
	dest := filepath.Join(repoDir, component(tag))
	backup := filepath.Join(repoDir, ".backup-"+component(tag))
	if _, err := root.Lstat(backup); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := directory(root, backup); err != nil {
		return err
	}
	m, err := readManifest(root, backup)
	if err != nil || m.Repository != repo || m.Tag != tag {
		return fmt.Errorf("backup has no matching release manifest: %s", backup)
	}
	if _, err := root.Lstat(dest); errors.Is(err, fs.ErrNotExist) {
		return root.Rename(backup, dest)
	} else if err != nil {
		return err
	}
	if err := directory(root, dest); err != nil {
		return err
	}
	current, err := readManifest(root, dest)
	if err != nil || current.Repository != repo || current.Tag != tag {
		return fmt.Errorf("replacement has no matching release manifest: %s", dest)
	}
	return root.RemoveAll(backup)
}

func publish(root *os.Root, stage, dest, repo, tag string) error {
	_, err := root.Lstat(dest)
	if errors.Is(err, fs.ErrNotExist) {
		return root.Rename(stage, dest)
	}
	if err != nil {
		return err
	}
	if err := directory(root, dest); err != nil {
		return err
	}
	old, err := readManifest(root, dest)
	if err != nil || old.Repository != repo || old.Tag != tag {
		return fmt.Errorf("refuse to replace an unmanaged directory: %s", dest)
	}
	backup := filepath.Join(filepath.Dir(dest), ".backup-"+component(tag))
	if err := root.Rename(dest, backup); err != nil {
		return err
	}
	if err := root.Rename(stage, dest); err != nil {
		if restoreErr := root.Rename(backup, dest); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	return root.RemoveAll(backup)
}

func prune(root *os.Root, repoDir, repo string, keep map[string]bool) error {
	dir, err := root.Open(repoDir)
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || keep[entry.Name()] {
			continue
		}
		path := filepath.Join(repoDir, entry.Name())
		m, err := readManifest(root, path)
		if err != nil || m.Repository != repo || component(m.Tag) != entry.Name() {
			continue
		}
		if err := root.RemoveAll(path); err != nil {
			return fmt.Errorf("remove older release %s: %w", m.Tag, err)
		}
	}
	return nil
}
