package runner_test

// Tests for the pre-spawn check of the deployed definitions on the catalogue
// path. After the deploy step and before the harness starts, every definition
// the deployment port reported is read back through the adapter and its model
// field is compared with the model selected for the tier its test catalog
// file declares (the stub selection defaults to the subject selection). A missing, empty or
// different value aborts the run before spawn, through the setup-failure path.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

const (
	selectedSubjectModel = "selected-subject-model"
	selectedStubModel    = "selected-stub-model"
)

// twoAgentDeploy makes the deploy double report the subject ("orchestrator")
// first and a stub ("researcher") second, and returns the reported paths.
func twoAgentDeploy(h *harness) (subjectPath, stubPath *string) {
	subjectPath, stubPath = new(string), new(string)
	h.Deployer.deployFn = func(req domain.DeployRequest) (domain.DeployResult, error) {
		dir := filepath.Join(req.WorkspaceRoot, ".claude", "agents")
		*subjectPath = filepath.Join(dir, "orchestrator.md")
		*stubPath = filepath.Join(dir, "researcher.md")
		return domain.DeployResult{Agents: []domain.DeployedAgent{
			{Key: "orchestrator", DestinationPath: *subjectPath},
			{Key: "researcher", DestinationPath: *stubPath},
		}}, nil
	}
	return subjectPath, stubPath
}

// definitionCheckRequest is a catalogue-path request with distinct subject and
// stub selections.
func definitionCheckRequest(testID string) runner.Request {
	return catalogueRequestWithModels(testID, "", "", domain.ModelSelection{
		Subject: selectedSubjectModel,
		Stub:    selectedStubModel,
	})
}

// overrideDefinition scripts what the adapter reports for the definition file.
func overrideDefinition(h *harness, fileName string, f domain.DefinitionModelField, err error) {
	if h.Adapter.definitionOverrides == nil {
		h.Adapter.definitionOverrides = map[string]definitionModelResult{}
	}
	h.Adapter.definitionOverrides[fileName] = definitionModelResult{field: f, err: err}
}

// runAborted runs the request and requires that it aborted with a
// *runner.DefinitionModelError before any harness start: the spawn plan was
// never requested, nothing was launched and the sandbox was torn down.
func runAborted(t *testing.T, h *harness, req runner.Request) *runner.DefinitionModelError {
	t.Helper()
	_, err := runner.Run(context.Background(), h.Deps, req, nil)
	if err == nil {
		t.Fatal("Run returned no error, want the pre-spawn check to abort the run")
	}
	var defErr *runner.DefinitionModelError
	if !errors.As(err, &defErr) {
		t.Fatalf("Run error = %v, want a *runner.DefinitionModelError", err)
	}
	calls := h.Rec.all()
	for _, c := range calls {
		if c == "adapter.SpawnPlan" || c == "launcher.Launch" {
			t.Errorf("%s was called after a failed definition check; the harness must not start. calls: %v", c, calls)
		}
	}
	if !containsPath(calls, "workspace.Teardown") {
		t.Errorf("workspace.Teardown not recorded after a failed definition check; calls: %v", calls)
	}
	return defErr
}

// runProceeds runs the request and requires that it was not stopped by the check.
func runProceeds(t *testing.T, h *harness, req runner.Request) {
	t.Helper()
	if _, err := runner.Run(context.Background(), h.Deps, req, nil); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if !containsPath(h.Rec.all(), "launcher.Launch") {
		t.Errorf("launcher.Launch not recorded; calls: %v", h.Rec.all())
	}
}

func TestDefinitionCheck_CorrectDefinitionsLetTheRunProceed(t *testing.T) {
	h := newHarness(t)
	twoAgentDeploy(h)

	runProceeds(t, h, definitionCheckRequest("check-correct"))
}

