package app

// transform_plan_shared_eligibility_test.go pins the results of the code transform shares
// with other flows: the harness-only eligibility rule, the workspace agent scan Update uses
// and the eligibility check promote applies. Transform recognises more files than these (it
// accepts deployed utility and standalone agents); none of that may change what they report.

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// tpScanCatalog knows the catalog agent "code-reviewer" (numeric id 55) and the main
// orchestrator "orchestrator".
type tpScanCatalog struct{ catalog.Catalog }

func (tpScanCatalog) AgentByNumericID(id string) (domain.Agent, bool) {
	if id == "55" {
		return domain.Agent{Key: "code-reviewer", NumericID: "55"}, true
	}
	return domain.Agent{}, false
}

func (tpScanCatalog) Agent(key string) (domain.Agent, bool) {
	if key == "orchestrator" {
		return domain.Agent{Key: "orchestrator"}, true
	}
	return domain.Agent{}, false
}

func TestEligibleHarnessOnly_FixtureVerdictsAreUnchanged(t *testing.T) {
	cases := []struct {
		fixture  string
		eligible bool
	}{
		{"subagent.md", true},
		{"orchestrator.md", true},
		{"harness-only.md", true},
		{"utility.md", false},    // stamped, no canonical section
		{"standalone.md", false}, // stamped, no canonical section
		{"plain-notes.md", false},
		{"no-frontmatter.md", false},
		{"deploy-todo.md", false},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			v := eligibleHarnessOnly([]byte(tpFixture(t, "agents/"+c.fixture)))

			if v.Eligible != c.eligible {
				t.Errorf("Eligible = %v (%s), want %v", v.Eligible, v.Reason, c.eligible)
			}
			if !c.eligible && v.Reason == "" {
				t.Errorf("ineligible verdict carries no reason")
			}
		})
	}
}

func TestScanWorkspaceAgents_ClassificationOfTransformFixturesIsUnchanged(t *testing.T) {
	ws := t.TempDir()
	for _, f := range []string{"subagent", "orchestrator", "orchestrator-script", "utility", "standalone", "harness-only", "plain-notes", "no-frontmatter", "deploy-todo"} {
		tpPlaceAgent(t, ws, ".claude/agents", f+".md", f+".md")
	}

	scan, _ := scanWorkspaceAgents(ws, ".claude/agents", tpScanCatalog{}, nil)

	var matched []string
	for _, m := range scan.Matched {
		matched = append(matched, m.FileName+"="+m.AgentKey)
	}
	sort.Strings(matched)
	if want := []string{"orchestrator.md=orchestrator", "subagent.md=code-reviewer"}; !reflect.DeepEqual(matched, want) {
		t.Errorf("Matched = %v, want %v", matched, want)
	}
	var only []string
	for _, h := range scan.HarnessOnly {
		only = append(only, h.FileName)
	}
	if want := []string{"harness-only.md"}; !reflect.DeepEqual(only, want) {
		t.Errorf("HarnessOnly = %v, want %v (utility, standalone, script orchestrator and non-agent files stay out)", only, want)
	}
}

func TestScanWorkspaceAgents_IgnoresDeployedUtilityAndStandaloneAgentsStampedOnly(t *testing.T) {
	ws := t.TempDir()
	tpPlaceAgent(t, ws, ".claude/agents", "utility.md", "utility.md")
	tpPlaceAgent(t, ws, ".claude/agents", "standalone.md", "standalone.md")

	scan, _ := scanWorkspaceAgents(ws, ".claude/agents", tpScanCatalog{}, nil)

	if len(scan.Matched) != 0 || len(scan.HarnessOnly) != 0 {
		t.Errorf("scan = %+v, want empty for stamped files without a canonical section", scan)
	}
}

func TestBuildGenericAgent_StampedFileWithoutCanonicalSectionIsStillNotPromotable(t *testing.T) {
	for _, f := range []string{"utility.md", "standalone.md", "plain-notes.md"} {
		t.Run(f, func(t *testing.T) {
			_, err := buildGenericAgent(PromoteInput{Source: []byte(tpFixture(t, "agents/"+f)), NumericID: "90"})

			if !errors.Is(err, ErrPromoteNotTransformed) {
				t.Errorf("err = %v, want ErrPromoteNotTransformed", err)
			}
		})
	}
}

func TestBuildGenericAgent_EligibleFixtureIsStillPromotable(t *testing.T) {
	out, err := buildGenericAgent(PromoteInput{Source: []byte(tpFixture(t, "agents/harness-only.md")), NumericID: "90", Key: "team-bot"})

	if err != nil {
		t.Fatalf("buildGenericAgent: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("promote produced no output for an eligible agent")
	}
}
