package runner_test

// The setup deploy and the preflight dry-run deploy must send the same
// tier-to-model map for the same selection. Preflight sends the selection's
// own derivation, so the runner's setup deploy must send exactly that map.
// The authored model fields are empty, as the definition parser leaves them.
// Regression coverage: these cases hold under the retired fallback chain too.

import (
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
)

func TestSetup_DeployTierMap_EqualsTheSelectionsDerivation(t *testing.T) {
	cases := []struct {
		name string
		sel  domain.ModelSelection
	}{
		{"subject and stub selected", domain.ModelSelection{Subject: "selected-subject", Stub: "selected-stub"}},
		{"stub follows subject", domain.ModelSelection{Subject: "selected-subject"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := catalogueRequestWithModels("shared-derivation", "", "", tc.sel)

			got := setupDeployTierMap(t, req)

			if want := tc.sel.TierModels(); !reflect.DeepEqual(got, want) {
				t.Errorf("setup Deploy TierModels = %v, want the selection's derivation %v", got, want)
			}
		})
	}
}
