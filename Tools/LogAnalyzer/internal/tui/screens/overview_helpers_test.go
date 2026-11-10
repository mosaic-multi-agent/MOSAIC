package screens_test

// Fixture and helpers for the Overview tests: a run that carries every new
// run-level analysis with distinct figures, the two versions of the screen
// (visual and figures-only) and text lookups that ignore spacing.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// asciiOn switches the shared render mode to colour off with ASCII glyphs for the
// duration of the test.
func asciiOn(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphASCII})
	t.Cleanup(restore)
}

// overviewTallH is tall enough for the whole Overview body of analysesRun.
const overviewTallH = 150

// overviewVersion opens the Overview in one of its two versions.
type overviewVersion struct {
	name string
	open func(run domain.RunReport, w, h int) screens.Screen
}

func (v overviewVersion) view(run domain.RunReport, h int) string {
	return v.open(run, wideW, h).View(wideW, h)
}

// overviewVersions are the visual version (the default) and the figures-only one.
func overviewVersions() []overviewVersion {
	return []overviewVersion{
		{"visual", func(run domain.RunReport, w, h int) screens.Screen {
			return screens.NewOverviewScreen(run, w, h, paletteStyles())
		}},
		{"figures only", func(run domain.RunReport, w, h int) screens.Screen {
			return press(screens.NewOverviewScreen(run, w, h, paletteStyles()), screens.KeyToggleView)
		}},
	}
}

func overviewView(run domain.RunReport, h int) string { return overviewVersions()[0].view(run, h) }

func overviewFiguresView(run domain.RunReport, h int) string {
	return overviewVersions()[1].view(run, h)
}

// analysesRun is a priced run with every run-level analysis filled in. Run cost
// split 40% / 60%; tokens 1000 / 3000 / 500 / 200 (21% / 64% / 11% / 4%); 27
// dispatches ending 17 SUCCESS (63%), 6 other (22%), 4 open or without status
// (15%); wall time 25m waiting (21%), 1h06m subagents (55%), 19m orchestrator
// only (16%), 10m between sessions (8%); three open-ended waits.
func analysesRun() domain.RunReport {
	run := overviewRun()
	run.Overview.Totals.Tokens.Reasoning = domain.Tokens(123)
	run.Totals = run.Overview.Totals
	run.Overview.Subagents.Invocations = 27
	run.Overview.Outcomes = domain.OutcomeSplit{
		Success:     domain.GroupFigures{Invocations: 17},
		OtherStatus: domain.GroupFigures{Invocations: 6},
		OpenOrNone:  domain.GroupFigures{Invocations: 4},
	}
	run.Overview.ByModel = []domain.ModelFigures{
		{Model: "model-a", Priced: true, Requests: 3, Totals: modelTotals(650, 100_000)},
		{Model: "model-b", Priced: true, Requests: 2, Totals: modelTotals(275, 900_000)},
		{Model: "", Priced: true, Requests: 1, Totals: modelTotals(75, 400_000)},
	}
	run.Cache = domain.CacheFigures{
		HitRatio:          run.Overview.CacheHitRatio,
		WriteTokens:       domain.Tokens(500),
		WriteCost:         known(333),
		NetBenefit:        known(1234),
		UnreadWriteTokens: domain.Tokens(777),
		UnreadWriteCost:   known(456),
		Complete:          true,
	}
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	run.WallTime = domain.WallTimeSplit{
		Span:             domain.TimeInterval{Start: start, End: start.Add(2 * time.Hour)},
		HasSpan:          true,
		Waiting:          25 * time.Minute,
		Subagents:        66 * time.Minute,
		OrchestratorOnly: 19 * time.Minute,
		BetweenSessions:  10 * time.Minute,
		CountedTools: []domain.CountedTool{
			{Tool: "AskUserQuestion", Harness: "claude-code", Calls: 5},
			{Tool: "mcp__contact-user__ask_user_questions", Harness: "claude-code", Calls: 2},
		},
		OpenWaits: []domain.OpenWait{
			{Actor: domain.Orchestrator(), At: start.Add(time.Minute), Kind: domain.OpenWaitToolCall, Tool: "AskUserQuestion"},
			{Actor: domain.Orchestrator(), At: start.Add(2 * time.Minute), Kind: domain.OpenWaitPermission},
			{Actor: domain.Orchestrator(), At: start.Add(3 * time.Minute), Kind: domain.OpenWaitIdlePrompt},
		},
		MissingContactToolsHint: true,
		UnconfiguredTools:       []string{"mcp__other__ask"},
	}
	return run
}

// modelTotals are the totals of one by-model row: money and input tokens.
func modelTotals(cents, inputTokens int64) domain.Totals {
	return domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(inputTokens)},
		Money:  domain.CategoryMoney{Total: known(cents), Complete: true},
	}
}

// withCacheHit sets the cache hit ratio wherever the run carries it.
func withCacheHit(run domain.RunReport, numerator, denominator int64) domain.RunReport {
	r := domain.Ratio{Numerator: numerator, Denominator: denominator}
	run.Overview.CacheHitRatio = r
	run.Cache.HitRatio = r
	return run
}

// scrollThrough renders the screen at every scroll position reached by pressing
// down steps times and returns all the views joined.
func scrollThrough(s screens.Screen, width, height, steps int) string {
	views := []string{s.View(width, height)}
	for i := 0; i < steps; i++ {
		s = press(s, "down")
		views = append(views, s.View(width, height))
	}
	return strings.Join(views, "\n")
}

// assertMatches fails unless every pattern matches the styling-free view.
func assertMatches(t *testing.T, view string, patterns ...string) {
	t.Helper()
	plainView := stripStyling(view)
	for _, p := range patterns {
		if !regexp.MustCompile(p).MatchString(plainView) {
			t.Errorf("view has no text matching %q\n%s", p, plainView)
		}
	}
}

// firstLineMatching is the first styling-free line matching the pattern, or "".
func firstLineMatching(view, pattern string) string {
	re := regexp.MustCompile(pattern)
	for _, l := range strings.Split(stripStyling(view), "\n") {
		if re.MatchString(l) {
			return l
		}
	}
	return ""
}

// rawLineMatching is the first line whose styling-free text matches the pattern;
// the line keeps its escape sequences.
func rawLineMatching(t *testing.T, view, pattern string) string {
	t.Helper()
	re := regexp.MustCompile(pattern)
	for _, l := range strings.Split(view, "\n") {
		if re.MatchString(stripStyling(l)) {
			return l
		}
	}
	t.Fatalf("no line matches %q\n%s", pattern, stripStyling(view))
	return ""
}

// windowAround is the first line matching the pattern with n lines on each side,
// joined; "" when no line matches.
func windowAround(view, pattern string, n int) string {
	re := regexp.MustCompile(pattern)
	lines := strings.Split(stripStyling(view), "\n")
	for i, l := range lines {
		if re.MatchString(l) {
			from, to := max(0, i-n), min(len(lines), i+n+1)
			return strings.Join(lines[from:to], "\n")
		}
	}
	return ""
}

// patternLine is the index of the first styling-free line matching the pattern, or -1.
func patternLine(view, pattern string) int {
	re := regexp.MustCompile(pattern)
	for i, l := range strings.Split(stripStyling(view), "\n") {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

// textAfterWord is the text of the first line containing word (any case), from
// the word to the end of the line; "" when no line has it.
func textAfterWord(view, word string) string {
	for _, l := range strings.Split(stripStyling(view), "\n") {
		if i := strings.Index(strings.ToLower(l), strings.ToLower(word)); i >= 0 {
			return l[i:]
		}
	}
	return ""
}
