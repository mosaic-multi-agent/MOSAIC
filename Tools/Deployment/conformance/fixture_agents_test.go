package conformance_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// fixtureAgentCases returns one goldenCase per agent source for the smoke test, read from
// the agent-fixtures corpus. A missing fixture file fails the test; it never skips.
func fixtureAgentCases(t *testing.T) []goldenCase {
	t.Helper()
	dir := agentFixturesDir(t)
	keys := []string{"contracts-review", "test-runner", "planner-tdd-soft", "orchestrator"}
	cases := make([]goldenCase, 0, len(keys))
	for _, key := range keys {
		p := filepath.Join(dir, key+".md")
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("agent fixture %s: %v", p, err)
		}
		cases = append(cases, goldenCase{
			name:       key,
			sourcePath: p,
			key:        key,
			kind:       domain.ArtifactAgent,
		})
	}
	return cases
}

// fixtureAgentSourcePaths returns the absolute paths of every .md agent source in the
// agent-fixtures corpus, sorted. A missing or empty directory fails the test; it never skips.
func fixtureAgentSourcePaths(t *testing.T) []string {
	t.Helper()
	dir := agentFixturesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read agent fixtures dir %s: %v", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.EqualFold(e.Name(), "readme.md") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	if len(paths) == 0 {
		t.Fatalf("no .md agent sources found in %s", dir)
	}
	sort.Strings(paths)
	return paths
}
