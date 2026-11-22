package screens_test

// The start moment of a run and how the runs list shows it: the run ID's
// timestamp first, else the timeline axis, else the absent marker; always in the
// zone the styles carry, never the machine's.

import (
	"testing"
	"time"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	startRunID   = "20261112T175027Z-2d24" // 2026-11-12 17:50:27 UTC
	startNamedID = "release-check"         // not a run ID
)

var (
	zonePlusOne = time.FixedZone("T+1", 3600)
	zonePlusTwo = time.FixedZone("T+2", 2*3600)
)

func axisFrom(start time.Time) domain.Timeline {
	return domain.Timeline{Axis: domain.TimeInterval{Start: start, End: start.Add(time.Minute)}, HasAxis: true}
}

func startRun(id string, tl domain.Timeline) domain.RunReport {
	return domain.RunReport{
		Run:          domain.NamedRun(id),
		Totals:       domain.Totals{Money: completeMoney(100)},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		Timeline:     tl,
	}
}

func startReport(runs ...domain.RunReport) domain.Report {
	return domain.Report{Runs: runs, Currency: domain.Currency, Quality: domain.NewQualitySummary(nil)}
}

func startStyles(loc *time.Location) screens.Styles {
	st := paletteStyles()
	st.Location = loc
	return st
}

func startView(report domain.Report, loc *time.Location, width int) string {
	return screens.NewRunsScreen(report, width, tallH, startStyles(loc)).View(width, tallH)
}

func TestRunStart_TakesTheRunIDTimestampOfANamedRun(t *testing.T) {
	got, ok := screens.RunStart(startRun(startRunID, domain.Timeline{}))

	want := time.Date(2026, 11, 12, 17, 50, 27, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("RunStart = %v, %v; want %v, true", got, ok, want)
	}
}

func TestRunStart_PrefersTheRunIDOverTheTimelineAxis(t *testing.T) {
	axis := axisFrom(time.Date(2026, 11, 12, 17, 49, 0, 0, time.UTC))

	got, ok := screens.RunStart(startRun(startRunID, axis))

	want := time.Date(2026, 11, 12, 17, 50, 27, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("RunStart = %v, %v; want the run ID's moment %v", got, ok, want)
	}
}

func TestRunStart_FallsBackToTheTimelineAxisForANonTimestampName(t *testing.T) {
	start := time.Date(2026, 3, 4, 9, 15, 0, 0, time.UTC)

	got, ok := screens.RunStart(startRun(startNamedID, axisFrom(start)))

	if !ok || !got.Equal(start) {
		t.Fatalf("RunStart = %v, %v; want %v, true", got, ok, start)
	}
}

func TestRunStart_IsAbsentWithoutAnyUsableSource(t *testing.T) {
	zeroAxis := domain.Timeline{HasAxis: true}
	unflagged := axisFrom(time.Date(2026, 3, 4, 9, 15, 0, 0, time.UTC))
	unflagged.HasAxis = false
	cases := []struct {
		name string
		run  domain.RunReport
	}{
		{"name without timestamp and no axis", startRun(startNamedID, domain.Timeline{})},
		{"axis flag unset", startRun(startNamedID, unflagged)},
		{"axis with a zero start", startRun(startNamedID, zeroAxis)},
		{"unattributable bucket", domain.RunReport{Run: domain.UnattributableRun(), Timeline: axisFrom(time.Date(2026, 3, 4, 9, 15, 0, 0, time.UTC))}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := screens.RunStart(c.run)

			if ok || !got.IsZero() {
				t.Errorf("RunStart = %v, %v; want the zero time and false", got, ok)
			}
		})
	}
}

func TestFormatRunStart_ShowsDateAndMinuteInTheGivenZone(t *testing.T) {
	utc := time.Date(2026, 11, 12, 17, 50, 27, 0, time.UTC)

	if got := screens.FormatRunStart(utc, true, zonePlusOne); got != "2026-11-12 18:50" {
		t.Errorf("got %q, want %q", got, "2026-11-12 18:50")
	}
	if got := screens.FormatRunStart(utc, true, time.UTC); got != "2026-11-12 17:50" {
		t.Errorf("got %q, want %q", got, "2026-11-12 17:50")
	}
}

func TestFormatRunStart_ConvertsToTheNextDayWhenTheZoneCrossesMidnight(t *testing.T) {
	late := time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC)

	if got := screens.FormatRunStart(late, true, zonePlusTwo); got != "2027-01-01 01:30" {
		t.Errorf("got %q, want %q", got, "2027-01-01 01:30")
	}
}

