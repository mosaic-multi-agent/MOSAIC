package evaluate_test

// Tests that Evaluate applies the run-validity decision on every return path
// and carries the observed models with the result.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/evaluate"
)

func TestEvaluate_ObservedEqualsSelection_RunIsEvaluatedNormally(t *testing.T) {
	ev := baseEvidence()

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelVerified {
		t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelVerified)
	}
	if got.Verdict != domain.VerdictPass {
		t.Errorf("Verdict = %q, want PASS; a verified run is judged by its assertions only", got.Verdict)
	}
	if hasReason(got.Reasons, domain.ReasonModelUnverified) {
		t.Errorf("Reasons = %v, must not contain %q for a verified run", got.Reasons, domain.ReasonModelUnverified)
	}
	if got.ModelValidityDetail != "" {
		t.Errorf("ModelValidityDetail = %q, want empty for a verified run", got.ModelValidityDetail)
	}
	if evaluate.ExclusionOf(got) != "" {
		t.Errorf("ExclusionOf = %q, want the run counted", evaluate.ExclusionOf(got))
	}
}

func TestEvaluate_CarriesObservedModelsAlongsideSelection(t *testing.T) {
	ev := baseEvidence()
	ev.SubjectModel = "anthropic/model-a"
	ev.StubModel = "anthropic/model-stub"
	ev.ObservedSubjectModels = observedModel("anthropic/model-a")

	got := evaluate.Evaluate(ev)

	if len(got.ObservedSubjectModels) != 1 || got.ObservedSubjectModels[0] != "anthropic/model-a" {
		t.Errorf("ObservedSubjectModels = %v, want [anthropic/model-a]", got.ObservedSubjectModels)
	}
	if got.SubjectModel != "anthropic/model-a" || got.StubModel != "anthropic/model-stub" {
		t.Errorf("selection = (%q, %q), want it carried unchanged next to the observed models", got.SubjectModel, got.StubModel)
	}
	if got.ModelValidity != domain.ModelVerified {
		t.Errorf("ModelValidity = %q, want %q for a provider-qualified observation equal to the selection", got.ModelValidity, domain.ModelVerified)
	}
}

func TestEvaluate_StubModelDifferingFromObservedSubject_DoesNotAffectValidity(t *testing.T) {
	ev := baseEvidence()
	ev.SubjectModel = "model-a"
	ev.StubModel = "model-stub-never-observed"
	ev.ObservedSubjectModels = observedModel("model-a")

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelVerified {
		t.Errorf("ModelValidity = %q, want %q; the stub model is neither verified nor compared with the subject observation", got.ModelValidity, domain.ModelVerified)
	}
	if len(got.ObservedSubjectModels) != 1 || got.ObservedSubjectModels[0] != "model-a" {
		t.Errorf("observed subject models = %v, want only model-a", got.ObservedSubjectModels)
	}
}

func TestEvaluate_ObservedDiffersFromSelection_RunIsInvalidAndFailed(t *testing.T) {
	ev := baseEvidence()
	ev.SubjectModel = "model-a"
	ev.ObservedSubjectModels = observedModel("model-b")

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelMismatch {
		t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelMismatch)
	}
	if got.Verdict != domain.VerdictFail {
		t.Errorf("Verdict = %q, want FAIL even though every assertion passes", got.Verdict)
	}
	if !hasReason(got.Reasons, domain.ReasonModelUnverified) {
		t.Errorf("Reasons = %v, want it to include %q", got.Reasons, domain.ReasonModelUnverified)
	}
	for _, want := range []string{"model-a", "model-b"} {
		if !strings.Contains(got.ModelValidityDetail, want) {
			t.Errorf("ModelValidityDetail = %q, want it to contain %q", got.ModelValidityDetail, want)
		}
	}
	if len(got.ObservedSubjectModels) != 1 || got.ObservedSubjectModels[0] != "model-b" {
		t.Errorf("ObservedSubjectModels = %v, want the observed value kept even for an invalid run", got.ObservedSubjectModels)
	}
}

func TestEvaluate_NoEvidence_RunIsInvalidAndFailed(t *testing.T) {
	ev := baseEvidence()
	ev.ObservedSubjectModels = domain.ObservedModelEvidence{
		NoEvidence: domain.NoEvidenceNoSubjectUsage,
		Detail:     "no token-bearing requests",
	}

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelNoEvidence {
		t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelNoEvidence)
	}
	if got.Verdict != domain.VerdictFail {
		t.Errorf("Verdict = %q, want FAIL", got.Verdict)
	}
	if !hasReason(got.Reasons, domain.ReasonModelUnverified) {
		t.Errorf("Reasons = %v, want it to include %q", got.Reasons, domain.ReasonModelUnverified)
	}
	if !strings.Contains(got.ModelValidityDetail, selectedModel) {
		t.Errorf("ModelValidityDetail = %q, want it to name the selected model %q", got.ModelValidityDetail, selectedModel)
	}
	if len(got.ObservedSubjectModels) != 0 {
		t.Errorf("ObservedSubjectModels = %v, want none", got.ObservedSubjectModels)
	}
}

