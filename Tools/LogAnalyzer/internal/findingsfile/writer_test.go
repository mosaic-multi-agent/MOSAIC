package findingsfile_test

// The findings export document: one entry per finding, in input order, with all
// seven fields always present (detail empty rather than missing for findings that
// carry none, as digest-carried findings do) and per-kind counts equal to the
// overlay's.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestWriteFindings_OneEntryPerFindingInInputOrder(t *testing.T) {
	in := mixedFindings()

	doc := readDoc(t, writeDoc(t, in))

	if len(doc.Findings) != len(in) {
		t.Fatalf("%d entries, want %d", len(doc.Findings), len(in))
	}
	for i, f := range in {
		if got := doc.Findings[i]["kind"]; got != string(f.Kind) {
			t.Errorf("entry %d kind = %v, want %q (input order)", i, got, f.Kind)
		}
	}
}

func TestWriteFindings_EveryEntryCarriesAllSevenKeys(t *testing.T) {
	doc := readDoc(t, writeDoc(t, mixedFindings()))

	for i, e := range doc.Findings {
		if len(e) != len(sevenKeys) {
			t.Errorf("entry %d has %d keys, want exactly %d: %v", i, len(e), len(sevenKeys), e)
		}
		for _, k := range sevenKeys {
			if _, ok := e[k]; !ok {
				t.Errorf("entry %d lacks the key %q: %v", i, k, e)
			}
		}
	}
}

func TestWriteFindings_FieldValuesOfAFullyPopulatedFinding(t *testing.T) {
	full := domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Run: domain.NamedRun(runA),
		Actor: domain.AgentInstance("coder#8"), Path: "logs/a/events.jsonl", Line: 42, Detail: "bad json",
	}

	doc := readDoc(t, writeDoc(t, []domain.Finding{full}))

	want := map[string]any{
		"kind": "malformed_line", "severity": "warning", "path": "logs/a/events.jsonl",
		"line": float64(42), "run_id": runA, "actor": "coder#8", "detail": "bad json",
	}
	for k, v := range want {
		if got := doc.Findings[0][k]; got != v {
			t.Errorf("%s = %v, want %v", k, got, v)
		}
	}
}

func TestWriteFindings_DetailIsAnEmptyStringNotMissingWhenTheFindingHasNone(t *testing.T) {
	digestCarried := finding(domain.FindingCrossStreamRecord, domain.SeverityWarning, domain.NamedRun(runA))

	doc := readDoc(t, writeDoc(t, []domain.Finding{digestCarried}))

	detail, ok := doc.Findings[0]["detail"]
	if !ok {
		t.Fatalf("detail is missing; it must be written as an empty string: %v", doc.Findings[0])
	}
	if detail != "" {
		t.Errorf("detail = %v, want an empty string", detail)
	}
	for _, k := range []string{"path", "line"} {
		if _, ok := doc.Findings[0][k]; !ok {
			t.Errorf("%s is missing; empty values are written too: %v", k, doc.Findings[0])
		}
	}
	if doc.Findings[0]["line"] != float64(0) {
		t.Errorf("line = %v, want 0", doc.Findings[0]["line"])
	}
}

func TestWriteFindings_SeverityUsesTheWireLabels(t *testing.T) {
	a := domain.NamedRun(runA)
	in := []domain.Finding{
		finding(domain.FindingMissingModel, domain.SeverityInfo, a),
		finding(domain.FindingMalformedLine, domain.SeverityWarning, a),
	}

	doc := readDoc(t, writeDoc(t, in))

	if got := []any{doc.Findings[0]["severity"], doc.Findings[1]["severity"]}; got[0] != "info" || got[1] != "warning" {
		t.Errorf("severities = %v, want [info warning]", got)
	}
}

