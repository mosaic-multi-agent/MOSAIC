package runner_test

// Test catalogs for the pre-spawn check. A catalog is generated in a temp
// directory in the layout the check reads (Orchestrator/*.md and
// Subagents/**/*.md, each file carrying a recommended_tier frontmatter key).
// Tests never read the product Catalog/ or the tool's own catalog/ folder.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
)

// catalogFile is one agent file of a generated test catalog. An empty tier
// writes a file without any recommended_tier key.
type catalogFile struct {
	rel  string // path below the catalog folder, slash separated
	tier string
}

// agentFileContent renders an agent file with the given declared tier.
func agentFileContent(tier string) string {
	if tier == "" {
		return "---\nname: agent\n---\nBody.\n"
	}
	return "---\nname: agent\nrecommended_tier: " + tier + "\n---\nBody.\n"
}

// newTestCatalog writes the files into a fresh temp directory and returns it.
func newTestCatalog(t *testing.T, files ...catalogFile) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(agentFileContent(f.tier)), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}

// defaultCatalogFiles holds the agents the runner tests deploy by default:
// two agents declaring the subject tier and two declaring the stub tier.
func defaultCatalogFiles() []catalogFile {
	return []catalogFile{
		{"Orchestrator/orchestrator.md", domain.TierTestSubject},
		{"Subagents/general/lead.md", domain.TierTestSubject},
		{"Subagents/general/researcher.md", domain.TierTestStub},
		{"Subagents/general/reviewer.md", domain.TierTestStub},
	}
}

// useCatalog points both the deployment double (which writes each agent's
// model by the tier its catalog file declares) and the runner's pre-spawn
// check at the same catalog folder.
func (h *harness) useCatalog(folder string) {
	h.Deployer.catalogFolder = folder
	h.Deps.CatalogFolder = folder
}

// catalogTierFor returns the tier the catalog folder's file for the agent key
// declares. found is false when no file matches or none declares a tier; the
// first match in walk order is used.
func catalogTierFor(folder, key string) (tier string, found bool) {
	if folder == "" {
		return "", false
	}
	var match string
	_ = filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err == nil && match == "" && !d.IsDir() && d.Name() == key+".md" {
			match = path
		}
		return nil
	})
	if match == "" {
		return "", false
	}
	data, err := os.ReadFile(match)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "recommended_tier:"); ok {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			return v, v != ""
		}
	}
	return "", false
}
