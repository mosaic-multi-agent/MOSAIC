package cli

import (
	"errors"
	"fmt"
)

// ErrHarnessNotSupported is wrapped by the usage error for a --harness value
// naming a catalog harness this tool knows but refuses.
var ErrHarnessNotSupported = errors.New("harness not supported by mosaic-agent-test")

// unsupportedHarnessError is the usage error for a known-but-refused harness.
type unsupportedHarnessError struct {
	id string
}

func (e unsupportedHarnessError) Error() string {
	return fmt.Sprintf("harness %q is not supported by mosaic-agent-test", e.id)
}

func (e unsupportedHarnessError) Unwrap() error { return ErrHarnessNotSupported }

// harnessSelectionError validates a --harness value against the supported and
// unsupported catalogs. It returns nil when the value is acceptable.
func harnessSelectionError(id string, o Options) error {
	for _, h := range o.UnsupportedHarnesses {
		if h.ID == id {
			return unsupportedHarnessError{id: id}
		}
	}
	if len(o.Harnesses) > 0 && !harnessRecognised(id, o.Harnesses) {
		return usageError{fmt.Sprintf("unrecognised --harness value %q", id)}
	}
	return nil
}
