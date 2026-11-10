package runner_test

// Tests for the stub-agents (render) path: the subject and each stub are
// rendered with their selected models, and the pre-spawn check reads the
// rendered definitions back. The subject render expects the subject
// selection; every stub render expects the stub selection, which defaults to
// the subject selection.

import (
	"context"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// renderPathRequest declares both a subject agent key and one stub agent, so
// setup renders the subject and the stub. The authored model fields hold
// values that must never reach a render.
func renderPathRequest(testID string, sel domain.ModelSelection) runner.Request {
	req := renderingRequest(testID)
	req.Test.Definition.StubAgents = []domain.StubAgent{
		{
			Identity:   domain.CollaboratorIdentity{ToolName: "dispatch", AgentIdentity: "researcher"},
			SourcePath: "/workspace/agents/researcher.md",
		},
	}
	req.Test.Definition.Subject.Model = authoredSubjectValue
	req.Test.Definition.Subject.StubModel = authoredStubValue
	req.Test.Models = sel
	return req
}

// renderedModels runs the request and returns the Model of the subject render
// and of the stub render.
func renderedModels(t *testing.T, req runner.Request) (subject, stub string) {
	t.Helper()
	h := newHarness(t)
	if _, err := runner.Run(context.Background(), h.Deps, req, nil); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	calls := h.Deployer.allCalls()
	subjectCalls := renderCallsByKey(calls, "orchestrator")
	stubCalls := renderCallsBySourcePath(calls)
	if len(subjectCalls) != 1 || len(stubCalls) != 1 {
		t.Fatalf("want one subject render and one stub render, got %d and %d: %+v", len(subjectCalls), len(stubCalls), calls)
	}
	return subjectCalls[0].Model, stubCalls[0].Model
}

func TestRenderPath_SubjectAndStubAreRenderedWithTheirSelectedModels(t *testing.T) {
	req := renderPathRequest("render-models", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

	subject, stub := renderedModels(t, req)

	if subject != selectedSubjectModel {
		t.Errorf("subject render Model = %q, want the subject selection %q", subject, selectedSubjectModel)
	}
	if stub != selectedStubModel {
		t.Errorf("stub render Model = %q, want the stub selection %q", stub, selectedStubModel)
	}
}

func TestRenderPath_StubIsRenderedWithTheSubjectSelectionWhenNoStubWasSelected(t *testing.T) {
	req := renderPathRequest("render-stub-follows", domain.ModelSelection{Subject: selectedSubjectModel})

	_, stub := renderedModels(t, req)

	if stub != selectedSubjectModel {
		t.Errorf("stub render Model = %q, want the subject selection %q", stub, selectedSubjectModel)
	}
}

func TestRenderPath_AuthoredModelFieldsNeverReachARender(t *testing.T) {
	req := renderPathRequest("render-authored-ignored", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

	subject, stub := renderedModels(t, req)

	for name, got := range map[string]string{"subject": subject, "stub": stub} {
		if got == authoredSubjectValue || got == authoredStubValue {
			t.Errorf("%s render Model = %q, an authored value; only the selection may supply render models", name, got)
		}
	}
}

func TestRenderPath_CorrectlyRenderedDefinitionsLetTheRunProceed(t *testing.T) {
	h := newHarness(t)
	req := renderPathRequest("render-check-ok", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

	runProceeds(t, h, req)
}

func TestRenderPath_DefinitionWithoutItsSelectedModelAbortsBeforeSpawn(t *testing.T) {
	cases := []struct {
		name         string
		file         string
		field        domain.DefinitionModelField
		wantExpected string
		wantProblem  runner.DefinitionModelProblem
	}{
		{"subject model line removed", "orchestrator.md", domain.DefinitionModelField{}, selectedSubjectModel, runner.DefinitionModelMissing},
		{"subject model changed", "orchestrator.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, selectedSubjectModel, runner.DefinitionModelMismatch},
		{"stub model line removed", "stub.md", domain.DefinitionModelField{}, selectedStubModel, runner.DefinitionModelMissing},
		{"stub model changed", "stub.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, selectedStubModel, runner.DefinitionModelMismatch},
		{"stub carries subject model", "stub.md", domain.DefinitionModelField{Present: true, Value: selectedSubjectModel}, selectedStubModel, runner.DefinitionModelMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			overrideDefinition(h, tc.file, tc.field, nil)
			req := renderPathRequest("render-check-bad", domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel})

			got := runAborted(t, h, req)

			if got.Expected != tc.wantExpected {
				t.Errorf("Expected = %q, want %q", got.Expected, tc.wantExpected)
			}
			if got.Problem != tc.wantProblem {
				t.Errorf("Problem = %q, want %q", got.Problem, tc.wantProblem)
			}
		})
	}
}

// A stub-only definition (no subject render) still has every rendered stub
// checked against the stub selection.
func TestRenderPath_StubOnlyDefinitionIsCheckedAgainstTheStubSelection(t *testing.T) {
	h := newHarness(t)
	overrideDefinition(h, "stub.md", domain.DefinitionModelField{Present: true, Value: "other-model"}, nil)
	req := stubAgentsRenderRequest("render-check-stub-only")
	req.Test.Models = domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel}

	got := runAborted(t, h, req)

	if got.Expected != selectedStubModel {
		t.Errorf("Expected = %q, want the stub selection %q", got.Expected, selectedStubModel)
	}
}
