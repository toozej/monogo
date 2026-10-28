package lister

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func copyMatches(ctx context.Context, options Options, result *Result) error {
	source, err := filepath.EvalSymlinks(options.Source)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source must be a directory")
	}
	if err = os.MkdirAll(options.Destination, 0o750); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	destination, err := filepath.EvalSymlinks(options.Destination)
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	if source == destination {
		return fmt.Errorf("source and destination must differ")
	}
	wanted := make(map[string]bool, len(result.Filenames))
	for _, name := range result.Filenames {
		wanted[name] = true
	}
	matches := make(map[string][]string)
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && path == destination {
			return filepath.SkipDir
		}
		// WalkDir does not follow directory symlinks. Skip file symlinks too.
		if !wanted[entry.Name()] || !entry.Type().IsRegular() {
			return nil
		}
		matches[entry.Name()] = append(matches[entry.Name()], path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("search source: %w", err)
	}
	var failures []error
	for _, name := range result.Filenames {
		if err := ctx.Err(); err != nil {
			return err
		}
		paths := matches[name]
		switch len(paths) {
		case 0:
			result.Missing = append(result.Missing, name)
		case 1:
			err := copyFile(ctx, paths[0], filepath.Join(destination, name), options.NoOverwrite)
			switch {
			case options.NoOverwrite && errors.Is(err, fs.ErrExist):
				result.Skipped = append(result.Skipped, name)
			case err != nil:
				failures = append(failures, fmt.Errorf("copy %q: %w", name, err))
			default:
				result.Copied = append(result.Copied, name)
			}
		default:
			result.Ambiguous = append(result.Ambiguous, name)
		}
	}
	return errors.Join(failures...)
}

func copyFile(ctx context.Context, source, destination string, noOverwrite bool) error {
	// OpenRoot prevents a replaced source symlink from escaping the source parent.
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	input, err := root.Open(filepath.Base(source))
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular file")
	}
	if target, err := os.Stat(destination); err == nil && os.SameFile(info, target) {
		return fmt.Errorf("source and destination refer to the same file")
	}
	return atomicWrite(destination, func(output *os.File) error {
		buffer := make([]byte, 128*1024)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, readErr := input.Read(buffer)
			if n > 0 {
				if _, err := output.Write(buffer[:n]); err != nil {
					return err
				}
			}
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			if readErr != nil {
				return readErr
			}
		}
	}, noOverwrite)
}

func atomicWrite(path string, write func(*os.File) error, noOverwrite bool) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	temporaryName := ".gphotos-lister-" + hex.EncodeToString(random)
	temporary, err := root.OpenFile(temporaryName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporaryName); _ = temporary.Close() }()
	if err := write(temporary); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	target := filepath.Base(path)
	if noOverwrite {
		reservation, err := root.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if err := reservation.Close(); err != nil {
			_ = root.Remove(target)
			return err
		}
		if err := root.Rename(temporaryName, target); err != nil {
			_ = root.Remove(target)
			return err
		}
		return nil
	}
	// Rename replaces the destination entry itself, including a destination symlink.
	return root.Rename(temporaryName, target)
}
