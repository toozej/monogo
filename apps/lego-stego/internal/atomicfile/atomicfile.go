package atomicfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Write replaces path only after writeTo has successfully written and synced
// the complete new contents to a temporary file in the same directory.
func Write(path string, mode os.FileMode, writeTo func(io.Writer) error) error {
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
	if err := writeTo(temp); err != nil {
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

// WriteFile atomically replaces path with data.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	return Write(path, mode, func(w io.Writer) error {
		_, err := io.Copy(w, bytes.NewReader(data))
		return err
	})
}
