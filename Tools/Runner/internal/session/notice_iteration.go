package session

import "mosaic-run/internal/domain"

// iterationNoticeValue renders the recorded stage of a dispatched row as the
// iteration value of a progress notice: the 3.0 form `{ID}[.{Group}]`, or an
// empty value for a row outside every block. A recorded value that cannot be
// read with its row is shown verbatim, so a notice never hides what was
// recorded.
func iterationNoticeValue(table domain.RoutingTable, rowIndex int, recorded string) string {
	if recorded == "" {
		return ""
	}
	if rowIndex < 0 || rowIndex >= len(table.Rows) {
		return recorded
	}
	value, err := domain.ReadIterationValue(recorded, table.Rows[rowIndex])
	if err != nil {
		return recorded
	}
	if value.IsZero() {
		return ""
	}
	return domain.FormatIterationValue(value)
}
