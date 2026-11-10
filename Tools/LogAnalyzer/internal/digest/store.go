package digest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// Store implements domain.DigestStore on the local filesystem.
type Store struct{}

var _ domain.DigestStore = (*Store)(nil)

// NewStore returns a Store.
func NewStore() *Store { return &Store{} }

// Discover lists digest files at path: the file itself, or the direct children
// of a directory, sorted ascending. Anything else yields nil.
func (s *Store) Discover(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.Mode().IsRegular() {
		if domain.IsDigestFileName(path) {
			return []string{path}
		}
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !domain.IsDigestFileName(name) {
			continue
		}
		child := filepath.Join(path, name)
		if fi, err := os.Stat(child); err == nil && fi.Mode().IsRegular() {
			found = append(found, child)
		}
	}
	sort.Strings(found)
	return found
}

// Read decodes one digest file. It errors only when ctx is cancelled.
func (s *Store) Read(ctx context.Context, path string) (domain.Digest, []domain.Finding, error) {
	if err := ctx.Err(); err != nil {
		return domain.Digest{}, nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		finding := loadFinding(invalidf("file cannot be read"), path)
		return domain.Digest{}, []domain.Finding{finding}, nil
	}
	d, findings := decode(raw, path)
	return d, findings, nil
}

// Write encodes d to path. See domain.DigestStore.Write.
func (s *Store) Write(ctx context.Context, path string, d domain.Digest, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s: %w", path, domain.ErrDigestExists)
		}
	}
	data, err := encode(d)
	if err != nil {
		return fmt.Errorf("encode digest: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create digest directory: %w", err)
	}
	tmp, err := writeTemp(dir, data)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return publish(tmp, path, overwrite)
}

// writeTemp writes data to a new temporary file in dir and returns its name.
func writeTemp(dir string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, ".digest-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary digest file: %w", err)
	}
	name := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("write digest file: %w", err)
	}
	return name, nil
}

// publish moves the finished temporary file to path. Without overwrite it never
// replaces an existing file, even one that appeared since the first check.
func publish(tmp, path string, overwrite bool) error {
	if overwrite {
		if err := os.Rename(tmp, path); err != nil {
			return fmt.Errorf("replace digest file: %w", err)
		}
		return nil
	}
	if err := os.Link(tmp, path); err == nil {
		return nil
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return fmt.Errorf("%s: %w", path, domain.ErrDigestExists)
	}
	// The filesystem cannot hard-link; the existence check above still holds.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write digest file: %w", err)
	}
	return nil
}
