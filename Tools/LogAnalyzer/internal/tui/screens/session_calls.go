package screens

import (
	"strconv"
	"strings"

	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The tools and growth cells of the Session screen's per-call table.

// Texts of the tools and growth cells.
const (
	notProvidedText    = "not provided"
	noToolsText        = "none"
	negativeGrowthText = "negative"
	unmeasurableText   = "not measurable"
)

// toolsCell lists the tools a call requested, sorted by name with a count where a
// tool was requested more than once. A call whose log carries no requested tools
// reads "not provided"; one that requested none reads "none".
func toolsCell(r domain.RequestedTools) table.Cell {
	if !r.IsProvided() {
		return mutedCell(notProvidedText)
	}
	names := r.Names()
	if len(names) == 0 {
		return mutedCell(noToolsText)
	}
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name
		if n := r.Count(name); n > 1 {
			parts[i] += " x" + strconv.Itoa(n)
		}
	}
	return table.Cell{Text: strings.Join(parts, ", ")}
}

// growthCell is the context growth of a singular call: the measured tokens, or
// the reason there is no figure. A call that is not singular shows the absent
// marker, and one without requested tools "not provided".
func growthCell(c domain.ContextCall) table.Cell {
	if !c.RequestedTools.IsProvided() {
		return mutedCell(notProvidedText)
	}
	switch c.Growth.State {
	case domain.GrowthMeasured:
		return tokensCell(domain.Tokens(c.Growth.Tokens))
	case domain.GrowthNegative:
		return mutedCell(negativeGrowthText)
	case domain.GrowthNotMeasurable:
		return mutedCell(unmeasurableText)
	}
	return mutedCell(AbsentMarker)
}
