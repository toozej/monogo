//go:build !windows

package url2anki

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
