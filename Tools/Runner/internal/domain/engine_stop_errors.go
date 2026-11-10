package domain

import "fmt"

// MissingScheduleError is StopDecision.Err when Next stops because the
// effective schedules hold no entry for block Block. Error() returns the full
// stop reason, so StopDecision.Reason == Err.Error().
type MissingScheduleError struct {
	Block  BlockName
	Source string // declared Source of the descriptor; "" with no descriptor
	Path   string // descriptor Path; "" with no descriptor
	Seeded bool   // descriptor Seeded; false with no descriptor
	Base   string // the call site's base text (free text; not part of the contract)
}

// Error returns the stop reason: the call site's base text, extended with the
// path the schedule was looked for at when a source descriptor is known. Seeded
// selects between "was seeded there and is missing" and "no schedule was found".
func (e *MissingScheduleError) Error() string {
	switch {
	case e.Path == "" && !e.Seeded:
		return e.Base
	case e.Seeded:
		return fmt.Sprintf("%s: a stage table was seeded at %s but is missing%s", e.Base, e.Path, e.blockClause())
	default:
		return fmt.Sprintf("%s: no stage table was found at %s%s", e.Base, e.Path, e.blockClause())
	}
}

// blockClause names the block and its declared Source when they are known.
func (e *MissingScheduleError) blockClause() string {
	switch {
	case e.Block != "" && e.Source != "":
		return fmt.Sprintf(" (block %s, source %s)", e.Block, e.Source)
	case e.Block != "":
		return fmt.Sprintf(" (block %s)", e.Block)
	default:
		return ""
	}
}

// UnroutableEntryError is StopDecision.Err when Next stops on a completed
// row's routing entry it does not route.
type UnroutableEntryError struct {
	Row    WorkflowRow // 1-based row of the completed row
	Column string      // "On Success" or "On Findings"
	Kind   string      // the entry's kind: "" (zero kind) or "fork" (SuccessFork outside Mode 1)
}

// Error names the row and the column.
func (e *UnroutableEntryError) Error() string {
	return fmt.Sprintf("row %d: %s entry of kind %q is not routable", e.Row, e.Column, e.Kind)
}
