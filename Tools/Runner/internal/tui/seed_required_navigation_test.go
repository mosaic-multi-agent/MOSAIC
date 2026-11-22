package tui

// seed_required_navigation_test.go verifies the seed screen's wording follows the
// workflow selected in the current setup, and that a blank required seed never
// silently starts the run. Workflows are inline test content, never catalog files.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/setup"
	"mosaic-run/internal/workflow"
)

// Workflow whose first row reads (and also writes) Requirements.md.
const seedWorkflowWithInput = `## With Input

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | first-agent | TRUE | COMPLETE | - | Requirements.md | Requirements.md |
| REVIEW | second-agent | FALSE | COMPLETE | first-agent | Other.md | Review.md |
`

// Workflow whose first row declares no input; a later row does.
const seedWorkflowWithoutInput = `## Without Input

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | TRUE | COMPLETE | - | - | Plan.md |
| REVIEW | reviewer | FALSE | COMPLETE | planner | Plan.md | Review.md |
`

// newSeedWorkflowModel returns a new-run model on the workflow screen listing
// the "with-input" and "without-input" workflows, in that order.
func newSeedWorkflowModel(t *testing.T) *rootModel {
	t.Helper()
	m := newTestModelNewRun()
	regions := []domain.WorkflowRegion{
		{Info: domain.WorkflowInfo{ID: "with-input"}, Content: []byte(seedWorkflowWithInput)},
		{Info: domain.WorkflowInfo{ID: "without-input"}, Content: []byte(seedWorkflowWithoutInput)},
	}
	for _, r := range regions {
		if _, err := workflow.Parse(r.Content, r.Info); err != nil {
			t.Fatalf("test workflow %s does not parse: %v", r.Info.ID, err)
		}
	}
	m.workflows = regions
	m.workflowScreen = setup.NewWorkflowSelectScreen(regions, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenSetupWorkflow
	return m
}

// selectWorkflowAndTask picks the workflow `downs` entries below the first and
// answers the task screen, leaving the model on the seed screen.
func selectWorkflowAndTask(t *testing.T, m *rootModel, downs int) {
	t.Helper()
	for i := 0; i < downs; i++ {
		sendKey(m, tea.KeyDown)
	}
	sendKey(m, tea.KeyEnter)
	if m.screen != screenSetupTask {
		t.Fatalf("after workflow selection: screen = %v, want screenSetupTask", m.screen)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	sendKey(m, tea.KeyEnter)
	if m.screen != screenSetupSeedInput {
		t.Fatalf("after task entry: screen = %v, want screenSetupSeedInput", m.screen)
	}
}

func assertSeedWordingRequired(t *testing.T, m *rootModel) {
	t.Helper()
	view := strings.Join(strings.Fields(m.seedInputScreen.View()), " ")
	if !strings.Contains(view, "(required)") || !strings.Contains(view, "Requirements.md") {
		t.Errorf("seed screen should show required wording naming Requirements.md:\n%s", view)
	}
}

func assertSeedWordingOptional(t *testing.T, m *rootModel) {
	t.Helper()
	view := strings.Join(strings.Fields(m.seedInputScreen.View()), " ")
	if !strings.Contains(view, "Seed input path (optional):") || strings.Contains(view, "(required)") {
		t.Errorf("seed screen should show today's optional wording:\n%s", view)
	}
}

func TestSeedWording_SelectedWorkflowWithInput_ShowsRequiredWording(t *testing.T) {
	m := newSeedWorkflowModel(t)

	selectWorkflowAndTask(t, m, 0)

	assertSeedWordingRequired(t, m)
}

func TestSeedWording_SelectedWorkflowWithoutInput_ShowsOptionalWording(t *testing.T) {
	m := newSeedWorkflowModel(t)

	selectWorkflowAndTask(t, m, 1)

	assertSeedWordingOptional(t, m)
}

func TestSeedWording_FollowsReselection_RequiredThenOptional(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 0)
	sendKey(m, tea.KeyEsc) // seed -> task
	sendKey(m, tea.KeyEsc) // task -> workflow
	if m.screen != screenSetupWorkflow {
		t.Fatalf("precondition: screen = %v, want screenSetupWorkflow", m.screen)
	}

	m.taskScreen.Reset()
	selectWorkflowAndTask(t, m, 1)

	assertSeedWordingOptional(t, m)
}

func TestSeedWording_FollowsReselection_OptionalThenRequired(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 1)
	sendKey(m, tea.KeyEsc) // seed -> task
	sendKey(m, tea.KeyEsc) // task -> workflow
	if m.screen != screenSetupWorkflow {
		t.Fatalf("precondition: screen = %v, want screenSetupWorkflow", m.screen)
	}

	m.taskScreen.Reset()
	sendKey(m, tea.KeyUp) // the list cursor persists on "without-input"; move back to "with-input"
	selectWorkflowAndTask(t, m, 0)

	assertSeedWordingRequired(t, m)
}

