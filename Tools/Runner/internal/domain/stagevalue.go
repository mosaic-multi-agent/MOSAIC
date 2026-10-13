package domain

// validIterationID reports whether s is non-empty, made of ASCII letters,
// digits and '-', and starts with a letter or digit.
func validIterationID(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		alnum := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !alnum && (c != '-' || i == 0) {
			return false
		}
	}
	return true
}

// RecordedPhaseName returns the phase value read from an artifact. The runner
// records the bare phase name, taken from RoutingRow.PhaseParsed.Name, so the
// value is used as read.
func RecordedPhaseName(recorded string) string {
	return recorded
}