func TestDefinitionCheck_SubjectDefinitionWithoutItsSelectedModelAbortsBeforeSpawn(t *testing.T) {
	cases := []struct {
		name        string
		field       domain.DefinitionModelField
		wantProblem runner.DefinitionModelProblem
		wantFound   string
	}{
		{"model line removed", domain.DefinitionModelField{}, runner.DefinitionModelMissing, ""},
		{"model value empty", domain.DefinitionModelField{Present: true, Value: ""}, runner.DefinitionModelMissing, ""},
		{"model changed", domain.DefinitionModelField{Present: true, Value: "other-model"}, runner.DefinitionModelMismatch, "other-model"},
		{"case differs", domain.DefinitionModelField{Present: true, Value: "Selected-Subject-Model"}, runner.DefinitionModelMismatch, "Selected-Subject-Model"},
		{"suffix differs", domain.DefinitionModelField{Present: true, Value: selectedSubjectModel + "-latest"}, runner.DefinitionModelMismatch, selectedSubjectModel + "-latest"},
		{"stub model instead of subject model", domain.DefinitionModelField{Present: true, Value: selectedStubModel}, runner.DefinitionModelMismatch, selectedStubModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			subjectPath, _ := twoAgentDeploy(h)
			overrideDefinition(h, "orchestrator.md", tc.field, nil)

			got := runAborted(t, h, definitionCheckRequest("check-subject"))

			if got.Path != *subjectPath {
				t.Errorf("Path = %q, want the subject definition %q", got.Path, *subjectPath)
			}
			if got.Expected != selectedSubjectModel {
				t.Errorf("Expected = %q, want the subject selection %q", got.Expected, selectedSubjectModel)
			}
			if got.Found != tc.wantFound {
				t.Errorf("Found = %q, want %q", got.Found, tc.wantFound)
			}
			if got.Problem != tc.wantProblem {
				t.Errorf("Problem = %q, want %q", got.Problem, tc.wantProblem)
			}
		})
	}
}

func TestDefinitionCheck_StubDefinitionWithoutItsSelectedModelAbortsBeforeSpawn(t *testing.T) {
	cases := []struct {
		name        string
		field       domain.DefinitionModelField
		wantProblem runner.DefinitionModelProblem
		wantFound   string
	}{
		{"model line removed", domain.DefinitionModelField{}, runner.DefinitionModelMissing, ""},
		{"model value empty", domain.DefinitionModelField{Present: true, Value: ""}, runner.DefinitionModelMissing, ""},
		{"model changed", domain.DefinitionModelField{Present: true, Value: "other-model"}, runner.DefinitionModelMismatch, "other-model"},
		{"subject model instead of stub model", domain.DefinitionModelField{Present: true, Value: selectedSubjectModel}, runner.DefinitionModelMismatch, selectedSubjectModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, stubPath := twoAgentDeploy(h)
			overrideDefinition(h, "researcher.md", tc.field, nil)

			got := runAborted(t, h, definitionCheckRequest("check-stub"))

			if got.Path != *stubPath {
				t.Errorf("Path = %q, want the stub definition %q", got.Path, *stubPath)
			}
			if got.Expected != selectedStubModel {
				t.Errorf("Expected = %q, want the stub selection %q", got.Expected, selectedStubModel)
			}
			if got.Found != tc.wantFound {
				t.Errorf("Found = %q, want %q", got.Found, tc.wantFound)
			}
			if got.Problem != tc.wantProblem {
				t.Errorf("Problem = %q, want %q", got.Problem, tc.wantProblem)
			}
		})
	}
}

func TestDefinitionCheck_StubExpectationFollowsTheSubjectSelectionWhenNoStubWasSelected(t *testing.T) {
	sel := domain.ModelSelection{Subject: selectedSubjectModel}

	t.Run("stub carrying the subject model proceeds", func(t *testing.T) {
		h := newHarness(t)
		twoAgentDeploy(h)
		overrideDefinition(h, "researcher.md", domain.DefinitionModelField{Present: true, Value: selectedSubjectModel}, nil)

		runProceeds(t, h, catalogueRequestWithModels("check-follow-ok", "", "", sel))
	})

	t.Run("stub carrying another model aborts", func(t *testing.T) {
		h := newHarness(t)
		twoAgentDeploy(h)
		overrideDefinition(h, "researcher.md", domain.DefinitionModelField{Present: true, Value: selectedStubModel}, nil)

		got := runAborted(t, h, catalogueRequestWithModels("check-follow-bad", "", "", sel))

		if got.Expected != selectedSubjectModel {
			t.Errorf("Expected = %q, want the subject selection %q (the stub selection defaults to it)", got.Expected, selectedSubjectModel)
		}
	})
}

func TestDefinitionCheck_UnreadableDefinitionAbortsBeforeSpawn(t *testing.T) {
	h := newHarness(t)
	twoAgentDeploy(h)
	overrideDefinition(h, "researcher.md", domain.DefinitionModelField{}, errors.New("frontmatter cannot be parsed"))

	got := runAborted(t, h, definitionCheckRequest("check-unreadable"))

	if got.Problem != runner.DefinitionModelUnreadable {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelUnreadable)
	}
}

