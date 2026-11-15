package agentformat

// Setting is one native setting of a deployed agent file.
//
// Value has one of these dynamic types: string, bool, int64, float64, time.Time
// (offset date-time), toml.LocalDateTime, toml.LocalDate, toml.LocalTime, []any, or
// map[string]any (a table; nested values use the same types).
type Setting struct {
	// Key is the top-level key exactly as declared, in its unquoted form.
	Key   string
	Value any
}

// SettingsReader is the optional read-side capability of a Translator.
type SettingsReader interface {
	// ReadSettings returns every native setting the deployed file declares, in file
	// declaration order, with its decoded value. It never modifies its input and
	// performs no I/O. A file that Decode would reject is rejected here with the same
	// error classification (ErrMalformedDeployed wrapped in *ArtifactError, Phase
	// "decode").
	ReadSettings(deployed []byte) ([]Setting, error)
}
