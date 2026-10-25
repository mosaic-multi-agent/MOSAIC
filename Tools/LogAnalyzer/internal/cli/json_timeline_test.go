package cli_test

// Tests for the timeline section of the report JSON (EncodeReport) and for the
// total JSON staying unchanged when a run carries a timeline.
//
// Rules under test:
//   - Every run object (named and unattributable) carries a timeline with
//     entries and compactions; both are arrays, never null.
//   - Times are RFC3339 strings or null; durations are integer milliseconds or
//     null; the state is "closed" or "open"; an open invocation has no end and
//     no duration; an empty agent type or status code is kept as "".
//   - Compactions carry actor label, time, trigger and token counts, with null
//     for an absent time or token count.
//   - Overlaps are the sorted instance ids, as an array.
//   - The total JSON keeps its exact field set.

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// tlRun returns a run with a timeline holding a closed entry that overlaps a
// second one, an open entry, an entry with an unusable start, and compactions
// from the orchestrator and from an agent.
func tlRun() domain.RunReport {
	closed := func(id, typ, status string, start, end time.Time) domain.Invocation {
		return domain.Invocation{
			Actor:      domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: typ},
			State:      domain.InvocationClosed,
			StatusCode: status,
			StartedAt:  start,
			EndedAt:    end,
			EndEvents:  1,
		}
	}
	run := vwRun()
	run.Timeline = domain.Timeline{
		Entries: []domain.TimelineEntry{
			{
				Invocation:  closed("coder#1", "coder", "SUCCESS", vwStart, vwStart.Add(90*time.Second)),
				Duration:    90 * time.Second,
				HasDuration: true,
				Overlaps:    []domain.AgentInstanceID{"coder#2", "coder#3"},
			},
			{
				Invocation:  closed("coder#2", "coder", "", vwStart.Add(30*time.Second), vwStart.Add(45*time.Second)),
				Duration:    15 * time.Second,
				HasDuration: true,
				Overlaps:    []domain.AgentInstanceID{"coder#1"},
			},
			{
				Invocation: domain.Invocation{
					Actor:     domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "x#2"},
					State:     domain.InvocationOpen,
					StartedAt: vwStart.Add(10 * time.Second),
				},
			},
			{
				Invocation: closed("z#9", "", "SUCCESS", time.Time{}, vwStart.Add(120*time.Second)),
			},
		},
		Compactions: []domain.Compaction{
			{Actor: domain.Orchestrator(), At: vwStart.Add(20 * time.Second), Trigger: "auto",
				TokensBefore: domain.Tokens(100000), TokensAfter: domain.Tokens(20000)},
			{Actor: domain.AgentInstance("coder#1"), Trigger: "manual"},
		},
	}
	return run
}

func tlTimelineJSON(t *testing.T, run domain.RunReport) map[string]any {
	t.Helper()
	return objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "timeline")
}

func tlEntryJSON(t *testing.T, tl map[string]any, i int) map[string]any {
	t.Helper()
	entries := vwList(t, tl, "entries")
	if len(entries) <= i {
		t.Fatalf("timeline.entries has %d items, want at least %d", len(entries), i+1)
	}
	return vwObj(t, entries[i])
}

func TestEncodeReport_RunCarriesTheTimeline(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(tlRun())))

	tl, ok := run["timeline"].(map[string]any)

	if !ok {
		t.Fatalf("run object has no timeline object; keys = %v", sortedKeys(run))
	}
	assertKeySet(t, "timeline", sortedKeys(tl), []string{"entries", "compactions"})
}

func TestEncodeReport_Timeline_ClosedEntryHasTimesDurationAndOverlaps(t *testing.T) {
	e := tlEntryJSON(t, tlTimelineJSON(t, tlRun()), 0)

	if e["actor"] != "coder#1" || e["agent_type"] != "coder" || e["state"] != "closed" || e["status_code"] != "SUCCESS" {
		t.Errorf("entry = %v, want closed coder#1 of type coder with status SUCCESS", e)
	}
	if e["duration_ms"] != float64(90_000) {
		t.Errorf("duration_ms = %v, want 90000", e["duration_ms"])
	}
	started, _ := e["started_at"].(string)
	ended, _ := e["ended_at"].(string)
	if got, err := time.Parse(time.RFC3339Nano, started); err != nil || !got.Equal(vwStart) {
		t.Errorf("started_at = %v, want %s as RFC3339", e["started_at"], vwStart.Format(time.RFC3339Nano))
	}
	if got, err := time.Parse(time.RFC3339Nano, ended); err != nil || !got.Equal(vwStart.Add(90*time.Second)) {
		t.Errorf("ended_at = %v, want %s as RFC3339", e["ended_at"], vwStart.Add(90*time.Second).Format(time.RFC3339Nano))
	}
	overlaps := vwList(t, e, "overlaps")
	if len(overlaps) != 2 || overlaps[0] != "coder#2" || overlaps[1] != "coder#3" {
		t.Errorf("overlaps = %v, want [coder#2 coder#3] in order", overlaps)
	}
}

func TestEncodeReport_Timeline_EntryKeysAreExactlyTheContractedSet(t *testing.T) {
	e := tlEntryJSON(t, tlTimelineJSON(t, tlRun()), 0)

	assertKeySet(t, "timeline entry", sortedKeys(e), []string{
		"actor", "agent_type", "state", "status_code", "started_at", "ended_at", "duration_ms", "overlaps",
	})
}

