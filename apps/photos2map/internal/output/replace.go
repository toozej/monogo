//go:build !windows

package output

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
