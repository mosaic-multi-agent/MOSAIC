package digest_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// futureMinorDoc is a same-major document with unknown keys at every level and
// many facts omitted.
func futureMinorDoc() map[string]any {
	doc := validDoc("1.7.3")
	doc["future_top_key"] = map[string]any{"a": 1}
	doc["runs"] = []any{map[string]any{
		"run_id":         runA,
		"future_run_key": true,
		"orchestrator": map[string]any{
			"future_key": 1,
			"usage": []any{map[string]any{
				"record_id": "r1", "model": "m", "future_key": "x",
				"tokens": map[string]any{"input_tokens": 5, "future_tokens": 9},
			}},
			"tool_calls": []any{map[string]any{"call_id": "c1", "pairing": "paired", "future_key": 1}},
		},
		"sessions":      []any{map[string]any{"session_id": "s1", "future_key": 1}},
		"invocations":   []any{map[string]any{"instance_id": "a#1", "agent_type": "a", "state": "closed", "future_key": 1}},
		"turns":         []any{map[string]any{"actor": map[string]any{"kind": "orchestrator", "future_key": 1}}},
		"agents":        []any{map[string]any{"instance_id": "a#1", "agent_type": "a", "future_key": 1, "usage": []any{map[string]any{"record_id": "r2", "future_key": 1}}}},
		"notifications": []any{map[string]any{"actor": map[string]any{"kind": "orchestrator"}, "notification_type": "idle", "future_key": 1}},
		"compactions":   []any{map[string]any{"actor": map[string]any{"kind": "orchestrator"}, "trigger": "auto", "future_key": 1}},
	}}
	doc["unattributable"] = map[string]any{"future_key": 1, "outcome": "failure"}
	doc["findings"] = []any{map[string]any{
		"kind": "malformed_line", "severity": "warning", "scope": "general", "future_key": 1,
	}, map[string]any{"kind": "future_kind", "severity": "info", "scope": "general"}}
	return doc
}

func TestRead_SameMajorWithUnknownKeysAndOmittedFactsLoadsWithFactsAbsent(t *testing.T) {
	path := writeRawDoc(t, futureMinorDoc())

	got, findings := readDigest(t, path)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	if got.Header.Version != "1.7.3" {
		t.Errorf("Header.Version = %q, want the version as read", got.Header.Version)
	}
	if len(got.Aggregate.Runs) != 1 || got.Aggregate.Runs[0].Run.ID != runA {
		t.Fatalf("runs = %+v", got.Aggregate.Runs)
	}
	run := got.Aggregate.Runs[0]
	if run.Provisional || run.Facts.Outcome != "" || !run.Facts.StartedAt.IsZero() || len(run.Facts.Harnesses) != 0 {
		t.Errorf("omitted run facts did not read as absent: %+v", run.Facts)
	}
	requireAtLeast(t, "orchestrator usage", len(run.Orchestrator.Invocations), 1)
	requireAtLeast(t, "sessions", len(run.Sessions), 1)
	requireAtLeast(t, "orchestrator tool calls", len(run.Activity.OrchestratorToolCalls), 1)
	u := run.Orchestrator.Invocations[0]
	if v, ok := u.Usage.Input.Value(); !ok || v != 5 {
		t.Errorf("input = %v present=%v, want 5", v, ok)
	}
	if u.Usage.Output.IsPresent() || u.Usage.CacheRead.IsPresent() || u.Usage.Reasoning.IsPresent() {
		t.Errorf("omitted token fields read as present: %+v", u.Usage)
	}
	if u.Provider != "" || u.ServiceTier != "" || !u.At.IsZero() {
		t.Errorf("omitted usage facts did not read as absent: %+v", u)
	}
	if run.Sessions[0].ID != "s1" || run.Sessions[0].Resumed != domain.BoolAbsent || !run.Sessions[0].StartedAt.IsZero() {
		t.Errorf("session = %+v", run.Sessions[0])
	}
	if call := run.Activity.OrchestratorToolCalls[0]; call.CallID != "c1" || call.Status != "" || !call.StartedAt.IsZero() {
		t.Errorf("tool call = %+v", call)
	}
	if len(got.Findings) != 2 || got.Findings[0].Kind != domain.FindingMalformedLine || got.Findings[1].Kind != "future_kind" {
		t.Errorf("carried findings = %+v", got.Findings)
	}
	if len(run.Agents) != 1 || len(run.Notifications) != 1 || len(run.Activity.Compactions) != 1 {
		t.Errorf("entries with unknown keys were lost: agents %d, notifications %d, compactions %d",
			len(run.Agents), len(run.Notifications), len(run.Activity.Compactions))
	}
	if got.Aggregate.Unattributable == nil || got.Aggregate.Unattributable.Facts.Outcome != "failure" {
		t.Errorf("bucket with an unknown key = %+v", got.Aggregate.Unattributable)
	}
}