func TestEncodeReport_Timeline_OpenEntryHasNoEndNoDurationAndEmptyOverlaps(t *testing.T) {
	e := tlEntryJSON(t, tlTimelineJSON(t, tlRun()), 2)

	if e["state"] != "open" || e["actor"] != "x#2" {
		t.Fatalf("entry = %v, want the open x#2", e)
	}
	for _, key := range []string{"ended_at", "duration_ms"} {
		if v, present := e[key]; !present || v != nil {
			t.Errorf("open entry %s = %v (present %v), want an explicit null", key, v, present)
		}
	}
	if v, present := e["started_at"]; !present || v == nil {
		t.Errorf("open entry started_at = %v, want the logged start", v)
	}
	if overlaps := vwList(t, e, "overlaps"); len(overlaps) != 0 {
		t.Errorf("open entry overlaps = %v, want an empty array", overlaps)
	}
	if v, present := e["agent_type"]; !present || v != "" {
		t.Errorf("open entry agent_type = %v (present %v), want an empty string kept", v, present)
	}
	if v, present := e["status_code"]; !present || v != "" {
		t.Errorf("open entry status_code = %v (present %v), want an empty string kept", v, present)
	}
}

func TestEncodeReport_Timeline_EntryWithoutUsableStartHasNullStartAndDuration(t *testing.T) {
	e := tlEntryJSON(t, tlTimelineJSON(t, tlRun()), 3)

	if e["actor"] != "z#9" || e["state"] != "closed" {
		t.Fatalf("entry = %v, want closed z#9", e)
	}
	for _, key := range []string{"started_at", "duration_ms"} {
		if v, present := e[key]; !present || v != nil {
			t.Errorf("z#9 %s = %v (present %v), want an explicit null", key, v, present)
		}
	}
	if v, ok := e["ended_at"].(string); !ok || v == "" {
		t.Errorf("z#9 ended_at = %v, want the logged end time", e["ended_at"])
	}
}

func TestEncodeReport_Timeline_CompactionsCarryActorTimeTriggerAndTokens(t *testing.T) {
	tl := tlTimelineJSON(t, tlRun())

	compactions := vwList(t, tl, "compactions")

	if len(compactions) != 2 {
		t.Fatalf("compactions has %d items, want 2", len(compactions))
	}
	first := vwObj(t, compactions[0])
	assertKeySet(t, "compaction", sortedKeys(first), []string{"actor", "at", "trigger", "tokens_before", "tokens_after"})
	if first["actor"] != "orchestrator" || first["trigger"] != "auto" ||
		first["tokens_before"] != float64(100000) || first["tokens_after"] != float64(20000) {
		t.Errorf("first compaction = %v, want orchestrator auto 100000 -> 20000", first)
	}
	at, _ := first["at"].(string)
	if got, err := time.Parse(time.RFC3339Nano, at); err != nil || !got.Equal(vwStart.Add(20*time.Second)) {
		t.Errorf("compaction at = %v, want %s as RFC3339", first["at"], vwStart.Add(20*time.Second).Format(time.RFC3339Nano))
	}
}

func TestEncodeReport_Timeline_CompactionWithAbsentTimeAndTokensEncodesNulls(t *testing.T) {
	second := vwObj(t, vwList(t, tlTimelineJSON(t, tlRun()), "compactions")[1])

	if second["actor"] != "coder#1" || second["trigger"] != "manual" {
		t.Fatalf("compaction = %v, want coder#1 manual", second)
	}
	for _, key := range []string{"at", "tokens_before", "tokens_after"} {
		if v, present := second[key]; !present || v != nil {
			t.Errorf("compaction %s = %v (present %v), want an explicit null", key, v, present)
		}
	}
}

func TestEncodeReport_RunWithoutTimeline_EncodesEmptyArraysNotNull(t *testing.T) {
	tl := tlTimelineJSON(t, domain.RunReport{Run: domain.NamedRun(cliTestRunID)})

	if got := vwList(t, tl, "entries"); len(got) != 0 {
		t.Errorf("timeline.entries = %v, want an empty array", got)
	}
	if got := vwList(t, tl, "compactions"); len(got) != 0 {
		t.Errorf("timeline.compactions = %v, want an empty array", got)
	}
}

func TestEncodeReport_UnattributableRunCarriesTheTimelineToo(t *testing.T) {
	unattributable := tlRun()
	unattributable.Run = domain.UnattributableRun()
	report := vwReport()
	report.Unattributable = &unattributable

	run := objectAt(t, decodeReportJSON(t, report), "unattributable")

	tl := objectAt(t, run, "timeline")
	if len(vwList(t, tl, "entries")) != 4 {
		t.Errorf("unattributable timeline entries = %v, want the 4 entries", tl["entries"])
	}
}

func TestEncodeRunTotal_RunWithTimeline_KeepsExactFieldSetAndBytes(t *testing.T) {
	plainRun := vwRun()
	plainRun.Totals = vwTotals(500, true)
	withTimeline := tlRun()
	withTimeline.Totals = vwTotals(500, true)

	encode := func(run domain.RunReport) []byte {
		total, ok := app.RunTotalFor(vwReport(run), cliTestRunID)
		if !ok {
			t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
		}
		var buf bytes.Buffer
		if err := cli.EncodeRunTotal(&buf, total); err != nil {
			t.Fatalf("EncodeRunTotal: %v", err)
		}
		return buf.Bytes()
	}

	got, want := encode(withTimeline), encode(plainRun)

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	assertKeySet(t, "top level", sortedKeys(m), []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
	})
	if !bytes.Equal(got, want) {
		t.Errorf("total output changed when the run carried a timeline:\n with %s\n without %s", got, want)
	}
}