func TestEvaluate_ZeroValueEvidence_IsNotAValidRun(t *testing.T) {
	ev := baseEvidence()
	ev.ObservedSubjectModels = domain.ObservedModelEvidence{}

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelNoEvidence {
		t.Errorf("ModelValidity = %q, want %q; unknown must never read as valid", got.ModelValidity, domain.ModelNoEvidence)
	}
	if got.Verdict == domain.VerdictPass {
		t.Errorf("Verdict = PASS for a run with no model evidence")
	}
}

func TestEvaluate_SeveralObservedOneDifferent_RunIsInvalid(t *testing.T) {
	ev := baseEvidence()
	ev.ObservedSubjectModels = observedModel(selectedModel, "model-other")

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelMismatch {
		t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelMismatch)
	}
	if !strings.Contains(got.ModelValidityDetail, "model-other") {
		t.Errorf("ModelValidityDetail = %q, want it to name the differing model", got.ModelValidityDetail)
	}
}

func TestEvaluate_ModelCheckOverridesPassingOutcomes(t *testing.T) {
	t.Run("negative test inversion", func(t *testing.T) {
		ev := baseEvidence()
		ev.Definition.Negative = true
		ev.ObservedSubjectModels = observedModel("model-b")

		got := evaluate.Evaluate(ev)

		if got.Verdict != domain.VerdictFail || !hasReason(got.Reasons, domain.ReasonModelUnverified) {
			t.Errorf("negative run with wrong model: verdict %q reasons %v, want FAIL with %q (the reason is what shows the model check applied, not the inversion)", got.Verdict, got.Reasons, domain.ReasonModelUnverified)
		}
	})
	t.Run("timed out run keeps the reason", func(t *testing.T) {
		ev := baseEvidence()
		ev.SubjectResult.Disposition = domain.DispositionTimedOut
		ev.ObservedSubjectModels = observedModel("model-b")

		got := evaluate.Evaluate(ev)

		if !hasReason(got.Reasons, domain.ReasonModelUnverified) {
			t.Errorf("Reasons = %v, want it to include %q", got.Reasons, domain.ReasonModelUnverified)
		}
		if got.ModelValidity != domain.ModelMismatch {
			t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelMismatch)
		}
	})
}

func TestEvaluate_SpawnFailedPath_CarriesModelFields(t *testing.T) {
	ev := baseEvidence()
	ev.SubjectResult = domain.SubjectResult{Disposition: domain.DispositionSpawnFailed, ExitCode: 1}
	ev.ObservedSubjectModels = domain.ObservedModelEvidence{
		NoEvidence: domain.NoEvidenceNoData,
		Detail:     "no logs",
	}

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelNoEvidence {
		t.Errorf("ModelValidity = %q, want %q on the spawn-failed path", got.ModelValidity, domain.ModelNoEvidence)
	}
	if got.ModelValidityDetail == "" {
		t.Errorf("ModelValidityDetail is empty, want the no-evidence explanation")
	}
	if !hasReason(got.Reasons, domain.ReasonInfrastructure) || !hasReason(got.Reasons, domain.ReasonModelUnverified) {
		t.Errorf("Reasons = %v, want both %q and %q", got.Reasons, domain.ReasonInfrastructure, domain.ReasonModelUnverified)
	}
}

func TestEvaluate_SpawnFailedPath_WrongModelIsMismatch(t *testing.T) {
	ev := baseEvidence()
	ev.SubjectResult = domain.SubjectResult{Disposition: domain.DispositionSpawnFailed, ExitCode: 1}
	ev.ObservedSubjectModels = observedModel("model-b")

	got := evaluate.Evaluate(ev)

	if got.ModelValidity != domain.ModelMismatch {
		t.Errorf("ModelValidity = %q, want %q", got.ModelValidity, domain.ModelMismatch)
	}
	if len(got.ObservedSubjectModels) != 1 || got.ObservedSubjectModels[0] != "model-b" {
		t.Errorf("ObservedSubjectModels = %v, want [model-b] on the spawn-failed path", got.ObservedSubjectModels)
	}
}
