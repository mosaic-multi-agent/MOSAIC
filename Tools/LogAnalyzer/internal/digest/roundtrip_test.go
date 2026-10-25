package digest_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

func TestRoundTrip_FullAggregateReturnsEqualAggregate(t *testing.T) {
	want := fullDigest()

	got, findings := roundTrip(t, want)

	if len(findings) != 0 {
		t.Fatalf("a valid digest yielded findings: %+v", findings)
	}
	requireSame(t, "aggregate", want.Aggregate, got.Aggregate)
}

func TestRoundTrip_FindingsKeepKindSeverityRunAndActorOnly(t *testing.T) {
	want := fullDigest()

	got, _ := roundTrip(t, want)

	if len(got.Findings) != len(want.Findings) {
		t.Fatalf("got %d findings, want %d", len(got.Findings), len(want.Findings))
	}
	for i, f := range want.Findings {
		requireSame(t, "finding", stripFinding(f), got.Findings[i])
	}
}

func TestRoundTrip_UnattributableBucketSurvivesWithoutNamedRuns(t *testing.T) {
	want := domain.Digest{Aggregate: domain.Aggregate{Unattributable: bucketRun()}}

	got, _ := roundTrip(t, want)

	requireSame(t, "aggregate", want.Aggregate, got.Aggregate)
}

func TestRoundTrip_AbsentBucketStaysNil(t *testing.T) {
	want := domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{richRun()}}}

	got, _ := roundTrip(t, want)

	if got.Aggregate.Unattributable != nil {
		t.Fatal("decoded a bucket that was never written")
	}
}

func TestRoundTrip_PresentZeroAndAbsentStayDistinct(t *testing.T) {
	run := domain.RunAggregate{
		Run: domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: []domain.InvocationUsage{
			{Model: "m", RecordID: "r0", Usage: domain.TokenUsage{Input: domain.Tokens(0), Output: domain.AbsentTokens()}},
			{Model: "m", RecordID: "r1", Usage: domain.TokenUsage{Input: domain.AbsentTokens(), Output: domain.Tokens(0)}},
		}},
		Sessions: []domain.RunSession{
			{ID: "s-false", Resumed: domain.BoolFalse},
			{ID: "s-absent", Resumed: domain.BoolAbsent},
		},
	}

	got, _ := roundTrip(t, domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}}})

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	gotRun := got.Aggregate.Runs[0]
	requireAtLeast(t, "orchestrator usage", len(gotRun.Orchestrator.Invocations), 2)
	requireAtLeast(t, "sessions", len(gotRun.Sessions), 2)
	u0, u1 := gotRun.Orchestrator.Invocations[0].Usage, gotRun.Orchestrator.Invocations[1].Usage
	if v, ok := u0.Input.Value(); !ok || v != 0 {
		t.Errorf("present-zero input became %v present=%v", v, ok)
	}
	if u0.Output.IsPresent() {
		t.Error("absent output became present")
	}
	if u1.Input.IsPresent() {
		t.Error("absent input became present")
	}
	if v, ok := u1.Output.Value(); !ok || v != 0 {
		t.Errorf("present-zero output became %v present=%v", v, ok)
	}
	if gotRun.Sessions[0].Resumed != domain.BoolFalse {
		t.Errorf("resumed=false read back as %v", gotRun.Sessions[0].Resumed)
	}
	if gotRun.Sessions[1].Resumed != domain.BoolAbsent {
		t.Errorf("absent resumed read back as %v", gotRun.Sessions[1].Resumed)
	}
}

func TestEncode_WritesPresentZeroAndOmitsAbsentTokenKeys(t *testing.T) {
	run := domain.RunAggregate{
		Run: domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: []domain.InvocationUsage{
			{Model: "m", RecordID: "r0", Usage: domain.TokenUsage{Input: domain.Tokens(0)}},
		}},
	}
	path := writeDigest(t, domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}}})

	doc := readJSONObject(t, path)

	runs, _ := doc["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("runs = %v", doc["runs"])
	}
	orch, _ := runs[0].(map[string]any)["orchestrator"].(map[string]any)
	usage, _ := orch["usage"].([]any)
	if len(usage) != 1 {
		t.Fatalf("orchestrator usage = %v", orch["usage"])
	}
	tokens, _ := usage[0].(map[string]any)["tokens"].(map[string]any)
	if v, ok := tokens["input_tokens"]; !ok || v != float64(0) {
		t.Errorf("present zero input must be written as 0, tokens = %v", tokens)
	}
	for _, k := range []string{"output_tokens", "cache_read_tokens", "cache_creation_tokens", "reasoning_tokens"} {
		if _, ok := tokens[k]; ok {
			t.Errorf("absent %s was written", k)
		}
	}
}

