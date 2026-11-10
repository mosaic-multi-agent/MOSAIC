package runner_test

// Tests that the selected subject model, and nothing else, reaches the subject
// the runner provisions and asks the adapter to spawn, on both provisioning
// paths. The authored Subject.Model holds a different value in every case.

import (
	"context"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

func launchPinRequests() map[string]runner.Request {
	sel := domain.ModelSelection{Subject: selectedSubjectModel, Stub: selectedStubModel}
	return map[string]runner.Request{
		"catalogue path":    catalogueRequestWithModels("pin-catalogue", authoredSubjectValue, authoredStubValue, sel),
		"stub-agents path":  renderPathRequest("pin-render", sel),
		"no deployment use": withModels(newRequest("pin-plain"), authoredSubjectValue, sel),
	}
}

func withModels(req runner.Request, authoredSubject string, sel domain.ModelSelection) runner.Request {
	req.Test.Definition.Subject.Model = authoredSubject
	req.Test.Models = sel
	return req
}

func TestLaunchPin_SpawnPlanSubjectCarriesTheSelectedSubjectModel(t *testing.T) {
	for name, req := range launchPinRequests() {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			if _, err := runner.Run(context.Background(), h.Deps, req, nil); err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}

			if got := h.Adapter.lastSpawnPlanSubject.Model; got != selectedSubjectModel {
				t.Errorf("subject handed to SpawnPlan has Model %q, want the selected subject model %q", got, selectedSubjectModel)
			}
		})
	}
}

func TestLaunchPin_ProvisionedSubjectCarriesTheSelectedSubjectModel(t *testing.T) {
	for name, req := range launchPinRequests() {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			if _, err := runner.Run(context.Background(), h.Deps, req, nil); err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}

			if got := h.Adapter.lastProvisionReq.Subject.Model; got != selectedSubjectModel {
				t.Errorf("subject handed to Provision has Model %q, want the selected subject model %q", got, selectedSubjectModel)
			}
		})
	}
}
