package guidance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mosaic-run/internal/domain"
)

type fileStore struct{}

// NewFileStore returns the file-backed domain.GuidanceStore.
func NewFileStore() domain.GuidanceStore { return fileStore{} }

// Load reads RunnerGuidance.txt of runFolder.
func (fileStore) Load(runFolder string) (domain.Guidance, bool, error) {
	path := filepath.Join(runFolder, FileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.Guidance{}, false, nil
	}
	if err != nil {
		return domain.Guidance{}, false, fmt.Errorf("guidance file %s: %w", path, err)
	}
	g, err := Parse(data)
	if err != nil {
		return domain.Guidance{}, false, fmt.Errorf("guidance file %s: %w", path, err)
	}
	return g, true, nil
}

// Save replaces RunnerGuidance.txt of runFolder atomically.
func (fileStore) Save(runFolder string, g domain.Guidance) error {
	path := filepath.Join(runFolder, FileName)
	tmp, err := os.CreateTemp(runFolder, FileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("guidance file %s: %w", path, err)
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(Format(g))
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("guidance file %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("guidance file %s: %w", path, err)
	}
	return nil
}
