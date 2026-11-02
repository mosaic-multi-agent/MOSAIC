package cli

import (
	"fmt"
	"io"

	"mosaic-deploy/internal/domain"
)

// writeRegionWarnings prints one "warning: " line per TODO item that records orchestrator
// content that was not delivered or a role-forbidden region. Only deploy and update runs
// print them; items keep the order of the run summary.
func writeRegionWarnings(out io.Writer, summary domain.RunSummary) {
	if summary.Mode != domain.ModeDeployWorkspace && summary.Mode != domain.ModeUpdateWorkspace {
		return
	}
	for _, item := range summary.Todos {
		switch item.Kind {
		case domain.GapUndeliveredOrchestratorInjection, domain.GapRoleForbiddenRegion:
			fmt.Fprintf(out, "warning: %s\n", item.Detail)
		}
	}
}
