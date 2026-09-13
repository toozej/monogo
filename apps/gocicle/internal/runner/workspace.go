// Package runner executes leased work on Linux hosts.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	containerruntime "github.com/toozej/monogo/apps/gocicle/internal/runtime"
)

type Workspace struct {
	Root, Path, GitDir, Commit, Branch, URL string
	env                                     []string
	local                                   bool
}

func Prepare(ctx context.Context, state string, a protocol.Assignment) (*Workspace, error) {
	root, err := os.MkdirTemp(state, "workspace-")
	if err != nil {
		return nil, err
	}
	w := &Workspace{Root: root, Path: filepath.Join(root, "source"), GitDir: filepath.Join(root, "git")}
	ok := false
	defer func() {
		if !ok {
			_ = w.Close()
		}
	}()
	if a.Source.Local != nil {
		w.local = true
		target, err := filepath.EvalSymlinks(a.Source.Local.HostPath)
		if err != nil {
			return nil, err
		}
		approved := false
		for _, root := range a.ApprovedRoots {
			base, err := filepath.EvalSymlinks(root)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(base, target)
			if err == nil {
				resolved, err := containerruntime.Resolve(base, rel)
				if err == nil && resolved == target {
					approved = true
				}
			}
		}
		if !approved {
			return nil, errors.New("local source is outside approved roots")
		}
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			return nil, errors.New("local source must be a directory")
		}
		resolvedState, err := filepath.EvalSymlinks(state)
		if err != nil {
			return nil, err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(home); err == nil {
			home = resolved
		}
		if containerruntime.ContainsPath(target, home) || containerruntime.ContainsPath(target, resolvedState) || containerruntime.ContainsPath(resolvedState, target) {
			return nil, errors.New("local source must exclude the runner home and state directories")
		}
		w.Path = target
		ok = true
		return w, nil
	}
	git := a.Source.Git
	w.Branch = git.Branch
	w.URL = git.URL
	w.env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1"}
	if git.Credential != "" {
		credential, exists := a.Credentials[git.Credential]
		if !exists || credential.Repository != git.URL {
			return nil, errors.New("repository credential scope does not match the source")
		}
		if strings.HasPrefix(git.URL, "ssh://") {
			if credential.PrivateKey == "" || credential.KnownHosts == "" {
				return nil, errors.New("SSH requires a private key and verified known_hosts entries")
			}
			key := filepath.Join(root, "key")
			hosts := filepath.Join(root, "known_hosts")
			if err := os.WriteFile(key, []byte(credential.PrivateKey), 0600); err != nil {
				return nil, err
			}
			if err := os.WriteFile(hosts, []byte(credential.KnownHosts), 0600); err != nil {
				return nil, err
			}
			ssh := "ssh -F /dev/null -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=" + quote(hosts) + " -i " + quote(key)
			w.env = append(w.env, "GIT_SSH_COMMAND="+ssh)
		} else {
			askpass := filepath.Join(root, "askpass")
			script := "#!/bin/sh\ncase \"$1\" in *Username*) printf '%s\\n' \"$GOCICLE_GIT_USER\" ;; *) printf '%s\\n' \"$GOCICLE_GIT_PASSWORD\" ;; esac\n"
			if err := os.WriteFile(askpass, []byte(script), 0700); err != nil { // #nosec G306 -- Git must execute this helper. Only its owner can access it.
				return nil, err
			}
			w.env = append(w.env, "GIT_ASKPASS="+askpass, "GOCICLE_GIT_USER="+credential.Username, "GOCICLE_GIT_PASSWORD="+credential.Password)
		}
	} else if strings.HasPrefix(git.URL, "ssh://") {
		return nil, errors.New("SSH source requires an explicit credential")
	}
	if err := w.clone(ctx); err != nil {
		return nil, err
	}
	ok = true
	return w, nil
}
func (w *Workspace) git(ctx context.Context, args ...string) (string, error) {
	base := []string{"-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never", "-c", "commit.gpgsign=false"}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...) // #nosec G204 -- Git receives an argument array. Validated repository values follow an explicit option terminator.
	cmd.Env = w.env
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("git operation failed")
	}
	return string(out), nil
}
func (w *Workspace) Push(ctx context.Context, name, email string) (string, error) {
	if w.local {
		return "disabled", errors.New("local sources cannot push")
	}
	// Ignore any new .git file. Git metadata stays outside the mounted source.
	if _, err := w.git(ctx, "add", "--all", "--", ":/", ":(exclude).git"); err != nil {
		return "failed", err
	}
	diff, err := w.git(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return "failed", err
	}
	if strings.TrimSpace(diff) == "" {
		return "unchanged", nil
	}
	if strings.ContainsAny(name+email, "\n\r\x00") {
		return "failed", errors.New("commit identity is invalid")
	}
	if _, err := w.git(ctx, "-c", "user.name="+name, "-c", "user.email="+email, "commit", "-m", "gocicle: apply job changes"); err != nil {
		return "failed", err
	}
	// The URL and branch come from the immutable assignment. Push never reads them from mounted files.
	if _, err := w.git(ctx, "push", "--porcelain", "--", w.URL, "HEAD:refs/heads/"+w.Branch); err != nil {
		return "rejected", errors.New("push failed: the branch or credentials may have changed")
	}
	return "pushed", nil
}
func (w *Workspace) Close() error                  { return os.RemoveAll(w.Root) }
func quote(value string) string                    { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func workspaceError(stage string, err error) error { return fmt.Errorf("%s failed: %w", stage, err) }

func (w *Workspace) clone(ctx context.Context) error {
	if _, err := w.git(ctx, "clone", "--no-local", "--single-branch", "--branch", w.Branch, "--separate-git-dir", w.GitDir, "--", w.URL, w.Path); err != nil {
		return errors.New("git clone failed: check the branch and repository credentials")
	}
	if err := os.Remove(filepath.Join(w.Path, ".git")); err != nil {
		return err
	}
	w.env = append(w.env, "GIT_DIR="+w.GitDir, "GIT_WORK_TREE="+w.Path)
	commit, err := w.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	w.Commit = strings.TrimSpace(commit)
	return nil
}
