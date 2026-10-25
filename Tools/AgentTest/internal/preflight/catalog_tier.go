package preflight

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/domain"
)

// missingTierValue is the value reported for an agent file that declares no
// recommended_tier at all.
const missingTierValue = "<missing>"

// checkCatalogTiers verifies that every agent file of the catalog the run
// deploys from declares a recommended_tier from the AgentTest tier
// vocabulary. A tier outside it matches no entry of the run's tier-to-model
// map, so the agent would be deployed without the selected model.
//
// An empty catalog folder means the deployment tool reads the product
// catalog, whose tiers cannot be checked here; that is refused when a deploy
// will happen, and ignored when nothing is deployed.
func checkCatalogTiers(report *authoring.Report, catalogFolder string, willDeploy bool) {
	if catalogFolder == "" {
		if willDeploy {
			report.Add(authoring.Diagnostic{
				Severity: authoring.SeverityError,
				Code:     DiagCatalogTierUnverifiable,
				Message: "the run deploys from the product catalog (no catalog folder configured), " +
					"so the recommended_tier of its agents cannot be verified; configure a test catalog folder",
			})
		}
		return
	}
	files, err := catalogAgentFiles(catalogFolder)
	if err != nil {
		report.Add(authoring.Diagnostic{
			Severity: authoring.SeverityError,
			Code:     DiagCatalogTierUnverifiable,
			Path:     catalogFolder,
			Message:  fmt.Sprintf("catalog folder %q cannot be read, so agent tiers cannot be verified: %v", catalogFolder, err),
		})
		return
	}
	for _, path := range files {
		value, err := declaredTier(path)
		if err != nil {
			report.Add(authoring.Diagnostic{
				Severity: authoring.SeverityError,
				Code:     DiagCatalogTierUnverifiable,
				Path:     path,
				Message:  fmt.Sprintf("agent file %q cannot be read, so its tier cannot be verified: %v", path, err),
			})
			continue
		}
		if value == domain.TierTestSubject || value == domain.TierTestStub {
			continue
		}
		report.Add(authoring.Diagnostic{
			Severity: authoring.SeverityError,
			Code:     DiagInvalidCatalogTier,
			Path:     path,
			Pointer:  "recommended_tier",
			Message: fmt.Sprintf("%s declares recommended_tier %q; permitted values: %s, %s",
				filepath.Base(path), value, domain.TierTestSubject, domain.TierTestStub),
		})
	}
}

// catalogAgentFiles lists, ordered by path, the agent markdown files of a
// catalog: Orchestrator/*.md and every .md below Subagents/.
func catalogAgentFiles(catalogFolder string) ([]string, error) {
	info, err := os.Stat(catalogFolder)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	var files []string
	entries, err := os.ReadDir(filepath.Join(catalogFolder, "Orchestrator"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(catalogFolder, "Orchestrator", e.Name()))
		}
	}
	subagents := filepath.Join(catalogFolder, "Subagents")
	walkErr := filepath.WalkDir(subagents, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == subagents {
				return nil
			}
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

// CatalogAgentFilesByKey indexes the agent files of a test catalog by agent
// key: the file's base name without the ".md" suffix, compared exactly
// (case-sensitive), the same key Deployment assigns to a catalog agent.
// Covered layout: Orchestrator/*.md and every .md file below Subagents/.
// Each key's paths are ordered by path. A key with more than one path is
// ambiguous; this function reports it as found and does not choose.
//
// An empty catalogFolder returns an empty, non-nil map and a nil error: no
// agent has a catalog entry. A non-empty catalogFolder that does not exist,
// is not a directory, or cannot be listed returns a non-nil error.
func CatalogAgentFilesByKey(catalogFolder string) (map[string][]string, error) {
	index := map[string][]string{}
	if catalogFolder == "" {
		return index, nil
	}
	files, err := catalogAgentFiles(catalogFolder)
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		key := strings.TrimSuffix(filepath.Base(path), ".md")
		index[key] = append(index[key], path)
	}
	return index, nil
}

// DeclaredTier returns the recommended_tier value declared in the YAML
// frontmatter of the agent file at path, with surrounding whitespace and
// quotes trimmed. declared is false when the file has no frontmatter, no
// recommended_tier key, or an empty value. err is non-nil only when the file
// cannot be read; tier and declared are then zero.
func DeclaredTier(path string) (tier string, declared bool, err error) {
	value, err := declaredTier(path)
	if err != nil {
		return "", false, err
	}
	if value == missingTierValue || value == "" {
		return "", false, nil
	}
	return value, true, nil
}

// declaredTier returns the recommended_tier value in the file's frontmatter,
// or missingTierValue when the file has no frontmatter or no such key.
func declaredTier(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return missingTierValue, nil
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if rest, ok := strings.CutPrefix(line, "recommended_tier:"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"'`), nil
		}
	}
	return missingTierValue, nil
}
