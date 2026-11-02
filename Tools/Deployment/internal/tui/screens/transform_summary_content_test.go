package screens_test

// transform_summary_content_test.go verifies that the transform summary screen reports a
// whole-deployment move: the plan listing of every outcome, skip and failure reasons,
// unresolved-model agents, unmapped tools, notes (both-forms warning, newer-injections
// note), the TODO path and, when no agents were found, the folder transform looked in.
// It also covers the transform mode entry text.

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	tcAgentsDir = "/ws/.claude/agents"
	tcBothMsg   = "reviewer exists in both agent file forms"
	// tcOrphanAgent appears in no file outcome, so seeing it proves the unresolved-model
	// section is rendered.
	tcOrphanAgent = "orphan-agent"
	tcNewerMsg    = "injections and hooks come from the target and may be newer"
)

func tcDeploymentResult() app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID:  tsSource,
		TargetHarnessID:  tsTarget,
		Workspace:        "/ws",
		SourceAgentsDir:  tcAgentsDir,
		UnresolvedModels: []string{tcOrphanAgent},
		TodoPath:         "/ws/mosaic-deploy-todo.md",
		Transformed:      2,
		Skipped:          2,
		Failed:           1,
		Notes: []app.TransformNote{
			{Kind: app.NoteBothForms, Message: tcBothMsg,
				Paths: []string{".opencode/agents/reviewer.md", ".opencode/agents/reviewer.agent.md"}},
			{Kind: app.NoteNewerTargetContent, Message: tcNewerMsg},
		},
		Files: []app.TransformFileOutcome{
			{Kind: domain.ArtifactAgent, Key: "writer", SourcePath: tcAgentsDir + "/writer.md",
				DestinationPath: ".opencode/agents/writer.md", Action: app.PlanWriteCreate,
				Status: app.StatusTransformed, UnmappedTools: []string{"WebSearch"}},
			{Kind: domain.ArtifactSkill, Key: "lint", SourcePath: "/ws/.claude/skills/lint",
				DestinationPath: ".opencode/skills/lint", Action: app.PlanWriteOverwrite,
				Status: app.StatusTransformed},
			{Kind: domain.ArtifactAgent, Key: "notes", SourcePath: tcAgentsDir + "/notes.md",
				Action: app.PlanSkip, Status: app.StatusSkipped, Skip: app.SkipNotAgent,
				Reason: "no version stamp in frontmatter"},
			{Kind: domain.ArtifactHook, Key: "logger", DestinationPath: ".opencode/hooks/logger",
				Action: app.PlanSkip, Status: app.StatusSkipped, Skip: app.SkipNoTargetVariant,
				Reason: "bundle has no opencode variant"},
			{Kind: domain.ArtifactAgent, Key: "broken", SourcePath: tcAgentsDir + "/broken.md",
				DestinationPath: ".opencode/agents/broken.md", Action: app.PlanWriteCreate,
				Status: app.StatusFailed, Reason: "frontmatter could not be parsed"},
		},
	}
}

