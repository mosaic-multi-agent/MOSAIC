// Package gantt renders time-based lanes on a shared axis. It carries no tool-specific concepts.
package gantt

import (
	"time"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

// Axis is the time range that lanes and tick labels are laid out on.
type Axis struct {
	Start, End time.Time
	// Collapsed are stretches cut out of the axis and drawn as fixed-width markers.
	// Sorted, non-overlapping (touching allowed), each strictly inside (Start, End).
	// nil = linear axis.
	Collapsed []Collapse
}

// Collapse is one stretch of time removed from the axis.
type Collapse struct{ Start, End time.Time }

// CollapseMarkerWidth is the number of columns one collapsed stretch occupies.
const CollapseMarkerWidth = 10

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

// Shade is a background span. As a render-wide shade it is drawn in every lane where no
// interval or point covers the cell; as a Lane shade it follows the cell precedence rule.
type Shade struct {
	Start, End time.Time
	Role       tui.StyleRole
	// Flat draws the shade as a blank cell in Role's style (Role carries a background)
	// in Unicode mode, instead of the dotted glyph. Ignored in ASCII mode.
	Flat bool
	// ASCII is the glyph drawn in ASCII mode; 0 = '.'.
	ASCII byte
}

// Lane is one labelled row of intervals and points.
type Lane struct {
	Label     string
	LabelRole tui.StyleRole
	Intervals []Interval
	Points    []Point
	// Shades are drawn in this lane's row only, over the lane's own intervals.
	Shades []Shade
}
