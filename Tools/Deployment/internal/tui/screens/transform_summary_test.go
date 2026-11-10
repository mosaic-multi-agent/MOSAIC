package screens_test

// transform_summary_test.go verifies TransformSummaryScreen basics: harness and count
// display, per-line status styling, overall-outcome header styling and keyboard dismissal.
// Line texts are not pinned: lines are located by the paths they mention. The whole-
// deployment content (plan listing, reasons, notes) is in transform_summary_content_test.go.

import (
	"io"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// ---------------------------------------------------------------------------
// Style fixtures
// ---------------------------------------------------------------------------

// coloredStyles returns a Styles where each role has a distinct TrueColor foreground, so
// the role applied to a line can be told from the escape sequence it starts with.
func coloredStyles() screens.Styles {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return screens.Styles{
		Success: r.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		Error:   r.NewStyle().Foreground(lipgloss.Color("#ff0000")),
		Warning: r.NewStyle().Foreground(lipgloss.Color("#ffff00")),
		Muted:   r.NewStyle().Foreground(lipgloss.Color("#808080")),
		Help:    r.NewStyle().Foreground(lipgloss.Color("#c0c0c0")),
		Body:    r.NewStyle(),
	}
}

// rolePrefix returns the escape sequence a line rendered in the given style starts with.
func rolePrefix(style lipgloss.Style) string {
	return strings.SplitN(style.Render("x"), "x", 2)[0]
}

// lineWith returns the first line of view that contains substr, or "".
func lineWith(view, substr string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, substr) {
			return l
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Result fixtures
// ---------------------------------------------------------------------------

const (
	tsSource = "claude-code"
	tsTarget = "opencode"
)

func tsTransformedResult() app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID: tsSource,
		TargetHarnessID: tsTarget,
		Transformed:     1,
		Files: []app.TransformFileOutcome{{
			Kind:            domain.ArtifactAgent,
			Key:             "my-agent",
			SourcePath:      "/agents/my-agent.md",
			DestinationPath: "/out/my-agent.md",
			Action:          app.PlanWriteCreate,
			Status:          app.StatusTransformed,
		}},
	}
}

func tsFailedResult() app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID: tsSource,
		TargetHarnessID: tsTarget,
		Failed:          1,
		Files: []app.TransformFileOutcome{{
			Kind:            domain.ArtifactAgent,
			Key:             "my-agent",
			SourcePath:      "/agents/my-agent.md",
			DestinationPath: "/out/my-agent.md",
			Action:          app.PlanWriteCreate,
			Status:          app.StatusFailed,
			Reason:          "frontmatter could not be parsed",
		}},
	}
}

func tsMixedResult() app.TransformHarnessResult {
	ok := tsTransformedResult().Files[0]
	ok.SourcePath, ok.DestinationPath, ok.Key = "/agents/ok-agent.md", "/out/ok-agent.md", "ok-agent"
	bad := tsFailedResult().Files[0]
	bad.SourcePath, bad.DestinationPath, bad.Key = "/agents/fail-agent.md", "/out/fail-agent.md", "fail-agent"
	return app.TransformHarnessResult{
		SourceHarnessID: tsSource,
		TargetHarnessID: tsTarget,
		Transformed:     1,
		Failed:          1,
		Files:           []app.TransformFileOutcome{ok, bad},
	}
}

func tsView(result app.TransformHarnessResult, styles screens.Styles) string {
	return screens.NewTransformSummaryScreen(result, 200, 40, styles).View()
}

// ---------------------------------------------------------------------------
// Content tests
// ---------------------------------------------------------------------------

func TestTransformSummaryScreen_View_ShowsSourceAndTargetHarness(t *testing.T) {
	view := collapseWhitespace(tsView(tsTransformedResult(), plainStyles()))

	if !strings.Contains(view, tsSource) || !strings.Contains(view, tsTarget) {
		t.Errorf("view does not contain both harness ids %q and %q:\n%s", tsSource, tsTarget, view)
	}
}

