package domain

// RowRef is a resolved target: the whole Subagent cell it names and the
// zero-based index of the one row holding that cell.
// Its zero value (Cell "", Index 0) is not "no target": Index 0 is row 1.
// Consumers read a RowRef only where the entry's Kind says it is set.
type RowRef struct {
	Cell  string // the whole Subagent cell the target names
	Index int    // zero-based index of the row holding that cell
}

// SuccessKind is the kind of an On Success entry.
type SuccessKind string

const (
	SuccessTarget     SuccessKind = "target"      // Target set
	SuccessBlockEntry SuccessKind = "block-entry" // Block set ("[Name]")
	SuccessFork       SuccessKind = "fork"        // Fork set (two or more distinct targets)
	SuccessNext       SuccessKind = "next"        // NEXT
	SuccessComplete   SuccessKind = "complete"    // COMPLETE
)

// SuccessEntry is the typed On Success cell. Target is read only when Kind is
// SuccessTarget, Block only when Kind is SuccessBlockEntry, Fork only when Kind
// is SuccessFork.
type SuccessEntry struct {
	Kind   SuccessKind
	Target RowRef
	Block  BlockName
	Fork   []RowRef
	Raw    string // cell text verbatim
}

// FindingsKind is the kind of an On Findings entry.
type FindingsKind string

const (
	FindingsNone       FindingsKind = "none"       // "-"
	FindingsTarget     FindingsKind = "target"     // Target set
)

// FindingsEntry is the typed On Findings cell. Target is read only when Kind
// is FindingsTarget.
type FindingsEntry struct {
	Kind   FindingsKind
	Target RowRef
	Raw    string // cell text verbatim
}
