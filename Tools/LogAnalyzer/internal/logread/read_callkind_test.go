package logread_test

// Tests for decoding the call kind, call label and tool name fields.
//
// Rules under test:
//   - usage_record.call_kind decodes to the matching kind; an absent or
//     unknown value reads as a main-loop call.
//   - usage_record.call_label is kept verbatim ("" when absent).
//   - tool_call_end.tool_name is decoded; absent reads as "".
//   - A log written without the fields loads without a finding.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func callKindUsageLine(extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return rmLine("usage_record", "2026-10-01T10:00:01.000Z",
		`"record_id":"r1","record_index":0,"model":"m","token_usage":{"input_tokens":5}`+extra)
}

func TestReader_UsageRecord_DecodesCallKindAndLabel(t *testing.T) {
	cases := []struct {
		name string
		kind string
		want domain.CallKind
	}{
		{"main", "main", domain.CallMain},
		{"compaction", "compaction", domain.CallCompaction},
		{"auxiliary", "auxiliary", domain.CallAuxiliary},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, findings := readOne(t, callKindUsageLine(`"call_kind":"`+c.kind+`","call_label":"summarizeConversationHistory-x"`))

			if ev.UsageRecord == nil {
				t.Fatal("UsageRecord block is nil")
			}
			if ev.UsageRecord.CallKind != c.want {
				t.Errorf("CallKind = %q, want %q", ev.UsageRecord.CallKind, c.want)
			}
			if ev.UsageRecord.CallLabel != "summarizeConversationHistory-x" {
				t.Errorf("CallLabel = %q, want it verbatim", ev.UsageRecord.CallLabel)
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}

func TestReader_UsageRecord_AbsentOrUnknownCallKind_ReadsAsMain(t *testing.T) {
	cases := map[string]string{
		"absent":        "",
		"unknown value": `"call_kind":"background"`,
		"empty value":   `"call_kind":""`,
		"wrong case":    `"call_kind":"Compaction"`,
		"null":          `"call_kind":null`,
		"label no kind": `"call_label":"title"`,
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			ev, _ := readOne(t, callKindUsageLine(extra))

			if ev.UsageRecord == nil {
				t.Fatal("UsageRecord block is nil")
			}
			if ev.UsageRecord.CallKind != domain.CallMain {
				t.Errorf("CallKind = %q, want %q", ev.UsageRecord.CallKind, domain.CallMain)
			}
		})
	}
}

func TestReader_UsageRecord_AbsentCallLabel_IsEmpty(t *testing.T) {
	ev, findings := readOne(t, callKindUsageLine(`"call_kind":"auxiliary"`))

	if ev.UsageRecord.CallLabel != "" {
		t.Errorf("CallLabel = %q, want empty", ev.UsageRecord.CallLabel)
	}
	if ev.UsageRecord.CallKind != domain.CallAuxiliary {
		t.Errorf("CallKind = %q, want auxiliary", ev.UsageRecord.CallKind)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_ToolCallEnd_DecodesToolName(t *testing.T) {
	ev, findings := readOne(t, rmLine("tool_call_end", "2026-10-01T10:00:02.000Z",
		`"call_id":"c1","status":"error","tool_name":"runSubagent"`))

	if ev.ToolCallEnd == nil {
		t.Fatal("ToolCallEnd block is nil")
	}
	if ev.ToolCallEnd.ToolName != "runSubagent" {
		t.Errorf("ToolName = %q, want runSubagent", ev.ToolCallEnd.ToolName)
	}
	if ev.ToolCallEnd.CallID != "c1" || ev.ToolCallEnd.Status != domain.ToolCallError {
		t.Errorf("block = %+v, want CallID c1 and status error", *ev.ToolCallEnd)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_ToolCallEnd_AbsentToolName_IsEmptyWithoutFinding(t *testing.T) {
	ev, findings := readOne(t, rmLine("tool_call_end", "2026-10-01T10:00:02.000Z",
		`"call_id":"c1","status":"success"`))

	if ev.ToolCallEnd == nil {
		t.Fatal("ToolCallEnd block is nil")
	}
	if ev.ToolCallEnd.ToolName != "" {
		t.Errorf("ToolName = %q, want empty", ev.ToolCallEnd.ToolName)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}
