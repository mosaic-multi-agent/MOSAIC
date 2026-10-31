package digest_test

// The format-change alarm finding is kept in digests by the existing finding
// encoding, so a digest-loaded run keeps it.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestRoundTrip_TranscriptFormatChangedFinding_KeepsKindSeverityAndRun(t *testing.T) {
	want := domain.Digest{
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{{Run: domain.NamedRun(runA)}}},
		Findings: []domain.Finding{{
			Kind: domain.FindingTranscriptFormatChanged, Severity: domain.SeverityWarning, Run: domain.NamedRun(runA),
		}},
	}

	got, findings := roundTrip(t, want)

	if len(findings) != 0 {
		t.Fatalf("read findings = %+v", findings)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("carried findings = %+v, want the alarm finding", got.Findings)
	}
	f := got.Findings[0]
	if f.Kind != domain.FindingTranscriptFormatChanged || f.Severity != domain.SeverityWarning || f.Run != domain.NamedRun(runA) {
		t.Errorf("finding = %+v, want kind, severity and run kept", f)
	}
	if !domain.NewQualitySummary(got.Findings).Incomplete() {
		t.Error("a digest-loaded run with the alarm finding is complete, want incomplete")
	}
}
