package tts

import (
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
	f, err := os.CreateTemp(filepath.Dir(absPath), "."+filepath.Base(absPath)+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary output for %s: %w", outputPath, err)
	}
	return &atomicOutput{file: f, tempPath: f.Name(), finalPath: absPath}, nil
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
