package cli

import (
	"fmt"
	"io"

	"mosaic-deploy/internal/domain"
)

// writeExperimentalWarnings prints one "warning: " line per experimental harness the
// operation involved, in the order the result lists them. Nothing is written when the
// slice is empty, so output for non-experimental runs is unchanged.
func writeExperimentalWarnings(out io.Writer, harnesses []domain.ExperimentalHarness) {
	for _, h := range harnesses {
		fmt.Fprintf(out, "warning: %s\n", h.Message())
	}
}