func TestRoundTrip_PreservesModelOrder(t *testing.T) {
	want := fullDigest()

	got, _ := roundTrip(t, want)

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 2)
	run := got.Aggregate.Runs[0]
	requireAtLeast(t, "orchestrator tool calls", len(run.Activity.OrchestratorToolCalls), 3)
	requireAtLeast(t, "compactions", len(run.Activity.Compactions), 2)
	requireAtLeast(t, "agents", len(run.Agents), 2)
	requireAtLeast(t, "orchestrator usage", len(run.Orchestrator.Invocations), len(want.Aggregate.Runs[0].Orchestrator.Invocations))
	if ids := []string{run.Activity.OrchestratorToolCalls[0].CallID, run.Activity.OrchestratorToolCalls[1].CallID,
		run.Activity.OrchestratorToolCalls[2].CallID}; ids[0] != "z-call" || ids[1] != "a-call" || ids[2] != "m-call" {
		t.Errorf("tool call order = %v, want recorded order z, a, m", ids)
	}
	if run.Activity.Compactions[0].Trigger != "auto" || run.Activity.Compactions[1].Trigger != "manual" {
		t.Errorf("compactions were reordered: %+v", run.Activity.Compactions)
	}
	if got.Aggregate.Runs[0].Run.ID != runA || got.Aggregate.Runs[1].Run.ID != runB {
		t.Errorf("run order = %s, %s", got.Aggregate.Runs[0].Run.ID, got.Aggregate.Runs[1].Run.ID)
	}
	if run.Agents[0].Actor.Instance != "coder#1" || run.Agents[1].Actor.Instance != "planner#1" {
		t.Errorf("agent order = %v, %v", run.Agents[0].Actor.Instance, run.Agents[1].Actor.Instance)
	}
	for i, u := range run.Orchestrator.Invocations {
		if u.RecordID != want.Aggregate.Runs[0].Orchestrator.Invocations[i].RecordID {
			t.Errorf("usage order differs at %d: %q", i, u.RecordID)
		}
	}
}

func TestRoundTrip_InvocationWithoutAgentEntryGetsActorFromTypeAndNoUsage(t *testing.T) {
	want := fullDigest()

	got, _ := roundTrip(t, want)

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	requireAtLeast(t, "invocations", len(got.Aggregate.Runs[0].Activity.Invocations), 2)
	inv := got.Aggregate.Runs[0].Activity.Invocations[1]
	wantActor := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "reviewer#2", Type: "reviewer"}
	requireSame(t, "actor", wantActor, inv.Actor)
	if inv.Usage != nil {
		t.Errorf("usage = %+v, want nil", inv.Usage)
	}
}

func TestRoundTrip_HeaderCarriesAdapterVersionAndGivenValues(t *testing.T) {
	want := domain.Digest{
		Header:    domain.DigestHeader{Version: "9.9.9", ToolVersion: "4.5.6", WrittenAt: at(77)},
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{sparseRun()}},
	}

	got, _ := roundTrip(t, want)

	if got.Header.Version != digest.Version {
		t.Errorf("Version = %q, want the adapter's own %q", got.Header.Version, digest.Version)
	}
	if got.Header.ToolVersion != "4.5.6" {
		t.Errorf("ToolVersion = %q", got.Header.ToolVersion)
	}
	if !got.Header.WrittenAt.Equal(at(77)) {
		t.Errorf("WrittenAt = %v", got.Header.WrittenAt)
	}
}

func TestRoundTrip_TimestampOffsetIsPreserved(t *testing.T) {
	plus2 := time.FixedZone("", 2*3600)
	run := sparseRun()
	run.Facts.StartedAt = time.Date(2026, 10, 23, 21, 0, 0, 5, plus2)

	got, findings := roundTrip(t, domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}}})

	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	gotStart := got.Aggregate.Runs[0].Facts.StartedAt
	if _, off := gotStart.Zone(); off != 2*3600 {
		t.Errorf("zone offset = %d, want 7200", off)
	}
	if !gotStart.Equal(run.Facts.StartedAt) {
		t.Errorf("instant changed: %v", gotStart)
	}
}

func TestWrite_SameDigestProducesSameBytes(t *testing.T) {
	d := fullDigest()
	// Many harnesses and schema versions give an unordered encoder many chances
	// to reorder its output between writes.
	run := d.Aggregate.Runs[0]
	run.Facts.Harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI, domain.HarnessOpenCode, domain.HarnessVSCodeGHCP}
	run.Facts.SchemaVersions = []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0", "3.0.0", "3.1.0", "4.0.0"}
	d.Aggregate.Runs = append([]domain.RunAggregate{run}, d.Aggregate.Runs[1:]...)

	first, err := os.ReadFile(writeDigest(t, d))
	if err != nil || len(first) == 0 {
		t.Fatalf("read back: %v (%d bytes)", err, len(first))
	}
	for i := 0; i < 8; i++ {
		again, err := os.ReadFile(writeDigest(t, d))
		if err != nil || !bytes.Equal(first, again) {
			t.Fatalf("write %d differs from the first or is unreadable (%v)", i+2, err)
		}
	}
}