func TestSeedWording_UnparseableWorkflow_FallsBackToOptionalWording(t *testing.T) {
	m := newTestModelNewRun()
	regions := []domain.WorkflowRegion{
		{Info: domain.WorkflowInfo{ID: "broken"}, Content: []byte("no table here")},
	}
	m.workflows = regions
	m.workflowScreen = setup.NewWorkflowSelectScreen(regions, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenSetupWorkflow

	selectWorkflowAndTask(t, m, 0)

	assertSeedWordingOptional(t, m)
}

func TestSeedRequired_BlankFirstEnter_StaysOnSeedScreen(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 0)

	sendKey(m, tea.KeyEnter)

	if m.screen != screenSetupSeedInput {
		t.Errorf("screen = %v after blank Enter on a required seed, want to stay on screenSetupSeedInput", m.screen)
	}
}

func TestSeedRequired_BlankSecondEnter_ContinuesWithEmptySeed(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 0)

	sendKey(m, tea.KeyEnter)
	sendKey(m, tea.KeyEnter)

	if m.screen != screenSetupConfig {
		t.Fatalf("screen = %v after confirming a blank required seed, want screenSetupConfig", m.screen)
	}
	if m.selections.seedInput != "" {
		t.Errorf("selections.seedInput = %q, want empty", m.selections.seedInput)
	}
}

func TestSeedRequired_PathEntered_ContinuesWithPath(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 0)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/path/to/Requirements.md")})
	sendKey(m, tea.KeyEnter)

	if m.screen != screenSetupConfig {
		t.Fatalf("screen = %v after a path on a required seed, want screenSetupConfig", m.screen)
	}
	if m.selections.seedInput != "/path/to/Requirements.md" {
		t.Errorf("selections.seedInput = %q, want the entered path", m.selections.seedInput)
	}
}

func TestSeedRequired_BackToSeedScreen_RestartsBlankConfirmation(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 0)
	sendKey(m, tea.KeyEnter)
	sendKey(m, tea.KeyEnter) // confirmed blank -> config
	sendKey(m, tea.KeyEsc)   // config -> seed
	if m.screen != screenSetupSeedInput {
		t.Fatalf("precondition: screen = %v, want screenSetupSeedInput", m.screen)
	}

	sendKey(m, tea.KeyEnter)

	if m.screen != screenSetupSeedInput {
		t.Errorf("screen = %v: the first blank Enter after re-entry must warn again", m.screen)
	}
}

func TestSeedOptional_BlankEnter_ContinuesImmediately(t *testing.T) {
	m := newSeedWorkflowModel(t)
	selectWorkflowAndTask(t, m, 1)

	sendKey(m, tea.KeyEnter)

	if m.screen != screenSetupConfig {
		t.Errorf("screen = %v after blank Enter on an optional seed, want screenSetupConfig", m.screen)
	}
}
