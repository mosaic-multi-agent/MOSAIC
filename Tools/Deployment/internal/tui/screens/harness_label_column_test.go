package screens_test

// harness_label_column_test.go verifies that the harness list sizes its label column to the
// longest label shown: no label is cut, and every row's badge starts at the same column.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// labelRows renders the screen wide enough for the rows to stay on one line and returns one
// list row per harness, in the order given. A list row is the line whose content, after the
// two-column cursor prefix, starts with the harness label (display name, or ID when empty).
// Detail-pane text on the same line is never mistaken for a row. It fails the test when a
// harness has no row.
func labelRows(t *testing.T, hs []domain.HarnessRef) []string {
	t.Helper()
	view := screens.NewHarnessScreen(hs, 400, 30, plainStyles()).View()
	lines := strings.Split(view, "\n")
	var rows []string
	for _, h := range hs {
		label := h.DisplayName
		if label == "" {
			label = h.ID
		}
		found := ""
		for _, l := range lines {
			r := []rune(l)
			if len(r) > 2 && strings.HasPrefix(string(r[2:]), label) {
				found = l
				break
			}
		}
		if found == "" {
			t.Fatalf("no list row for label %q in view:\n%s", label, view)
		}
		rows = append(rows, found)
	}
	return rows
}

// badgeColumn is the rune offset of the tier badge in a row.
func badgeColumn(row string) int {
	i := strings.Index(row, "[")
	if i < 0 {
		return -1
	}
	return len([]rune(row[:i]))
}

func rowFor(rows []string, marker string) string {
	for _, r := range rows {
		if strings.Contains(r, marker) {
			return r
		}
	}
	return ""
}

func labelFixtures() []domain.HarnessRef {
	return []domain.HarnessRef{
		{ID: "short", DisplayName: "Short", Tier: domain.TierBuiltin, Usable: true},
		{ID: "long", DisplayName: "GHCP CLI / VS Code GHCP Copilot Extra", Tier: domain.TierDescriptor, Usable: true},
		{ID: "ext", DisplayName: "External Module", Tier: domain.TierExternal, Usable: true,
			ExecutablePath: "/bin/ext"},
	}
}

func TestHarnessScreen_LongLabel_IsNotTruncated_Regression(t *testing.T) {
	const longName = "GHCP CLI / VS Code GHCP Copilot Extra"
	if len([]rune(longName)) < 31 {
		t.Fatal("fixture label must be 31 characters or longer")
	}

	rows := labelRows(t, labelFixtures())

	if rowFor(rows, longName) == "" {
		t.Errorf("full label %q not found in rows: %q", longName, rows)
	}
}

func TestHarnessScreen_AllBadges_StartAtSameColumn(t *testing.T) {
	rows := labelRows(t, labelFixtures())

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(rows), rows)
	}
	want := badgeColumn(rows[0])
	for _, r := range rows {
		if got := badgeColumn(r); got != want {
			t.Errorf("badge column = %d, want %d in row %q", got, want, r)
		}
	}
}

func TestHarnessScreen_BadgeStartsTwoSpacesAfterLongestLabel(t *testing.T) {
	hs := labelFixtures()
	longest := 0
	for _, h := range hs {
		if n := len([]rune(h.DisplayName)); n > longest {
			longest = n
		}
	}

	rows := labelRows(t, hs)

	// Each row starts with a two-column cursor prefix before the label.
	wantCol := 2 + longest + 2
	for _, r := range rows {
		if got := badgeColumn(r); got != wantCol {
			t.Errorf("badge column = %d, want %d in row %q", got, wantCol, r)
		}
	}
}

func TestHarnessScreen_ShortLabelsOnly_ColumnShrinksToLongest(t *testing.T) {
	hs := []domain.HarnessRef{
		{ID: "a", DisplayName: "Alpha", Tier: domain.TierBuiltin, Usable: true},
		{ID: "b", DisplayName: "Bravo One", Tier: domain.TierDescriptor, Usable: true},
	}

	rows := labelRows(t, hs)

	wantCol := 2 + len("Bravo One") + 2
	for _, r := range rows {
		if got := badgeColumn(r); got != wantCol {
			t.Errorf("badge column = %d, want %d (fixed width is gone) in row %q", got, wantCol, r)
		}
	}
}

func TestHarnessScreen_EmptyDisplayName_UsesIDAsLabelInColumn(t *testing.T) {
	hs := []domain.HarnessRef{
		{ID: "an-id-without-a-display-name-over-31", Tier: domain.TierBuiltin, Usable: true},
		{ID: "b", DisplayName: "Bravo", Tier: domain.TierDescriptor, Usable: true},
	}

	rows := labelRows(t, hs)

	if rowFor(rows, "an-id-without-a-display-name-over-31") == "" {
		t.Fatalf("ID label missing: %q", rows)
	}
	wantCol := 2 + len("an-id-without-a-display-name-over-31") + 2
	for _, r := range rows {
		if got := badgeColumn(r); got != wantCol {
			t.Errorf("badge column = %d, want %d in row %q", got, wantCol, r)
		}
	}
}

func TestHarnessScreen_ExternalIndicator_FollowsBadgeAfterTwoSpaces_Regression(t *testing.T) {
	rows := labelRows(t, labelFixtures())

	row := rowFor(rows, "External Module")
	if !strings.Contains(row, "[external]  [active external module]") {
		t.Errorf("indicator must follow the badge after two spaces: %q", row)
	}
}
