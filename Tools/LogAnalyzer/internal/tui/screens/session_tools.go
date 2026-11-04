package screens

import (
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The tools section of the Session screen: the actor's tools with the figures of
// the Tools view, one line per tool, below the per-call table.

// sessionToolsOf are the tool figures of the actor; none when the report has no
// figures for it.
func sessionToolsOf(run domain.RunReport, actor domain.ActorRef) []domain.ToolFigures {
	session, ok := run.ActorSession(actor)
	if !ok {
		return nil
	}
	return session.Tools
}

// sessionToolsTable is the actor's tools under the calls sort, without a selection.
func sessionToolsTable(tools []domain.ToolFigures, width int, st Styles) table.Table {
	return toolsTable(tools, SortOrder{SortCalls, true}, table.NoCursor, width, st)
}

// sessionToolsLines draws the tools table with at most rows tool rows, and the
// blank line that separates it from the table above. No tools give no lines.
func sessionToolsLines(t table.Table, width, rows int, st Styles) []string {
	if len(t.Rows) == 0 {
		return nil
	}
	return append([]string{""}, renderTableLines(t, width, rows, st)...)
}

// sessionToolsLegend is the legend group of the tools table with its growth columns.
func sessionToolsLegend() []columnDoc {
	return append(append([]columnDoc(nil), toolDocs()...), growthDocs()...)
}
