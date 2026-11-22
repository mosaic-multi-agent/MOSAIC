package opencodev2_test

// Tests for the environment check: scope-based only. The harness runs no
// interpreter and the check does not look for the executable; a missing
// executable is reported by the launcher at spawn.

import (
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func TestCheckEnvironment_DeclaresNoInterpreterAndCarriesScopeFindings(t *testing.T) {
	a := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}})

	report, err := a.CheckEnvironment(testContext())
	if err != nil {
		t.Fatalf("CheckEnvironment: %v", err)
	}

	if report.InterpreterApplicable || report.InterpreterCmd != "" || len(report.InterpreterTried) != 0 {
		t.Errorf("report = %+v, want no interpreter declared", report)
	}
	want, err := a.InspectScopes(testContext())
	if err != nil {
		t.Fatalf("InspectScopes: %v", err)
	}
	if len(want) == 0 || !reflect.DeepEqual(report.Scopes, want) {
		t.Errorf("report.Scopes = %+v, want InspectScopes' findings %+v", report.Scopes, want)
	}
	if !report.OK() {
		t.Errorf("report problems = %+v, want a usable environment when every scope is absent", report.Problems)
	}
}

func TestCheckEnvironment_CompetingRewriterInUserScopeIsAProblem(t *testing.T) {
	probe := fixtureScopeProbe{docs: map[string][]byte{"user-config-jsonc": readScopeFixture(t, "user-config-with-plugins.jsonc")}}

	report, err := opencodev2.New(opencodev2.Options{ScopeProbe: probe}).CheckEnvironment(testContext())
	if err != nil {
		t.Fatalf("CheckEnvironment: %v", err)
	}

	if report.OK() {
		t.Fatalf("report is OK, want a problem for a user scope that can rewrite the call")
	}
	var found bool
	for _, p := range report.Problems {
		if p.Kind == domain.ProblemCompetingRewriter {
			found = true
			if p.Detail == "" {
				t.Errorf("competing-rewriter problem has an empty Detail")
			}
		}
	}
	if !found {
		t.Errorf("problems = %+v, want a competing_rewriter entry", report.Problems)
	}
}

func TestCheckEnvironment_MCPOnlyConfigIsUsable(t *testing.T) {
	probe := fixtureScopeProbe{docs: map[string][]byte{"user-config": readScopeFixture(t, "user-config-clean.json")}}

	report, err := opencodev2.New(opencodev2.Options{ScopeProbe: probe}).CheckEnvironment(testContext())
	if err != nil {
		t.Fatalf("CheckEnvironment: %v", err)
	}

	if !report.OK() {
		t.Errorf("problems = %+v, want none for a config with only MCP servers", report.Problems)
	}
}
