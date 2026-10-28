package screens

import tuicommon "mosaic-common/tui"

// SortKey names the figure a table is sorted by.
type SortKey string

const (
	SortCost        SortKey = "cost"
	SortTokens      SortKey = "tokens"
	SortInvocations SortKey = "invocations"
	SortCalls       SortKey = "calls"
	SortDuration    SortKey = "duration"
	SortName        SortKey = "name"
	SortPeakContext SortKey = "peak context"
	SortContext     SortKey = "context"
	SortCallOrder   SortKey = "call order"
	SortCostChange  SortKey = "cost change"
	SortStart       SortKey = "start"
	SortErrors      SortKey = "errors"
	SortFinal       SortKey = "final"
)

// SortOrder is one offered sort: a key and a direction.
type SortOrder struct {
	Key  SortKey
	Desc bool
}

// SortCycle cycles through the sort orders a view offers.
type SortCycle struct {
	orders []SortOrder
	active int
}

// NewSortCycle returns a cycle whose first order is the default; it panics when
// no order is given.
func NewSortCycle(orders ...SortOrder) SortCycle {
	if len(orders) == 0 {
		panic("screens: NewSortCycle needs at least one sort order")
	}
	return SortCycle{orders: append([]SortOrder(nil), orders...)}
}

// Active is the current sort order.
func (c SortCycle) Active() SortOrder { return c.orders[c.active] }

// Next returns the cycle advanced to the next order, wrapping around.
func (c SortCycle) Next() SortCycle {
	c.active = (c.active + 1) % len(c.orders)
	return c
}

// Label is "sort: <key>" plus a direction marker.
func (c SortCycle) Label() string {
	o := c.Active()
	down, up := "↓", "↑"
	if tuicommon.CurrentRenderMode().Glyphs == tuicommon.GlyphASCII {
		down, up = "v", "^"
	}
	if o.Desc {
		return "sort: " + string(o.Key) + " " + down
	}
	return "sort: " + string(o.Key) + " " + up
}
