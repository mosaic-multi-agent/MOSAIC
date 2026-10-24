package evaluate_test

// Tests for the pure run-validity decision: a run is valid only when every
// observed subject model equals the selected subject model by exact string
// equality. Everything else is invalid, and the detail names both sides.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/evaluate"
)

func TestSubjectModelValidity_Decision(t *testing.T) {
	tests := []struct {
		name     string
		selected string
		evidence domain.ObservedModelEvidence
		want     domain.ModelValidity
	}{
		{"observed equals selection", "model-a", observedModel("model-a"), domain.ModelVerified},
		{"provider qualified equal", "anthropic/model-a", observedModel("anthropic/model-a"), domain.ModelVerified},
		{"several observed all equal", "model-a", observedModel("model-a", "model-a"), domain.ModelVerified},
		{"different model", "model-a", observedModel("model-b"), domain.ModelMismatch},
		{"case difference", "model-a", observedModel("Model-A"), domain.ModelMismatch},
		{"alias instead of full id", "claude-sonnet-5-5", observedModel("sonnet"), domain.ModelMismatch},
		{"observed has a dated suffix", "claude-sonnet-5-5", observedModel("claude-sonnet-5-5-20260101"), domain.ModelMismatch},
		{"selection has a suffix", "claude-sonnet-5-5-20260101", observedModel("claude-sonnet-5-5"), domain.ModelMismatch},
		{"surrounding whitespace", "model-a", observedModel("model-a "), domain.ModelMismatch},
		{"provider prefix missing", "anthropic/model-a", observedModel("model-a"), domain.ModelMismatch},
		{"several observed one different", "model-a", observedModel("model-a", "model-b"), domain.ModelMismatch},
		{"several observed all different", "model-a", observedModel("model-b", "model-c"), domain.ModelMismatch},
		{"empty selection never verifies", "", observedModel("model-a"), domain.ModelMismatch},
		{"zero value evidence", "model-a", domain.ObservedModelEvidence{}, domain.ModelNoEvidence},
		{"explicit no evidence", "model-a",
			domain.ObservedModelEvidence{NoEvidence: domain.NoEvidenceToolFailed, Detail: "exit 3"}, domain.ModelNoEvidence},
		{"no evidence with empty selection", "",
			domain.ObservedModelEvidence{NoEvidence: domain.NoEvidenceNoData, Detail: "no logs"}, domain.ModelNoEvidence},
		{"models present but no evidence flagged", "model-a",
			domain.ObservedModelEvidence{Models: []string{"model-a"}, NoEvidence: domain.NoEvidenceUnparseable, Detail: "bad json"},
			domain.ModelNoEvidence},
		{"empty observed entry", "model-a", observedModel(""), domain.ModelNoEvidence},
		{"empty observed entry among valid ones", "model-a", observedModel("model-a", ""), domain.ModelNoEvidence},
		{"empty models slice", "model-a", observedModel(), domain.ModelNoEvidence},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := evaluate.SubjectModelValidity(tc.selected, tc.evidence)
			if got != tc.want {
				t.Errorf("SubjectModelValidity(%q, %+v) = %q, want %q", tc.selected, tc.evidence, got, tc.want)
			}
		})
	}
}

func TestSubjectModelValidity_VerifiedHasNoDetail(t *testing.T) {
	_, detail := evaluate.SubjectModelValidity("model-a", observedModel("model-a"))
	if detail != "" {
		t.Errorf("detail for a verified run = %q, want empty", detail)
	}
}

func TestSubjectModelValidity_MismatchDetailNamesSelectedAndObserved(t *testing.T) {
	validity, detail := evaluate.SubjectModelValidity("model-a", observedModel("model-b", "model-c"))

	if validity != domain.ModelMismatch {
		t.Fatalf("validity = %q, want %q", validity, domain.ModelMismatch)
	}
	for _, want := range []string{"model-a", "model-b", "model-c"} {
		if !strings.Contains(detail, want) {
			t.Errorf("mismatch detail %q does not contain %q; it must show the selected and every observed value", detail, want)
		}
	}
}

func TestSubjectModelValidity_NoEvidenceDetailNamesSelectionReasonAndCause(t *testing.T) {
	validity, detail := evaluate.SubjectModelValidity("model-a", domain.ObservedModelEvidence{
		NoEvidence: domain.NoEvidenceRunNotFound,
		Detail:     "run 20260101T000000Z-abcd not in report",
	})

	if validity != domain.ModelNoEvidence {
		t.Fatalf("validity = %q, want %q", validity, domain.ModelNoEvidence)
	}
	for _, want := range []string{"model-a", "none", string(domain.NoEvidenceRunNotFound), "not in report"} {
		if !strings.Contains(detail, want) {
			t.Errorf("no-evidence detail %q does not contain %q", detail, want)
		}
	}
}

func TestSubjectModelValidity_EmptyEvidenceDetailSaysSo(t *testing.T) {
	_, detail := evaluate.SubjectModelValidity("model-a", domain.ObservedModelEvidence{})

	for _, want := range []string{"model-a", "evidence was empty"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q does not contain %q", detail, want)
		}
	}
}

func TestSubjectModelValidity_MalformedEvidenceDetailSaysSo(t *testing.T) {
	_, detail := evaluate.SubjectModelValidity("model-a", observedModel("model-a", ""))

	for _, want := range []string{"model-a", "evidence was malformed"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q does not contain %q", detail, want)
		}
	}
}
