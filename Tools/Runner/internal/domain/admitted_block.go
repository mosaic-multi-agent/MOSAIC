package domain

// SeedClass says how a block's schedule source comes to exist. Its zero value
// is the explicit "unclassified" value: no code may read it as seeded or as
// produced.
type SeedClass string

const (
	// SeedUnclassified is the zero value: admission has not classified the block.
	SeedUnclassified SeedClass = ""
	// SeedSeeded: the Source is a seed (first row's Inputs, or row 1 begins the block).
	SeedSeeded SeedClass = "seeded"
	// SeedProduced: the Source is output, before the block is entered, by a row outside the block.
	SeedProduced SeedClass = "produced"
)

// AdmittedBlock is the admitted model of one declared block.
type AdmittedBlock struct {
	Name       BlockName
	Source     string
	Section    string
	StartRow   int              // zero-based, inclusive
	EndRow     int              // zero-based, exclusive
	Grouped    bool             // true when the block's own rows carry groups
	Groups     []ExecutionGroup // first-appearance order; one unnamed group spanning the block when ungrouped
	Approaches ApproachTable    // this block's approach rows only
	Seed       SeedClass

	// ProducingRows and WriterRows are filled by later admission work and stay
	// empty until then.
	ProducingRows []int // zero-based rows outside the block whose Output produces Source before entry
	WriterRows    []int // zero-based rows of the block whose Output produces Source
}

// Contains reports whether rowIdx lies in the block's row range
// (StartRow <= rowIdx < EndRow).
func (b AdmittedBlock) Contains(rowIdx int) bool {
	return b.StartRow <= rowIdx && rowIdx < b.EndRow
}

// GroupOf returns the entry of b.Groups whose range holds rowIdx, and true.
// For an ungrouped block that is the one unnamed group spanning the block. A row
// outside the block, or a row of the block that no entry holds, yields
// (ExecutionGroup{}, false).
func (b AdmittedBlock) GroupOf(rowIdx int) (ExecutionGroup, bool) {
	if !b.Contains(rowIdx) {
		return ExecutionGroup{}, false
	}
	for _, g := range b.Groups {
		if g.StartRow <= rowIdx && rowIdx < g.EndRow {
			return g, true
		}
	}
	return ExecutionGroup{}, false
}

// BlockOf returns the first block of w.Blocks (table order) that contains
// rowIdx, and true; (AdmittedBlock{}, false) when none does, including a
// negative index and an index beyond the table's rows.
func (w AdmittedWorkflow) BlockOf(rowIdx int) (AdmittedBlock, bool) {
	if rowIdx < 0 || rowIdx >= len(w.Table.Rows) {
		return AdmittedBlock{}, false
	}
	for _, b := range w.Blocks {
		if b.Contains(rowIdx) {
			return b, true
		}
	}
	return AdmittedBlock{}, false
}

// BlockByName returns the first block of w.Blocks whose Name equals name
// exactly (case-sensitive), and true; (AdmittedBlock{}, false) when none does.
func (w AdmittedWorkflow) BlockByName(name BlockName) (AdmittedBlock, bool) {
	for _, b := range w.Blocks {
		if b.Name == name {
			return b, true
		}
	}
	return AdmittedBlock{}, false
}
