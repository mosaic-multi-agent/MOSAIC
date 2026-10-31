package orchstate

import "fmt"

// MalformedError describes why an orchestration document could not be read.
// It matches ErrDocumentMalformed through errors.Is.
type MalformedError struct {
	Path    string // "" from Parse
	Section string // "document" or "ExecutionLog"
	Line    int    // 1-based file line; 0 when not known
	Reason  string
}

// Error renders the path (or "(in memory)"), the section, the line when known,
// and the reason.
func (e *MalformedError) Error() string {
	path := e.Path
	if path == "" {
		path = "(in memory)"
	}
	where := "section " + e.Section
	if e.Line > 0 {
		where += fmt.Sprintf(", line %d", e.Line)
	}
	return fmt.Sprintf("orchstate: orchestration document malformed: %s: %s: %s", path, where, e.Reason)
}

// Unwrap returns ErrDocumentMalformed.
func (e *MalformedError) Unwrap() error { return ErrDocumentMalformed }
