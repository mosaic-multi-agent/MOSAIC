package runner_test

// Tests that the pre-spawn check on the catalogue path expects, for every
// deployed agent, the model selected for the tier the agent declares in the
// test catalog, whatever agent is the subject under test. Subject and stub
// models differ throughout, so an expectation taken from the wrong tier shows.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

const (
	sonnetModel = "claude-sonnet-5-5"
	haikuModel  = "claude-haiku-5-5"
)

// sonnetHaiku is a selection with distinct subject and stub models.
func sonnetHaiku() domain.ModelSelection {
	return domain.ModelSelection{Subject: sonnetModel, Stub: haikuModel}
}

// incidentCatalogFiles has the agent set of a status-routing-shaped
// deployment: two orchestrators declaring the subject tier and the stubs of
// the workflow declaring the stub tier.
func incidentCatalogFiles() []catalogFile {
	return []catalogFile{
		{"Orchestrator/orchestrator.md", domain.TierTestSubject},
		{"Orchestrator/orchestrator-script.md", domain.TierTestSubject},
		{"Subagents/general/researcher.md", domain.TierTestStub},
		{"Subagents/general/reviewer.md", domain.TierTestStub},
		{"Subagents/general/coder.md", domain.TierTestStub},
	}
}

// deployKeys makes the deploy double report one agent per key, in order, and
// returns the reported definition paths by key.
func deployKeys(h *harness, keys ...string) map[string]string {
	paths := map[string]string{}
	h.Deployer.deployFn = func(req domain.DeployRequest) (domain.DeployResult, error) {
		var agents []domain.DeployedAgent
		for _, k := range keys {
			p := filepath.Join(req.WorkspaceRoot, ".claude", "agents", k+".md")
			paths[k] = p
			agents = append(agents, domain.DeployedAgent{Key: k, DestinationPath: p})
		}
		return domain.DeployResult{Agents: agents}, nil
	}
	return paths
}

// incidentHarness wires the incident catalog and deploys all of its agents.
func incidentHarness(t *testing.T) (*harness, map[string]string) {
	t.Helper()
	h := newHarness(t)
	h.useCatalog(newTestCatalog(t, incidentCatalogFiles()...))
	paths := deployKeys(h, "orchestrator", "orchestrator-script", "researcher", "reviewer", "coder")
	return h, paths
}

func tierRequest(testID string) runner.Request {
	return catalogueRequestWithModels(testID, "", "", sonnetHaiku())
}

func TestTierExpectation_StatusRoutingShapedDeploymentProceeds(t *testing.T) {
	h, _ := incidentHarness(t)

	runProceeds(t, h, tierRequest("tier-incident"))
}

func TestTierExpectation_SecondSubjectTierAgentThatIsNotTheSubjectIsExpectedTheSubjectModel(t *testing.T) {
	t.Run("carrying the subject model proceeds", func(t *testing.T) {
		h, _ := incidentHarness(t)
		overrideDefinition(h, "orchestrator-script.md", domain.DefinitionModelField{Present: true, Value: sonnetModel}, nil)

		runProceeds(t, h, tierRequest("tier-two-subjects-ok"))
	})

	t.Run("carrying the stub model fails naming agent, file, tier and both models", func(t *testing.T) {
		h, paths := incidentHarness(t)
		overrideDefinition(h, "orchestrator-script.md", domain.DefinitionModelField{Present: true, Value: haikuModel}, nil)

		got := runAborted(t, h, tierRequest("tier-subject-tier-gets-stub"))

		assertMismatch(t, got, "orchestrator-script", paths["orchestrator-script"], domain.TierTestSubject, sonnetModel, haikuModel)
	})
}

func TestTierExpectation_StubTierAgentCarryingTheSubjectModelFails(t *testing.T) {
	h, paths := incidentHarness(t)
	overrideDefinition(h, "researcher.md", domain.DefinitionModelField{Present: true, Value: sonnetModel}, nil)

	got := runAborted(t, h, tierRequest("tier-stub-gets-subject"))

	assertMismatch(t, got, "researcher", paths["researcher"], domain.TierTestStub, haikuModel, sonnetModel)
}

// assertMismatch checks a mismatch failure's fields and its message content.
func assertMismatch(t *testing.T, got *runner.DefinitionModelError, agent, path, tier, expected, found string) {
	t.Helper()
	if got.Problem != runner.DefinitionModelMismatch {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelMismatch)
	}
	if got.Agent != agent {
		t.Errorf("Agent = %q, want %q", got.Agent, agent)
	}
	if got.Path != path {
		t.Errorf("Path = %q, want %q", got.Path, path)
	}
	if got.Tier != tier {
		t.Errorf("Tier = %q, want the declared tier %q", got.Tier, tier)
	}
	if got.Expected != expected || got.Found != found {
		t.Errorf("Expected/Found = %q/%q, want %q/%q", got.Expected, got.Found, expected, found)
	}
	text := got.Error()
	for _, want := range []string{agent, path, tier, expected, found} {
		if !strings.Contains(text, want) {
			t.Errorf("error text %q does not contain %q", text, want)
		}
	}
	if strings.Contains(text, "every other deployed agent") {
		t.Errorf("error text %q still states the subject-identity rule", text)
	}
}
