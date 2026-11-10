package artifact

import (
	"mosaic-run/internal/domain"
)

// sectionUsable reports whether a required or opened section can be read: it
// has its opening tag and its closing tag. A section that is not usable is
// reported as damage and its content is not read, because without the closing
// tag the content runs on into whatever follows.
func sectionUsable(sc sectionScan, d *damage) bool {
	if !sc.openFound {
		d.addBody(sc.name, 0, "missing "+sc.openTag+" section")
		return false
	}
	if !sc.closed {
		d.addBody(sc.name, sc.openLine, "missing </"+sc.name+"> closing tag; the file appears truncated")
		return false
	}
	return true
}

// readExecutionLog reads the Execution Log section, reporting every structural
// problem and every bad Seq or Agent cell as damage. logClean is true when the
// section is present and had no problem at all. A pre-3.0 layout is not damage:
// it is returned as its own refusal.
func readExecutionLog(sc sectionScan, d *damage) (entries []domain.ExecutionLogEntry, logClean bool, refusal error) {
	before := len(d.body)
	if !sectionUsable(sc, d) {
		return nil, false, nil
	}
	st, ok := readSectionTable(sc, d)
	if !ok {
		return nil, false, nil
	}
	// The pre-3.0 column "Stage" marks the pre-3.0 layout and is not read.
	if st.table.Column("Stage") >= 0 {
		return nil, false, &domain.RefusalError{
			Component: "artifact",
			Reason:    "failed to parse execution log: " + ErrPre30Layout.Error(),
			Cause:     ErrPre30Layout,
			Section:   sc.name,
			Line:      st.headerLine,
		}
	}
	// An older layout without an Inputs or Checkpoint column is still accepted:
	// only the columns the strict checks read are required.
	if st.requireColumns(sc.name, d, "Seq", "Agent") {
		checkLogRows(sc, st, d)
	}
	return parseExecutionLog(st.table), len(d.body) == before, nil
}

// readArtifactRegistry reads the Artifacts section.
func readArtifactRegistry(sc sectionScan, d *damage) []domain.ArtifactRegistryEntry {
	if !sectionUsable(sc, d) {
		return nil
	}
	st, ok := readSectionTable(sc, d)
	if !ok || !st.requireColumns(sc.name, d, "Artifact", "Created In", "Created By") {
		return nil
	}
	return parseArtifactRegistry(st.table)
}

// readWorkflowNotes reads the optional WorkflowNotes section. An opening tag
// makes it required to be readable; without one there are no notes.
func readWorkflowNotes(sc sectionScan, d *damage) []domain.WorkflowNote {
	if !sc.openFound || !sectionUsable(sc, d) {
		return nil
	}
	st, ok := readSectionTable(sc, d)
	if !ok || !st.requireColumns(sc.name, d, "Seq", "Note") {
		return nil
	}
	return parseWorkflowNotes(st.table)
}
