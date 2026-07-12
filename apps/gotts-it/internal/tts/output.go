package tts

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type atomicOutput struct {
	file      *os.File
	tempPath  string
	finalPath string
	committed bool
}

func newAtomicOutput(outputPath string) (*atomicOutput, error) {
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path %s: %w", outputPath, err)
	}
	var existingMode os.FileMode
	existing := false
	info, statErr := os.Stat(absPath)
	if statErr == nil {
		existingMode = info.Mode().Perm()
		existing = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect output %s: %w", outputPath, statErr)
	}

	f, err := createAtomicTemp(filepath.Dir(absPath))
	if err != nil {
		return nil, fmt.Errorf("create temporary output for %s: %w", outputPath, err)
	}
	if existing {
		if err := f.Chmod(existingMode); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return nil, fmt.Errorf("preserve output permissions for %s: %w", outputPath, err)
		}
	}
	return &atomicOutput{file: f, tempPath: f.Name(), finalPath: absPath}, nil
}

func createAtomicTemp(dir string) (*os.File, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		name := ".gotts-it-" + hex.EncodeToString(random[:])
		// #nosec G302 -- audio outputs intentionally use normal 0666-under-umask creation permissions.
		f, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not allocate a unique temporary filename")
}

func (o *atomicOutput) commit() error {
	if err := o.file.Sync(); err != nil {
		return fmt.Errorf("sync temporary output: %w", err)
	}
	if err := o.file.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := os.Rename(o.tempPath, o.finalPath); err != nil {
		return fmt.Errorf("replace output %s: %w", o.finalPath, err)
	}
	o.committed = true
	return nil
}

func (o *atomicOutput) abort() {
	if o == nil || o.committed {
		return
	}
	_ = o.file.Close()
	_ = os.Remove(o.tempPath)
}
