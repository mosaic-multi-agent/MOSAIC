package runner_test

// The authored Subject.Model and Subject.StubModel fields take no part in the
// setup deploy's tier map. Only the user's model selection counts: the subject
// tier gets the subject selection, the stub tier gets the stub selection or,
// when none was made, the subject selection. A model outside the selection
// must never reach the deployed definitions.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

const (
	authoredSubjectValue = "authored-subject-value"
	authoredStubValue    = "authored-stub-value"
)

// setupDeployTierMap runs the request and returns the tier map of the first
// (setup) Deploy call.
func setupDeployTierMap(t *testing.T, req runner.Request) map[string]string {
	t.Helper()
	h := newHarness(t)
	// A selection without a subject model deploys definitions that carry no
	// model, which the pre-spawn check rejects after the deploy call; the tier
	// map under test was sent before that, so that outcome is tolerated here.
	var defErr *runner.DefinitionModelError
	if _, err := runner.Run(context.Background(), h.Deps, req, nil); err != nil && !errors.As(err, &defErr) {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	calls := h.Deployer.allDeployCalls()
	if len(calls) == 0 {
		t.Fatal("Deploy was never called for a catalogue-path test")
	}
	return calls[0].TierModels
}

func TestSetup_DeployTierMap_IgnoresAuthoredModelFields(t *testing.T) {
	cases := []struct {
		name string
		sel  domain.ModelSelection
	}{
		{"subject and stub selected", domain.ModelSelection{Subject: "selected-subject", Stub: "selected-stub"}},
		{"stub follows subject selection", domain.ModelSelection{Subject: "selected-subject"}},
		{"empty selection", domain.ModelSelection{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := catalogueRequestWithModels("authored-ignored", authoredSubjectValue, authoredStubValue, tc.sel)

			got := setupDeployTierMap(t, req)

			if want := tc.sel.TierModels(); !reflect.DeepEqual(got, want) {
				t.Errorf("setup Deploy TierModels = %v, want the selection's derivation %v", got, want)
			}
		})
	}
}

func TestSetup_DeployTierMap_AuthoredSubjectModelAloneDoesNotReachAnyTier(t *testing.T) {
	// The selection is empty, so a fallback to the authored value would be the
	// only way for it to reach a tier.
	req := catalogueRequestWithModels("authored-subject-only", authoredSubjectValue, "", domain.ModelSelection{})

	got := setupDeployTierMap(t, req)

	if subject := got[domain.TierTestSubject]; subject != "" {
		t.Errorf("subject tier = %q, want the empty subject selection", subject)
	}
	for tier, model := range got {
		if model == authoredSubjectValue {
			t.Errorf("tier %q carries the authored Subject.Model %q; only the selection may supply tier models", tier, model)
		}
	}
}

func TestSetup_DeployTierMap_StubOnlySelectionLeavesSubjectTierEmpty(t *testing.T) {
	req := catalogueRequestWithModels("stub-only-selection", authoredSubjectValue, authoredStubValue, domain.ModelSelection{Stub: "selected-stub"})

	got := setupDeployTierMap(t, req)

	want := domain.ModelSelection{Stub: "selected-stub"}.TierModels()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("setup Deploy TierModels = %v, want the selection's derivation %v", got, want)
	}
	if subject := got[domain.TierTestSubject]; subject != "" {
		t.Errorf("subject tier = %q, want empty when no subject was selected", subject)
	}
}

func TestSetup_DeployTierMap_AuthoredStubModelDoesNotReachStubTier(t *testing.T) {
	req := catalogueRequestWithModels("authored-stub-only", "", authoredStubValue, domain.ModelSelection{Subject: "selected-subject"})

	got := setupDeployTierMap(t, req)

	if stub := got[domain.TierTestStub]; stub != "selected-subject" {
		t.Errorf("stub tier = %q, want the subject selection %q when no stub was selected", stub, "selected-subject")
	}
}

func TestSetup_DeployTierMap_HasExactlyTheTwoTierEntries(t *testing.T) {
	sels := []domain.ModelSelection{
		{Subject: "selected-subject", Stub: "selected-stub"},
		{Subject: "selected-subject"},
		{},
	}
	for _, sel := range sels {
		req := catalogueRequestWithModels("two-entries", authoredSubjectValue, authoredStubValue, sel)

		got := setupDeployTierMap(t, req)

		if len(got) != 2 {
			t.Errorf("selection %+v: TierModels has %d entries, want exactly 2: %v", sel, len(got), got)
		}
		for _, tier := range []string{domain.TierTestSubject, domain.TierTestStub} {
			if _, ok := got[tier]; !ok {
				t.Errorf("selection %+v: TierModels missing tier %q: %v", sel, tier, got)
			}
		}
	}
}
