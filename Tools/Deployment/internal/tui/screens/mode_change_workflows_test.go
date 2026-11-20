package screens_test

// mode_change_workflows_test.go verifies the wording of the workflow-change mode on the mode
// screen: its label and the description/detail of what the mode does.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// workflowChangeDetailView renders a tall, wide mode screen with the workflow-change entry
// focused so the whole detail text is visible.
func workflowChangeDetailView() string {
	s := screens.NewModeScreen(220, 60, plainStyles())
	s.Update(modeDownKey())
	s.Update(modeDownKey())
	return collapseWhitespace(s.View())
}

func TestModeScreen_ChangeWorkflows_LabelReplacesUpdateWorkflows(t *testing.T) {
	view := collapseWhitespace(newModeScreen().View())

	if !strings.Contains(view, "Change workflows") {
		t.Errorf("mode list does not show 'Change workflows':\n%s", view)
	}
	if strings.Contains(view, "Update workflows") {
		t.Errorf("mode list still shows the old label 'Update workflows':\n%s", view)
	}
}

func TestModeScreen_ChangeWorkflows_KeepsRunModeIdentifier(t *testing.T) {
	s := newModeScreen()
	s.Update(modeDownKey())
	s.Update(modeDownKey())
	s.Update(modeEnterKey())

	if s.SelectedMode() != domain.ModeUpdateWorkflows || string(s.SelectedMode()) != "update-workflows" {
		t.Errorf("SelectedMode() = %q; want the unchanged run mode update-workflows", s.SelectedMode())
	}
}

func TestModeScreen_ChangeWorkflows_DoesNotClaimNothingIsChanged(t *testing.T) {
	view := workflowChangeDetailView()

	for _, banned := range []string{"Only the orchestrator file is rewritten", "no agent, skill, or hook artifact is touched"} {
		if strings.Contains(view, banned) {
			t.Errorf("workflow-change detail still contains the outdated claim %q:\n%s", banned, view)
		}
	}
}

func TestModeScreen_ChangeWorkflows_DetailDescribesDeployingMissingAgentsAndSkills(t *testing.T) {
	view := strings.ToLower(workflowChangeDetailView())

	for _, want := range []string{"agents", "skills"} {
		if !strings.Contains(view, want) {
			t.Errorf("workflow-change detail does not mention %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "missing") && !strings.Contains(view, "not yet") && !strings.Contains(view, "not in the workspace") {
		t.Errorf("workflow-change detail does not say the agents and skills are missing from the workspace:\n%s", view)
	}
	if !strings.Contains(view, "deploy") && !strings.Contains(view, "added") && !strings.Contains(view, "installed") {
		t.Errorf("workflow-change detail does not say missing agents and skills are deployed:\n%s", view)
	}
}

func TestModeScreen_ChangeWorkflows_DetailPaneIsTitledChangeWorkflows(t *testing.T) {
	s := screens.NewModeScreen(220, 60, plainStyles())
	s.Update(modeDownKey())
	s.Update(modeDownKey())
	full := collapseWhitespace(s.View())

	// The list row also carries the label, so the old label must be absent everywhere.
	if strings.Contains(full, "Update workflows") {
		t.Errorf("focused view still shows 'Update workflows':\n%s", full)
	}
	if strings.Count(full, "Change workflows") < 2 {
		t.Errorf("detail pane does not carry the title 'Change workflows' in addition to the list row:\n%s", full)
	}
}

func TestModeScreen_ChangeWorkflows_DetailSaysOtherDeployedFilesAreUntouched(t *testing.T) {
	view := strings.ToLower(workflowChangeDetailView())

	untouched := regexp.MustCompile(`(no|nothing|not).{0,40}other.{0,30}agents?.{0,30}skills?`)
	if !untouched.MatchString(view) {
		t.Errorf("workflow-change detail does not say other agents, skills and hooks are left alone:\n%s", view)
	}
	if !strings.Contains(view, "orchestrator") || !(strings.Contains(view, "remov") || strings.Contains(view, "drop")) {
		t.Errorf("workflow-change detail does not describe the orchestrator rewrite and removal of unselected workflows:\n%s", view)
	}
}
