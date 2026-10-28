package screens_test

// The token, duration and orchestrator net-benefit cells of the Dispatches view.
// Each dispatch has its own token sum and duration, so a cell found on a row can
// only have come from that dispatch.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestDispatches_EachRowShowsItsTokenSumAndDuration(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	for _, c := range []struct {
		id     string
		tokens string
		secs   string
	}{
		{"builder#1", "1,000", "10s"},
		{longInstance, "5,000", "50s"},
		{"reviewer#1", "9,000", "30s"},
		{"builder#2", "2,000", "20s"},
		{"builder#3", "3,000", "40s"},
	} {
		assertFields(t, rowFor(t, view, c.id), c.tokens, c.secs)
	}
}

func TestDispatches_TokenCellIsTheSumOfCategoriesWithoutReasoning(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	// builder#1: input 400, cache read 300, cache write 200, output 100, reasoning 50.
	row := rowFor(t, view, "builder#1")
	assertFields(t, row, "1,000")
	assertNoFields(t, row, "400", "300", "200", "100", "1,050")
	// the long-named dispatch: input 2,000, cache read 1,500, cache write 1,000, output 500, reasoning 250.
	row = rowFor(t, view, longInstance)
	assertFields(t, row, "5,000")
	assertNoFields(t, row, "2,000", "5,250")
}

func TestDispatches_TokenAndDurationCellsOfOneDispatchNeverAppearOnAnotherRow(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertNoFields(t, rowFor(t, view, "builder#1"), "5,000", "9,000", "2,000", "50s", "30s", "20s")
	assertNoFields(t, rowFor(t, view, longInstance), "1,000", "9,000", "10s", "30s")
}

// assertMarkerUnderDurationHeader fails unless the row shows the absent marker in
// the column headed "duration", so a marker from another cell cannot satisfy it.
// The marker may sit anywhere under the header text or just right of it (right
// aligned in a column wider than the header).
func assertMarkerUnderDurationHeader(t *testing.T, view, row string) {
	t.Helper()
	var header string
	for _, l := range strings.Split(stripStyling(view), "\n") {
		if strings.Contains(strings.ToLower(l), "duration") && !strings.Contains(l, "sort:") {
			header = strings.ToLower(l)
			break
		}
	}
	if header == "" {
		t.Fatalf("the table has no column headed duration\n%s", stripStyling(view))
	}
	start := len([]rune(header[:strings.Index(header, "duration")]))
	end := start + len("duration")
	for i, r := range []rune(stripStyling(row)) {
		if r == '—' && i >= start && i < end+4 {
			return
		}
	}
	t.Errorf("no absent marker under the duration header (columns %d-%d): %q", start, end, row)
}

func TestDispatches_ADispatchWithoutADurationShowsTheMarkerNotATime(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)
	row := rowFor(t, view, "builder#4")

	assertMarkerUnderDurationHeader(t, view, row)
	if regexp.MustCompile(`[0-9](h|m|s)\b`).MatchString(row) {
		t.Errorf("a dispatch with no start and end must show no duration: %q", row)
	}
}

func TestDispatches_NetCacheBenefitIsAbsentWhenUnpricedAndSignedWhenNegative(t *testing.T) {
	colourOff(t)
	base := dispatchesView(dispatchRun(), tableW, tallH)
	unpriced := dispatchesView(withDispatch(dispatchRun(), "builder#1", func(d *domain.Dispatch) {
		d.Cache.NetBenefit = domain.NoMoneyData()
	}), tableW, tallH)
	negative := dispatchesView(withDispatch(dispatchRun(), "builder#1", func(d *domain.Dispatch) {
		d.Cache.NetBenefit = known(-40)
	}), tableW, tallH)

	if got := countOf(rowFor(t, unpriced, "builder#1"), "—") - countOf(rowFor(t, base, "builder#1"), "—"); got != 1 {
		t.Errorf("an unpriced net benefit shows the absent marker, %d more markers than a priced one", got)
	}
	assertNotContains(t, rowFor(t, unpriced, "builder#1"), "$1.11", "$0.00")
	if !regexp.MustCompile(`(-\s?\$0\.40|\$-0\.40)`).MatchString(rowFor(t, negative, "builder#1")) {
		t.Errorf("a negative net benefit shows its sign: %q", rowFor(t, negative, "builder#1"))
	}
}

func TestDispatches_OrchestratorRowShowsItsNetCacheBenefit(t *testing.T) {
	colourOff(t)

	row := rowFor(t, dispatchesView(dispatchRun(), tableW, tallH), "orchestrator")

	assertContains(t, row, "$0.17")
	assertNotContains(t, row, "$1.11", "$0.46", "$0.12", "$0.05")
}
