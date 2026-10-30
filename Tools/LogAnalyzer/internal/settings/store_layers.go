package settings

import (
	"os"
	"path/filepath"

	"mosaic-log-analyzer/internal/domain"
)

// globalFileName is the settings file's name inside the global folder.
const globalFileName = "analysis.yaml"

// globalPath returns the global file location, or "" when the level is unavailable.
func (s *Store) globalPath() string {
	if s.globalDir == "" {
		return ""
	}
	return filepath.Join(s.globalDir, globalFileName)
}

// Paths reports both configuration levels.
func (s *Store) Paths() domain.ConfigPaths {
	global := domain.ConfigFile{Level: domain.LevelGlobal, Unavailable: "user config folder not found"}
	if p := s.globalPath(); p != "" {
		global = domain.ConfigFile{Level: domain.LevelGlobal, Path: p, Exists: fileExists(p)}
	}
	return domain.ConfigPaths{
		Global:  global,
		Project: domain.ConfigFile{Level: domain.LevelProject, Path: s.path, Exists: fileExists(s.path)},
	}
}

// EnsureGlobalFile writes the comment-only template to the global file when the
// global folder is available and the file is absent; it never overwrites.
func (s *Store) EnsureGlobalFile() (created bool, err error) {
	return domain.WriteFileIfAbsent(s.globalPath(), settingsTemplate)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
