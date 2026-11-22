package analysis_test

// Builders for the session-aware figure tests. A call's session is the
// envelope session id; "" means the record carries none.

import "mosaic-log-analyzer/internal/domain"

// sessCall returns the record with its session id set.
func sessCall(session string, u domain.InvocationUsage) domain.InvocationUsage {
	u.SessionID = session
	return u
}

// sessAll returns the records with the same session id set on each.
func sessAll(session string, calls ...domain.InvocationUsage) []domain.InvocationUsage {
	out := make([]domain.InvocationUsage, 0, len(calls))
	for _, c := range calls {
		out = append(out, sessCall(session, c))
	}
	return out
}

// sessJoin concatenates groups of records in order.
func sessJoin(groups ...[]domain.InvocationUsage) []domain.InvocationUsage {
	var out []domain.InvocationUsage
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
