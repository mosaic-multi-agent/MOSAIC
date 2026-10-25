package digest_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// docWithFinding returns a valid doc carrying one finding object.
func docWithFinding(finding map[string]any) map[string]any {
	doc := validDoc("1.0.0")
	doc["findings"] = []any{finding}
	return doc
}

// missingRequiredDocs are same-major documents where a value that has no absent
// form in the domain is omitted.
func missingRequiredDocs() map[string]any {
	return map[string]any{
		"invocation without state": withRun(map[string]any{"run_id": runA, "invocations": []any{
			map[string]any{"instance_id": "a#1", "agent_type": "a"}}}),
		"orchestrator tool call without pairing": withRun(map[string]any{"run_id": runA, "orchestrator": map[string]any{
			"tool_calls": []any{map[string]any{"call_id": "c1", "tool_name": "Read"}}}}),
		"invocation tool call without pairing": withRun(map[string]any{"run_id": runA, "invocations": []any{
			map[string]any{"instance_id": "a#1", "state": "closed",
				"tool_calls": []any{map[string]any{"call_id": "c1"}}}}}),
		"finding without severity": docWithFinding(map[string]any{"kind": "k", "scope": "general"}),
		"finding without scope":    docWithFinding(map[string]any{"kind": "k", "severity": "info"}),
		"finding run scope without run_id": docWithFinding(map[string]any{
			"kind": "k", "severity": "info", "scope": "run"}),
		"turn actor without kind": withRun(map[string]any{"run_id": runA, "turns": []any{
			map[string]any{"actor": map[string]any{"instance_id": "a#1"}}}}),
		"notification actor without kind": withRun(map[string]any{"run_id": runA, "notifications": []any{
			map[string]any{"actor": map[string]any{}}}}),
		"compaction actor without kind": withRun(map[string]any{"run_id": runA, "compactions": []any{
			map[string]any{"actor": map[string]any{"agent_type": "a"}}}}),
		"finding actor without kind": docWithFinding(map[string]any{
			"kind": "k", "severity": "info", "scope": "general", "actor": map[string]any{}}),
	}
}

func TestRead_MissingValueWithNoAbsentFormIsInvalidNotDefaulted(t *testing.T) {
	for name, doc := range missingRequiredDocs() {
		t.Run(name, func(t *testing.T) {
			path := writeRawDoc(t, doc)

			got, findings := readDigest(t, path)

			assertOneInvalidFinding(t, path, got, findings)
		})
	}
}

func TestRead_AbsentActorKeyReadsAsOrchestrator(t *testing.T) {
	doc := withRun(map[string]any{
		"run_id":        runA,
		"turns":         []any{map[string]any{"role": "assistant"}},
		"notifications": []any{map[string]any{"notification_type": "idle"}},
		"compactions":   []any{map[string]any{"trigger": "auto"}},
	})
	doc["findings"] = []any{map[string]any{"kind": "k", "severity": "info", "scope": "general"}}
	path := writeRawDoc(t, doc)

	got, findings := readDigest(t, path)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	run := got.Aggregate.Runs[0]
	requireAtLeast(t, "turns", len(run.Turns), 1)
	requireAtLeast(t, "notifications", len(run.Notifications), 1)
	requireAtLeast(t, "compactions", len(run.Activity.Compactions), 1)
	requireAtLeast(t, "findings", len(got.Findings), 1)
	want := domain.Orchestrator()
	requireSame(t, "turn actor", want, run.Turns[0].Actor)
	requireSame(t, "notification actor", want, run.Notifications[0].Actor)
	requireSame(t, "compaction actor", want, run.Activity.Compactions[0].Actor)
	requireSame(t, "finding actor", want, got.Findings[0].Actor)
}

func TestWrite_FindingWithZeroActorIsWrittenAsOrchestratorKind(t *testing.T) {
	d := domain.Digest{Findings: []domain.Finding{{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning,
	}}}

	obj := readJSONObject(t, writeDigest(t, d))

	findings, _ := obj["findings"].([]any)
	requireAtLeast(t, "findings", len(findings), 1)
	f, _ := findings[0].(map[string]any)
	actor, ok := f["actor"].(map[string]any)
	if !ok {
		t.Fatalf("finding has no actor object: %v", f)
	}
	if actor["kind"] != "orchestrator" || len(actor) != 1 {
		t.Errorf("actor = %v, want only {\"kind\": \"orchestrator\"}", actor)
	}
}