func TestRead_UsageWithoutTokensIsAMissingUsageMarker(t *testing.T) {
	doc := validDoc("1.0.0")
	doc["runs"] = []any{map[string]any{
		"run_id":       runA,
		"orchestrator": map[string]any{"usage": []any{map[string]any{"record_id": "r1"}, map[string]any{"record_id": "r2", "tokens": map[string]any{}}}},
	}}

	got, _ := readDigest(t, writeRawDoc(t, doc))

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	usage := got.Aggregate.Runs[0].Orchestrator.Invocations
	if len(usage) != 2 {
		t.Fatalf("usage entries = %d, want 2", len(usage))
	}
	for i, u := range usage {
		if !u.Usage.IsEmpty() || u.Usage.Reasoning.IsPresent() || u.Usage.CacheCreation5m.IsPresent() {
			t.Errorf("entry %d has token counts: %+v", i, u.Usage)
		}
	}
}

func TestRead_SameMajorHigherMinorNumbersCompareNumerically(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.10.0", "1.0.12", "1.99.99"} {
		t.Run(version, func(t *testing.T) {
			doc := validDoc(version)
			doc["runs"] = []any{map[string]any{"run_id": runA}}

			got, findings := readDigest(t, writeRawDoc(t, doc))

			if len(findings) != 0 || len(got.Aggregate.Runs) != 1 {
				t.Fatalf("runs = %d, findings = %+v", len(got.Aggregate.Runs), findings)
			}
		})
	}
}

func TestRead_DifferentMajorYieldsOneFindingAndNoRuns(t *testing.T) {
	for _, version := range []string{"2.0.0", "0.9.0", "10.0.0", "3.1.4"} {
		t.Run(version, func(t *testing.T) {
			doc := validDoc(version)
			// Shapes this tool cannot know: the file must not be decoded further.
			doc["runs"] = "not-a-list-in-this-version"
			doc["findings"] = 12
			path := writeRawDoc(t, doc)

			got, findings := readDigest(t, path)

			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", findings)
			}
			f := findings[0]
			if f.Kind != domain.FindingUnsupportedDigestVersion || f.Severity != domain.SeverityWarning || f.Path != path {
				t.Errorf("finding = %+v", f)
			}
			if !got.Aggregate.IsEmpty() || len(got.Findings) != 0 {
				t.Errorf("a different major contributed data: %+v", got)
			}
		})
	}
}

func TestRead_DifferentMajorWithValidLookingRunsStillYieldsNoRuns(t *testing.T) {
	doc := validDoc("2.0.0")
	doc["runs"] = []any{map[string]any{"run_id": runA}}

	got, findings := readDigest(t, writeRawDoc(t, doc))

	if len(findings) != 1 || findings[0].Kind != domain.FindingUnsupportedDigestVersion {
		t.Fatalf("findings = %+v", findings)
	}
	if !got.Aggregate.IsEmpty() {
		t.Errorf("runs loaded from an unsupported version: %+v", got.Aggregate.Runs)
	}
}
