package cli

import (
	"fmt"
	"io"

	"mosaic-run/internal/domain"
)

// reportInfraFailures lists every infrastructure-agent failure of the run on
// errOut; it prints nothing when there are none.
func reportInfraFailures(errOut io.Writer, failures []domain.InfraFailure) {
	for _, line := range domain.InfraFailureReportLines(failures) {
		fmt.Fprintln(errOut, line)
	}
}
