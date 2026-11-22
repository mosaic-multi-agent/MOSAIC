package harness

import (
	"strings"

	commonharness "mosaic-common/harness"
)

// ExperimentalLabel is the marker word every picker shows.
const ExperimentalLabel = "Experimental"

// experimentalHarnesses lists harnesses the Runner marks experimental, with
// the reason shown to the user. It is declared in the Runner, independent of
// any other tool's own marking. Every reason must be non-empty. Codex is marked
// because its support is not yet proven in real runs.
var experimentalHarnesses = map[string]string{
	commonharness.HarnessIDCodex: "Codex support is not yet proven in real runs.",
}

// ExperimentalReason reports whether id is marked experimental in the Runner,
// and why. The Runner's own marking wins; without one, the shared descriptor's
// experimental reason applies. The reason of every marking is non-empty.
func ExperimentalReason(id string) (reason string, ok bool) {
	if reason, ok = experimentalHarnesses[id]; ok {
		return reason, true
	}
	if entry, found := commonharness.LookupCLIHarness(id); found && strings.TrimSpace(entry.ExperimentalReason) != "" {
		return entry.ExperimentalReason, true
	}
	return "", false
}

// DisplayLabel is the label a picker shows: the plain label, followed by the
// experimental marker and reason when the harness is marked.
func (s Selection) DisplayLabel() string {
	if s.ExperimentalReason == "" {
		return s.Label
	}
	return s.Label + " [" + ExperimentalLabel + ": " + s.ExperimentalReason + "]"
}

// MarkExperimentalForTest marks id experimental with reason for the current
// process; the returned function restores the previous state. Tests only;
// production code never calls it. Panics on an empty or blank reason.
func MarkExperimentalForTest(id, reason string) (restore func()) {
	if strings.TrimSpace(reason) == "" {
		panic("harness: experimental marking requires a non-empty reason")
	}
	previous, had := experimentalHarnesses[id]
	experimentalHarnesses[id] = reason
	return func() {
		if had {
			experimentalHarnesses[id] = previous
		} else {
			delete(experimentalHarnesses, id)
		}
	}
}
