package settings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mosaic-log-analyzer/internal/domain"
)

// ConfigRelPath is the settings file's location relative to the MOSAIC runtime root.
const ConfigRelPath = "MosaicLogAnalyzer/config/analysis.yaml"

// Store owns the external YAML settings file.
type Store struct {
	path string
}

// NewStore returns a store rooted at the given MOSAIC runtime root.
func NewStore(mosaicRoot string) *Store {
	return &Store{path: filepath.Join(mosaicRoot, ConfigRelPath)}
}

// Path returns the settings file location.
func (s *Store) Path() string { return s.path }

// Load reads the settings file. A missing file yields the built-in defaults with
// no finding and is never created. Invalid entries yield findings and their
// defaults; an unreadable file or an unparseable document is an error.
func (s *Store) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.DefaultAnalysisSettings(), nil, nil
		}
		return domain.AnalysisSettings{}, nil, fmt.Errorf("read settings file: %w", err)
	}
	doc, err := parseDocument(data)
	if err != nil {
		return domain.AnalysisSettings{}, nil, fmt.Errorf("parse settings file: %w", err)
	}
	loaded, findings := fromDocument(doc, s.path)
	return loaded, findings, nil
}
