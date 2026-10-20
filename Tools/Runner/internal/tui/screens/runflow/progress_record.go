package runflow

// RecordStep finishes the running step row with row (every field replaced)
// when a step row is running; otherwise appends row. No step row is running
// afterwards.
func (s *ProgressScreen) RecordStep(row ProgressRow) {
	if s.runningIdx >= 0 && s.runningIdx < len(s.rows) {
		s.rows[s.runningIdx] = row
	} else {
		s.rows = append(s.rows, row)
	}
	s.runningIdx = -1
}
