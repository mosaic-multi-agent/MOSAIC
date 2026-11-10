package screens_test

// review_injection_versions_test.go specifies how the plan review shows the two injection
// version changes of an orchestrator: each changed version on its own line with deployed and
// source values, labelled by the file it comes from, and readable at a typical terminal width.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

const (
	sharedFileLabel       = "HarnessInjections.md version"
	orchestratorFileLabel = "HarnessInjectionsOrchestrator.md version"
)

func injectionReviewView(t *testing.T, width int, deltas ...domain.VersionDelta) string {
	t.Helper()
	p := planWithItems(domain.ModeUpdateWorkspace, []domain.PlanItem{
		{Ref: agentRef("orchestrator"), Action: domain.ActionUpdate, Stale: deltas},
	})
	return screens.NewReviewScreen(p, width, 80, plainStyles()).View()
}

func lineContaining(view, text string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return line
		}
	}
	return ""
}

func TestReviewScreen_OrchestratorBothInjectionVersionsChanged_ShowsTwoDistinctLines(t *testing.T) {
	for _, width := range []int{120, 80} {
		view := injectionReviewView(t, width,
			domain.VersionDelta{Field: "injections_version", Deployed: "1.2.1", Source: "1.4.0"},
			domain.VersionDelta{Field: "orchestrator_injections_version", Deployed: "2.0.0", Source: "2.3.0"},
		)

		shared := lineContaining(view, sharedFileLabel+":")
		if shared == "" || !strings.Contains(shared, "1.2.1") || !strings.Contains(shared, "1.4.0") {
			t.Errorf("width %d: no line shows %q with deployed 1.2.1 and source 1.4.0:\n%s", width, sharedFileLabel, view)
		}
		orch := lineContaining(view, orchestratorFileLabel+":")
		if orch == "" || !strings.Contains(orch, "2.0.0") || !strings.Contains(orch, "2.3.0") {
			t.Errorf("width %d: no line shows %q with deployed 2.0.0 and source 2.3.0:\n%s", width, orchestratorFileLabel, view)
		}
		if shared != "" && shared == orch {
			t.Errorf("width %d: both changes share one line:\n%s", width, view)
		}
		if strings.Contains(shared, "2.3.0") || strings.Contains(orch, "1.4.0") {
			t.Errorf("width %d: a line mixes values of the two files:\n%s", width, view)
		}
	}
}

func TestReviewScreen_OrchestratorOnlySharedVersionChanged_ShowsOnlySharedLine(t *testing.T) {
	view := injectionReviewView(t, 120,
		domain.VersionDelta{Field: "injections_version", Deployed: "1.2.1", Source: "1.4.0"},
	)

	if lineContaining(view, sharedFileLabel+":") == "" {
		t.Errorf("shared-file change not shown with label %q:\n%s", sharedFileLabel, view)
	}
	if strings.Contains(view, orchestratorFileLabel) {
		t.Errorf("orchestrator-file line shown although only the shared file changed:\n%s", view)
	}
}

func TestReviewScreen_OrchestratorOnlyOrchestratorVersionChanged_ShowsOnlyOrchestratorLine(t *testing.T) {
	view := injectionReviewView(t, 120,
		domain.VersionDelta{Field: "orchestrator_injections_version", Deployed: "2.0.0", Source: "2.3.0"},
	)

	if lineContaining(view, orchestratorFileLabel+":") == "" {
		t.Errorf("orchestrator-file change not shown with label %q:\n%s", orchestratorFileLabel, view)
	}
	// "HarnessInjectionsOrchestrator.md version" does not contain the shared label as a
	// substring, so this also guards against the shared line appearing.
	if strings.Contains(view, sharedFileLabel) {
		t.Errorf("shared-file line shown although only the orchestrator file changed:\n%s", view)
	}
}
