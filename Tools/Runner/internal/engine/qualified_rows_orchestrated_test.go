package engine_test

// Mode 1 over a table that repeats one agent through mode qualifiers: after a
// recorded step of either qualified row the engine hands the decision to the
// consultant and names the exact recorded row as the current position, never
// the other row that holds the same bare agent id.

import (
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func TestQualifiedRows_Orchestrated_ConsultNamesTheExactRecordedRow(t *testing.T) {
	cases := []struct {
		name    string
		row     int // 1-based recorded row
		stage   string
		wantRow int // zero-based position named by the consultation
	}{
		{"first qualified row", rowTestsReviewer + 1, "1.Test", rowTestsReviewer},
		{"second qualified row", rowImplReviewer + 1, "1.Implementation", rowImplReviewer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmitScope(t, qualifiedRows(), groupedStageTables(), domain.ExecutionModeOrchestrated)
			log := (&runLog{}).
				workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
				workflowStep("build-review", tc.stage, domain.StatusSUCCESS, tc.row)

			dec := engine.Next(engine.NextInput{
				Workflow:  aw,
				Schedules: qualifiedSchedules(),
				State:     log.state(),
				Agents:    scopeAgents(aw),
				Seq:       len(log.entries),
				Now:       time.Unix(0, 0),
				Mode:      domain.ExecutionModeOrchestrated,
			})

			if dec.Consult == nil {
				t.Fatalf("want a consultation in Mode 1, got %+v", dec)
			}
			if dec.Consult.CurrentRow != tc.wantRow {
				t.Errorf("want the consultation to name row %d (the recorded row), got %d", tc.wantRow, dec.Consult.CurrentRow)
			}
		})
	}
}
