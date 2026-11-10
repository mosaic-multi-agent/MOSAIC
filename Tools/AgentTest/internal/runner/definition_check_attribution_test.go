package runner_test

// Tests that the pre-spawn check attributes each deployed definition to its
// tier by the declared CatalogAgentKey, not by report position or by a fixed
// key, and that every reported definition is checked. Also covers the
// unsupported and unreadable outcomes on the stub-agents (render) path.

import (
	"errors"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

const leadKey = "lead"

// leadFirstStubDeploy makes the deploy double report the stubs first and the
// subject (key "lead", the declared CatalogAgentKey) last. It returns the
// reported paths in report order.
func leadFirstStubDeploy(h *harness, keys ...string) []string {
	paths := make([]string, len(keys))
	h.Deployer.deployFn = func(req domain.DeployRequest) (domain.DeployResult, error) {
		var agents []domain.DeployedAgent
		for i, k := range keys {
			paths[i] = filepath.Join(req.WorkspaceRoot, ".claude", "agents", k+".md")
			agents = append(agents, domain.DeployedAgent{Key: k, DestinationPath: paths[i]})
		}
		return domain.DeployResult{Agents: agents}, nil
	}
	return paths
}

// leadRequest is a catalogue-path request whose subject key is "lead".
func leadRequest(testID string) runner.Request {
	req := definitionCheckRequest(testID)
	req.Test.Definition.Subject.CatalogAgentKey = leadKey
	return req
}

func TestDefinitionCheck_SubjectIsIdentifiedByCatalogAgentKeyNotByPosition(t *testing.T) {
	t.Run("correct definitions with the subject reported last proceed", func(t *testing.T) {
		h := newHarness(t)
		leadFirstStubDeploy(h, "researcher", leadKey)

		runProceeds(t, h, leadRequest("attr-ok"))
	})

	t.Run("wrong model on the subject reported last is checked against the subject selection", func(t *testing.T) {
		h := newHarness(t)
		paths := leadFirstStubDeploy(h, "researcher", leadKey)
		overrideDefinition(h, leadKey+".md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)

		got := runAborted(t, h, leadRequest("attr-subject-last"))

		if got.Path != paths[1] {
			t.Errorf("Path = %q, want the subject definition %q", got.Path, paths[1])
		}
		if got.Expected != selectedSubjectModel {
			t.Errorf("Expected = %q, want the subject selection %q", got.Expected, selectedSubjectModel)
		}
	})

	t.Run("wrong model on the stub reported first is checked against the stub selection", func(t *testing.T) {
		h := newHarness(t)
		paths := leadFirstStubDeploy(h, "researcher", leadKey)
		overrideDefinition(h, "researcher.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)

		got := runAborted(t, h, leadRequest("attr-stub-first"))

		if got.Path != paths[0] {
			t.Errorf("Path = %q, want the stub definition %q", got.Path, paths[0])
		}
		if got.Expected != selectedStubModel {
			t.Errorf("Expected = %q, want the stub selection %q", got.Expected, selectedStubModel)
		}
	})

	t.Run("an agent keyed orchestrator is expected by its declared tier even when the subject key is another", func(t *testing.T) {
		h := newHarness(t)
		paths := leadFirstStubDeploy(h, leadKey, "orchestrator")
		overrideDefinition(h, "orchestrator.md", domain.DefinitionModelField{Present: true, Value: selectedStubModel}, nil)

		got := runAborted(t, h, leadRequest("attr-orchestrator-by-tier"))

		if got.Path != paths[1] {
			t.Errorf("Path = %q, want %q", got.Path, paths[1])
		}
		if got.Expected != selectedSubjectModel {
			t.Errorf("Expected = %q, want the subject selection %q (orchestrator declares the subject tier)", got.Expected, selectedSubjectModel)
		}
	})

	t.Run("an agent keyed orchestrator carrying the subject model proceeds when the subject key is another", func(t *testing.T) {
		h := newHarness(t)
		leadFirstStubDeploy(h, leadKey, "orchestrator")

		runProceeds(t, h, leadRequest("attr-orchestrator-ok"))
	})
}

func TestDefinitionCheck_EveryReportedDefinitionIsChecked(t *testing.T) {
	h := newHarness(t)
	paths := leadFirstStubDeploy(h, leadKey, "researcher", "reviewer")
	overrideDefinition(h, "reviewer.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)

	got := runAborted(t, h, leadRequest("attr-last-wrong"))

	if got.Path != paths[2] {
		t.Errorf("Path = %q, want the last reported definition %q", got.Path, paths[2])
	}
	if got.Expected != selectedStubModel {
		t.Errorf("Expected = %q, want the stub selection %q", got.Expected, selectedStubModel)
	}
}

func TestRenderPath_UnreadableDefinitionAbortsBeforeSpawn(t *testing.T) {
	for _, file := range []string{"orchestrator.md", "stub.md"} {
		t.Run(file, func(t *testing.T) {
			h := newHarness(t)
			overrideDefinition(h, file, domain.DefinitionModelField{}, errors.New("frontmatter cannot be parsed"))
			req := renderPathRequest("render-unreadable", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

			got := runAborted(t, h, req)

			if got.Problem != runner.DefinitionModelUnreadable {
				t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelUnreadable)
			}
		})
	}
}

func TestRenderPath_AdapterWithoutAReaderFailsWhenDefinitionsWereRendered(t *testing.T) {
	h := newHarness(t)
	// Only the HarnessAdapter methods are visible through the embedded
	// interface, so this adapter is not a domain.DefinitionModelReader.
	h.Deps.Adapter = struct{ domain.HarnessAdapter }{h.Adapter}
	req := renderPathRequest("render-unsupported", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

	got := runAborted(t, h, req)

	if got.Problem != runner.DefinitionModelUnsupported {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelUnsupported)
	}
}
