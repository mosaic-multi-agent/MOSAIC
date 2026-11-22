package screens

import (
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// RunStart returns the start moment of a run: the run ID's timestamp when the
// run is named and its ID is a run ID, else Timeline.Axis.Start when
// Timeline.HasAxis and the start is non-zero. ok is false otherwise, and always
// for the unattributable bucket.
func RunStart(r domain.RunReport) (t time.Time, ok bool) {
	if r.Run.IsUnattributable() {
		return time.Time{}, false
	}
	if t, ok := domain.RunIDTime(r.Run.ID); ok {
		return t, true
	}
	if r.Timeline.HasAxis && !r.Timeline.Axis.Start.IsZero() {
		return r.Timeline.Axis.Start, true
	}
	return time.Time{}, false
}

// FormatRunStart renders t in loc (nil reads as time.Local) as
// "2006-01-02 15:04", or AbsentMarker when ok is false.
func FormatRunStart(t time.Time, ok bool, loc *time.Location) string {
	if !ok {
		return AbsentMarker
	}
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

// startCell is the run's start in the zone, the absent marker when unknown.
func startCell(r domain.RunReport, loc *time.Location) table.Cell {
	t, ok := RunStart(r)
	if !ok {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: FormatRunStart(t, true, loc), Role: tuicommon.RoleDataTime}
}
