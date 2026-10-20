package catalog

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// loadSkills scans skill directories for a given catalogue root and populates skills,
// skillIdx, and sourcePaths on the receiver. The new target layout path (Skills/) is
// scanned first; the legacy path (Agents/Generic/Skills/) is scanned second. The first
// occurrence of any given key wins; duplicates from the fallback path are skipped.
func (c *catalogImpl) loadSkills(root string) []Issue {
	skillsDirCandidates := []string{
		catalogpaths.SkillsDir(root),
		filepath.Join(root, "Agents", "Generic", "Skills"),
	}
	for _, skillsDir := range skillsDirCandidates {
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			key := entry.Name()
			if _, exists := c.skillIdx[key]; exists {
				continue // Already loaded from a higher-priority path.
			}
			skillDir := filepath.Join(skillsDir, key)
			skill, err := parseSkillDir(skillDir, key)
			if err != nil {
				c.failures.addFailure(filepath.Join(skillDir, "SKILL.md"), SourceKindSkill, err)
				continue
			}
			c.skills = append(c.skills, skill)
			c.skillIdx[key] = skill

			// Register all skill file paths so ReadSource can serve them.
			for _, f := range skill.Files {
				c.sourcePaths[filepath.Join(skill.SourceDir, f)] = true
			}
		}
	}
	return nil
}

// parseSkillDir reads SKILL.md from the given directory and enumerates every file in the
// directory as part of the skill's file set.
func parseSkillDir(dir, key string) (domain.Skill, error) {
	entryFile := "SKILL.md"
	skillMd := filepath.Join(dir, entryFile)
	doc, err := readAndParse(skillMd)
	if err != nil {
		return domain.Skill{}, err
	}
	fm := doc.Frontmatter()

	skill := domain.Skill{
		Key:       key,
		SourceDir: dir,
		EntryFile: entryFile,
	}

	if v, ok := fm.Get("name"); ok && v.Kind == domain.KindScalar {
		skill.Name = v.Scalar
	}
	if v, ok := fm.Get("description"); ok && v.Kind == domain.KindScalar {
		skill.Description = v.Scalar
	}
	if v, ok := fm.Get("version"); ok && v.Kind == domain.KindScalar {
		skill.Version = v.Scalar
	}

	// Enumerate all files recursively (relative paths, lexicographic depth-first order).
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil // descend into subdirectories but do not add them to Files
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		skill.Files = append(skill.Files, rel)
		return nil
	})
	if err != nil {
		// A directory walk failure is an I/O problem, not a SKILL.md parse failure.
		return domain.Skill{}, &sourceIssue{problem: SourceProblemUnreadable, err: err}
	}

	return skill, nil
}