func TestTransformSummaryScreen_View_ShowsCounts(t *testing.T) {
	view := collapseWhitespace(tsView(tsMixedResult(), plainStyles()))

	counts := regexp.MustCompile(`(?i)transformed\W+1\W+skipped\W+0\W+failed\W+1`)
	if !counts.MatchString(view) {
		t.Errorf("view does not show the transformed/skipped/failed counts:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_ShowsSourceAndDestinationPaths(t *testing.T) {
	view := collapseWhitespace(tsView(tsTransformedResult(), plainStyles()))

	if !strings.Contains(view, "/agents/my-agent.md") || !strings.Contains(view, "/out/my-agent.md") {
		t.Errorf("view does not contain the source and destination paths:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_ShowsFailedFileWithReason(t *testing.T) {
	view := collapseWhitespace(tsView(tsFailedResult(), plainStyles()))

	if !strings.Contains(view, "/agents/my-agent.md") ||
		!strings.Contains(view, "frontmatter could not be parsed") {
		t.Errorf("view does not contain the failed file and its reason:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_ShowsExitHint(t *testing.T) {
	view := strings.ToLower(collapseWhitespace(tsView(tsTransformedResult(), plainStyles())))

	if !strings.Contains(view, "press q") && !strings.Contains(view, "enter to exit") {
		t.Errorf("view does not contain an exit hint:\n%s", view)
	}
}

func TestTransformSummaryScreen_View_DryRun_ShowsDryRunIndicator(t *testing.T) {
	result := tsTransformedResult()
	result.DryRun = true
	view := strings.ToLower(collapseWhitespace(tsView(result, plainStyles())))

	if !strings.Contains(view, "dry") {
		t.Errorf("dry-run result does not show a dry-run indicator:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Styling tests
// ---------------------------------------------------------------------------

func TestTransformSummaryScreen_View_FailedEntry_RendersInErrorStyleOnly(t *testing.T) {
	styles := coloredStyles()
	line := lineWith(tsView(tsFailedResult(), styles), "/agents/my-agent.md")

	if !strings.HasPrefix(line, rolePrefix(styles.Error)) {
		t.Errorf("failed entry line is not in the Error style: %q", line)
	}
}

func TestTransformSummaryScreen_View_TransformedEntry_RendersInSuccessStyle(t *testing.T) {
	styles := coloredStyles()
	line := lineWith(tsView(tsTransformedResult(), styles), "/agents/my-agent.md")

	if !strings.HasPrefix(line, rolePrefix(styles.Success)) {
		t.Errorf("transformed entry line is not in the Success style: %q", line)
	}
}

func TestTransformSummaryScreen_View_MixedResult_FailedEntryInErrorStyle(t *testing.T) {
	styles := coloredStyles()
	view := tsView(tsMixedResult(), styles)

	if line := lineWith(view, "/agents/fail-agent.md"); !strings.HasPrefix(line, rolePrefix(styles.Error)) {
		t.Errorf("failed entry is not in the Error style: %q", line)
	}
	if line := lineWith(view, "/agents/ok-agent.md"); !strings.HasPrefix(line, rolePrefix(styles.Success)) {
		t.Errorf("transformed entry is not in the Success style: %q", line)
	}
}

// The header line (the one naming the harnesses) takes the style of the overall outcome.
func TestTransformSummaryScreen_View_HeaderStyleFollowsOverallOutcome(t *testing.T) {
	styles := coloredStyles()
	skipped := tsTransformedResult()
	skipped.Skipped = 1
	cases := []struct {
		name   string
		result app.TransformHarnessResult
		want   lipgloss.Style
	}{
		{"failures", tsFailedResult(), styles.Error},
		{"skips only", skipped, styles.Warning},
		{"clean run", tsTransformedResult(), styles.Success},
	}
	for _, tc := range cases {
		line := lineWith(tsView(tc.result, styles), tsSource)
		if !strings.HasPrefix(line, rolePrefix(tc.want)) {
			t.Errorf("%s: header line is not in the expected style: %q", tc.name, line)
		}
	}
}

func TestTransformSummaryScreen_View_ExitHint_RendersInHelpStyle(t *testing.T) {
	styles := coloredStyles()
	line := lineWith(strings.ToLower(tsView(tsTransformedResult(), styles)), "press q")

	if !strings.HasPrefix(line, rolePrefix(styles.Help)) {
		t.Errorf("exit hint is not in the Help style: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Keyboard dismissal tests
// ---------------------------------------------------------------------------

func TestTransformSummaryScreen_Update_KeysDismiss(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"q":     {Type: tea.KeyRunes, Runes: []rune{'q'}},
		"enter": {Type: tea.KeyEnter},
		"esc":   {Type: tea.KeyEsc},
	}
	for name, key := range keys {
		s := screens.NewTransformSummaryScreen(tsTransformedResult(), 80, 40, plainStyles())
		if s.Done() {
			t.Fatalf("%s: screen is already done before any key press", name)
		}
		if !s.Update(key) || !s.Done() {
			t.Errorf("%s: key did not dismiss the screen", name)
		}
	}
}
