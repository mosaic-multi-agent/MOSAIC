package screens_test

// The findings of one run: the same set its header badge counts, found by the
// finding's Run, in QualitySummary order.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestRunFindingsOf_SelectsExactlyTheFindingsWhoseRunEqualsTheRun(t *testing.T) {
	a, b := domain.NamedRun("20260101T000000Z-aaaa"), domain.NamedRun("20260102T000000Z-bbbb")
	q := domain.NewQualitySummary(findingsFor(a, b))

	got := screens.RunFindingsOf(q, a)

	if len(got) != 5 {
		t.Fatalf("got %d findings, want the 5 of run a: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Run != a {
			t.Errorf("finding %+v belongs to another run", f)
		}
	}
}

func TestRunFindingsOf_KeepsTheOrderOfTheQualitySummary(t *testing.T) {
	a, b := domain.NamedRun("20260101T000000Z-aaaa"), domain.NamedRun("20260102T000000Z-bbbb")
	q := domain.NewQualitySummary(findingsFor(a, b))
	var want []domain.Finding
	for _, f := range q.Findings {
		if f.Run == a {
			want = append(want, f)
		}
	}

	got := screens.RunFindingsOf(q, a)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRunFindingsOf_CountEqualsTheHeaderBadgeCount(t *testing.T) {
	a, b := domain.NamedRun("20260101T000000Z-aaaa"), domain.NamedRun("20260102T000000Z-bbbb")
	q := domain.NewQualitySummary(findingsFor(a, b))

	for _, run := range []domain.RunRef{a, b, domain.NamedRun("20260103T000000Z-cccc")} {
		if got, want := len(screens.RunFindingsOf(q, run)), screens.RunQualityOf(q, run).Findings; got != want {
			t.Errorf("run %s: %d findings in the overlay scope, %d in the header badge", run.Label(), got, want)
		}
	}
}

func TestRunFindingsOf_LeavesOutTheUnattributableAndRunlessFindings(t *testing.T) {
	a := domain.NamedRun("20260101T000000Z-aaaa")
	q := domain.NewQualitySummary([]domain.Finding{
		findingOf(domain.FindingMalformedLine, domain.SeverityWarning, domain.UnattributableRun()),
		findingOf(domain.FindingInvalidPricingEntry, domain.SeverityWarning, domain.RunRef{}),
		findingOf(domain.FindingMissingModel, domain.SeverityInfo, a),
	})

	got := screens.RunFindingsOf(q, a)

	if len(got) != 1 || got[0].Kind != domain.FindingMissingModel {
		t.Errorf("got %+v, want only the missing_model finding of the run", got)
	}
}

func TestRunFindingsOf_NilWhenTheRunHasNoFindings(t *testing.T) {
	a := domain.NamedRun("20260101T000000Z-aaaa")
	other := domain.NamedRun("20260102T000000Z-bbbb")
	q := domain.NewQualitySummary([]domain.Finding{findingOf(domain.FindingMissingModel, domain.SeverityInfo, other)})

	if got := screens.RunFindingsOf(q, a); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
	if got := screens.RunFindingsOf(domain.QualitySummary{}, a); got != nil {
		t.Errorf("empty summary: got %+v, want nil", got)
	}
}

func TestRunFindingsOf_TheExistingRunQualityIsUnchanged(t *testing.T) {
	a := domain.NamedRun("20260101T000000Z-aaaa")
	q := domain.NewQualitySummary([]domain.Finding{
		findingOf(domain.FindingMalformedLine, domain.SeverityWarning, a),
		findingOf(domain.FindingMissingModel, domain.SeverityInfo, a),
	})

	if got := screens.RunQualityOf(q, a); got != (screens.RunQuality{Findings: 2, DataLoss: true}) {
		t.Errorf("RunQualityOf = %+v, want 2 findings with data loss", got)
	}
}
