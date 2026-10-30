package tui

// The runs list and the run detail show, directly below their unpriced-model
// notice, the reason of each model left unpriced by a currency mismatch. A model
// unpriced for another reason shows no reason. No line exceeds the render width.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
	"mosaic-log-analyzer/internal/domain"
)

const (
	reasonGlobalFile = "/home/u/.config/MOSAIC/LogAnalyzer/pricing.yaml"
	reasonText       = "priced only in the global file (" + reasonGlobalFile + ") in USD; the project file's currency is EUR"
)

var (
	keyMismatchA = domain.PricingKey{Model: "claude-a"}
	keyMismatchB = domain.PricingKey{Provider: "github-copilot", Model: "gpt-b"}
	keyOther     = domain.PricingKey{Model: "claude-c"}
)

// reasonsReport has one run with three unpriced models; the first two were excluded
// by a currency mismatch (reason texts differ per model), the third is simply missing.
func reasonsReport(reasonA, reasonB string) domain.Report {
	report := navViewsReport(navViewsRun(runA, 700))
	unpriced := []domain.PricingKey{keyMismatchA, keyMismatchB, keyOther}
	report.Currency = domain.CurrencyEUR
	report.UnpricedModels = unpriced
	report.Runs[0].UnpricedModels = unpriced
	report.UnpricedReasons = []domain.UnpricedReason{
		{Key: keyMismatchA, Reason: reasonA},
		{Key: keyMismatchB, Reason: reasonB},
	}
	return report
}

func detailOf(report domain.Report, width int) Model {
	m := sizedModel(report, width, 60)
	return send(m, "enter")
}

// normalised joins the words of a view with single spaces, so a reason that wrapped
// over several lines can be found again.
func normalised(view string) string { return strings.Join(strings.Fields(view), " ") }

func linesOf(view string) []string { return strings.Split(view, "\n") }

func lineWith(view, needle string) (string, bool) {
	for _, l := range linesOf(view) {
		if strings.Contains(l, needle) {
			return l, true
		}
	}
	return "", false
}

func TestUnpricedNotices_RunsList_ShowsEachReasonBelowTheCountNotice(t *testing.T) {
	view := plainView(sizedModel(reasonsReport(reasonText, "reason-b"), 220, 60))

	if _, ok := lineWith(view, "Some models have no pricing"); !ok {
		t.Fatalf("the count notice stays as it is\n%s", view)
	}
	for _, want := range []string{"  claude-a: " + reasonText, "  gpt-b [github-copilot]: reason-b"} {
		if !strings.Contains(view, want) {
			t.Errorf("runs list must show the reason line %q\n%s", want, view)
		}
	}
}

func TestUnpricedNotices_RunsList_ShowsReasonsInTheOrderOfTheUnpricedList(t *testing.T) {
	view := plainView(sizedModel(reasonsReport("reason-a-text", "reason-b-text"), 220, 60))

	a, b := strings.Index(view, "reason-a-text"), strings.Index(view, "reason-b-text")

	if a < 0 || b < 0 || a > b {
		t.Errorf("reason of claude-a (at %d) must precede the reason of gpt-b (at %d)\n%s", a, b, view)
	}
}

func TestUnpricedNotices_RunsList_ModelWithoutReasonGetsNoReasonLine(t *testing.T) {
	view := plainView(sizedModel(reasonsReport("reason-a-text", "reason-b-text"), 220, 60))

	if _, ok := lineWith(view, "claude-c:"); ok {
		t.Errorf("claude-c is unpriced for another reason and must get no reason line\n%s", view)
	}
}

func TestUnpricedNotices_RunsList_NoReasonsMeansNoReasonLines(t *testing.T) {
	report := reasonsReport("", "")
	report.UnpricedReasons = nil

	view := plainView(sizedModel(report, 220, 60))

	if _, ok := lineWith(view, "claude-a:"); ok {
		t.Errorf("no reason was recorded, so no reason line may appear\n%s", view)
	}
	if _, ok := lineWith(view, "Some models have no pricing"); !ok {
		t.Errorf("the count notice stays as it is\n%s", view)
	}
}

func TestUnpricedNotices_RunDetail_ShowsEachReasonBelowTheUnpricedLine(t *testing.T) {
	view := plainView(detailOf(reasonsReport(reasonText, "reason-b"), 220))

	if _, ok := lineWith(view, "Unpriced models: claude-a, gpt-b [github-copilot], claude-c"); !ok {
		t.Fatalf("the unpriced line stays as it is\n%s", view)
	}
	for _, want := range []string{"  claude-a: " + reasonText, "  gpt-b [github-copilot]: reason-b"} {
		if !strings.Contains(view, want) {
			t.Errorf("run detail must show the reason line %q\n%s", want, view)
		}
	}
	if _, ok := lineWith(view, "claude-c:"); ok {
		t.Errorf("claude-c is unpriced for another reason and must get no reason line\n%s", view)
	}
}

