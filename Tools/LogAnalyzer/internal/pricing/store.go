package pricing

import (
	"context"
	"path/filepath"

	"mosaic-log-analyzer/internal/domain"
)

// ConfigRelPath is the pricing file's location relative to the MOSAIC runtime root.
const ConfigRelPath = "MosaicLogAnalyzer/config/pricing.yaml"

// Store owns the external YAML pricing file end to end.
type Store struct {
	globalDir string // "" = global level unavailable
	path      string // project file
}

// NewStore returns a store with the given global folder ("" = global level
// unavailable) and project root (the folder LogAnalyzer is started from).
// The project file is <projectRoot>/MosaicLogAnalyzer/config/pricing.yaml.
func NewStore(globalDir, projectRoot string) *Store {
	return &Store{
		globalDir: globalDir,
		path:      filepath.Join(projectRoot, ConfigRelPath),
	}
}

// Path returns the config file's absolute location so a frontend can tell the user
// exactly which file to edit or which file was written.
func (s *Store) Path() string {
	return s.path
}

// Load reads the pricing table: global entries first, then project entries
// replacing global ones for the same (provider, model). Each entry carries the
// level it came from.
//
// Absent file contract: a missing file at either level contributes nothing and is
// not an error, following the same convention as the Deployment tool's config
// store. A comments-only file is a valid empty table.
//
// Currency: the table's currency is the project file's currency key, else the
// global file's, else USD. When both files state different currencies the global
// entries are ignored and one pricing_currency_mismatch warning is raised.
//
// Per-entry isolation: an entry that fails rate parsing or domain.ModelPricing.Validate
// is skipped and reported as a domain.FindingInvalidPricingEntry naming the model;
// the remaining entries still load. An error is returned only when a document
// itself cannot be parsed; it names that file.
func (s *Store) Load(_ context.Context) (domain.PricingTable, []domain.Finding, error) {
	var global, project loadedFile
	var err error
	if gp := s.globalPath(); gp != "" {
		if global, err = loadFile(gp, domain.LevelGlobal); err != nil {
			return domain.EmptyPricingTable(), nil, err
		}
	}
	if project, err = loadFile(s.path, domain.LevelProject); err != nil {
		return domain.EmptyPricingTable(), nil, err
	}
	table, findings := resolveLevels(global, project, s.globalPath(), s.path)
	return table, findings, nil
}

// entryLabel names an entry in a finding: the bare model ID for the top-level
// section, provider/model otherwise.
func entryLabel(provider, model string) string {
	if provider == "" {
		return model
	}
	return provider + "/" + model
}