func TestWriteFindings_RunAndActorLabels(t *testing.T) {
	w := domain.SeverityWarning
	in := []domain.Finding{
		finding(domain.FindingCrossStreamRecord, w, domain.NamedRun(runA)),
		finding(domain.FindingMalformedLine, w, domain.UnattributableRun()),
		finding(domain.FindingInvalidSettingsEntry, w, domain.RunRef{}),
	}

	doc := readDoc(t, writeDoc(t, in))

	wantRun := []string{runA, domain.UnattributableRun().Label(), ""}
	for i, want := range wantRun {
		if got := doc.Findings[i]["run_id"]; got != want {
			t.Errorf("entry %d run_id = %v, want %q", i, got, want)
		}
		if got := doc.Findings[i]["actor"]; got != "orchestrator" {
			t.Errorf("entry %d actor = %v, want orchestrator for a finding with no actor", i, got)
		}
	}
}

func TestWriteFindings_CountsEqualThePerKindCountsOfTheOverlay(t *testing.T) {
	in := mixedFindings()

	doc := readDoc(t, writeDoc(t, in))

	want := map[string]int{}
	for _, row := range domain.SummarizeFindingKinds(in) {
		want[string(row.Kind)] = row.Count
	}
	if len(want) != 4 || want["malformed_line"] != 2 || want["cross_stream_record"] != 2 {
		t.Fatalf("setup: fixture counts = %v", want)
	}
	if len(doc.Counts) != len(want) {
		t.Errorf("counts = %v, want %v", doc.Counts, want)
	}
	for k, n := range want {
		if doc.Counts[k] != n {
			t.Errorf("counts[%s] = %d, want %d", k, doc.Counts[k], n)
		}
	}
}

func TestWriteFindings_RunScopeWritesOnlyItsFindingsAndTheirCounts(t *testing.T) {
	b := domain.NamedRun(runB)
	var scoped []domain.Finding
	for _, f := range mixedFindings() {
		if f.Run == b {
			scoped = append(scoped, f)
		}
	}

	doc := readDoc(t, writeDoc(t, scoped))

	if len(doc.Findings) != 2 {
		t.Fatalf("%d entries, want the 2 of run b", len(doc.Findings))
	}
	if len(doc.Counts) != 2 || doc.Counts["cross_stream_record"] != 1 || doc.Counts["missing_model"] != 1 {
		t.Errorf("counts = %v, want cross_stream_record 1 and missing_model 1 only", doc.Counts)
	}
	for i, e := range doc.Findings {
		if e["run_id"] != runB {
			t.Errorf("entry %d run_id = %v, want %q", i, e["run_id"], runB)
		}
	}
}

func TestWriteFindings_NoFindingsWritesEmptyCountsAndFindings(t *testing.T) {
	for name, in := range map[string][]domain.Finding{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			doc := readDoc(t, writeDoc(t, in))

			if got := strings.TrimSpace(string(doc.raw["findings"])); got != "[]" {
				t.Errorf("findings = %s, want []", got)
			}
			if got := strings.TrimSpace(string(doc.raw["counts"])); got != "{}" {
				t.Errorf("counts = %s, want {}", got)
			}
		})
	}
}

func TestWriteFindings_DocumentIsOneObjectWithOnlyCountsAndFindings(t *testing.T) {
	doc := readDoc(t, writeDoc(t, mixedFindings()))

	if len(doc.raw) != 2 {
		t.Errorf("top-level keys = %d, want counts and findings only", len(doc.raw))
	}
}

func TestWriteFindings_IsTwoSpaceIndentedUTF8(t *testing.T) {
	doc := readDoc(t, writeDoc(t, []domain.Finding{
		{Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Detail: "café"},
	}))

	if !strings.Contains(doc.text, "\n  \"counts\"") || !strings.Contains(doc.text, "\n  \"findings\"") {
		t.Errorf("top-level keys must be indented by two spaces:\n%s", doc.text)
	}
	if !strings.Contains(doc.text, "café") {
		t.Errorf("non-ASCII text must be written as UTF-8, not escaped:\n%s", doc.text)
	}
}
