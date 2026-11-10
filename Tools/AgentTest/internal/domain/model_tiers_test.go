package domain_test

import (
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
)

// TestModelSelection_EffectiveStub covers the stub fallback: an unselected
// stub model means the subject's model.
func TestModelSelection_EffectiveStub(t *testing.T) {
	cases := []struct {
		name string
		sel  domain.ModelSelection
		want string
	}{
		{"explicit stub wins", domain.ModelSelection{Subject: "subj", Stub: "stub"}, "stub"},
		{"empty stub falls back to subject", domain.ModelSelection{Subject: "subj"}, "subj"},
		{"stub without subject keeps stub", domain.ModelSelection{Stub: "stub"}, "stub"},
		{"zero value stays empty", domain.ModelSelection{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sel.EffectiveStub(); got != tc.want {
				t.Errorf("EffectiveStub() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestModelSelection_TierModels pins the one tier-to-model derivation: exactly
// the two AgentTest tiers, stub falling back to subject.
func TestModelSelection_TierModels(t *testing.T) {
	cases := []struct {
		name string
		sel  domain.ModelSelection
		want map[string]string
	}{
		{
			"distinct subject and stub",
			domain.ModelSelection{Subject: "subj", Stub: "stub"},
			map[string]string{domain.TierTestSubject: "subj", domain.TierTestStub: "stub"},
		},
		{
			"stub defaults to subject",
			domain.ModelSelection{Subject: "subj"},
			map[string]string{domain.TierTestSubject: "subj", domain.TierTestStub: "subj"},
		},
		{
			"zero value keeps both entries present and empty",
			domain.ModelSelection{},
			map[string]string{domain.TierTestSubject: "", domain.TierTestStub: ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.sel.TierModels()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TierModels() = %v, want %v", got, tc.want)
			}
		})
	}
}
