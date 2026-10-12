package domain

// StageSource describes where one block's schedule was read or looked for, so
// a stop caused by its absence can name the cause rather than the symptom.
// The legacy name is kept; the descriptor is per block.
//
// The zero value means "not stated"; consumers must degrade to a generic
// message rather than rendering empty fields.
type StageSource struct {
	// Block is the block whose schedule the descriptor is about. Empty for a
	// descriptor that is not tied to a block.
	Block BlockName

	// Source is the Source the block declares.
	Source string

	// Path is the absolute path the schedule was read from or looked for at.
	Path string

	// Seeded is true when the block's Source is a seed. It distinguishes "a schedule was supposed to be seeded here and
	// is missing" from "this run never had one".
	Seeded bool
}
