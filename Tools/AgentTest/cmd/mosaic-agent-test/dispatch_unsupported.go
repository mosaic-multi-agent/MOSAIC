package main

import (
	"errors"
	"fmt"

	commonharness "mosaic-common/harness"
)

// ErrUnsupportedHarness reports a catalog harness this tool knows but does
// not support.
var ErrUnsupportedHarness = errors.New("mosaic-agent-test: harness not supported")

// unsupportedHarnesses returns the catalog entries this tool does not support,
// in catalog order (catalog minus supportedHarnesses()).
func unsupportedHarnesses() []commonharness.CLIHarness {
	all := commonharness.CLIHarnesses()
	out := make([]commonharness.CLIHarness, 0, len(all))
	for _, e := range all {
		if !isSupportedHarness(e.ID) {
			out = append(out, e)
		}
	}
	return out
}

// isUnsupportedCatalogHarness reports whether id names a catalog harness this
// tool knows but does not support.
func isUnsupportedCatalogHarness(id string) bool {
	for _, e := range unsupportedHarnesses() {
		if e.ID == id {
			return true
		}
	}
	return false
}

// unsupportedHarnessError builds the refusal for an unsupported catalog id; it
// names the harness and is distinct from ErrUnknownHarness.
func unsupportedHarnessError(id string) error {
	return fmt.Errorf("%w: %q", ErrUnsupportedHarness, id)
}
