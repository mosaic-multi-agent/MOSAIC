package app_test

// End-to-end test of the transcript integrity findings with the real scanner,
// reader and transcript adapter on the synthetic run in testdata/transcript_integrity.
//
// The run holds an orchestrator whose transcript has one message without a usage
// record, an agent folder planner#9 whose usage records and transcript each have
// one id the other lacks and whose first prompt names coder#8, and a clean agent
// folder coder#8.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/transcript"
)

const (
	tiLogs = "../../testdata/transcript_integrity/runs"
	tiRun  = "20261101T090000Z-e201"
)

func TestAnalyze_RealTranscriptsOfASyntheticRun_ReportBothFindingKindsForTheRightActors(t *testing.T) {
	store := newFakePricingStore()
	store.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing("claude-sonnet")})
	svc := app.New(app.Deps{
		Source: logscan.NewScanner(), Reader: logread.NewReader(), Transcripts: transcript.NewReader(),
		Pricing: store, Clock: newFakeClock(), Interaction: &alwaysSkippingInteraction{}, WorkDir: t.TempDir(),
	})

	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: tiLogs})

	if err != nil || res.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze: err=%v outcome=%v", err, res.Outcome)
	}
	gaps := map[domain.ActorRef]int{}
	for _, f := range dgFindings(res.Report, domain.FindingTranscriptCaptureGap) {
		if f.Run != domain.NamedRun(tiRun) {
			t.Errorf("gap finding run = %+v, want %s", f.Run, tiRun)
		}
		gaps[f.Actor]++
	}
	wantGaps := map[domain.ActorRef]int{domain.AgentInstance("planner#9"): 2, domain.Orchestrator(): 1}
	if len(gaps) != len(wantGaps) || gaps[domain.AgentInstance("planner#9")] != 2 || gaps[domain.Orchestrator()] != 1 {
		t.Errorf("capture gap findings per actor = %v, want %v (one per side for planner#9, side a for the orchestrator)", gaps, wantGaps)
	}
	mismatches := dgFindings(res.Report, domain.FindingInstancePromptMismatch)
	if len(mismatches) != 1 || mismatches[0].Actor != domain.AgentInstance("planner#9") || mismatches[0].Run != domain.NamedRun(tiRun) {
		t.Errorf("instance prompt mismatch findings = %+v, want exactly one for planner#9", mismatches)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("transcript integrity findings must not make the report incomplete")
	}
}
