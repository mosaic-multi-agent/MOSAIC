package cli_test

// Every totals object of the report JSON, not only the run, actor and
// all-runs totals, carries assumed_cache_write_tiers.
//
// Rules under test:
//   - The overview, the agent-type groups with their dispatches and the
//     across-runs runs and agent types each name the tiers of their own figures.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func tiersTotals(tokens int64, tiers []domain.CacheWriteTier) domain.Totals {
	return domain.Totals{AssumedTierCacheWrite: tokens, AssumedTiers: tiers}
}

// objectAtPath follows a dotted path of object keys and list indexes (a.b.0.c).
func objectAtPath(t *testing.T, m map[string]any, path string) map[string]any {
	t.Helper()
	var cur any = m
	for _, step := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[step]
		case []any:
			idx := int(step[0] - '0')
			if idx >= len(node) {
				t.Fatalf("path %q: index %d out of range", path, idx)
			}
			cur = node[idx]
		default:
			t.Fatalf("path %q: cannot step into %v with %q", path, cur, step)
		}
	}
	obj, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("path %q is not an object: %v", path, cur)
	}
	return obj
}

func viewLevelReport() domain.Report {
	run := domain.RunReport{
		Run: domain.NamedRun(cliTestRunID),
		Overview: domain.RunOverview{
			Totals:       tiersTotals(10, bothTier),
			Orchestrator: tiersTotals(20, minTier),
			Subagents:    domain.GroupFigures{Totals: tiersTotals(30, hourTier)},
		},
		ByAgentType: []domain.AgentTypeGroup{{
			AgentType: "coder",
			Figures:   domain.GroupFigures{Totals: tiersTotals(40, hourTier)},
			Dispatches: []domain.Dispatch{{
				Totals: tiersTotals(50, minTier),
			}},
		}},
	}
	return domain.Report{
		Currency: domain.Currency,
		Runs:     []domain.RunReport{run},
		AcrossRuns: domain.AcrossRuns{
			Runs: []domain.RunSummary{{
				Run:       domain.NamedRun(cliTestRunID),
				Totals:    tiersTotals(60, hourTier),
				Subagents: domain.GroupFigures{Totals: tiersTotals(70, minTier)},
			}},
			ByAgentType: []domain.AgentTypeAcrossRuns{{
				AgentType: "coder",
				Figures:   domain.GroupFigures{Totals: tiersTotals(80, bothTier)},
			}},
		},
	}
}

func TestEncodeReport_AssumedTiers_AppearOnEveryViewLevelTotals(t *testing.T) {
	m := decodeReportJSON(t, viewLevelReport())

	want := map[string][]string{
		"runs.0.overview.totals":                     {"5m", "1h"},
		"runs.0.overview.orchestrator":               {"5m"},
		"runs.0.overview.subagents.totals":           {"1h"},
		"runs.0.by_agent_type.0.figures.totals":      {"1h"},
		"runs.0.by_agent_type.0.dispatches.0.totals": {"5m"},
		"across_runs.runs.0.totals":                  {"1h"},
		"across_runs.runs.0.subagents.totals":        {"5m"},
		"across_runs.by_agent_type.0.figures.totals": {"5m", "1h"},
	}
	got := map[string][]string{}
	for path := range want {
		got[path] = decodeTiers(t, path, objectAtPath(t, m, path))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assumed tiers = %v, want %v", got, want)
	}
}
