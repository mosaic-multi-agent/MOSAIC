package runselect_test

// The announcement of a resumed run names the recorded iteration with the key
// "iteration", not "stage".

import (
	"strings"
	"testing"

	"mosaic-run/internal/runselect"
)

func TestAnnounce_ResumedRunWithIteration_LabelUsesIterationKey(t *testing.T) {
	for _, value := range []string{"1.Test", "1", "A"} {
		t.Run(value, func(t *testing.T) {
			id := runselect.Identity{
				RunID: "20260701T120000Z-a3f9",
				Position: &runselect.Position{
					Phase:     "EXECUTION",
					Stage:     value,
					LastAgent: "implementation-tdd#2",
				},
			}

			text := runselect.Announce(id)

			if !strings.Contains(text, "iteration="+value) {
				t.Errorf("Announce text %q does not contain %q", text, "iteration="+value)
			}
			if strings.Contains(text, "stage=") {
				t.Errorf("Announce text %q still uses the stage key", text)
			}
		})
	}
}

func TestAnnounce_ResumedRunWithoutIteration_NamesNoIteration(t *testing.T) {
	id := runselect.Identity{
		RunID:    "20260701T120000Z-a3f9",
		Position: &runselect.Position{Phase: "PLANNING", LastAgent: "planner#1"},
	}

	text := runselect.Announce(id)

	if strings.Contains(text, "iteration=") || strings.Contains(text, "stage=") {
		t.Errorf("Announce text %q names an iteration for a run that has none", text)
	}
}
