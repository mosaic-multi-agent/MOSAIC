package screens_test

// mode_promote_batch_test.go verifies the wording of the promote entry on the mode screen: the
// label stays, the description and detail describe a batch promote from an agents folder.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/tui/screens"
)

// promoteDetailView renders a tall, wide mode screen with the promote entry focused.
func promoteDetailView() string {
	s := screens.NewModeScreen(220, 60, plainStyles())
	for i := 0; i < 5; i++ {
		s.Update(modeDownKey())
	}
	return collapseWhitespace(s.View())
}

func TestModeScreen_Promote_KeepsItsLabel(t *testing.T) {
	view := collapseWhitespace(newModeScreen().View())

	if !strings.Contains(view, "Promote to generic") {
		t.Errorf("mode list does not show 'Promote to generic':\n%s", view)
	}
}

func TestModeScreen_Promote_DetailDescribesChoosingAnAgentsFolderAndSelectingAgents(t *testing.T) {
	view := strings.ToLower(promoteDetailView())

	for _, want := range []string{"folder", "one or many"} {
		if !strings.Contains(view, want) {
			t.Errorf("promote detail does not mention %q:\n%s", want, view)
		}
	}
}

func TestModeScreen_Promote_DetailStatesSourceFilesAreUntouched(t *testing.T) {
	view := strings.ToLower(promoteDetailView())

	if !strings.Contains(view, "untouched") && !strings.Contains(view, "not changed") && !strings.Contains(view, "never changed") {
		t.Errorf("promote detail does not say the source files are left untouched:\n%s", view)
	}
}
