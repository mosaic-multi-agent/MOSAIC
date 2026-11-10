package logread_test

// Tests for the major-version schema rule.
//
// Rules under test:
//   - Every event whose schema_version is MAJOR.MINOR.PATCH of major 1 is read
//     with no finding, whatever the minor or patch; unknown fields and unknown
//     event types are ignored.
//   - A schema_version of another major version, or one that is not
//     MAJOR.MINOR.PATCH, raises exactly one unknown_schema_version finding per
//     file, on the first such line; the events are still read.
//   - compaction.phase decodes to pre / post; absent or any other value is none.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

func TestReader_MajorOneVersions_RaiseNoFinding(t *testing.T) {
	for _, version := range []string{"1.3.7", "1.4.0", "1.5.0", "1.9.0", "1.10.25"} {
		t.Run(version, func(t *testing.T) {
			events, findings := readTierLines(t,
				v130Line(version, "run_start", `,"model":"m"`),
				v130Line(version, "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)),
			)

			if len(events) != 2 {
				t.Fatalf("events = %d, want 2", len(events))
			}
			if len(findings) != 0 {
				t.Errorf("findings = %v, want none for schema %s", findings, version)
			}
		})
	}
}

func TestReader_MajorOneNewerVersion_IgnoresUnknownFieldsAndEventTypes(t *testing.T) {
	events, findings := readTierLines(t,
		v130Line("1.9.0", "run_start", `,"model":"m","field_from_the_future":{"a":1}`),
		v130Line("1.9.0", "event_type_from_the_future", `,"x":1`),
	)

	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	if len(events) != 2 || events[1].Type != domain.EventOther || events[1].RawType != "event_type_from_the_future" {
		t.Errorf("events = %+v, want run_start then the unknown type as EventOther", events)
	}
}

func TestReader_UnsupportedVersions_OneFindingPerFileAndEventsStillRead(t *testing.T) {
	for _, version := range []string{"2.0.0", "0.9.0", "abc", "1.4", "", "v1.4.0", "1.4.0-rc1", "1.4.0.1"} {
		t.Run("version="+version, func(t *testing.T) {
			line := v130Line(version, "run_start", `,"model":"m"`)

			events, findings := readTierLines(t, line, line, line)

			if len(events) != 3 {
				t.Errorf("events = %d, want 3 (still read best-effort)", len(events))
			}
			if n := unknownSchemaFindings(findings); n != 1 {
				t.Errorf("unknown_schema_version findings = %d, want exactly 1 per file", n)
			}
		})
	}
}

func TestReader_SupportedFirstLineThenUnsupportedLines_OneFindingAndAllEventsRead(t *testing.T) {
	events, findings := readTierLines(t,
		v130Line("1.4.0", "run_start", ""),
		v130Line("2.0.0", "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)),
		v130Line("abc", "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)),
	)

	if n := unknownSchemaFindings(findings); n != 1 {
		t.Errorf("unknown_schema_version findings = %d, want 1 (the 1.4.0 line raises none, the later ones raise one per file)", n)
	}
	if len(events) != 3 {
		t.Errorf("events = %d, want 3 (every line still read)", len(events))
	}
}

func TestReader_UnsupportedVersionInEachFile_OneFindingPerFile(t *testing.T) {
	for i := 0; i < 2; i++ {
		_, findings := readTierLines(t, v130Line("2.0.0", "run_start", ""), v130Line("2.0.0", "run_start", ""))
		if n := unknownSchemaFindings(findings); n != 1 {
			t.Errorf("file %d: unknown_schema_version findings = %d, want 1", i, n)
		}
	}
}

func TestReader_Fixture_Schema140FromCopilotRaisesNoSchemaFinding(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "pendingusage", "runs", "20261031T110000Z-c001", "00_orchestrator_events.jsonl")

	events, findings, err := logread.NewReader().ReadAll(context.Background(), path)

	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("fixture yielded no events")
	}
	if n := unknownSchemaFindings(findings); n != 0 {
		t.Errorf("unknown_schema_version findings = %d, want 0 for a 1.4.0 fixture", n)
	}
}

func TestReader_CompactionPhase_DecodesPreAndPost(t *testing.T) {
	for _, phase := range []domain.CompactionPhase{domain.CompactionPhasePre, domain.CompactionPhasePost} {
		t.Run(string(phase), func(t *testing.T) {
			events, findings := readTierLines(t, v130Line("1.5.0", "compaction", `,"phase":"`+string(phase)+`","trigger":"auto"`))

			if len(findings) != 0 || len(events) != 1 || events[0].Compaction == nil {
				t.Fatalf("events = %+v, findings = %v; want one compaction and no findings", events, findings)
			}
			if events[0].Compaction.Phase != phase {
				t.Errorf("Phase = %q, want %q", events[0].Compaction.Phase, phase)
			}
		})
	}
}

func TestReader_CompactionPhase_AbsentOrUnknownValueIsNone(t *testing.T) {
	cases := map[string]string{
		"absent":        `,"trigger":"auto"`,
		"unknown value": `,"phase":"during","trigger":"auto"`,
		"empty":         `,"phase":"","trigger":"auto"`,
		"wrong case":    `,"phase":"PRE","trigger":"auto"`,
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			events, findings := readTierLines(t, v130Line("1.5.0", "compaction", extra))

			if len(findings) != 0 || len(events) != 1 || events[0].Compaction == nil {
				t.Fatalf("events = %+v, findings = %v; want one compaction and no findings", events, findings)
			}
			if events[0].Compaction.Phase != domain.CompactionPhaseNone {
				t.Errorf("Phase = %q, want none", events[0].Compaction.Phase)
			}
			if events[0].Compaction.Trigger != "auto" {
				t.Errorf("Trigger = %q, want auto: the other fields still decode", events[0].Compaction.Trigger)
			}
		})
	}
}

func TestReader_CompactionPhase_DoesNotDisturbOtherCompactionFields(t *testing.T) {
	events, _ := readTierLines(t, v130Line("1.5.0", "compaction", `,"phase":"post","tokens_after":42000`))

	if len(events) != 1 || events[0].Compaction == nil {
		t.Fatalf("events = %+v, want one compaction", events)
	}
	c := events[0].Compaction
	requireTokens(t, "TokensAfter", c.TokensAfter, 42000)
	requireAbsentTokens(t, "TokensBefore", c.TokensBefore)
	if c.Phase != domain.CompactionPhasePost {
		t.Errorf("Phase = %q, want post", c.Phase)
	}
}
