package domain

// Conflict report carriers are grouped in this file. They are data only (no behaviour): the
// report describes every place where a higher catalog layer shadows an item of a lower layer.
//
// JSON tags (a deliberate deviation): internal/domain has no json tags elsewhere. The carriers in
// this file carry lowerCamel tags because they are nested inside lowerCamel-tagged app result
// types. Do not "align" either side.

// ConflictSeverity grades a conflict line.
type ConflictSeverity string

const (
	// ConflictInfo marks an override the winning catalog declared in its identity.
	ConflictInfo ConflictSeverity = "info"
	// ConflictWarning marks an undeclared override.
	ConflictWarning ConflictSeverity = "warning"
)

// ConflictLine reports one shadowing: the item, the winning catalog and the shadowed catalogs.
type ConflictLine struct {
	Item              CatalogItem       `json:"item"`
	Winner            CatalogIdentity   `json:"winner"`
	Shadowed          []CatalogIdentity `json:"shadowed"` // highest first
	Severity          ConflictSeverity  `json:"severity"`
	AffectedWorkflows []string          `json:"affectedWorkflows,omitempty"`
}

// ConflictReport is the list of conflict lines of one run. Empty without shadowing.
type ConflictReport struct {
	Lines []ConflictLine `json:"lines,omitempty"`
}
