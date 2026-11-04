package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// maxMarkedRuns is the number of runs that can be marked for a comparison.
const maxMarkedRuns = 2

// markLabels are the markers of the marked runs, in the order they were marked.
var markLabels = [maxMarkedRuns]string{"A", "B"}

// tooManyMarksStatus is shown when a third run is marked.
const tooManyMarksStatus = "Two runs are already marked: unmark one first."

// runMarks is the state of the runs marked for a comparison: the ids in the
// order they were marked, the status of the last key and the open request.
type runMarks struct {
	ids       []string
	status    string
	requested bool
}

// markerOf is the marker of the run, "" when it is not marked.
func (m runMarks) markerOf(id string) string {
	for i, marked := range m.ids {
		if marked == id {
			return markLabels[i]
		}
	}
	return ""
}

// toggle marks the run, or unmarks it when it is marked; a third mark is refused.
func (m *runMarks) toggle(id string) {
	for i, marked := range m.ids {
		if marked == id {
			m.ids = append(m.ids[:i:i], m.ids[i+1:]...)
			return
		}
	}
	if len(m.ids) >= maxMarkedRuns {
		m.status = tooManyMarksStatus
		return
	}
	m.ids = append(m.ids, id)
}

// request asks for the comparison; it has no effect without two marked runs.
func (m *runMarks) request() {
	m.requested = len(m.ids) == maxMarkedRuns
}

// clear drops the marks and any request.
func (m *runMarks) clear() { *m = runMarks{} }

// markCell is the cell of the marker column: A or B, empty for an unmarked run.
func markCell(marker string) table.Cell {
	return table.Cell{Text: marker, Role: tuicommon.RoleSignalWarning}
}

// toggleMark marks or unmarks the selected run.
func (s *RunsScreen) toggleMark() {
	if id := s.SelectedRunID(); id != "" {
		s.marks.toggle(id)
	}
}

// CompareRequested returns the ids of the two marked runs, A (the earlier mark)
// first, once the user asked for the comparison; ok is false otherwise.
func (s *RunsScreen) CompareRequested() (idA, idB string, ok bool) {
	if !s.marks.requested || len(s.marks.ids) != maxMarkedRuns {
		return "", "", false
	}
	return s.marks.ids[0], s.marks.ids[1], true
}
