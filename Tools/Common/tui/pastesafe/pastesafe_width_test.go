package pastesafe_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui/pastesafe"
)

// wideValue has 62 distinct characters, so any visible slice identifies its position.
const wideValue = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func assertWithin(t *testing.T, view string, width int) {
	t.Helper()
	if strings.Contains(view, "\n") {
		t.Fatalf("view spans several lines: %q", view)
	}
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("view width = %d, want <= %d: %q", got, width, view)
	}
}

func assertShows(t *testing.T, view string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(view, p) {
			t.Fatalf("view %q does not show %q", view, p)
		}
	}
}

func newWideDriver(width int) *keyDriver {
	d := newDriver(pastesafe.WithWidth(width))
	d.f.SetValue(wideValue)
	return d
}

func TestWideValueAtEndStaysWithinWidthAndShowsTail(t *testing.T) {
	d := newWideDriver(20)

	view := d.f.View()

	assertWithin(t, view, 20)
	assertShows(t, view, wideValue[len(wideValue)-5:])
}

func TestWideValueAtStartStaysWithinWidthAndShowsHead(t *testing.T) {
	d := newWideDriver(20)
	d.send(typed, key(tea.KeyHome))

	view := d.f.View()

	assertWithin(t, view, 20)
	assertShows(t, view, wideValue[:5])
}

func TestWideValueInMiddleShowsTextAroundCursor(t *testing.T) {
	d := newWideDriver(20)
	d.send(typed, key(tea.KeyHome))
	for i := 0; i < 30; i++ {
		d.send(typed, key(tea.KeyRight))
	}

	view := d.f.View()

	assertWithin(t, view, 20)
	assertShows(t, view, wideValue[28:33])
}

func TestWindowFollowsCursorWhenMovingAcrossValue(t *testing.T) {
	d := newWideDriver(20)
	d.send(typed, key(tea.KeyHome))
	assertShows(t, d.f.View(), wideValue[:3])

	d.send(typed, key(tea.KeyEnd))
	view := d.f.View()

	assertWithin(t, view, 20)
	assertShows(t, view, wideValue[len(wideValue)-3:])
	if strings.Contains(view, wideValue[:3]) {
		t.Fatalf("view %q still shows the head after moving to the end", view)
	}
}

func TestTypingAtEndKeepsLatestTextVisible(t *testing.T) {
	d := newDriver(pastesafe.WithWidth(15))

	d.runes(typed, wideValue)

	view := d.f.View()
	assertWithin(t, view, 15)
	assertShows(t, view, wideValue[len(wideValue)-5:])
}

func TestValueThatFitsIsShownInFull(t *testing.T) {
	d := newDriver(pastesafe.WithWidth(40))
	d.f.SetValue("short value")

	view := d.f.View()

	assertWithin(t, view, 40)
	assertShows(t, view, "short value")
}

func TestWidthChangeKeepsViewWithinNewWidthAndCursorVisible(t *testing.T) {
	d := newWideDriver(30)
	tail := wideValue[len(wideValue)-4:]
	assertShows(t, d.f.View(), tail)

	d.f.SetWidth(12)
	narrow := d.f.View()
	assertWithin(t, narrow, 12)
	assertShows(t, narrow, tail)

	d.f.SetWidth(25)
	wider := d.f.View()
	assertWithin(t, wider, 25)
	assertShows(t, wider, tail)
}

func TestWidthChangeKeepsValueAndCursorPosition(t *testing.T) {
	d := newWideDriver(30)
	d.send(typed, key(tea.KeyHome))
	d.send(typed, key(tea.KeyRight))

	d.f.SetWidth(10)
	d.f.SetWidth(30)

	if got := d.f.Value(); got != wideValue {
		t.Fatalf("value changed by width change: %q", got)
	}
	assertShows(t, d.f.View(), wideValue[:3])
}

func TestZeroWidthIsUnbounded(t *testing.T) {
	d := newWideDriver(10)

	d.f.SetWidth(0)

	assertShows(t, d.f.View(), wideValue)
}

func TestResetKeepsWidth(t *testing.T) {
	d := newWideDriver(20)

	d.f.Reset()

	assertWithin(t, d.f.View(), 20)
}

func TestLongPlaceholderIsCutToWidth(t *testing.T) {
	d := newDriver(
		pastesafe.WithWidth(16),
		pastesafe.WithPlaceholder("a very long placeholder that cannot fit"),
	)

	assertWithin(t, d.f.View(), 16)
}

func TestWidthDoesNotChangePasteHandling(t *testing.T) {
	d := newDriver(pastesafe.WithWidth(12))

	d.runes(typed, "line1")
	d.send(inBurst, key(tea.KeyEnter))
	d.runes(inBurst, "line2")

	if got := d.f.Value(); got != "line1 line2" {
		t.Fatalf("value = %q, want %q", got, "line1 line2")
	}
	if d.f.Done() {
		t.Fatal("a pasted line break submitted the field")
	}
}
