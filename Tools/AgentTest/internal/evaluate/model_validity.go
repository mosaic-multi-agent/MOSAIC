package evaluate

import (
	"fmt"
	"strings"

	"mosaic-agent-test/internal/domain"
)

// SubjectModelValidity compares the selected subject model with the observed
// evidence. It fails closed and does not rely on the ObservedModelEvidence
// invariant. Rules, first match wins:
//  1. len(evidence.Models) == 0 (whatever NoEvidence holds)      -> ModelNoEvidence
//  2. evidence.NoEvidence != "" (even with Models non-empty)     -> ModelNoEvidence
//  3. any observed entry == ""                                   -> ModelNoEvidence
//  4. selected != "" and every observed entry == selected        -> ModelVerified
//  5. otherwise (including selected == "")                       -> ModelMismatch
//
// detail is "" for ModelVerified. Otherwise it contains the selected value and
// the observed values ("none" when Models is empty), plus, for ModelNoEvidence,
// the no-evidence reason and detail when NoEvidence is set, or the text
// "evidence was empty" / "evidence was malformed" otherwise.
func SubjectModelValidity(selected string, evidence domain.ObservedModelEvidence) (domain.ModelValidity, string) {
	models := evidence.Models
	observed := "none"
	if len(models) > 0 {
		observed = strings.Join(models, ", ")
	}
	base := fmt.Sprintf("selected %q, observed %s", selected, observed)

	if len(models) == 0 || evidence.NoEvidence != "" || containsEmpty(models) {
		return domain.ModelNoEvidence, base + "; " + noEvidenceCause(evidence)
	}
	for _, m := range models {
		if m != selected || selected == "" {
			return domain.ModelMismatch, base
		}
	}
	return domain.ModelVerified, ""
}

func containsEmpty(models []string) bool {
	for _, m := range models {
		if m == "" {
			return true
		}
	}
	return false
}

func noEvidenceCause(evidence domain.ObservedModelEvidence) string {
	if evidence.NoEvidence != "" {
		cause := string(evidence.NoEvidence)
		if evidence.Detail != "" {
			cause += ": " + evidence.Detail
		}
		return cause
	}
	if len(evidence.Models) == 0 {
		return "evidence was empty"
	}
	return "evidence was malformed"
}

// applyModelValidity records the observed models and the validity decision on
// result, adding the model-unverified reason and forcing a failing verdict
// when the subject model is not verified.
func applyModelValidity(result domain.TestResult, ev domain.RunEvidence) domain.TestResult {
	result.ObservedSubjectModels = append([]string(nil), ev.ObservedSubjectModels.Models...)
	validity, detail := SubjectModelValidity(ev.SubjectModel, ev.ObservedSubjectModels)
	result.ModelValidity = validity
	result.ModelValidityDetail = detail
	if validity != domain.ModelVerified {
		result.Reasons = append(result.Reasons, domain.ReasonModelUnverified)
		if result.Verdict == domain.VerdictPass {
			result.Verdict = domain.VerdictFail
		}
	}
	return result
}
