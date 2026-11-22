package screens_test

// harness_experimental_test.go verifies how the harness picker presents a harness whose
// descriptor declares it experimental: it is still offered and selectable, its list label
// carries the experimental marker, its reason is shown in the detail pane when highlighted,
// and the label column stays aligned. Non-experimental harnesses look exactly as before.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

const experimentalTestReason = "agents deploy to .codex/agents but MOSAIC orchestration is not supported"

func experimentalPickerFixtures() []domain.HarnessRef {
	return []domain.HarnessRef{
		{ID: "stable", DisplayName: "Stable", Tier: domain.TierBuiltin, Usable: true},
		{ID: "exp", DisplayName: "Tool With A Long Name", Tier: domain.TierBuiltin, Usable: true,
			ExperimentalReason: experimentalTestReason},
		{ID: "other", DisplayName: "Other", Tier: domain.TierDescriptor, Usable: true},
	}
}

func rowOf(t *testing.T, rows []string, label string) string {
	t.Helper()
	for _, r := range rows {
		if strings.HasPrefix(string([]rune(r)[2:]), label) {
			return r
		}
	}
	t.Fatalf("no row for %q in %q", label, rows)
	return ""
}

// TestHarnessScreen_ExperimentalHarness_ListLabelCarriesMarkerAfterBadge verifies the marker
// follows the tier badge on the experimental row.
func TestHarnessScreen_ExperimentalHarness_ListLabelCarriesMarkerAfterBadge(t *testing.T) {
	rows := labelRows(t, experimentalPickerFixtures())

	row := rowOf(t, rows, "Tool With A Long Name")

	want := "[built-in]  [" + domain.ExperimentalLabel + "]"
	if !strings.Contains(row, want) {
		t.Errorf("experimental row lacks %q: %q", want, row)
	}
}

// TestHarnessScreen_NonExperimentalHarness_RowsCarryNoMarker verifies other rows are unchanged.
func TestHarnessScreen_NonExperimentalHarness_RowsCarryNoMarker(t *testing.T) {
	rows := labelRows(t, experimentalPickerFixtures())

	for _, label := range []string{"Stable", "Other"} {
		row := rowOf(t, rows, label)
		if strings.Contains(row, "["+domain.ExperimentalLabel+"]") {
			t.Errorf("non-experimental row %q carries the marker: %q", label, row)
		}
	}
}

// TestHarnessScreen_ExperimentalHarness_BadgeColumnStaysAligned verifies a marked entry does
// not shift the name column or the tier badge of any row.
func TestHarnessScreen_ExperimentalHarness_BadgeColumnStaysAligned(t *testing.T) {
	hs := experimentalPickerFixtures()
	longest := 0
	for _, h := range hs {
		if n := len([]rune(h.DisplayName)); n > longest {
			longest = n
		}
	}

	rows := labelRows(t, hs)

	wantCol := 2 + longest + 2
	for _, r := range rows {
		if got := badgeColumn(r); got != wantCol {
			t.Errorf("badge column = %d, want %d in row %q", got, wantCol, r)
		}
	}
}

// TestHarnessScreen_ExperimentalHarness_IsSelectable verifies an experimental, usable harness
// can be chosen with Enter.
func TestHarnessScreen_ExperimentalHarness_IsSelectable(t *testing.T) {
	s := screens.NewHarnessScreen(experimentalPickerFixtures(), 120, 30, plainStyles())
	s.Update(harnessDownKey())

	s.Update(harnessEnterKey())

	if !s.Done() {
		t.Fatal("Done() = false after Enter on an experimental usable harness")
	}
	if got := s.SelectedID(); got != "exp" {
		t.Errorf("SelectedID() = %q, want %q", got, "exp")
	}
}

// TestHarnessScreen_ExperimentalHarness_NotUsable_StaysUnselectable verifies the marker does
// not make an unusable harness selectable.
func TestHarnessScreen_ExperimentalHarness_NotUsable_StaysUnselectable(t *testing.T) {
	hs := []domain.HarnessRef{
		{ID: "stable", DisplayName: "Stable", Tier: domain.TierBuiltin, Usable: true},
		{ID: "exp", DisplayName: "Experimental Tool", Tier: domain.TierBuiltin, Usable: false,
			ExperimentalReason: experimentalTestReason},
	}
	s := screens.NewHarnessScreen(hs, 120, 30, plainStyles())
	s.Update(harnessDownKey())

	s.Update(harnessEnterKey())

	if s.SelectedID() == "exp" {
		t.Error("unusable experimental harness was selected")
	}
}

// TestHarnessScreen_ExperimentalHarness_HighlightedShowsReasonInDetail verifies the reason is
// shown once the experimental harness is highlighted.
func TestHarnessScreen_ExperimentalHarness_HighlightedShowsReasonInDetail(t *testing.T) {
	s := screens.NewHarnessScreen(experimentalPickerFixtures(), 400, 30, plainStyles())
	s.Update(harnessDownKey())

	view := collapseWhitespace(s.View())

	if !strings.Contains(view, "Experimental: "+experimentalTestReason) {
		t.Errorf("detail pane does not show the reason for the highlighted experimental harness:\n%s", view)
	}
}

// TestHarnessScreen_NonExperimentalHighlighted_DetailShowsNoReason verifies the reason does not
// leak into the detail of a highlighted non-experimental harness.
func TestHarnessScreen_NonExperimentalHighlighted_DetailShowsNoReason(t *testing.T) {
	s := screens.NewHarnessScreen(experimentalPickerFixtures(), 400, 30, plainStyles())

	view := collapseWhitespace(s.View())

	if strings.Contains(view, experimentalTestReason) {
		t.Errorf("reason shown while a non-experimental harness is highlighted:\n%s", view)
	}
}

// TestHarnessScreen_ExperimentalReason_IsNotTruncated verifies a long reason is shown in full.
func TestHarnessScreen_ExperimentalReason_IsNotTruncated(t *testing.T) {
	long := strings.Repeat("word ", 60) + "final-marker"
	hs := []domain.HarnessRef{{ID: "exp", DisplayName: "Exp", Tier: domain.TierBuiltin, Usable: true,
		ExperimentalReason: long}}

	view := collapseWhitespace(screens.NewHarnessScreen(hs, 400, 60, plainStyles()).View())

	if !strings.Contains(view, "final-marker") {
		t.Errorf("long reason was truncated:\n%s", view)
	}
}

// TestHarnessScreen_NoExperimentalHarness_ShowsNoExperimentalText verifies that a list without
// experimental harnesses renders no experimental text anywhere.
func TestHarnessScreen_NoExperimentalHarness_ShowsNoExperimentalText(t *testing.T) {
	s := screens.NewHarnessScreen(labelFixtures(), 400, 30, plainStyles())

	view := s.View()

	if strings.Contains(view, domain.ExperimentalLabel) {
		t.Errorf("view of non-experimental harnesses mentions %q:\n%s", domain.ExperimentalLabel, view)
	}
}
