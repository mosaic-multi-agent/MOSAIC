package cli_test

// The Timeline collapses long breaks for drawing only: the JSON report of a run
// with a 3 day break keeps the real figures and carries no collapse figure.

import (
	"sort"
	"strings"
	"testing"
)

const longBreakRun = "20261031T090000Z-3d01"

func TestReportJSON_RunWithALongBreak_KeepsRealWallTimeFigures(t *testing.T) {
	report := e2eReport(t, "../../testdata/longbreak/logs")

	wt := objectAt(t, e2eRunsOf(report)[longBreakRun], "wall_time")

	want := map[string]float64{"span_ms": 277_200_000, "between_sessions_ms": 266_400_000}
	for key, v := range want {
		if wt[key] != v {
			t.Errorf("wall_time.%s = %v, want the real %v", key, wt[key], v)
		}
	}
}

func TestReportJSON_WallTime_HasNoCollapseFigure(t *testing.T) {
	report := e2eReport(t, "../../testdata/longbreak/logs")

	wt := objectAt(t, e2eRunsOf(report)[longBreakRun], "wall_time")

	var keys []string
	for k := range wt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Contains(strings.ToLower(k), "collaps") {
			t.Errorf("wall_time carries %q, but collapsing changes the drawing only", k)
		}
	}
	// The existing wall time keys stay.
	for _, k := range []string{"span_ms", "waiting_ms", "subagents_ms", "orchestrator_only_ms", "between_sessions_ms"} {
		if _, ok := wt[k]; !ok {
			t.Errorf("wall_time lacks %q; keys = %v", k, keys)
		}
	}
}
