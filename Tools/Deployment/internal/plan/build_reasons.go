package plan

// build_reasons.go holds the helpers that turn staleness findings into plan item reasons. Each
// cause becomes its own reason, added in the order the causes are evaluated.

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
)

// noVersionInfoReason is the reason of an item whose deployed file carries no version stamps.
const noVersionInfoReason = "deployed file carries no readable version stamps"

// addVersionDeltaReasons adds one reason per version delta.
func addVersionDeltaReasons(item *domain.PlanItem, deltas []domain.VersionDelta) {
	for _, d := range deltas {
		item.AddReason(formatVersionDelta(d))
	}
}

// addAgentUpdateReasons adds the reasons of an agent update, one per cause.
//
// Order: version-field deltas; no-version-info note; workflow drift; protocol drift; bundle
// drift. Workflow, protocol, bundle and infrastructure deltas have dedicated reason methods and
// are excluded from the generic version-field formatter.
func addAgentUpdateReasons(item *domain.PlanItem, deltas []domain.VersionDelta, noVersionInfo bool, drift WorkflowDrift, protocolDrift ProtocolDrift, bundleDrift BundleDrift) {
	for _, d := range deltas {
		if strings.HasPrefix(d.Field, WorkflowDeltaFieldPrefix) || strings.HasPrefix(d.Field, InfrastructureDeltaFieldPrefix) {
			continue
		}
		if d.Field == ProtocolDeltaField || d.Field == BundleDeltaField {
			continue
		}
		item.AddReason(formatVersionDelta(d))
	}
	if noVersionInfo {
		item.AddReason(noVersionInfoReason)
	}
	if drift.Stale() {
		item.AddReason(drift.Reason())
	}
	if protocolDrift.Stale() {
		item.AddReason(protocolDrift.Reason())
	}
	if bundleDrift.Stale() {
		item.AddReason(bundleDrift.Reason())
	}
}

// withCatalogDrift appends the catalog delta after deltas and adds the catalog reason to item,
// when the drift is stale. It returns the extended deltas.
func withCatalogDrift(item *domain.PlanItem, d CatalogDrift, deltas []domain.VersionDelta) []domain.VersionDelta {
	delta, ok := d.Delta()
	if !ok {
		return deltas
	}
	item.AddReason(d.Reason())
	return append(deltas, delta)
}

// formatVersionDelta returns the human-readable reason of one version delta.
func formatVersionDelta(d domain.VersionDelta) string {
	return fmt.Sprintf("%s changed from %q to %q", d.Field, d.Deployed, d.Source)
}
