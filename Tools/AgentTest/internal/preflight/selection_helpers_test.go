package preflight_test

// Shared fixtures for the model-guard tests: a model selection every ordinary
// preflight test supplies, and builders for catalog trees whose agent files
// declare a chosen recommended_tier.

import (
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
)

// testSubjectModel is the subject model selection ordinary preflight tests
// supply, because a run without a subject selection is rejected.
const testSubjectModel = "test-subject-model"

// selectedModels returns Overrides carrying only the subject selection.
func selectedModels() preflight.Overrides {
	return preflight.Overrides{SubjectModel: testSubjectModel}
}

// agentFileWithTier renders a catalog agent file whose frontmatter declares
// the given recommended_tier. An empty tier omits the key altogether.
func agentFileWithTier(name, tier string) string {
	out := "---\nversion: 1.0.0\nname: " + name + "\nrole: test\n"
	if tier != "" {
		out += "recommended_tier: " + tier + "\n"
	}
	return out + "---\n\n# " + name + "\n"
}

// validCatalogFiles is a catalog tree in which every agent uses the AgentTest
// tier vocabulary, including the scripted orchestrator.
func validCatalogFiles() map[string]string {
	return map[string]string{
		"Orchestrator/orchestrator.md":           agentFileWithTier("orchestrator", domain.TierTestSubject),
		"Orchestrator/orchestrator-script.md":    agentFileWithTier("orchestrator-script", domain.TierTestSubject),
		"Subagents/TestStubs/stub-researcher.md": agentFileWithTier("stub-researcher", domain.TierTestStub),
		"Subagents/TestStubs/Deep/stub-deep.md":  agentFileWithTier("stub-deep", domain.TierTestStub),
	}
}

// catalogWith writes validCatalogFiles with the given replacements (a
// relative path mapped to new content) and returns the catalog root.
func catalogWith(t *testing.T, changes map[string]string) string {
	t.Helper()
	files := validCatalogFiles()
	for rel, content := range changes {
		files[rel] = content
	}
	return writeTree(t, files)
}

// suiteTree writes the smallest passing suite and returns its suite path and
// fixture root.
func suiteTree(t *testing.T) (suitePath, fixtureRoot string) {
	t.Helper()
	root := writeTree(t, map[string]string{
		"s.suite.yaml":          validSuite,
		"happy-path.test.yaml":  validDefinition,
		"happy-path.stubs.json": validRegistry,
		"agents/researcher.md":  "# stub researcher placeholder",
	})
	return filepath.Join(root, "s.suite.yaml"), filepath.Join(root, "fixtures")
}

// diagnosticsWithCode returns the diagnostics of the report carrying code.
func diagnosticsWithCode(report authoring.Report, code string) []authoring.Diagnostic {
	var out []authoring.Diagnostic
	for _, d := range report.Diagnostics {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
