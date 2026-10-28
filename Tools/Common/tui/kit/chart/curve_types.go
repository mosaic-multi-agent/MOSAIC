package chart

import "mosaic-common/tui"

// Guide colours the rows of a curve whose value lies in [From, To); To <= From means open-ended upward.
type Guide struct {
	From, To float64
	Role     tui.StyleRole
	Label    string
}

// Marker is a vertical marker over the column holding point Index.
type Marker struct {
	Index int
	Kind  MarkerKind
}

// Curve is a series with guide bands and vertical markers.
type Curve struct {
	Values  []float64
	Guides  []Guide
	Markers []Marker
	YLabel  func(v float64) string // left gutter labels; nil = no gutter
}
