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
	globalDir string // "" = global level unavailable
	path      string // project file
}

// NewStore returns a store with the given global folder ("" = global level
// unavailable) and project root (the folder LogAnalyzer is started from).
func NewStore(globalDir, projectRoot string) *Store {
	return &Store{globalDir: globalDir, path: filepath.Join(projectRoot, ConfigRelPath)}
}

// Path returns the settings file location.
func (s *Store) Path() string { return s.path }

// Load reads the global then the project settings file and layers them over the
// built-in defaults: per setting, and per harness for the tool lists. A missing
// file at either level contributes nothing and is never created. Invalid entries
// yield findings (carrying their file's path) and leave the lower level's value;
// an unreadable file or an unparseable document is an error naming the file.
func (s *Store) Load(_ context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	merged := domain.DefaultAnalysisSettings()
	var findings []domain.Finding
	for _, path := range []string{s.globalPath(), s.path} {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return domain.AnalysisSettings{}, nil, fmt.Errorf("read settings file %s: %w", path, err)
		}
		doc, err := parseDocument(data)
		if err != nil {
			return domain.AnalysisSettings{}, nil, fmt.Errorf("parse settings file %s: %w", path, err)
		}
		findings = append(findings, applyDocument(&merged, doc, path)...)
	}
	return merged, findings, nil
}