func TestUnpricedNotices_RunDetail_ListsOnlyReasonsOfTheRunsOwnModels(t *testing.T) {
	report := reasonsReport("reason-a-text", "reason-b-text")
	report.Runs[0].UnpricedModels = []domain.PricingKey{keyMismatchA}

	view := plainView(detailOf(report, 220))

	if !strings.Contains(view, "reason-a-text") {
		t.Errorf("the run's own model must show its reason\n%s", view)
	}
	if strings.Contains(view, "reason-b-text") {
		t.Errorf("a model the run does not list as unpriced must show no reason\n%s", view)
	}
}

func TestUnpricedNotices_LongReason_WrapsWithinTheRenderWidth(t *testing.T) {
	for _, width := range []int{50, 60, 80} {
		for name, m := range map[string]Model{
			"runs list":  sizedModel(reasonsReport(reasonText, "reason-b"), width, 60),
			"run detail": detailOf(reasonsReport(reasonText, "reason-b"), width),
		} {
			view := plainView(m)

			assertViewWithin(t, view, width, 60)
			flat := normalised(view)
			for _, word := range strings.Fields(reasonText) {
				if textwidth.Of(word) > width-4 {
					continue // a word wider than a continuation line may be cut
				}
				if !strings.Contains(flat, word) {
					t.Errorf("%s at width %d: the word %q of the reason was dropped\n%s", name, width, word, view)
				}
			}
		}
	}
}

func TestUnpricedNotices_WrappedReason_ContinuesOnLinesIndentedFourSpaces(t *testing.T) {
	view := plainView(detailOf(reasonsReport(reasonText, "reason-b"), 60))

	lines := linesOf(view)
	margin, start := -1, -1
	for i, l := range lines {
		if at := strings.Index(l, "Unpriced models:"); at >= 0 {
			margin = at
		}
		if strings.Contains(l, "claude-a: priced only") {
			start = i
		}
	}
	if margin < 0 || start < 0 || start+1 >= len(lines) {
		t.Fatalf("the unpriced line or the reason block of claude-a was not found\n%s", view)
	}
	first, next := lines[start], lines[start+1]
	if !strings.HasPrefix(first, strings.Repeat(" ", margin+2)+"claude-a:") {
		t.Errorf("the reason block is indented two spaces; got %q", first)
	}
	cont := strings.Repeat(" ", margin+4)
	if !strings.HasPrefix(next, cont) || strings.HasPrefix(next, cont+" ") || strings.TrimSpace(next) == "" {
		t.Errorf("the continuation line is indented four spaces; got %q", next)
	}
}

func TestUnpricedNotices_SingleWordWiderThanTheLine_IsCutNotOverflowed(t *testing.T) {
	longPath := "/" + strings.Repeat("deeply-nested-directory/", 8) + "pricing.yaml"
	reason := "priced only in the global file (" + longPath + ") in USD; the project file's currency is EUR"
	const width = 50

	for name, m := range map[string]Model{
		"runs list":  sizedModel(reasonsReport(reason, "reason-b"), width, 60),
		"run detail": detailOf(reasonsReport(reason, "reason-b"), width),
	} {
		view := plainView(m)

		for _, l := range linesOf(view) {
			if textwidth.Of(l) > width {
				t.Errorf("%s: line %q is %d wide, over the render width %d", name, l, textwidth.Of(l), width)
			}
		}
		if !strings.Contains(normalised(view), "currency is EUR") {
			t.Errorf("%s: text after the cut word must still be shown\n%s", name, view)
		}
	}
}

func TestUnpricedNotices_AsciiMode_TheCutUsesTheAsciiEllipsisAndStaysInsideTheWidth(t *testing.T) {
	asciiMode(t)
	longPath := "/" + strings.Repeat("deeply-nested-directory/", 8) + "pricing.yaml"
	reason := "priced only in the global file (" + longPath + ") in USD; the project file's currency is EUR"
	const width = 50

	view := plainView(detailOf(reasonsReport(reason, "reason-b"), width))

	assertViewWithin(t, view, width, 60)
	if strings.Contains(view, "…") {
		t.Errorf("ASCII mode must not draw the unicode ellipsis\n%s", view)
	}
	if !strings.Contains(view, "claude-a: priced only") || !strings.Contains(view, "...") {
		t.Errorf("the over-wide word must be cut with the ASCII ellipsis\n%s", view)
	}
}

func TestUnpricedNotices_ReasonBlocks_UseTheWarningStyleOfTheNotice(t *testing.T) {
	colourOn(t)
	view := detailOf(reasonsReport("reason-a-text", "reason-b-text"), 220).View()

	warning := rolePrefix(tuicommon.RoleWarning)
	for _, needle := range []string{"Unpriced models: ", "reason-a-text", "reason-b-text"} {
		line := rawLineWith(t, view, needle)
		if !strings.Contains(line, warning) {
			t.Errorf("the line with %q must be drawn in the warning role; got %q", needle, line)
		}
	}
}
