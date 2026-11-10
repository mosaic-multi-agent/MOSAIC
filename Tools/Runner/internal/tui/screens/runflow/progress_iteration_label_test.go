package runflow

// The label of a running row names the step's iteration with the key
// "iteration", not "stage".

import (
	"strings"
	"testing"
)

func TestProgressRow_RunningWithIteration_LabelUsesIterationKey(t *testing.T) {
	for _, value := range []string{"1.Test", "1", "A"} {
		t.Run(value, func(t *testing.T) {
			s := NewProgressScreen(80, 24, progressStyles())
			s.AppendRow(ProgressRow{AgentInstance: "agent#1", Phase: "EXECUTION", Stage: value})

			line := firstRowLine(s)

			if !strings.Contains(line, "iteration="+value) {
				t.Errorf("running row label %q does not contain %q", line, "iteration="+value)
			}
			if strings.Contains(line, "stage=") {
				t.Errorf("running row label %q still uses the stage key", line)
			}
		})
	}
}

func TestProgressRow_RunningWithoutIteration_LabelHasNoIterationKey(t *testing.T) {
	line := extractRunningRowLine(progressStyles())

	if strings.Contains(line, "iteration=") || strings.Contains(line, "stage=") {
		t.Errorf("running row label %q names an iteration for a row that has none", line)
	}
}
