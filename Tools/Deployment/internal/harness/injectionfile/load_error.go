package injectionfile

// load_error.go defines the typed failure LoadDir returns for one harness content file.

import "fmt"

// LoadErrorKind classifies a LoadDir failure.
type LoadErrorKind int

const (
	// LoadErrorRead: the file is missing or cannot be read (errors.Is(err, fs.ErrNotExist)
	// distinguishes missing).
	LoadErrorRead LoadErrorKind = iota + 1
	// LoadErrorParse: the file was read but its content cannot be parsed.
	LoadErrorParse
)

// LoadError is the error LoadDir returns for one harness content file.
type LoadError struct {
	File string // SharedFileName or OrchestratorFileName
	Path string // full path of the file
	Kind LoadErrorKind
	Err  error // the os or parse error
}

// Error keeps today's text: "injectionfile: read <Path>: <Err>" (LoadErrorRead) or
// "injectionfile: parse <Path>: <Err>" (LoadErrorParse).
func (e *LoadError) Error() string {
	verb := "read"
	if e.Kind == LoadErrorParse {
		verb = "parse"
	}
	return fmt.Sprintf("injectionfile: %s %s: %v", verb, e.Path, e.Err)
}

// Unwrap returns Err.
func (e *LoadError) Unwrap() error { return e.Err }