func TestFormatRunStart_ANilZoneReadsAsTheMachineLocalZone(t *testing.T) {
	at := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	got := screens.FormatRunStart(at, true, nil)

	if want := at.In(time.Local).Format("2006-01-02 15:04"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatRunStart_UnknownStartIsTheAbsentMarkerNeverADate(t *testing.T) {
	got := screens.FormatRunStart(time.Time{}, false, zonePlusOne)

	if got != screens.AbsentMarker {
		t.Errorf("got %q, want %q", got, screens.AbsentMarker)
	}
}

func TestRuns_RowShowsTheRunStartInTheConfiguredZoneAndKeepsTheRunID(t *testing.T) {
	colourOff(t)

	view := startView(startReport(startRun(startRunID, domain.Timeline{})), zonePlusOne, 140)

	row := rowFor(t, view, startRunID)
	assertContains(t, row, "2026-11-12 18:50")
	assertNotContains(t, row, "17:50")
}

func TestRuns_StartTimeFollowsTheZoneNotTheMachine(t *testing.T) {
	colourOff(t)
	report := startReport(startRun(startRunID, domain.Timeline{}))

	inUTC := rowFor(t, startView(report, time.UTC, 140), startRunID)
	inPlusTwo := rowFor(t, startView(report, zonePlusTwo, 140), startRunID)

	assertContains(t, inUTC, "2026-11-12 17:50")
	assertContains(t, inPlusTwo, "2026-11-12 19:50")
}

func TestRuns_HeaderNamesTheStartedColumnRightAfterTheRun(t *testing.T) {
	colourOff(t)

	titles := headerTitles(startView(startReport(startRun(startRunID, domain.Timeline{})), time.UTC, 140), "compare")

	want := []string{"compare", "run", "started", "cost", "invocations", "duration", "markers"}
	if len(titles) != len(want) {
		t.Fatalf("columns = %q, want %q", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("columns = %q, want %q", titles, want)
		}
	}
}

func TestRuns_ANamedRunWithoutTimestampShowsTheAxisStartInTheZone(t *testing.T) {
	colourOff(t)
	run := startRun(startNamedID, axisFrom(time.Date(2026, 3, 4, 9, 15, 0, 0, time.UTC)))

	row := rowFor(t, startView(startReport(run), zonePlusOne, 140), startNamedID)

	assertContains(t, row, "2026-03-04 10:15")
}

func TestRuns_ARunWithoutAnyStartShowsTheAbsentMarkerNotADate(t *testing.T) {
	colourOff(t)
	run := startRun(startNamedID, domain.Timeline{})

	row := rowFor(t, startView(startReport(run), zonePlusOne, 140), startNamedID)

	// The start and the duration are both unknown; the cost is known.
	if got := countOf(row, screens.AbsentMarker); got != 2 {
		t.Errorf("start and duration must both show %q (got %d): %q", screens.AbsentMarker, got, row)
	}
	assertNotContains(t, row, "0001-01-01", "1970-01-01")
}

func TestRuns_DigestOpenedRunsShowTheirStartLikeAnyOther(t *testing.T) {
	colourOff(t)
	// A digest run is a report read back: named, with the axis it was exported with.
	digestRun := startRun("digest-run", axisFrom(time.Date(2026, 5, 6, 23, 45, 0, 0, time.UTC)))

	row := rowFor(t, startView(startReport(digestRun), zonePlusTwo, 140), "digest-run")

	assertContains(t, row, "2026-05-07 01:45")
}

func TestRuns_EveryRowShowsItsOwnStart(t *testing.T) {
	colourOff(t)
	report := startReport(
		startRun("20260101T120000Z-aaaa", domain.Timeline{}),
		startRun("20260202T130000Z-bbbb", domain.Timeline{}),
	)

	view := startView(report, time.UTC, 140)

	assertContains(t, rowFor(t, view, "20260101T120000Z-aaaa"), "2026-01-01 12:00")
	assertContains(t, rowFor(t, view, "20260202T130000Z-bbbb"), "2026-02-02 13:00")
	assertNotContains(t, rowFor(t, view, "20260101T120000Z-aaaa"), "2026-02-02")
}

func TestRuns_StartIsDrawnAsATimeFigure(t *testing.T) {
	colourOn(t)
	// The cheapest run is listed last, so the cursor row is another one.
	cheap := startRun("20260102T120000Z-bbbb", domain.Timeline{})
	dear := startRun("20260103T120000Z-cccc", domain.Timeline{})
	dear.Totals.Money = completeMoney(900)
	report := startReport(cheap, dear)

	view := startView(report, time.UTC, tableW)

	assertDrawnInRole(t, rawLineWith(t, view, "20260102T120000Z-bbbb"), "2026-01-02 12:00", tuicommon.RoleDataTime)
}

func TestRuns_NarrowTerminalsKeepTheRunIDAndTheCostWhenTheStartIsDropped(t *testing.T) {
	colourOff(t)
	report := startReport(startRun(startRunID, domain.Timeline{}))

	for _, width := range []int{140, 100, 80, 60, 50, 40} {
		view := startView(report, time.UTC, width)

		assertWithinBounds(t, view, width, tallH)
		assertNameShown(t, view, startRunID, 8)
		row := rowFor(t, view, startRunID[:8])
		assertContains(t, row, "$1.00")
	}
}

func TestRuns_AtAWideTerminalTheStartIsShownInFull(t *testing.T) {
	colourOff(t)

	row := rowFor(t, startView(startReport(startRun(startRunID, domain.Timeline{})), time.UTC, 140), startRunID)

	assertContains(t, row, "2026-11-12 17:50")
}
