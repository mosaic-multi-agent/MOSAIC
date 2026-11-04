package cli_test

// Tests for the assumed-tier disclosure in the report JSON encoder
// (EncodeReport) and its absence from the total JSON (EncodeRunTotal).
//
// Rules under test:
//   - jsonTotals gains assumed_tier_cache_write_tokens, emitted only when > 0,
//     on actor, run, all_runs and unattributable totals.
//   - Totals without assumed-tier tokens carry no such key.
//   - The total encoding for a run with assumed-tier tokens has no such key.

import (
	"bytes"
	"encoding/json"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const assumedKey = "assumed_tier_cache_write_tokens"

func decodeReportJSON(t *testing.T, report domain.Report) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeReport(&buf, report); err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	return m
}

func objectAt(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	o, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("key %q is not an object in %v", key, m)
	}
	return o
}

func firstRun(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	runs, _ := m["runs"].([]any)
	if len(runs) == 0 {
		t.Fatalf("no runs in report JSON: %v", m)
	}
	run, _ := runs[0].(map[string]any)
	return run
}

func assertAssumedCount(t *testing.T, where string, totals map[string]any, want float64) {
	t.Helper()
	got, ok := totals[assumedKey].(float64)
	if !ok || got != want {
		t.Errorf("%s.%s = %v, want %v", where, assumedKey, totals[assumedKey], want)
	}
}

func assertNoAssumedKey(t *testing.T, where string, totals map[string]any) {
	t.Helper()
	if v, present := totals[assumedKey]; present {
		t.Errorf("%s carries %s = %v although no tokens were priced at an assumed tier", where, assumedKey, v)
	}
}

func TestEncodeReport_AssumedTierTokens_AppearOnRunActorAndAllRunsTotals(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Totals:       domain.Totals{AssumedTierCacheWrite: 300},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: domain.Totals{AssumedTierCacheWrite: 100}},
		Agents: []domain.ActorReport{{
			Actor:  domain.AgentInstance("Implementer#3"),
			Totals: domain.Totals{AssumedTierCacheWrite: 200},
		}},
	}
	report := domain.Report{
		Currency: domain.Currency,
		AllRuns:  domain.Totals{AssumedTierCacheWrite: 300},
		Runs:     []domain.RunReport{run},
	}

	m := decodeReportJSON(t, report)

	assertAssumedCount(t, "all_runs", m["all_runs"].(map[string]any), 300)
	jr := firstRun(t, m)
	assertAssumedCount(t, "runs[0].totals", objectAt(t, jr, "totals"), 300)
	assertAssumedCount(t, "runs[0].orchestrator.totals", objectAt(t, objectAt(t, jr, "orchestrator"), "totals"), 100)
	agents, _ := jr["agents"].([]any)
	agent, _ := agents[0].(map[string]any)
	assertAssumedCount(t, "runs[0].agents[0].totals", objectAt(t, agent, "totals"), 200)
}

func TestEncodeReport_UnattributableAssumedTierTokens_AppearOnUnattributableTotals(t *testing.T) {
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 77},
	}
	report := domain.Report{Currency: domain.Currency, Unattributable: &unattributable}

	m := decodeReportJSON(t, report)

	assertAssumedCount(t, "unattributable.totals", objectAt(t, objectAt(t, m, "unattributable"), "totals"), 77)
}

func TestEncodeReport_NoAssumedTierTokens_OmitsTheKeyEverywhere(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		Agents:       []domain.ActorReport{{Actor: domain.AgentInstance("Implementer#3")}},
	}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}}

	m := decodeReportJSON(t, report)

	assertNoAssumedKey(t, "all_runs", objectAt(t, m, "all_runs"))
	jr := firstRun(t, m)
	assertNoAssumedKey(t, "runs[0].totals", objectAt(t, jr, "totals"))
	assertNoAssumedKey(t, "runs[0].orchestrator.totals", objectAt(t, objectAt(t, jr, "orchestrator"), "totals"))
}

func TestEncodeRunTotal_AssumedTierRun_HasNoAssumedTierKeyAnywhere(t *testing.T) {
	total := totalWithAssumedTier(t, true)

	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}

	if bytes.Contains(buf.Bytes(), []byte("assumed")) {
		t.Errorf("total JSON mentions an assumed tier; output:\n%s", buf.String())
	}
}
