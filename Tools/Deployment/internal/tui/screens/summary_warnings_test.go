package screens_test

// summary_warnings_test.go locks the summary screen's display of the two region warnings
// (orchestrator content not delivered, role-forbidden region): both appear in the follow-up
// list of a deploy and of an update summary, with their detail and owning agent.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

const (
	tuiUndeliveredDetail = "orchestrator source has no marker; harness content not deployed"
	tuiRoleForbidden     = "[role-forbidden-region] region not allowed for this role"
)

func summaryWithRegionWarnings(mode domain.RunMode) domain.RunSummary {
	s := cleanSummary()
	s.Mode = mode
	s.Outcome = domain.OutcomeCompletedWithGaps
	s.Todos = []domain.TodoItem{
		{Category: domain.TodoManual, Subject: "orchestrator", Owner: "orchestrator", Detail: tuiUndeliveredDetail,
			Kind: domain.GapUndeliveredOrchestratorInjection},
		{Category: domain.TodoManual, Subject: "OrchestratorHarnessConstraints", Owner: "test-runner", Detail: tuiRoleForbidden,
			Kind: domain.GapRoleForbiddenRegion},
	}
	return s
}

func TestSummaryScreen_ShowsBothRegionWarnings(t *testing.T) {
	for _, mode := range []domain.RunMode{domain.ModeDeployWorkspace, domain.ModeUpdateWorkspace} {
		t.Run(string(mode), func(t *testing.T) {
			s := screens.NewSummaryScreen(summaryWithRegionWarnings(mode), 120, 60, plainStyles())

			view := s.View()

			for _, want := range []string{tuiUndeliveredDetail, tuiRoleForbidden, "test-runner"} {
				if !strings.Contains(view, want) {
					t.Errorf("summary view does not show %q:\n%s", want, view)
				}
			}
			if strings.Count(view, tuiUndeliveredDetail) != 1 || strings.Count(view, tuiRoleForbidden) != 1 {
				t.Errorf("each warning must appear exactly once:\n%s", view)
			}
		})
	}
}
