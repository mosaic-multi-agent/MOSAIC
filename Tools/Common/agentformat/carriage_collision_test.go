package agentformat_test

// carriage_collision_test.go asserts that neither carriage key
// ("mosaic_carriage", "mosaic_carriage_markers") equals any agentfields
// Deployed name or Legacy name. A collision would cause the container to be
// treated as a MOSAIC stamp. The descriptor-list side of this rule is checked
// in Deploy, next to the descriptors.

import (
	"testing"

	"mosaic-common/agentfields"
	"mosaic-common/agentformat"
)

// TestCarriageKeyCollision_NotInAgentfields verifies that neither carriage key
// equals any agentfields Deployed or Legacy name. A collision would cause
// isMosaicOwned to return true for the container key, which would put the container
// into the stamp block rather than being consumed by the carriage path.
func TestCarriageKeyCollision_NotInAgentfields(t *testing.T) {
	for _, carriageKey := range agentformat.CarriageKeys() {
		for _, field := range agentfields.All() {
			if carriageKey == field.Deployed {
				t.Errorf("carriage key %q collides with agentfields Deployed name %q; this would cause the container to be treated as a MOSAIC stamp", carriageKey, field.Deployed)
			}
			if carriageKey == field.Legacy {
				t.Errorf("carriage key %q collides with agentfields Legacy name %q; this would cause isMosaicOwned to misclassify the container", carriageKey, field.Legacy)
			}
		}
	}
}
