package logread_test

// Tests for decoding the events the run model needs: tool_call_start,
// tool_call_end and compaction field blocks, plus the timestamps and
// status_code of invocation events.
//
// Rules under test:
//   - tool_call_start / tool_call_end / compaction decode to their own event
//     types with exactly one field block set.
//   - Absent fields decode as empty / absent, without a finding.
//   - An unknown status or trigger value is kept verbatim.
//   - Payload bodies (tool_input, tool_output, error) are not decoded.
//   - notification and unknown event types keep decoding as EventOther.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const rmRunID = "20261001T100000Z-ab01"

// rmLine builds one JSONL event line with the given event type, timestamp and
// extra JSON members (without a leading comma).
func rmLine(event, timestamp, extra string) string {
	ts := ""
	if timestamp != "" {
		ts = `"timestamp":"` + timestamp + `",`
	}
	if extra != "" {
		extra = "," + extra
	}
	return `{"schema_version":"1.2.0","event":"` + event + `",` + ts +
		`"harness":"claude-code","session_id":"s1","run_id":"` + rmRunID + `"` + extra + `}`
}

func readOne(t *testing.T, line string) (domain.Event, []domain.Finding) {
	t.Helper()
	events, findings := readTierLines(t, line)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	return events[0], findings
}

func TestReader_ToolCallStart_DecodesCallIDToolNameAndTimestamp(t *testing.T) {
	ev, findings := readOne(t, rmLine("tool_call_start", "2026-10-01T10:00:01.250Z",
		`"call_id":"toolu_01","tool_name":"Read","tool_input":{"file_path":"/x"}`))

	if ev.Type != domain.EventToolCallStart {
		t.Fatalf("Type = %q, want %q", ev.Type, domain.EventToolCallStart)
	}
	if ev.ToolCallStart == nil {
		t.Fatal("ToolCallStart block is nil")
	}
	if ev.ToolCallStart.CallID != "toolu_01" || ev.ToolCallStart.ToolName != "Read" {
		t.Errorf("block = %+v, want CallID toolu_01 and ToolName Read", *ev.ToolCallStart)
	}
	want := time.Date(2026, 10, 1, 10, 0, 1, 250_000_000, time.UTC)
	if !ev.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", ev.Timestamp, want)
	}
	if ev.ToolCallEnd != nil || ev.Compaction != nil {
		t.Error("only the ToolCallStart block may be set")
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_ToolCallStart_AbsentFieldsDecodeEmptyWithoutFinding(t *testing.T) {
	ev, findings := readOne(t, rmLine("tool_call_start", "2026-10-01T10:00:01.000Z", ""))

	if ev.ToolCallStart == nil {
		t.Fatal("ToolCallStart block is nil; a recognised type always carries its block")
	}
	if ev.ToolCallStart.CallID != "" || ev.ToolCallStart.ToolName != "" {
		t.Errorf("block = %+v, want empty call id and tool name", *ev.ToolCallStart)
	}
	if len(findings) != 0 {
		t.Errorf("absent optional fields must not produce findings: %v", findings)
	}
}

func TestReader_ToolCallEnd_DecodesStatus(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		want  domain.ToolCallStatus
	}{
		{"success", `"call_id":"c1","status":"success"`, domain.ToolCallSuccess},
		{"error", `"call_id":"c1","status":"error","error":"boom"`, domain.ToolCallError},
		{"absent status", `"call_id":"c1"`, ""},
		{"unknown status kept verbatim", `"call_id":"c1","status":"cancelled"`, domain.ToolCallStatus("cancelled")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, findings := readOne(t, rmLine("tool_call_end", "2026-10-01T10:00:02.000Z", tc.extra))

			if ev.Type != domain.EventToolCallEnd || ev.ToolCallEnd == nil {
				t.Fatalf("Type = %q, block nil = %v; want tool_call_end with block", ev.Type, ev.ToolCallEnd == nil)
			}
			if ev.ToolCallEnd.CallID != "c1" {
				t.Errorf("CallID = %q, want c1", ev.ToolCallEnd.CallID)
			}
			if ev.ToolCallEnd.Status != tc.want {
				t.Errorf("Status = %q, want %q", ev.ToolCallEnd.Status, tc.want)
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}

func TestReader_ToolCallEnd_AbsentCallIDDecodesEmptyWithoutFinding(t *testing.T) {
	ev, findings := readOne(t, rmLine("tool_call_end", "2026-10-01T10:00:02.000Z", `"status":"success"`))

	if ev.Type != domain.EventToolCallEnd || ev.ToolCallEnd == nil {
		t.Fatalf("Type = %q, block nil = %v; want tool_call_end with block", ev.Type, ev.ToolCallEnd == nil)
	}
	if ev.ToolCallEnd.CallID != "" || ev.ToolCallEnd.Status != domain.ToolCallSuccess {
		t.Errorf("block = %+v, want empty CallID and success status", *ev.ToolCallEnd)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_Compaction_DecodesTriggerAndTokenCounts(t *testing.T) {
	ev, findings := readOne(t, rmLine("compaction", "2026-10-01T10:05:00.000Z",
		`"trigger":"auto","tokens_before":180000,"tokens_after":42000`))

	if ev.Type != domain.EventCompaction || ev.Compaction == nil {
		t.Fatalf("Type = %q, block nil = %v; want compaction with block", ev.Type, ev.Compaction == nil)
	}
	if ev.Compaction.Trigger != "auto" {
		t.Errorf("Trigger = %q, want auto", ev.Compaction.Trigger)
	}
	requireTokens(t, "TokensBefore", ev.Compaction.TokensBefore, 180000)
	requireTokens(t, "TokensAfter", ev.Compaction.TokensAfter, 42000)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_Compaction_AbsentFieldsStayAbsent(t *testing.T) {
	ev, findings := readOne(t, rmLine("compaction", "2026-10-01T10:05:00.000Z", ""))

	if ev.Compaction == nil {
		t.Fatal("Compaction block is nil; a recognised type always carries its block")
	}
	if ev.Compaction.Trigger != "" {
		t.Errorf("Trigger = %q, want empty", ev.Compaction.Trigger)
	}
	requireAbsentTokens(t, "TokensBefore", ev.Compaction.TokensBefore)
	requireAbsentTokens(t, "TokensAfter", ev.Compaction.TokensAfter)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_Compaction_ZeroTokensIsPresentNotAbsent(t *testing.T) {
	ev, _ := readOne(t, rmLine("compaction", "2026-10-01T10:05:00.000Z",
		`"trigger":"manual","tokens_before":0,"tokens_after":0`))

	if ev.Compaction == nil {
		t.Fatal("Compaction block is nil")
	}
	if ev.Compaction.Trigger != "manual" {
		t.Errorf("Trigger = %q, want manual", ev.Compaction.Trigger)
	}
	requireTokens(t, "TokensBefore", ev.Compaction.TokensBefore, 0)
	requireTokens(t, "TokensAfter", ev.Compaction.TokensAfter, 0)
}

func TestReader_Compaction_UnknownTriggerKeptVerbatim(t *testing.T) {
	ev, _ := readOne(t, rmLine("compaction", "2026-10-01T10:05:00.000Z", `"trigger":"idle"`))

	if ev.Compaction == nil || ev.Compaction.Trigger != "idle" {
		t.Errorf("Compaction = %+v, want trigger idle kept verbatim", ev.Compaction)
	}
}

func TestReader_ToolAndCompactionEvents_MissingOrBadTimestampDecodesAsZeroTime(t *testing.T) {
	lines := []string{
		rmLine("tool_call_start", "", `"call_id":"c1","tool_name":"Read"`),
		rmLine("tool_call_end", "not-a-time", `"call_id":"c1","status":"success"`),
		rmLine("compaction", "", `"trigger":"auto"`),
	}

	events, _ := readTierLines(t, lines...)

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (bad timestamps must not drop events)", len(events))
	}
	for i, ev := range events {
		if !ev.Timestamp.IsZero() {
			t.Errorf("event %d Timestamp = %v, want zero time", i, ev.Timestamp)
		}
	}
	if events[0].ToolCallStart == nil || events[1].ToolCallEnd == nil || events[2].Compaction == nil {
		t.Error("field blocks must still decode when the timestamp is unusable")
	}
}

func TestReader_ToolCallEvents_AtPriorSchemaVersionsRaiseNoFinding(t *testing.T) {
	line := strings.Replace(rmLine("tool_call_start", "2026-10-01T10:00:01.000Z",
		`"call_id":"c1","tool_name":"Bash"`), `"1.2.0"`, `"1.1.0"`, 1)

	ev, findings := readOne(t, line)

	if ev.ToolCallStart == nil {
		t.Fatal("ToolCallStart block is nil at schema 1.1.0")
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_NotificationAndUnknownTypes_StillDecodeAsOther(t *testing.T) {
	events, findings := readTierLines(t,
		rmLine("notification", "2026-10-01T10:00:01.000Z", `"notification_type":"idle"`),
		rmLine("future_event_kind", "2026-10-01T10:00:02.000Z", `"x":1`))

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	for i, ev := range events {
		if ev.Type != domain.EventOther {
			t.Errorf("event %d Type = %q, want EventOther", i, ev.Type)
		}
		if ev.ToolCallStart != nil || ev.ToolCallEnd != nil || ev.Compaction != nil {
			t.Errorf("event %d must carry no run-model block", i)
		}
	}
	if events[0].RawType != "notification" || events[1].RawType != "future_event_kind" {
		t.Errorf("RawType = %q, %q; want the wire values preserved", events[0].RawType, events[1].RawType)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_InvocationEvents_DecodeTimestampsStatusAndType(t *testing.T) {
	events, findings := readTierLines(t,
		rmLine("invocation_start", "2026-10-01T10:00:01.000Z", `"agent_instance_id":"coder#1","agent_type":"coder"`),
		rmLine("invocation_end", "2026-10-01T10:01:31.000Z", `"agent_instance_id":"coder#1","status_code":"BLOCKED","model":"m"`),
		rmLine("invocation_end", "2026-10-01T10:02:00.000Z", `"agent_instance_id":"coder#2"`))

	if len(events) != 3 || len(findings) != 0 {
		t.Fatalf("events = %d, findings = %v; want 3 events and no findings", len(events), findings)
	}
	if events[0].InvocationStart == nil || events[0].InvocationStart.AgentType != "coder" {
		t.Errorf("invocation_start block = %+v, want agent type coder", events[0].InvocationStart)
	}
	if got := events[1].Timestamp.Sub(events[0].Timestamp); got != 90*time.Second {
		t.Errorf("timestamp gap = %v, want 90s", got)
	}
	if events[1].InvocationEnd == nil || events[1].InvocationEnd.StatusCode != "BLOCKED" {
		t.Errorf("invocation_end block = %+v, want status BLOCKED", events[1].InvocationEnd)
	}
	if events[2].InvocationEnd == nil || events[2].InvocationEnd.StatusCode != "" {
		t.Errorf("invocation_end without status_code = %+v, want empty status", events[2].InvocationEnd)
	}
}