func TestDefinitionCheck_AdapterWithoutAReaderFailsWhenDefinitionsWereDeployed(t *testing.T) {
	h := newHarness(t)
	twoAgentDeploy(h)
	// Only the HarnessAdapter methods are visible through the embedded
	// interface, so this adapter is not a domain.DefinitionModelReader.
	h.Deps.Adapter = struct{ domain.HarnessAdapter }{h.Adapter}

	got := runAborted(t, h, definitionCheckRequest("check-unsupported"))

	if got.Problem != runner.DefinitionModelUnsupported {
		t.Errorf("Problem = %q, want %q", got.Problem, runner.DefinitionModelUnsupported)
	}
}

func TestDefinitionCheck_FirstFailingDefinitionInReportOrderIsReported(t *testing.T) {
	h := newHarness(t)
	subjectPath, _ := twoAgentDeploy(h)
	// Both definitions are wrong; the subject is reported first.
	overrideDefinition(h, "orchestrator.md", domain.DefinitionModelField{Present: true, Value: "wrong-a"}, nil)
	overrideDefinition(h, "researcher.md", domain.DefinitionModelField{Present: true, Value: "wrong-b"}, nil)

	got := runAborted(t, h, definitionCheckRequest("check-first"))

	if got.Path != *subjectPath {
		t.Errorf("Path = %q, want the first reported definition %q", got.Path, *subjectPath)
	}
}

func TestDefinitionCheck_ErrorTextNamesFileExpectedAndFoundValue(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		h := newHarness(t)
		subjectPath, _ := twoAgentDeploy(h)
		overrideDefinition(h, "orchestrator.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)

		got := runAborted(t, h, definitionCheckRequest("check-text-mismatch"))

		text := got.Error()
		for _, want := range []string{"orchestrator", *subjectPath, domain.TierTestSubject, selectedSubjectModel, "other-model"} {
			if !strings.Contains(text, want) {
				t.Errorf("error text %q does not contain %q", text, want)
			}
		}
		if strings.Contains(text, "every other deployed agent") {
			t.Errorf("error text %q still states the subject-identity rule", text)
		}
	})

	t.Run("missing", func(t *testing.T) {
		h := newHarness(t)
		subjectPath, _ := twoAgentDeploy(h)
		overrideDefinition(h, "orchestrator.md", domain.DefinitionModelField{}, nil)

		got := runAborted(t, h, definitionCheckRequest("check-text-missing"))

		text := got.Error()
		for _, want := range []string{*subjectPath, selectedSubjectModel, "none"} {
			if !strings.Contains(text, want) {
				t.Errorf("error text %q does not contain %q", text, want)
			}
		}
	})
}

func TestDefinitionCheck_NothingDeployedPassesWithoutAnyBypass(t *testing.T) {
	h := newHarness(t)
	h.Deps.Deploy = nil
	// An adapter that cannot read definitions must not matter when nothing
	// was deployed.
	h.Deps.Adapter = struct{ domain.HarnessAdapter }{h.Adapter}

	runProceeds(t, h, definitionCheckRequest("check-nothing-deployed"))
}

// A wired deploy port that reports zero agents is an existing setup failure
// (the subject key is not among the deployed agents), not a definition-check
// failure: the run aborts before spawn and the check never reports a model
// problem for definitions that do not exist.
func TestDefinitionCheck_DeployReportingNoAgentsFailsSetupNotTheDefinitionCheck(t *testing.T) {
	h := newHarness(t)
	h.Deployer.deployFn = func(domain.DeployRequest) (domain.DeployResult, error) {
		return domain.DeployResult{Agents: nil}, nil
	}
	h.Deps.Adapter = struct{ domain.HarnessAdapter }{h.Adapter}

	_, err := runner.Run(context.Background(), h.Deps, definitionCheckRequest("check-zero-agents"), nil)

	if err == nil {
		t.Fatal("Run returned no error for a deployment without the subject agent")
	}
	var defErr *runner.DefinitionModelError
	if errors.As(err, &defErr) {
		t.Errorf("Run error = %v, want a setup error that is not a *runner.DefinitionModelError", err)
	}
	if containsPath(h.Rec.all(), "launcher.Launch") {
		t.Errorf("launcher.Launch was recorded after a failed deployment; calls: %v", h.Rec.all())
	}
}

// A wired deploy port on a path that renders no definition (no stub agents, no
// catalogue key) leaves nothing to check either.
func TestDefinitionCheck_NoRenderedDefinitionsPassesWithoutAnyBypass(t *testing.T) {
	h := newHarness(t)
	h.Deps.Adapter = struct{ domain.HarnessAdapter }{h.Adapter}
	req := newRequest("check-zero-renders")
	req.Test.Models = domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel}

	runProceeds(t, h, req)
}
