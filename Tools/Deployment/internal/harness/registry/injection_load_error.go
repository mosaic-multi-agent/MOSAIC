package registry

// injection_load_error.go defines the layer-attributed injection load failure of the
// stack-backed registry path.

import (
	"errors"
	"fmt"

	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/harness/injectionfile"
)

// InjectionLoadError attributes a built-in's injection load failure to the catalog layer that
// supplies its HarnessInjections directory. The file and failure kind are read from the
// *injectionfile.LoadError in Err's chain (errors.As).
type InjectionLoadError struct {
	Resolution injections.Resolution // HarnessDir, Dir and Supplier (Identity.ID, Root)
	Err        error                 // the factory error; its chain contains *injectionfile.LoadError
}

// Error names the supplier layer id and root, the harness directory, the file and failure kind,
// and the guidance that harness injections are not merged across layers; it ends with Err's text.
func (e *InjectionLoadError) Error() string {
	file, problem := "a required file", "cannot be loaded"
	var le *injectionfile.LoadError
	if errors.As(e.Err, &le) {
		file = le.File
		problem = "cannot be parsed"
		if le.Kind == injectionfile.LoadErrorRead {
			problem = "is missing or unreadable"
		}
	}
	s := e.Resolution.Supplier
	return fmt.Sprintf("catalog layer %q (root %s) supplies HarnessInjections/%s, but %s %s. "+
		"Harness injections are not merged across layers: the layer must supply a valid %s or drop the %s directory: %v",
		s.Identity.ID, s.Root, e.Resolution.HarnessDir, file, problem, file, e.Resolution.HarnessDir, e.Err)
}

// Unwrap returns Err, so errors.As reaches *injectionfile.LoadError and errors.Is reaches
// fs.ErrNotExist.
func (e *InjectionLoadError) Unwrap() error { return e.Err }
