package domain_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestSourceIsUsable_ByKind(t *testing.T) {
	cases := []struct {
		kind domain.SourceKind
		want bool
	}{
		{domain.SourceNotFound, false},
		{domain.SourceUnusable, false},
		{domain.SourceLogsRoot, true},
		{domain.SourceSingleRun, true},
		{domain.SourceDigestFile, true},
		{domain.SourceDigestDir, true},
	}
	for _, c := range cases {
		if got := (domain.Source{Kind: c.kind}).IsUsable(); got != c.want {
			t.Errorf("Source{Kind: %v}.IsUsable() = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestQualitySummary_LoadTimeDigestFindings_DataLossOnlyForUnreadableAndUnsupported(t *testing.T) {
	cases := []struct {
		kind domain.FindingKind
		want bool
	}{
		{domain.FindingInvalidDigest, true},
		{domain.FindingUnsupportedDigestVersion, true},
		{domain.FindingDuplicateRun, false},
	}
	for _, c := range cases {
		q := domain.NewQualitySummary([]domain.Finding{{Kind: c.kind}})
		if got := q.Incomplete(); got != c.want {
			t.Errorf("Incomplete() with only %s = %v, want %v", c.kind, got, c.want)
		}
	}
}
