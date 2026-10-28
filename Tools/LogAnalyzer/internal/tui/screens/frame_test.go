package screens_test

// Behaviour of the frame shared by every per-run view: header with run label and
// summary badges, tab bar, short help line, and the fixed line budget.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func bodyLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("body-%02d", i+1)
	}
	return out
}

// renderedFrame renders a frame for the run with colour off and returns its lines.
func renderedFrame(t *testing.T, f screens.Frame, body []string, width, height int) []string {
	t.Helper()
	colourOff(t)
	return framedLines(t, screens.RenderFrame(f, body, width, height, paletteStyles()))
}

func frameHeader(t *testing.T, run domain.RunReport, q screens.RunQuality) string {
	t.Helper()
	lines := renderedFrame(t, screens.Frame{Run: run, Quality: q, Active: screens.ScreenOverview}, bodyLines(3), 120, 20)
	if len(lines) == 0 {
		t.Fatal("frame rendered nothing")
	}
	return lines[0]
}

func TestFrameBodyHeight_IsHeightMinusChromeNeverNegative(t *testing.T) {
	cases := map[int]int{-5: 0, 0: 0, 3: 0, 4: 0, 5: 1, 24: 20}
	for height, want := range cases {
		if got := screens.FrameBodyHeight(height); got != want {
			t.Errorf("FrameBodyHeight(%d) = %d, want %d", height, got, want)
		}
	}
	if screens.FrameChromeLines != 4 {
		t.Errorf("FrameChromeLines = %d, want 4 (header, tab bar, separator, help)", screens.FrameChromeLines)
	}
}

func TestRenderFrame_HeaderShowsRunLabel(t *testing.T) {
	header := frameHeader(t, framedRun(), screens.RunQuality{})

	assertContains(t, header, framedRun().Run.Label())
	assertNotContains(t, header, screens.ProvisionalLabel)
}

func TestRenderFrame_HeaderMarksProvisionalRunInProgress(t *testing.T) {
	run := framedRun()
	run.Provisional = true

	header := frameHeader(t, run, screens.RunQuality{})

	assertContains(t, header, run.Run.Label(), screens.ProvisionalLabel)
}

func TestRenderFrame_HeaderCostBadge(t *testing.T) {
	cases := []struct {
		name    string
		total   domain.CategoryMoney
		want    []string
		notWant []string
	}{
		{"complete known total", domain.CategoryMoney{Total: known(1000), Complete: true},
			[]string{"$10.00"}, []string{"(partial)", screens.UnpricedMarker}},
		{"partial known total is marked", domain.CategoryMoney{Total: known(500), Complete: false},
			[]string{"$5.00 (partial)"}, nil},
		{"unpriced total is never zero dollars", domain.CategoryMoney{Total: domain.UnpricedMoney()},
			[]string{screens.UnpricedMarker}, []string{"$0.00"}},
		{"absent total shows the absent marker, not zero", domain.CategoryMoney{Total: domain.NoMoneyData()},
			[]string{screens.AbsentMarker}, []string{"$"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := framedRun()
			run.Totals.Money = c.total

			header := frameHeader(t, run, screens.RunQuality{})

			assertContains(t, header, c.want...)
			assertNotContains(t, header, c.notWant...)
		})
	}
}

func TestRenderFrame_HeaderShowsRunDuration(t *testing.T) {
	header := frameHeader(t, framedRun(), screens.RunQuality{})

	assertContains(t, header, screens.FormatDuration(framedRunSpan, true))
}

func TestRenderFrame_HeaderShowsAbsentMarkerWhenRunHasNoTimeAxis(t *testing.T) {
	run := framedRun()
	run.Timeline.HasAxis = false

	header := frameHeader(t, run, screens.RunQuality{})

	assertContains(t, header, screens.AbsentMarker)
	if zeroDurationRE.MatchString(header) {
		t.Errorf("an absent duration must not be shown as %s: %q", screens.FormatDuration(0, true), header)
	}
}

// zeroDurationRE matches a standalone zero duration, not the tail of "10s" or "1m0s".
var zeroDurationRE = regexp.MustCompile(`(^|[^0-9a-z])0s`)

