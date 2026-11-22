package plan_test

// build_withheld_capability_test.go covers the notices a harness module raises through
// ToolResult.Gaps: plan building carries them into the plan's gaps, once per agent, for an
// agent that is created.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// withheldModule returns a fake module whose Tools call raises one withheld-capability
// notice for the requesting agent.
func withheldModule() *fakeModule {
	m := newFakeModule()
	m.ToolsFn = func(req domain.ToolRequest) (domain.ToolResult, error) {
		return domain.ToolResult{
			Gaps: []domain.Gap{{
				Kind:    domain.GapToolCapabilityWithheld,
				Subject: "web_fetch",
				Owner:   req.AgentKey,
				Detail:  "no network access for " + req.AgentKey,
			}},
		}, nil
	}
	return m
}

func withheldGapsOf(p domain.Plan) []domain.Gap {
	var out []domain.Gap
	for _, g := range p.Gaps {
		if g.Kind == domain.GapToolCapabilityWithheld {
			out = append(out, g)
		}
	}
	return out
}

func TestBuild_ModuleToolGap_AppearsInPlanGapsOnceOwnedByAgent(t *testing.T) {
	// Arrange / Act
	p := buildCreatePlan(t, withheldModule())

	// Assert
	var forAgent []domain.Gap
	for _, g := range withheldGapsOf(p) {
		if g.Owner == "test-agent" {
			forAgent = append(forAgent, g)
		}
	}
	if len(forAgent) != 1 {
		t.Fatalf("withheld-capability gaps for test-agent = %+v, want exactly one; all gaps: %+v", forAgent, p.Gaps)
	}
	if forAgent[0].Subject != "web_fetch" {
		t.Errorf("gap Subject = %q, want web_fetch", forAgent[0].Subject)
	}
}

func TestBuild_ModuleWithoutToolGaps_ProducesNoWithheldGap(t *testing.T) {
	// Arrange / Act
	p := buildCreatePlan(t, newFakeModule())

	// Assert
	if gaps := withheldGapsOf(p); len(gaps) != 0 {
		t.Errorf("withheld-capability gaps = %+v, want none", gaps)
	}
}
