package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileIfAbsent writes content to path, creating parent folders, only when no
// file exists there. An empty path (unavailable level) writes nothing. It never
// overwrites and reports whether it created the file.
func WriteFileIfAbsent(path, content string) (created bool, err error) {
	if path == "" {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("create %s: %w", path, err)
	}
	_, werr := f.WriteString(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return false, fmt.Errorf("write %s: %w", path, werr)
	}
	return true, nil
}
