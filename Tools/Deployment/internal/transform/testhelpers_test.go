package transform_test

// testhelpers_test.go contains shared test infrastructure for the transform package tests:
//   - fixtureModule: a domain.HarnessModule backed by testdata/transform/fixture.yaml
//   - Standard model selection for tests
//   - Path helpers for locating the agent-source corpus and testdata

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// testdataDir is resolved relative to this package's test working directory.
// Go tests run in the package directory (internal/transform/); testdata lives two levels
// up at Tools/Deployment/testdata/transform/.
const testdataDir = "../../testdata/transform"

// newFixtureModule returns a domain.HarnessModule backed by the fixture harness descriptor
// at testdata/transform/fixture.yaml. All harness-specific behaviour goes through the shared
// descriptor algorithms (CD-2), so no test branch varies on a harness name.
func newFixtureModule(t *testing.T) domain.HarnessModule {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(testdataDir, "fixture.yaml"))
	if err != nil {
		t.Fatalf("read fixture descriptor: %v", err)
	}
	desc, err := descriptor.Parse(src, testdataDir+"/fixture.yaml")
	if err != nil {
		t.Fatalf("parse fixture descriptor: %v", err)
	}
	desc.InjectionsVersion = fixtureInjectionsVersion
	return &fixtureModule{desc: desc}
}

// fixtureInjectionsVersion is the injections version the fixture modules report. Real harness
// modules read it from HarnessInjections.md; the fixture descriptor does not declare one.
const fixtureInjectionsVersion = "1.2.0"

// fixtureModel returns the standard model selection used in golden and pipeline tests.
// Using OriginHarnessList reflects the normal flow where the user picks from the harness
// model list.
func fixtureModel() domain.ModelSelection {
	return domain.ModelSelection{
		ModelID: "claude/claude-sonnet",
		Origin:  domain.OriginHarnessList,
	}
}

// agentCorpusDir is the committed agent-source corpus used by the body-preservation test.
// It lives under testdata/transform/ and covers several body shapes: plain prose, project
// regions, mixed managed and project regions, and the orchestrator role.
const agentCorpusDir = testdataDir + "/agent-corpus"

// agentCorpusPaths returns the absolute paths of every .md agent source in the committed
// corpus, sorted, excluding README files (they carry no frontmatter and must not be
// dispatched as ArtifactAgent). A missing or too-small corpus fails the test; it never skips.
func agentCorpusPaths(t *testing.T) []string {
	t.Helper()
	dir, err := filepath.Abs(agentCorpusDir)
	if err != nil {
		t.Fatalf("resolve agent corpus directory: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read agent corpus directory %s: %v", dir, err)
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "readme.md") {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	sort.Strings(paths)
	if len(paths) < 2 {
		t.Fatalf("agent corpus %s has %d agent sources, want at least 2", dir, len(paths))
	}
	return paths
}

// fixtureProtocol returns a ProtocolContent with distinct orchestrator and subagent blocks
// for use in protocol region tests. The two blocks carry unique, non-overlapping content so
// that tests can assert both the presence of the correct block and the absence of the wrong one.
func fixtureProtocol(version string) domain.ProtocolContent {
	return domain.ProtocolContent{
		Version: version,
		Blocks: map[domain.ProtocolVariant][]byte{
			domain.ProtocolOrchestrator: []byte(orchestratorBlockContent),
			domain.ProtocolSubagent:     []byte(subagentBlockContent),
		},
	}
}


// truncateBytes returns the first n bytes of b for use in diagnostic messages.
func truncateBytes(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// ---------------------------------------------------------------------------
// fixtureModule — domain.HarnessModule backed by testdata/transform/fixture.yaml
// ---------------------------------------------------------------------------

// fixtureModule implements domain.HarnessModule entirely through the shared descriptor
// algorithms (CD-2). No harness-specific branch exists in this test helper; all behaviour
// is driven by the parsed descriptor.
type fixtureModule struct {
	desc *domain.HarnessDescriptor
}

func (m *fixtureModule) Ref() domain.HarnessRef {
	return domain.HarnessRef{
		ID:          m.desc.ID,
		DisplayName: m.desc.DisplayName,
		Tier:        domain.TierDescriptor,
		Usable:      true,
	}
}

func (m *fixtureModule) Descriptor() *domain.HarnessDescriptor {
	return m.desc
}

func (m *fixtureModule) Close() error {
	return nil
}

func (m *fixtureModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	return descriptor.MapTools(m.desc, req)
}

func (m *fixtureModule) Frontmatter(req domain.FrontmatterRequest) (domain.FrontmatterPlan, error) {
	return descriptor.ApplyFrontmatterSpec(m.desc, req)
}

func (m *fixtureModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	return descriptor.ResolveTargetPath(m.desc, req)
}

func (m *fixtureModule) Injection(req domain.InjectionRequest) (string, bool) {
	for _, inj := range m.desc.Injections {
		if inj.Name == req.Name {
			return inj.Content, true
		}
	}
	return "", false
}

func (m *fixtureModule) HookPlan(_ domain.HookPlanRequest) (domain.HookPlan, error) {
	return domain.HookPlan{
		Supported: false,
		Reason:    "fixture harness declares no hook support",
	}, nil
}
