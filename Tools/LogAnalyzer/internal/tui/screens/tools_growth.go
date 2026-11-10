package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The growth columns of the Tools tables: the tokens a tool's result added to the
// context, measured on calls that requested exactly one tool.

// growthGroup labels the header line above the growth columns.
const growthGroup = "context growth (tokens)"

// notProvidedLabel is shown for a figure the log does not carry.
const notProvidedLabel = "not provided"

// growthNoteIndent is the indent of the note, the width of the table gutter.
const growthNoteIndent = "  "

// growthNote explains what the growth figures are and what VS Code needs for them.
const growthNote = "Context growth is measured only on calls that requested exactly one tool: " +
	"the tokens its result added to the next call. VS Code needs OTel content capture for these figures."

// growthColumns returns the growth columns in display order. The sample count and
// the left-out counts are never dropped; the average, minimum and maximum may be.
func growthColumns() (average, min, max, samples, negative, notMeasurable table.Column) {
	right := table.AlignRight
	average = table.Column{Title: "average growth", Align: right, Priority: 5, Group: growthGroup}
	min = table.Column{Title: "minimum growth", Align: right, Priority: 6, Group: growthGroup}
	max = table.Column{Title: "maximum growth", Align: right, Priority: 6, Group: growthGroup}
	samples = table.Column{Title: "growth samples", Align: right, Group: growthGroup}
	negative = table.Column{Title: "negative growth", Align: right, Group: growthGroup}
	notMeasurable = table.Column{Title: "not measurable", Align: right, Group: growthGroup}
	return
}

// growthCells fills the growth cells of one tool row.
func growthCells(g domain.ToolGrowth, cells []table.Cell) {
	if g.IsProvided() {
		cells[tlGrowthAverage] = table.Cell{Text: formatGrouped(g.Average.ValueOr(0)), Role: tuicommon.RoleDataTokens}
		cells[tlGrowthMin] = table.Cell{Text: formatGrouped(g.Min.ValueOr(0)), Role: tuicommon.RoleDataTokens}
		cells[tlGrowthMax] = table.Cell{Text: formatGrouped(g.Max.ValueOr(0)), Role: tuicommon.RoleDataTokens}
	} else {
		cells[tlGrowthAverage] = table.Cell{Text: notProvidedLabel}
		cells[tlGrowthMin] = table.Cell{Text: AbsentMarker}
		cells[tlGrowthMax] = table.Cell{Text: AbsentMarker}
	}
	cells[tlGrowthSamples] = table.Cell{Text: formatGrouped(int64(g.Samples))}
	cells[tlGrowthNegative] = table.Cell{Text: formatGrouped(int64(g.Negative))}
	cells[tlGrowthNotMeasurable] = table.Cell{Text: formatGrouped(int64(g.NotMeasurable))}
}

// growthNoteLines is the note wrapped to the view width, behind a blank line.
func growthNoteLines(width int, st Styles) []string {
	out := []string{""}
	for _, l := range WrapText(growthNote, width-len(growthNoteIndent)) {
		out = append(out, clip(st.Muted, width, growthNoteIndent+l))
	}
	return out
}
