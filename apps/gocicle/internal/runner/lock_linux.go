//go:build linux

package runner

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockState(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600) // #nosec G304 -- The operator selects the private runner state directory.
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}
