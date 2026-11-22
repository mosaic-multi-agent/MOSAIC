package app_test

// promote_replace_fixtures_test.go holds the fixtures shared by the tests of promote's
// replacement rules (an existing agent with the same key in the target layer): utility agents
// in a fixture layer, a count of the files a layer holds for a key, and a request builder.
// Nothing here is a test.

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
)

// addUtilityAgent writes a utility agent with the given numeric id into layer id.
func (ls *plLayerSet) addUtilityAgent(t *testing.T, id, key, numericID string) {
	t.Helper()
	body := "---\nid: \"" + numericID + "\"\nversion: \"1.0.0\"\nname: " + key + "\n" +
		"description: Layer utility agent.\nrole: utility\nrequired_skills: []\n---\nBody.\n"
	writeSysFile(t, filepath.Join(ls.roots[id], "UtilityAgents", key+".md"), body)
}

// agentPath returns where an agent of the key lives in layer id: under the category for a
// subagent, or under UtilityAgents/ for app.PromoteCategoryUtility.
func (ls *plLayerSet) agentPath(id, category, key string) string {
	if category == app.PromoteCategoryUtility {
		return filepath.Join(ls.roots[id], "UtilityAgents", key+".md")
	}
	return filepath.Join(ls.roots[id], "Subagents", category, key+".md")
}

// addAgentIn adds the agent as a utility agent for app.PromoteCategoryUtility, otherwise as a
// subagent of the category.
func (ls *plLayerSet) addAgentIn(t *testing.T, id, category, key, numericID string) {
	t.Helper()
	if category == app.PromoteCategoryUtility {
		ls.addUtilityAgent(t, id, key, numericID)
		return
	}
	ls.addAgent(t, id, category, key, numericID)
}

// keyFiles returns every file under root named <key>.md: the agents the layer holds for key.
func keyFiles(t *testing.T, root, key string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == key+".md" {
			found = append(found, p)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	return found
}

// replaceRequest is a promote request for src into category with the given overwrite consent.
func replaceRequest(src, category string, overwrite bool) app.PromoteRequest {
	return app.PromoteRequest{FilePath: src, Category: category, HarnessID: "stub-harness", Overwrite: overwrite}
}

// readBytes returns the content of path.
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return b
}
