package api

import (
	"os"

	"github.com/toozej/monogo/apps/lego-stego/internal/atomicfile"
)

// WriteFileAtomic replaces path only after the complete output has been written.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	return atomicfile.WriteFile(path, data, mode)
}
