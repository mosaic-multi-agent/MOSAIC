package agentresolve_test

// Helpers that drive agent resolution and the definition check on workspaces
// whose agent files are not Markdown. The Format is passed to agentresolve
// directly; the test-only JSON format only needs to be loadable.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/agentformat/testjson"

	"mosaic-run/internal/agentfile"
)

// testJSONFormat is the format data of a harness whose agent files are the
// test-only JSON format with the .json extension.
var testJSONFormat = agentfile.Format{
	AgentsDir: ".seam/agents",
	FormatID:  string(testjson.ID),
	Extension: ".json",
}

// markdownWithExtension is the format data of a harness whose Markdown agent
// files carry the given extension besides .md and .agent.md.
func markdownWithExtension(ext string) agentfile.Format {
	return agentfile.Format{FormatID: "markdown", Extension: ext}
}

// useTestJSON makes the test-only JSON agent format loadable and returns its
// Format.
func useTestJSON(t *testing.T) agentfile.Format {
	t.Helper()
	testjson.Register()
	return testJSONFormat
}

// testJSONAgentsDir is the fixture workspace of non-Markdown agent files.
func testJSONAgentsDir() string {
	return filepath.Join(agentresolveTestdataDir, "agents-testjson")
}

// testJSONDefinition is a fixture definition file of the non-Markdown format.
func testJSONDefinition(name string) string {
	return filepath.Join(agentresolveTestdataDir, "definitions-testjson", name)
}

// writeFiles creates each named file with placeholder content in dir.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# placeholder\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
