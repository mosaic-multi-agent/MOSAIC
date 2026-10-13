package domain_test

// Tests of the infrastructure trigger vocabulary: the validity check and the
// refusal reason with its location-specific guidance. Assertions are on the
// presence of key tokens, not on exact wording.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestInfraTriggerVocabulary_ListsTheFourNamesInOrder(t *testing.T) {
	want := []string{"INVOCATION_INTERVAL", "ITERATION_END", "PHASE_END", "MANUAL"}

	got := domain.InfraTriggerVocabulary()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestInfraTriggerVocabulary_ReturnsAnIndependentSlice(t *testing.T) {
	first := domain.InfraTriggerVocabulary()
	if len(first) == 0 {
		t.Fatal("vocabulary is empty")
	}
	first[0] = "CHANGED"

	second := domain.InfraTriggerVocabulary()

	if len(second) == 0 || second[0] == "CHANGED" {
		t.Errorf("a caller's modification must not reach later calls, got %v", second)
	}
}

func TestIsValidInfraTrigger_VocabularyNames_Valid(t *testing.T) {
	for _, name := range []string{
		domain.InfraTriggerInvocationInterval,
		domain.InfraTriggerIterationEnd,
		domain.InfraTriggerPhaseEnd,
		domain.InfraTriggerManual,
	} {
		if !domain.IsValidInfraTrigger(name) {
			t.Errorf("%q must be valid", name)
		}
	}
}

func TestIsValidInfraTrigger_SurroundingWhitespace_Trimmed(t *testing.T) {
	if !domain.IsValidInfraTrigger("  ITERATION_END\t") {
		t.Error("whitespace around a vocabulary name must be ignored")
	}
}

func TestIsValidInfraTrigger_OutsideVocabulary_Invalid(t *testing.T) {
	for _, name := range []string{
		"",
		"   ",
		"STAGE_END",
		"iteration_end",
		"Iteration_End",
		"\"ITERATION_END\"",
		"ITERATION_END_",
		"ITERATION END",
		"EVERY_NIGHT",
	} {
		if domain.IsValidInfraTrigger(name) {
			t.Errorf("%q must not be valid", name)
		}
	}
}

func TestInfraTriggerRefusalReason_ValidTrigger_Empty(t *testing.T) {
	got := domain.InfraTriggerRefusalReason("agent-x", "ITERATION_END", domain.InfraTriggerInDeclaration)

	if got != "" {
		t.Errorf("want empty reason for a valid trigger, got %q", got)
	}
}

func TestInfraTriggerRefusalReason_UnknownTrigger_NamesAgentTriggerAndVocabulary(t *testing.T) {
	for _, loc := range []domain.InfraTriggerLocation{domain.InfraTriggerInDeclaration, domain.InfraTriggerInOverride} {
		got := domain.InfraTriggerRefusalReason("agent-x", "EVERY_NIGHT", loc)

		for _, token := range []string{"agent-x", "EVERY_NIGHT", "INVOCATION_INTERVAL", "ITERATION_END", "PHASE_END", "MANUAL"} {
			if !strings.Contains(got, token) {
				t.Errorf("loc %q: reason must contain %q, got %q", loc, token, got)
			}
		}
	}
}

func TestInfraTriggerRefusalReason_QuotedTrigger_NamedVerbatimWithQuotes(t *testing.T) {
	got := domain.InfraTriggerRefusalReason("agent-x", "\"ITERATION_END\"", domain.InfraTriggerInOverride)

	if !strings.Contains(got, "\"ITERATION_END\"") {
		t.Errorf("reason must quote the trigger verbatim including its quotes, got %q", got)
	}
}

func TestInfraTriggerRefusalReason_BlankTrigger_SaysNoTriggerIsNamed(t *testing.T) {
	got := domain.InfraTriggerRefusalReason("agent-x", "  ", domain.InfraTriggerInDeclaration)

	if got == "" {
		t.Fatal("a blank trigger must be refused with a reason")
	}
	if !strings.Contains(got, "agent-x") {
		t.Errorf("reason must name the agent, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "no trigger") {
		t.Errorf("reason must say no trigger is named, got %q", got)
	}
}

func TestInfraTriggerRefusalReason_StageEndInDeclaration_PointsToRedeploy(t *testing.T) {
	got := domain.InfraTriggerRefusalReason("agent-x", "STAGE_END", domain.InfraTriggerInDeclaration)

	for _, token := range []string{"agent-x", "STAGE_END", "ITERATION_END"} {
		if !strings.Contains(got, token) {
			t.Errorf("reason must contain %q, got %q", token, got)
		}
	}
	if !strings.Contains(strings.ToLower(got), "redeploy") {
		t.Errorf("reason must tell the user to redeploy, got %q", got)
	}
}

func TestInfraTriggerRefusalReason_StageEndInOverride_SaysOverrideMustNameIterationEnd(t *testing.T) {
	got := domain.InfraTriggerRefusalReason("agent-x", "STAGE_END", domain.InfraTriggerInOverride)

	for _, token := range []string{"agent-x", "STAGE_END", "ITERATION_END"} {
		if !strings.Contains(got, token) {
			t.Errorf("reason must contain %q, got %q", token, got)
		}
	}
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "override") {
		t.Errorf("reason must say the override has to change, got %q", got)
	}
	if strings.Contains(lower, "redeploy") {
		t.Errorf("redeploying does not fix an override; reason must not suggest it, got %q", got)
	}
}