func TestRenderFrame_HeaderShowsInvocationCount(t *testing.T) {
	header := frameHeader(t, framedRun(), screens.RunQuality{})

	assertContains(t, header, "3 invocations")
}

func TestRenderFrame_FindingBadgeAppearsOnlyWhenRunHasFindings(t *testing.T) {
	cases := []struct {
		name    string
		quality screens.RunQuality
		want    string
	}{
		{"findings", screens.RunQuality{Findings: 4}, "4 findings"},
		{"data loss findings", screens.RunQuality{Findings: 5, DataLoss: true}, "5 findings"},
	}
	for _, c := range cases {
		assertContains(t, frameHeader(t, framedRun(), c.quality), c.want)
	}
	assertNotContains(t, strings.ToLower(frameHeader(t, framedRun(), screens.RunQuality{})), "finding")
}

func TestRenderFrame_TabBarListsPerRunViewsAndMarksTheCurrentOneWithoutColour(t *testing.T) {
	all := []string{"1 Dispatches", "2 Overview", "3 By type", "4 By status", "5 Tools", "6 Timeline"}
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			lines := renderedFrame(t, screens.Frame{Run: framedRun(), Active: fv.id}, bodyLines(2), 140, 20)
			tabs := lines[1]

			assertContains(t, tabs, all...)
			assertContains(t, tabs, "["+fv.tab+"]")
			if n := strings.Count(tabs, "["); n != 1 {
				t.Errorf("exactly one tab must be highlighted, tab bar has %d brackets: %q", n, tabs)
			}
		})
	}
}

func TestRenderFrame_TabBarKeepsCurrentViewVisibleWhenNarrow(t *testing.T) {
	for _, fv := range allFrameViews() {
		lines := renderedFrame(t, screens.Frame{Run: framedRun(), Active: fv.id}, bodyLines(2), 30, 20)

		assertContains(t, lines[1], "["+fv.key+" ")
	}
}

func TestRenderFrame_ShortHelpHasOwnKeysThenQuestionMarkHint(t *testing.T) {
	f := screens.Frame{
		Run: framedRun(), Active: screens.ScreenTools,
		Keys: []tuicommon.HelpEntry{{Key: "enter", Desc: "drill down"}, {Key: "j/k", Desc: "scroll"}},
	}

	lines := renderedFrame(t, f, bodyLines(3), 120, 20)
	help := lines[len(lines)-1]

	assertContains(t, help, "enter", "drill down", "j/k", "scroll", "? help")
	if strings.Index(help, "scroll") > strings.Index(help, "? help") {
		t.Errorf("own keys must come before the ? hint: %q", help)
	}
	assertNotContains(t, strings.ToLower(help), "export", "dispatches", "timeline")
}

func TestRenderFrame_ShortHelpWithoutKeysStillHintsAtHelp(t *testing.T) {
	lines := renderedFrame(t, screens.Frame{Run: framedRun(), Active: screens.ScreenTools}, bodyLines(3), 120, 20)

	assertContains(t, lines[len(lines)-1], "? help")
}

func TestRenderFrame_ScrollHintPrecedesTheHelpHint(t *testing.T) {
	f := screens.Frame{Run: framedRun(), Active: screens.ScreenTools, Hint: "5-9 of 30"}

	lines := renderedFrame(t, f, bodyLines(3), 120, 20)
	help := lines[len(lines)-1]

	assertContains(t, help, "5-9 of 30", "? help")
	if strings.Index(help, "5-9 of 30") > strings.Index(help, "? help") {
		t.Errorf("scroll hint must precede the ? hint: %q", help)
	}
	noHint := renderedFrame(t, screens.Frame{Run: framedRun(), Active: screens.ScreenTools}, bodyLines(3), 120, 20)
	if scrollHintRE.MatchString(noHint[len(noHint)-1]) {
		t.Errorf("no scroll hint expected without one: %q", noHint[len(noHint)-1])
	}
}

