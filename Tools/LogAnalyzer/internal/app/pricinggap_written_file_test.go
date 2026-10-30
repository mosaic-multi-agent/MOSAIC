package app_test

// Tests that the analysis result reports the file the pricing-gap flow wrote, so
// a frontend that drops the flow's notice can still show it.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// analyzeWithGapFlow analyses the one-unpriced-model scenario with the gap flow
// enabled and returns the result.
func analyzeWithGapFlow(t *testing.T, ia *scriptedInteraction) app.Result {
	t.Helper()
	svc, _, _ := providerGapAnalysis(t, ia, []domain.ProviderID{""})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs", ResolvePricingGaps: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return res
}

func TestAnalyze_GapFlowSave_ResultNamesTheFileWritten(t *testing.T) {
	cases := []struct {
		option string
		path   string
	}{
		{"global", fakeGlobalPricingPath},
		{"project", "/test/pricing.yaml"},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			ia := enteringInteraction(func(string) bool { return true })
			ia.levelOption = c.option

			res := analyzeWithGapFlow(t, ia)

			if res.GapConfigPath != c.path {
				t.Errorf("GapConfigPath = %q, want the written file %q", res.GapConfigPath, c.path)
			}
		})
	}
}

func TestAnalyze_GapFlowSavedNothing_ResultNamesNoFile(t *testing.T) {
	skippedLevel := enteringInteraction(func(string) bool { return true })
	skippedLevel.skipLevel = true
	cases := map[string]*scriptedInteraction{
		"action skipped":         {},
		"level question skipped": skippedLevel,
		"every rate declined":    enteringInteraction(func(string) bool { return false }),
	}
	for name, ia := range cases {
		t.Run(name, func(t *testing.T) {
			res := analyzeWithGapFlow(t, ia)

			if res.GapConfigPath != "" {
				t.Errorf("GapConfigPath = %q, want none when nothing was saved", res.GapConfigPath)
			}
		})
	}
}
