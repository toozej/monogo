package output

import (
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

	tmp, err := os.CreateTemp(dir, ".photos2map-*")
	if err != nil {
		return nil, nil, err
	}
	file := &atomicFile{File: tmp, target: target}
	commit := func() error {
		if err := file.Sync(); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(file.Name(), target); err != nil {
			return err
		}
		return nil
	}
	return file, commit, nil
}

func (f *atomicFile) abort() {
	_ = f.Close()
	_ = os.Remove(f.Name())
}
