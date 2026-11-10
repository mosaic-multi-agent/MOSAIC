package logread_test

// Tests for decoding usage_record.requested_tools (schema 1.7.0).
//
// Rules under test:
//   - An object of non-empty names with integer counts >= 1 is a provided value.
//   - {} is provided and empty; a missing key is not provided.
//   - null and any invalid value or entry read as not provided, with no finding.
//   - Logs without the field, and records stamped 1.7.0, load without findings.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

func requestedToolsOf(t *testing.T, extra string) (domain.RequestedTools, []domain.Finding) {
	t.Helper()
	if extra != "" {
		extra = "," + extra
	}
	ev, findings := readOne(t, rmLine("usage_record", "2026-10-01T10:00:01.000Z",
		`"record_id":"r1","model":"m","token_usage":{"input_tokens":5}`+extra))
	if ev.UsageRecord == nil {
		t.Fatal("UsageRecord block is nil")
	}
	return ev.UsageRecord.RequestedTools, findings
}

func TestReader_UsageRecord_RequestedToolsWithCounts_IsProvided(t *testing.T) {
	got, findings := requestedToolsOf(t, `"requested_tools":{"read_file":2,"glob":1}`)

	if !got.IsProvided() || got.Count("read_file") != 2 || got.Count("glob") != 1 || got.Total() != 3 {
		t.Errorf("requested tools = %v, want read_file 2 and glob 1", got.Counts())
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_UsageRecord_EmptyRequestedTools_IsProvidedAndEmpty(t *testing.T) {
	got, findings := requestedToolsOf(t, `"requested_tools":{}`)

	if !got.IsProvided() || got.Total() != 0 {
		t.Errorf("provided %v total %d, want provided and empty", got.IsProvided(), got.Total())
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_UsageRecord_WithoutRequestedTools_IsNotProvided(t *testing.T) {
	got, findings := requestedToolsOf(t, "")

	if got.IsProvided() {
		t.Error("a record without the key reads as provided")
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_UsageRecord_InvalidRequestedTools_ReadsAsNotProvidedWithoutFinding(t *testing.T) {
	cases := map[string]string{
		"null":           `"requested_tools":null`,
		"array":          `"requested_tools":["glob"]`,
		"string":         `"requested_tools":"glob"`,
		"zero count":     `"requested_tools":{"glob":0}`,
		"negative count": `"requested_tools":{"glob":-1}`,
		"string count":   `"requested_tools":{"glob":"2"}`,
		"fractional":     `"requested_tools":{"glob":1.5}`,
		"empty name":     `"requested_tools":{"":1}`,
		"one bad entry":  `"requested_tools":{"glob":1,"edit":0}`,
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			got, findings := requestedToolsOf(t, extra)

			if got.IsProvided() {
				t.Errorf("invalid value read as provided: %v", got.Counts())
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}

func TestReader_Fixture_DecodesEveryRequestedToolsCase(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "requestedtools", "usage.jsonl")

	events, findings, err := logread.NewReader().ReadAll(context.Background(), path)

	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none for 1.7.0 and older records", findings)
	}
	provided := map[string]bool{}
	for _, ev := range events {
		provided[ev.UsageRecord.RecordID] = ev.UsageRecord.RequestedTools.IsProvided()
	}
	want := map[string]bool{"r-present": true, "r-empty": true, "r-absent": false,
		"r-null": false, "r-zero": false, "r-text": false, "r-array": false, "r-old": false}
	for id, w := range want {
		if got, ok := provided[id]; !ok || got != w {
			t.Errorf("%s provided = %v (seen %v), want %v", id, got, ok, w)
		}
	}
}

func TestReader_Record170_LoadsWithoutFindings(t *testing.T) {
	_, findings := readTierLines(t, v130Line("1.7.0", "usage_record",
		`,"record_id":"r1","model":"m","token_usage":{"input_tokens":5},"requested_tools":{"glob":1}`))

	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}
