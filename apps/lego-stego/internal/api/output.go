package api

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path only after the complete output has been written.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(absPath), "."+filepath.Base(absPath)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, absPath); err != nil {
		return fmt.Errorf("replace output %s: %w", path, err)
	}
	return nil
}
