package cli_test

// Tests that the cache hit figure keeps its JSON keys and carries the redefined
// values: capped cache reads over the previous call's context, over calls whose
// previous call is in the same session.
//
// Rules under test:
//   - cache.hit_ratio on the run, the agent-type group and the dispatch, and
//     overview.cache_hit_ratio, keep their keys and their {numerator,
//     denominator} shape.
//   - The values follow the new definition (a run with a single call per actor
//     yields 0/0, not reads over the whole context).
//   - The overview figure equals the run's cache.hit_ratio.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

var chjBase = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// chjCall is a usage record with all four categories present, at the given
// second. The context is input + cache read + cache write.
func chjCall(id domain.AgentInstanceID, recordID string, sec int, in, read, write int64) domain.Event {
	return domain.Event{
		Type:      domain.EventUsageRecord,
		Timestamp: chjBase.Add(time.Duration(sec) * time.Second),
		UsageRecord: &domain.UsageRecordFields{
			AgentInstanceID: id, RecordID: recordID, Model: "m-a",
			Usage: domain.TokenUsage{
				Input: domain.Tokens(in), CacheRead: domain.Tokens(read),
				CacheCreation: domain.Tokens(write), Output: domain.Tokens(10),
			},
			HasUsage: true,
		},
	}
}

// chjAgent wraps an agent's usage records in its invocation start and end.
func chjAgent(id domain.AgentInstanceID, calls ...domain.Event) []domain.Event {
	start := domain.Event{
		Type: domain.EventInvocationStart, Timestamp: chjBase,
		InvocationStart: &domain.InvocationStartFields{AgentInstanceID: id, AgentType: "coder"},
	}
	end := domain.Event{
		Type: domain.EventInvocationEnd, Timestamp: chjBase.Add(time.Hour),
		InvocationEnd: &domain.InvocationEndFields{AgentInstanceID: id, StatusCode: "SUCCESS", Model: "m-a"},
	}
	return append(append([]domain.Event{start}, calls...), end)
}

// chjRunObject analyses a run and returns the first run's JSON object.
func chjRunObject(t *testing.T, orch []domain.Event, agents map[domain.AgentInstanceID][]domain.Event) map[string]any {
	t.Helper()
	report := providerGapReportFrom(orch, agents)
	if len(report.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(report.Runs))
	}
	return firstRun(t, decodeReportJSON(t, report))
}

func TestEncodeReport_CacheHitRatio_CarriesTheRedefinedFigureUnderTheSameKeys(t *testing.T) {
	run := chjRunObject(t,
		[]domain.Event{chjCall("", "o1", 10, 200, 0, 0), chjCall("", "o2", 20, 100, 200, 0)},
		map[domain.AgentInstanceID][]domain.Event{
			"a#1": chjAgent("a#1", chjCall("a#1", "r1", 10, 1000, 0, 0), chjCall("a#1", "r2", 20, 600, 500, 0)),
		})

	group := vwObj(t, vwList(t, run, "by_agent_type")[0])
	dispatch := vwObj(t, vwList(t, group, "dispatches")[0])

	// Orchestrator 200/200 (its reads are capped at its previous context) plus the
	// agent's 500/1000.
	vwAssertRatio(t, "run cache", objectAt(t, run, "cache"), "hit_ratio", 700, 1200)
	vwAssertRatio(t, "overview", objectAt(t, run, "overview"), "cache_hit_ratio", 700, 1200)
	vwAssertRatio(t, "group cache", objectAt(t, group, "cache"), "hit_ratio", 500, 1000)
	vwAssertRatio(t, "dispatch cache", objectAt(t, dispatch, "cache"), "hit_ratio", 500, 1000)
}

func TestEncodeReport_CacheHitRatio_ReadsAboveThePreviousContextAreCappedInTheValues(t *testing.T) {
	run := chjRunObject(t, nil, map[domain.AgentInstanceID][]domain.Event{
		"a#1": chjAgent("a#1", chjCall("a#1", "r1", 10, 100, 0, 0), chjCall("a#1", "r2", 20, 100, 400, 0)),
	})

	vwAssertRatio(t, "run cache", objectAt(t, run, "cache"), "hit_ratio", 100, 100)
	vwAssertRatio(t, "overview", objectAt(t, run, "overview"), "cache_hit_ratio", 100, 100)
}

func TestEncodeReport_CacheHitRatio_UndefinedWhenNoCallHasAPreviousCall(t *testing.T) {
	run := chjRunObject(t, nil, map[domain.AgentInstanceID][]domain.Event{
		"a#1": chjAgent("a#1", chjCall("a#1", "r1", 10, 100, 900, 0)),
	})

	// One call: under the old definition this was 900/1000.
	vwAssertRatio(t, "run cache", objectAt(t, run, "cache"), "hit_ratio", 0, 0)
	vwAssertRatio(t, "overview", objectAt(t, run, "overview"), "cache_hit_ratio", 0, 0)
}

func TestEncodeReport_CacheHitRatio_EveryCacheObjectKeepsTheKey(t *testing.T) {
	run := chjRunObject(t, nil, map[domain.AgentInstanceID][]domain.Event{
		"a#1": chjAgent("a#1", chjCall("a#1", "r1", 10, 100, 0, 0), chjCall("a#1", "r2", 20, 100, 100, 0)),
	})
	group := vwObj(t, vwList(t, run, "by_agent_type")[0])
	dispatch := vwObj(t, vwList(t, group, "dispatches")[0])

	for name, cache := range map[string]map[string]any{
		"run": objectAt(t, run, "cache"), "group": objectAt(t, group, "cache"),
		"dispatch": objectAt(t, dispatch, "cache"),
	} {
		if _, ok := cache["hit_ratio"]; !ok {
			t.Errorf("%s cache has no hit_ratio key; keys = %v", name, sortedKeys(cache))
		}
	}
	if _, ok := objectAt(t, run, "overview")["cache_hit_ratio"]; !ok {
		t.Error("overview has no cache_hit_ratio key")
	}
}