func TestTransformSummaryScreen_View_ShowsEveryOutcomeWithAction(t *testing.T) {
	view := collapseWhitespace(tsView(tcDeploymentResult(), plainStyles()))

	for _, want := range []string{
		"writer.md", ".opencode/agents/writer.md", "create",
		"skills/lint", "overwrite",
		"notes.md", "no version stamp in frontmatter",
		"hooks/logger", "bundle has no opencode variant",
		"broken.md", "frontmatter could not be parsed",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	counts := regexp.MustCompile(`(?i)transformed\W+2\W+skipped\W+2\W+failed\W+1`)
	if !counts.MatchString(view) {
		t.Errorf("view lacks the counts:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_ShowsUnresolvedModelsAndUnmappedTools(t *testing.T) {
	result := tcDeploymentResult()
	result.UnresolvedModels = []string{tcOrphanAgent}
	raw := tsView(result, plainStyles())

	if !tcLineWithAll(raw, tcOrphanAgent, "model") {
		t.Errorf("view does not list %q on a line labelled as a model problem:\n%s", tcOrphanAgent, raw)
	}
	if view := collapseWhitespace(raw); !strings.Contains(view, "WebSearch") {
		t.Errorf("view does not list the unmapped tool:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_NoUnresolvedModels_AgentNotLabelledAsModelProblem(t *testing.T) {
	result := tcDeploymentResult()
	result.UnresolvedModels = nil
	result.Files = append(result.Files, app.TransformFileOutcome{Kind: domain.ArtifactAgent,
		Key: tcOrphanAgent, SourcePath: tcAgentsDir + "/" + tcOrphanAgent + ".md",
		DestinationPath: ".opencode/agents/" + tcOrphanAgent + ".md", Action: app.PlanWriteCreate,
		Status: app.StatusTransformed})
	raw := tsView(result, plainStyles())

	if !strings.Contains(raw, tcOrphanAgent) {
		t.Fatalf("fixture agent %q not shown at all:\n%s", tcOrphanAgent, raw)
	}
	if tcLineWithAll(raw, tcOrphanAgent, "model") {
		t.Errorf("agent shown as a model problem though UnresolvedModels is empty:\n%s", raw)
	}
}

// tcLineWithAll reports whether one line of view contains every word (case-insensitive).
func tcLineWithAll(view string, words ...string) bool {
	for _, l := range strings.Split(strings.ToLower(view), "\n") {
		all := true
		for _, w := range words {
			all = all && strings.Contains(l, strings.ToLower(w))
		}
		if all {
			return true
		}
	}
	return false
}

func TestTransformSummaryScreen_View_ShowsNotesAndTodoPath(t *testing.T) {
	view := collapseWhitespace(tsView(tcDeploymentResult(), plainStyles()))

	for _, want := range []string{tcBothMsg, ".opencode/agents/reviewer.md",
		".opencode/agents/reviewer.agent.md", tcNewerMsg, "/ws/mosaic-deploy-todo.md"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestTransformSummaryScreen_View_NoAgentsFound_NamesFolderTransformLookedIn(t *testing.T) {
	result := app.TransformHarnessResult{
		SourceHarnessID: tsSource, TargetHarnessID: tsTarget, Workspace: "/ws",
		SourceAgentsDir: tcAgentsDir,
		Notes: []app.TransformNote{{Kind: app.NoteNoAgentsFound, Message: "no MOSAIC agents were found",
			Paths: []string{tcAgentsDir}}},
	}
	view := collapseWhitespace(tsView(result, plainStyles()))

	if !strings.Contains(view, tcAgentsDir) {
		t.Errorf("view does not name the folder transform looked in:\n%s", view)
	}
	if !strings.Contains(view, "no MOSAIC agents were found") {
		t.Errorf("view does not show the no-agents note:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_DryRun_ListsSamePlanLinesAsRealRun(t *testing.T) {
	realRun := tcDeploymentResult()
	dry := tcDeploymentResult()
	dry.DryRun = true
	paths := []string{"writer.md", "skills/lint", "notes.md", "hooks/logger", "broken.md"}

	for _, p := range paths {
		a := strings.TrimSpace(lineWith(tsView(realRun, plainStyles()), p))
		b := strings.TrimSpace(lineWith(tsView(dry, plainStyles()), p))
		if a == "" || a != b {
			t.Errorf("listing line for %q differs between real and dry run:\nreal: %q\ndry:  %q", p, a, b)
		}
	}
}

// ---------------------------------------------------------------------------
// Mode entry and workspace-root path screen
// ---------------------------------------------------------------------------

func transformModeView(t *testing.T) string {
	t.Helper()
	s := newModeScreen()
	pos := positionOf(newModeScreen(), domain.ModeTransformHarness, maxModeNavigations)
	if pos < 0 {
		t.Fatal("transform mode not offered")
	}
	for i := 0; i < pos; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return strings.ToLower(collapseWhitespace(s.View()))
}

func TestModeScreen_Transform_DescribesMovingAWholeDeployment(t *testing.T) {
	view := transformModeView(t)

	for _, want := range []string{"workspace", "deployment", "skill", "hook"} {
		if !strings.Contains(view, want) {
			t.Errorf("transform mode text does not mention %q:\n%s", want, view)
		}
	}
	for _, old := range []string{"agent file", "non-recursive", "folder of agent", "single agent"} {
		if strings.Contains(view, old) {
			t.Errorf("transform mode text still describes file or folder input (%q):\n%s", old, view)
		}
	}
}
