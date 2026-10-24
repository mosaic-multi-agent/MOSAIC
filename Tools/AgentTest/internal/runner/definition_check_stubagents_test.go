package runner_test

// Tests that on the stub_agents path every entry is treated as declaring the
// stub tier: rendered with, expected to carry and checked for the stub model,
// with no tier read from its source file and no catalog folder consulted.
// Subject and stub models differ throughout.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// stubAgentsTierRequest is a stub_agents-path request whose single entry's
// source file declares no tier at all.
func stubAgentsTierRequest(t *testing.T, testID string) runner.Request {
	t.Helper()
	source := filepath.Join(t.TempDir(), "researcher.md")
	if err := os.WriteFile(source, []byte("# researcher\n\nNo frontmatter, no tier.\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", source, err)
	}
	req := stubAgentsRenderRequest(testID)
	req.Test.Definition.StubAgents[0].SourcePath = source
	req.Test.Models = sonnetHaiku()
	return req
}

func TestStubAgentsPath_EntryWithoutATierIsRenderedWithAndExpectedTheStubModel(t *testing.T) {
	h := newHarness(t)
	h.Deps.CatalogFolder = ""

	runProceeds(t, h, stubAgentsTierRequest(t, "stubagents-no-tier"))

	renders := renderCallsBySourcePath(h.Deployer.allCalls())
	if len(renders) != 1 {
		t.Fatalf("stub renders = %d, want 1: %+v", len(renders), h.Deployer.allCalls())
	}
	if renders[0].Model != haikuModel {
		t.Errorf("stub rendered with model %q, want the stub-tier model %q", renders[0].Model, haikuModel)
	}
}

func TestStubAgentsPath_EntryCarryingTheSubjectModelFailsAsAStubTierMismatch(t *testing.T) {
	h := newHarness(t)
	h.Deps.CatalogFolder = ""
	overrideDefinition(h, "stub.md", domain.DefinitionModelField{Present: true, Value: sonnetModel}, nil)

	got := runAborted(t, h, stubAgentsTierRequest(t, "stubagents-subject-model"))

	if got.Problem != runner.DefinitionModelMismatch {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelMismatch)
	}
	if got.Tier != domain.TierTestStub {
		t.Errorf("Tier = %q, want %q", got.Tier, domain.TierTestStub)
	}
	if got.Expected != haikuModel || got.Found != sonnetModel {
		t.Errorf("Expected/Found = %q/%q, want %q/%q", got.Expected, got.Found, haikuModel, sonnetModel)
	}
	if got.Agent == "" {
		t.Error("Agent is empty, want the rendered agent's key")
	}
}

func TestStubAgentsPath_ACatalogFolderTheEntryIsNotInIsNotConsulted(t *testing.T) {
	h := newHarness(t)
	h.useCatalog(newTestCatalog(t, catalogFile{"Subagents/general/other.md", domain.TierTestSubject}))

	runProceeds(t, h, stubAgentsTierRequest(t, "stubagents-foreign-catalog"))
}
