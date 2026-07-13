package output

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

type atomicFile struct {
	*os.File
	target string
}

func createAtomic(target string) (*atomicFile, func() error, error) {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("create output directory: %w", err)
	}

	tmp, err := createTemporaryFile(dir)
	if err != nil {
		return nil, nil, err
	}
	if info, statErr := os.Stat(target); statErr == nil && info.Mode().IsRegular() {
		// os.Create, which this helper replaces, preserves an existing file's mode.
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return nil, nil, err
		}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, nil, statErr
	}

	file := &atomicFile{File: tmp, target: target}
	commit := func() error {
		if err := file.Sync(); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := replaceFile(file.Name(), file.target); err != nil {
			return err
		}
		return nil
	}
	return file, commit, nil
}

func createTemporaryFile(dir string) (*os.File, error) {
	for {
		path := filepath.Join(dir, ".photos2map-"+rand.Text())
		// #nosec G304 G302 -- dir is the output target's parent, the basename is
		// cryptographically random, O_EXCL prevents replacement, and 0666 is
		// intentionally filtered through the user's umask to match os.Create.
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if os.IsExist(err) {
			continue
		}
		return file, err
	}
}

func (f *atomicFile) abort() {
	_ = f.Close()
	_ = os.Remove(f.Name())
}
