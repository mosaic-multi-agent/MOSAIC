package cli_test

// Tests for effort in the report JSON (EncodeReport) and its absence from the
// total JSON (EncodeRunTotal).
//
// Rules under test:
//   - runs[].orchestrator, runs[].agents[] (and the same in unattributable) and
//     runs[].by_agent_type[].dispatches[] carry "effort": an array of
//     {"level", "records"} in the given order, or null when no level was reported.
//   - The key is always present.
//   - The total JSON is byte-identical with and without effort data.

import (
	"bytes"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

var (
	effortMedium = domain.EffortLevels{{Level: "medium", Records: 4}}
	effortMixed  = domain.EffortLevels{{Level: "high", Records: 2}, {Level: "medium", Records: 1}}
)

// effortRun is vwRun with orchestrator, agent and dispatch effort set.
func effortRun(orch, agent, dispatch domain.EffortLevels) domain.RunReport {
	run := vwRun()
	run.Orchestrator.Effort = orch
	run.Agents = []domain.ActorReport{{
		Actor:  domain.AgentInstance("coder#1"),
		Totals: vwTotals(300, true),
		Effort: agent,
	}}
	run.ByAgentType[0].Dispatches[0].Effort = dispatch
	return run
}

// effortValue returns the "effort" entry of obj, failing when the key is absent.
func effortValue(t *testing.T, where string, obj map[string]any) any {
	t.Helper()
	v, present := obj["effort"]
	if !present {
		t.Fatalf("%s has no effort key; keys = %v", where, sortedKeys(obj))
	}
	return v
}

func assertEffortList(t *testing.T, where string, obj map[string]any, want ...[2]any) {
	t.Helper()
	list, ok := effortValue(t, where, obj).([]any)
	if !ok {
		t.Fatalf("%s.effort = %v, want an array", where, obj["effort"])
	}
	if len(list) != len(want) {
		t.Fatalf("%s.effort has %d entries (%v), want %d", where, len(list), list, len(want))
	}
	for i, w := range want {
		e := vwObj(t, list[i])
		if e["level"] != w[0] || e["records"] != w[1] {
			t.Errorf("%s.effort[%d] = %v, want level %v with %v records", where, i, e, w[0], w[1])
		}
	}
}

func assertEffortNull(t *testing.T, where string, obj map[string]any) {
	t.Helper()
	if v := effortValue(t, where, obj); v != nil {
		t.Errorf("%s.effort = %v, want null", where, v)
	}
}

func effortDispatch(t *testing.T, run map[string]any) map[string]any {
	t.Helper()
	agentType := vwObj(t, vwList(t, run, "by_agent_type")[0])
	return vwObj(t, vwList(t, agentType, "dispatches")[0])
}

func TestEncodeReport_OrchestratorMediumSubagentHigh_EffortOnEachInvocation(t *testing.T) {
	high := domain.EffortLevels{{Level: "high", Records: 3}}
	run := firstRun(t, decodeReportJSON(t, vwReport(effortRun(effortMedium, high, high))))

	assertEffortList(t, "orchestrator", objectAt(t, run, "orchestrator"), [2]any{"medium", float64(4)})
	assertEffortList(t, "agents[0]", vwObj(t, vwList(t, run, "agents")[0]), [2]any{"high", float64(3)})
	assertEffortList(t, "dispatch", effortDispatch(t, run), [2]any{"high", float64(3)})
}

func TestEncodeReport_MixedLevels_AreListedInOrderWithRecordCounts(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(effortRun(effortMixed, effortMixed, effortMixed))))

	want := [][2]any{{"high", float64(2)}, {"medium", float64(1)}}
	assertEffortList(t, "orchestrator", objectAt(t, run, "orchestrator"), want...)
	assertEffortList(t, "dispatch", effortDispatch(t, run), want...)
}

func TestEncodeReport_AbsentEffort_IsNullOnEveryInvocation(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(effortRun(nil, nil, nil))))

	assertEffortNull(t, "orchestrator", objectAt(t, run, "orchestrator"))
	assertEffortNull(t, "agents[0]", vwObj(t, vwList(t, run, "agents")[0]))
	assertEffortNull(t, "dispatch", effortDispatch(t, run))
}

func TestEncodeReport_UnattributableAgent_CarriesEffort(t *testing.T) {
	r := vwReport(effortRun(nil, nil, nil))
	un := effortRun(nil, effortMixed, nil)
	r.Unattributable = &un

	report := decodeReportJSON(t, r)

	agent := vwObj(t, vwList(t, objectAt(t, report, "unattributable"), "agents")[0])
	assertEffortList(t, "unattributable.agents[0]", agent, [2]any{"high", float64(2)}, [2]any{"medium", float64(1)})
}

func encodeEffortTotal(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, app.RunTotal{RunID: cliTestRunID}); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}
	return buf.Bytes()
}

func TestEncodeRunTotal_HasNoEffortKey(t *testing.T) {
	m := decodeObject(t, encodeEffortTotal(t))

	if _, has := m["effort"]; has {
		t.Errorf("total JSON carries an effort key: %v", m)
	}
}
