package domain_test

// The per-kind grouping behind the findings overlay: one entry per kind with its
// highest severity, count and the number of named runs it occurs in, sorted
// warning before info, then count descending, then kind ascending.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func kindFinding(kind domain.FindingKind, sev domain.FindingSeverity, run domain.RunRef) domain.Finding {
	return domain.Finding{Kind: kind, Severity: sev, Run: run}
}

func kindsOf(rows []domain.FindingKindSummary) []domain.FindingKind {
	var out []domain.FindingKind
	for _, r := range rows {
		out = append(out, r.Kind)
	}
	return out
}

func TestFindingSeverityLabel_IsTheWireValue(t *testing.T) {
	if got := domain.SeverityInfo.Label(); got != "info" {
		t.Errorf("SeverityInfo.Label() = %q, want info", got)
	}
	if got := domain.SeverityWarning.Label(); got != "warning" {
		t.Errorf("SeverityWarning.Label() = %q, want warning", got)
	}
}

func TestSummarizeFindingKinds_EmptyInputYieldsNil(t *testing.T) {
	if got := domain.SummarizeFindingKinds(nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := domain.SummarizeFindingKinds([]domain.Finding{}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestSummarizeFindingKinds_GroupsByKindWithCountAndSeverity(t *testing.T) {
	r1 := domain.NamedRun("20260101T000000Z-aaaa")
	findings := []domain.Finding{
		kindFinding(domain.FindingMalformedLine, domain.SeverityWarning, r1),
		kindFinding(domain.FindingMalformedLine, domain.SeverityWarning, r1),
		kindFinding(domain.FindingMalformedLine, domain.SeverityWarning, r1),
		kindFinding(domain.FindingMissingModel, domain.SeverityInfo, r1),
	}

	got := domain.SummarizeFindingKinds(findings)

	want := []domain.FindingKindSummary{
		{Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Count: 3, Runs: 1},
		{Kind: domain.FindingMissingModel, Severity: domain.SeverityInfo, Count: 1, Runs: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSummarizeFindingKinds_SortsWarningBeforeInfoThenCountDescendingThenKindAscending(t *testing.T) {
	r := domain.NamedRun("20260101T000000Z-aaaa")
	var findings []domain.Finding
	add := func(kind domain.FindingKind, sev domain.FindingSeverity, n int) {
		for i := 0; i < n; i++ {
			findings = append(findings, kindFinding(kind, sev, r))
		}
	}
	add(domain.FindingMissingModel, domain.SeverityInfo, 9)      // info, many: still after every warning
	add(domain.FindingUnidentifiedEvent, domain.SeverityInfo, 2) // info
	add(domain.FindingTruncatedLine, domain.SeverityWarning, 1)  // warning, few
	add(domain.FindingMalformedLine, domain.SeverityWarning, 4)  // warning, most
	add(domain.FindingCrossStreamRecord, domain.SeverityWarning, 2)
	add(domain.FindingCompactionMismatch, domain.SeverityWarning, 2) // ties with cross_stream_record: kind ascending

	got := kindsOf(domain.SummarizeFindingKinds(findings))

	want := []domain.FindingKind{
		domain.FindingMalformedLine, domain.FindingCompactionMismatch, domain.FindingCrossStreamRecord,
		domain.FindingTruncatedLine, domain.FindingMissingModel, domain.FindingUnidentifiedEvent,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSummarizeFindingKinds_SeverityOfAKindIsTheHighestAmongItsFindings(t *testing.T) {
	r := domain.NamedRun("20260101T000000Z-aaaa")
	findings := []domain.Finding{
		kindFinding(domain.FindingMissingModel, domain.SeverityInfo, r),
		kindFinding(domain.FindingMissingModel, domain.SeverityWarning, r),
		kindFinding(domain.FindingMissingModel, domain.SeverityInfo, r),
	}

	got := domain.SummarizeFindingKinds(findings)

	if len(got) != 1 || got[0].Severity != domain.SeverityWarning || got[0].Count != 3 {
		t.Errorf("got %+v, want one warning row with count 3", got)
	}
}

func TestSummarizeFindingKinds_RunsCountsDistinctNamedRunsOnly(t *testing.T) {
	a, b := domain.NamedRun("20260101T000000Z-aaaa"), domain.NamedRun("20260102T000000Z-bbbb")
	findings := []domain.Finding{
		kindFinding(domain.FindingCrossStreamRecord, domain.SeverityWarning, a),
		kindFinding(domain.FindingCrossStreamRecord, domain.SeverityWarning, a), // same run again
		kindFinding(domain.FindingCrossStreamRecord, domain.SeverityWarning, b),
		kindFinding(domain.FindingCrossStreamRecord, domain.SeverityWarning, domain.UnattributableRun()),
		kindFinding(domain.FindingCrossStreamRecord, domain.SeverityWarning, domain.RunRef{}), // tied to no run
	}

	got := domain.SummarizeFindingKinds(findings)

	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0].Count != 5 {
		t.Errorf("Count = %d, want 5 (every finding counts)", got[0].Count)
	}
	if got[0].Runs != 2 {
		t.Errorf("Runs = %d, want 2 (named runs only, distinct)", got[0].Runs)
	}
}

func TestSummarizeFindingKinds_KindWithOnlyRunlessFindingsOccursInNoRuns(t *testing.T) {
	findings := []domain.Finding{
		kindFinding(domain.FindingInvalidPricingEntry, domain.SeverityWarning, domain.RunRef{}),
		kindFinding(domain.FindingInvalidPricingEntry, domain.SeverityWarning, domain.UnattributableRun()),
	}

	got := domain.SummarizeFindingKinds(findings)

	if len(got) != 1 || got[0].Count != 2 || got[0].Runs != 0 {
		t.Errorf("got %+v, want one row with count 2 and 0 runs", got)
	}
}

func TestSummarizeFindingKinds_DoesNotReorderItsInput(t *testing.T) {
	r := domain.NamedRun("20260101T000000Z-aaaa")
	findings := []domain.Finding{
		kindFinding(domain.FindingMissingModel, domain.SeverityInfo, r),
		kindFinding(domain.FindingMalformedLine, domain.SeverityWarning, r),
	}
	before := append([]domain.Finding(nil), findings...)

	if rows := domain.SummarizeFindingKinds(findings); len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	if !reflect.DeepEqual(findings, before) {
		t.Errorf("input was modified: %+v", findings)
	}
}