func TestRenderFrame_BodyIsCutToTheBodyHeightBetweenHeaderAndHelp(t *testing.T) {
	f := screens.Frame{Run: framedRun(), Active: screens.ScreenOverview}

	lines := renderedFrame(t, f, bodyLines(30), 120, 10)

	if len(lines) != 10 {
		t.Fatalf("frame has %d lines, want exactly 10\n%s", len(lines), strings.Join(lines, "\n"))
	}
	text := strings.Join(lines, "\n")
	assertContains(t, text, "body-01", "body-06")
	assertNotContains(t, text, "body-07")
	assertContains(t, lines[0], framedRun().Run.Label())
	assertContains(t, lines[len(lines)-1], "? help")
}

func TestRenderFrame_ShortBodyKeepsOnlyItsOwnLines(t *testing.T) {
	f := screens.Frame{Run: framedRun(), Active: screens.ScreenOverview}

	lines := renderedFrame(t, f, bodyLines(2), 120, 40)

	if len(lines) != screens.FrameChromeLines+2 {
		t.Fatalf("frame has %d lines, want %d (chrome plus the 2 body lines)\n%s",
			len(lines), screens.FrameChromeLines+2, strings.Join(lines, "\n"))
	}
	first, second := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "body-01") {
			first = i
		}
		if strings.Contains(l, "body-02") {
			second = i
		}
	}
	if first < 2 || second != first+1 || second >= len(lines)-1 {
		t.Errorf("body lines must follow header and tab bar, in order, before the help line (at %d, %d of %d)",
			first, second, len(lines))
	}
}

func TestRenderFrame_NeverExceedsWidthOrHeight(t *testing.T) {
	run := framedRun()
	run.Provisional = true
	run.Totals.Money.Complete = false
	wide := make([]string, 30)
	for i := range wide {
		wide[i] = strings.Repeat("x", 200)
	}
	f := screens.Frame{
		Run: run, Active: screens.ScreenTimeline, Quality: screens.RunQuality{Findings: 12, DataLoss: true},
		Keys: []tuicommon.HelpEntry{{Key: "enter", Desc: "drill down into the selected row"}}, Hint: "1-9 of 30",
	}
	for _, width := range []int{120, 80, 60, 40, 30, 20, 10} {
		for _, height := range []int{0, 1, 2, 3, 4, 5, 8, 24} {
			colourOff(t)
			view := screens.RenderFrame(f, wide, width, height, paletteStyles())

			assertWithinBounds(t, view, width, height)
			if want := min(height, screens.FrameChromeLines+len(wide)); len(viewLines(view)) != want {
				t.Errorf("%dx%d: frame has %d lines, want exactly %d", width, height, len(viewLines(view)), want)
			}
		}
	}
}

func TestRunQualityOf_CountsOnlyFindingsOfTheRun(t *testing.T) {
	runA, runB := domain.NamedRun("20260101T120000Z-aaaa"), domain.NamedRun("20260202T120000Z-bbbb")
	q := domain.NewQualitySummary([]domain.Finding{
		{Kind: domain.FindingMalformedLine, Run: runA, Path: "a1"},
		{Kind: domain.FindingMissingTokenUsage, Run: runA, Path: "a2"},
		{Kind: domain.FindingMissingModel, Run: runB, Path: "b1"},
		{Kind: domain.FindingUnreadableFile, Run: domain.UnattributableRun(), Path: "u1"},
	})

	cases := []struct {
		name string
		run  domain.RunRef
		want screens.RunQuality
	}{
		{"run with a data-loss finding", runA, screens.RunQuality{Findings: 2, DataLoss: true}},
		{"run with only a warning finding", runB, screens.RunQuality{Findings: 1, DataLoss: false}},
		{"unattributable bucket", domain.UnattributableRun(), screens.RunQuality{Findings: 1, DataLoss: true}},
		{"run without findings", domain.NamedRun("20260303T120000Z-cccc"), screens.RunQuality{}},
	}
	for _, c := range cases {
		if got := screens.RunQualityOf(q, c.run); got != c.want {
			t.Errorf("%s: RunQualityOf = %+v, want %+v", c.name, got, c.want)
		}
	}
}
