// Package gantt renders time-based lanes on a shared axis. It carries no tool-specific concepts.
package gantt

import (
	"time"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

// Axis is the time range that lanes and tick labels are laid out on.
type Axis struct{ Start, End time.Time }

// Interval is a span of time drawn on a lane.
type Interval struct {
	Start, End time.Time
	Open       bool // drawn from Start to the axis end with an open-end glyph
	Role       tui.StyleRole
}

// Point is a marker at one instant.
type Point struct {
	At   time.Time
	Kind chart.MarkerKind
	Role tui.StyleRole
}

// Shade is a background span drawn where no interval or point covers the cell.
type Shade struct {
	Start, End time.Time
	Role       tui.StyleRole
}

// Lane is one labelled row of intervals and points.
type Lane struct {
	Label     string
	LabelRole tui.StyleRole
	Intervals []Interval
	Points    []Point
}
