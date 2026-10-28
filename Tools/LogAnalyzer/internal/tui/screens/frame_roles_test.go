package screens_test

// The frame draws its header badges and tab bar in the palette roles the mapping
// assigns, and keeps its fixed layout at the smallest heights. Colour is forced
// on here (true colour) so a style difference shows up in the rendered text.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// colourOn forces colour output for the duration of the test.
func colourOn(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: true, Glyphs: tuicommon.GlyphUnicode})
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		restore()
	})
}

// styledBadge is the badge the frame is expected to draw for text in role.
func styledBadge(text string, role tuicommon.StyleRole) string {
	return chrome.Badge(text, role, 200, tuicommon.PaletteTheme())
}

// rawHeader renders the frame with colour on and returns its first line, escape
// sequences included.
func rawHeader(t *testing.T, run domain.RunReport, q screens.RunQuality) string {
	t.Helper()
	colourOn(t)
	view := screens.RenderFrame(screens.Frame{Run: run, Quality: q, Active: screens.ScreenOverview},
		bodyLines(3), 200, 20, paletteStyles())
	return strings.SplitN(view, "\n", 2)[0]
}

func TestRenderFrame_FindingBadgeRoleIsWarningOrBadWhenDataIsLost(t *testing.T) {
	cases := []struct {
		name    string
		quality screens.RunQuality
		role    tuicommon.StyleRole
		other   tuicommon.StyleRole
	}{
		{"findings without data loss", screens.RunQuality{Findings: 5}, warning, bad},
		{"findings with data loss", screens.RunQuality{Findings: 5, DataLoss: true}, bad, warning},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			header := rawHeader(t, framedRun(), c.quality)

			assertContains(t, header, styledBadge("5 findings", c.role))
			assertNotContains(t, header, styledBadge("5 findings", c.other))
		})
	}
}

func TestRenderFrame_CostBadgeRoleFollowsTheMoneyMapping(t *testing.T) {
	cases := []struct {
		name  string
		total domain.CategoryMoney
	}{
		{"complete known total", domain.CategoryMoney{Total: known(1000), Complete: true}},
		{"partial known total", domain.CategoryMoney{Total: known(500), Complete: false}},
		{"unpriced total", domain.CategoryMoney{Total: domain.UnpricedMoney()}},
		{"absent total", domain.CategoryMoney{Total: domain.NoMoneyData()}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := framedRun()
			run.Totals.Money = c.total

			header := rawHeader(t, run, screens.RunQuality{})

			role := screens.MoneyRole(c.total.Total, c.total.Complete)
			assertContains(t, header, styledBadge(screens.FormatTotal(c.total), role))
		})
	}
}

func TestRenderFrame_ProvisionalMarkerIsDrawnInTheWarningRole(t *testing.T) {
	run := framedRun()
	run.Provisional = true

	header := rawHeader(t, run, screens.RunQuality{})

	plain := tuicommon.PaletteTheme().Style(warning).Render(screens.ProvisionalLabel)
	if !strings.Contains(header, plain) && !strings.Contains(header, styledBadge(screens.ProvisionalLabel, warning)) {
		t.Errorf("the provisional marker must be drawn in the warning role: %q", header)
	}
}

func TestRenderFrame_ActiveTabIsDrawnInTheSelectedRole(t *testing.T) {
	colourOn(t)
	view := screens.RenderFrame(screens.Frame{Run: framedRun(), Active: screens.ScreenOverview},
		bodyLines(3), 200, 20, paletteStyles())
	rows := strings.Split(view, "\n")
	if len(rows) < 2 {
		t.Fatalf("frame has no tab bar line: %q", view)
	}
	tabs := rows[1]

	assertContains(t, tabs, tuicommon.PaletteTheme().Style(tuicommon.RoleSelected).Render("[2 Overview]"))
}

func TestPerRunViews_DrawTheHeaderBadgesInTheirRoles(t *testing.T) {
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			colourOn(t)
			s := fv.make(fv.run, 200, 40, paletteStyles())
			qs, ok := s.(qualitySetter)
			if !ok {
				t.Fatalf("%s must accept the findings of its run (SetRunQuality)", fv.id)
			}
			qs.SetRunQuality(screens.RunQuality{Findings: 2, DataLoss: true})

			header := strings.SplitN(s.View(200, 40), "\n", 2)[0]

			m := fv.run.Totals.Money
			assertContains(t, header, styledBadge(screens.FormatTotal(m), screens.MoneyRole(m.Total, m.Complete)))
			assertContains(t, header, styledBadge("2 findings", bad))
		})
	}
}

func TestRenderFrame_ColourOffFromNoColorEnvironmentLeavesNoEscapeSequences(t *testing.T) {
	colourOn(t)
	mode := tuicommon.DetectRenderMode(func(k string) (string, bool) { return "1", k == "NO_COLOR" }, "linux")
	restore := tuicommon.SetRenderMode(mode)
	defer restore()
	run := framedRun()
	run.Provisional = true

	view := screens.RenderFrame(screens.Frame{Run: run, Active: screens.ScreenTools, Quality: screens.RunQuality{Findings: 2, DataLoss: true}},
		bodyLines(3), 120, 20, paletteStyles())

	if view != stripStyling(view) {
		t.Errorf("with NO_COLOR set the frame must contain no escape sequences: %q", view)
	}
	assertContains(t, view, "2 findings", screens.ProvisionalLabel)
}

func TestRenderFrame_AtHeightFourOnlyTheChromeIsDrawnInFixedOrder(t *testing.T) {
	lines := renderedFrame(t, screens.Frame{Run: framedRun(), Active: screens.ScreenTools}, bodyLines(5), 100, 4)

	if len(lines) != 4 {
		t.Fatalf("frame has %d lines, want 4\n%s", len(lines), strings.Join(lines, "\n"))
	}
	assertContains(t, lines[0], framedRun().Run.Label())
	assertContains(t, lines[1], "[5 Tools]")
	assertSeparator(t, lines[2])
	assertContains(t, lines[3], "? help")
	assertNotContains(t, strings.Join(lines, "\n"), "body-01")
}

func TestRenderFrame_AtHeightFiveTheFirstBodyLineSitsBetweenSeparatorAndHelp(t *testing.T) {
	lines := renderedFrame(t, screens.Frame{Run: framedRun(), Active: screens.ScreenTools}, bodyLines(5), 100, 5)

	if len(lines) != 5 {
		t.Fatalf("frame has %d lines, want 5\n%s", len(lines), strings.Join(lines, "\n"))
	}
	assertSeparator(t, lines[2])
	assertContains(t, lines[3], "body-01")
	assertContains(t, lines[4], "? help")
	assertNotContains(t, strings.Join(lines, "\n"), "body-02")
}

// assertSeparator fails unless the line is a non-empty rule: no letters or digits.
func assertSeparator(t *testing.T, line string) {
	t.Helper()
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		t.Errorf("separator line is empty")
	}
	if strings.ContainsAny(trimmed, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
		t.Errorf("separator line must be a rule, got %q", line)
	}
}
