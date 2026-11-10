package domain

// BlockSchedules holds the current schedule of each block. Read it with the
// comma-ok map form; reading a nil value is safe.
//
// For every entry the Runner builds, the key equals the value's Block. Every
// lookup is by key, so for a hand-built entry whose key and Block differ, the
// key wins: s[k] is block k's schedule, and the value's Block field is not
// read for routing, validation or path resolution.
type BlockSchedules map[BlockName]BlockSchedule
