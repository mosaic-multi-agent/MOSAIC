package screens

import (
	"fmt"

	"mosaic-deploy/internal/domain"
)

// Labels naming the catalog file each injection version comes from, so a reviewer can tell
// whether the shared file, the orchestrator file, or both changed.
const (
	reviewSharedInjectionLabel       = "HarnessInjections.md version"
	reviewOrchestratorInjectionLabel = "HarnessInjectionsOrchestrator.md version"
)

// formatReviewVersionDelta renders one staleness comparison for the plan review. The two
// injection version fields are labelled by their source file; every other field renders
// exactly as formatVersionDelta does.
func formatReviewVersionDelta(delta domain.VersionDelta) string {
	var label string
	switch delta.Field {
	case "injections_version":
		label = reviewSharedInjectionLabel
	case "orchestrator_injections_version":
		label = reviewOrchestratorInjectionLabel
	default:
		return formatVersionDelta(delta)
	}
	source := delta.Source
	if source == "" {
		source = "deleted"
	}
	return fmt.Sprintf("%s: %s → %s", label, delta.Deployed, source)
}
