package resumecheck_test

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/resumecheck"
)

// summaryLine renders one summary line: the label padded to the common column.
func summaryLine(label, value string) string {
	return "  " + label + ":" + strings.Repeat(" ", 19-len(label)) + value
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func fullyRecordedState() domain.ArtifactState {
	state := recordedState()
	state.Mode = domain.ExecutionModeAutoReview
	state.WorkflowVersion = "2.1"
	state.PreConsultation = true
	state.ManualResolution = false
	state.CommitBranchVariant = domain.CommitBranchMOSAICOwned
	state.ReviewLoopLimit = 4
	state.InfrastructureOverrides = []domain.InfrastructureOverride{
		{AgentName: "cp-b", Triggers: []domain.DeclaredInfraTrigger{
			{Trigger: domain.InfraTriggerInvocationInterval, Param: "5"},
			{Trigger: domain.InfraTriggerManual},
		}},
		{AgentName: "git-commit", Triggers: []domain.DeclaredInfraTrigger{{Trigger: domain.InfraTriggerIterationEnd}}},
	}
	return state
}

func TestSummaryLines_FullyRecordedRun_ListsEverySetting(t *testing.T) {
	summary := resumecheck.NewSummary(fullyRecordedState(), declaredAgents())

	got := summary.Lines()

	assertLines(t, got, []string{
		"Recorded settings for run " + fixtureRunID,
		summaryLine("Workflow", "build-flow (version 2.1)"),
		summaryLine("Mode", "auto-review"),
		summaryLine("Pre-consultation", "on"),
		summaryLine("Manual resolution", "off"),
		summaryLine("Checkpoints", "on (cp-b)"),
		summaryLine("Commits", "on (branch mosaic/run/"+fixtureRunID+", mosaic-owned)"),
		summaryLine("Review loop limit", "4"),
		summaryLine("Infrastructure", "checkpoint=cp-b, commit=git-commit"),
		summaryLine("Overrides", "cp-b: INVOCATION_INTERVAL=5, MANUAL; git-commit: ITERATION_END"),
	})
}

func TestSummaryLines_DisabledSettings_AreShownAsSuch(t *testing.T) {
	state := domain.ArtifactState{RunID: fixtureRunID, Workflow: fixtureWorkflow, WorkflowVersion: "2.1"}
	state.Mode = domain.ExecutionModeOrchestrated

	got := resumecheck.NewSummary(state, declaredAgents()).Lines()

	assertLines(t, got, []string{
		"Recorded settings for run " + fixtureRunID,
		summaryLine("Workflow", "build-flow (version 2.1)"),
		summaryLine("Mode", "orchestrated"),
		summaryLine("Pre-consultation", "off"),
		summaryLine("Manual resolution", "off"),
		summaryLine("Checkpoints", "off"),
		summaryLine("Commits", "off"),
		summaryLine("Review loop limit", "none"),
		summaryLine("Infrastructure", "none recorded"),
		summaryLine("Overrides", "none"),
	})
}

func TestSummaryLines_CheckpointsWithoutSelection_ListEveryCheckpointAgent(t *testing.T) {
	state := fullyRecordedState()
	state.InfraClassSelections = nil

	got := resumecheck.NewSummary(state, declaredAgents()).Lines()

	assertContainsLine(t, got, summaryLine("Checkpoints", "on (cp-a, cp-b)"))
	assertContainsLine(t, got, summaryLine("Infrastructure", "none recorded"))
}

func TestSummaryLines_CommitSetupPending_SaysBranchNotSetUp(t *testing.T) {
	state := fullyRecordedState()
	state.CommitBranch = ""
	state.CommitBranchVariant = domain.CommitBranchUserOwn

	got := resumecheck.NewSummary(state, declaredAgents()).Lines()

	assertContainsLine(t, got, summaryLine("Commits", "on (branch not set up yet, user-own)"))
}

func TestSummaryLines_UserOwnBranch_ShowsBranchAndVariant(t *testing.T) {
	state := fullyRecordedState()
	state.CommitBranch = "feature/x"
	state.CommitBranchVariant = domain.CommitBranchUserOwn

	got := resumecheck.NewSummary(state, declaredAgents()).Lines()

	assertContainsLine(t, got, summaryLine("Commits", "on (branch feature/x, user-own)"))
}

func assertContainsLine(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, l := range lines {
		if l == want {
			return
		}
	}
	t.Errorf("lines do not contain %q; got:\n%s", want, strings.Join(lines, "\n"))
}
