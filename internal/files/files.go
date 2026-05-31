package files

import (
	"os"
	"path/filepath"
)

func EnsureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
