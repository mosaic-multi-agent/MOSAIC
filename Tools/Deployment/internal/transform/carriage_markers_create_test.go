package transform_test

// carriage_markers_create_test.go specifies how Apply treats the marker-carried key container
// (agentformat.CarriageMarkersKey) of a deployed document, depending on the operation.
//
// The container only names spans that live in the deployed file's raw bytes. A create has no
// prior raw bytes, so the container must not travel into the output of a create, even when the
// deployed document at the target path is non-nil (a file of another agent sits there). Every
// other untouched deployed key is still carried. For an update, and for an unspecified
// operation, the container is carried as before.

import (
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// deployedWithMarkersAndUserKey holds the marker container and one other user-owned key.
var deployedWithMarkersAndUserKey = `---
mosaic_id: 200
version: 1.0.0
` + agentformat.CarriageMarkersKey + `: ["mcp_servers"]
customField: "user-value"
---
Body.
`

func applyWithOp(t *testing.T, op agentformat.Operation) transform.Result {
	t.Helper()
	mod := newDescriptorModule(t, fieldPreservationDescriptorYAML, "inline:field-preservation")
	result, err := transform.Apply(transform.Request{
		Source:   []byte(fieldPreservationSource),
		Deployed: []byte(deployedWithMarkersAndUserKey),
		Kind:     domain.ArtifactAgent,
		Key:      "carriage-markers-op-test",
		Module:   mod,
		Model:    domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:    domain.ScopeProject,
		Op:       op,
	})
	if err != nil {
		t.Fatalf("Apply with op %v: %v", op, err)
	}
	return result
}

func reportKeys(r transform.Result) map[string]bool {
	keys := map[string]bool{}
	for _, f := range r.Report.Fields {
		keys[f.Key] = true
	}
	return keys
}

func TestApply_OpCreate_DoesNotCarryMarkerContainerFromDeployed(t *testing.T) {
	// Act
	result := applyWithOp(t, agentformat.OpCreate)

	// Assert
	if reportKeys(result)[agentformat.CarriageMarkersKey] {
		t.Errorf("a create reported the marker container as preserved from the deployed document; its spans exist only in raw bytes a create does not have")
	}
}

func TestApply_OpCreate_StillCarriesOtherUserKeyFromDeployed(t *testing.T) {
	// Act
	result := applyWithOp(t, agentformat.OpCreate)

	// Assert
	if !reportKeys(result)["customField"] {
		t.Errorf("a create dropped the user-owned key customField of the deployed document; only the marker container is withheld")
	}
}

func TestApply_NonCreateOps_CarryMarkerContainerFromDeployed(t *testing.T) {
	for name, op := range map[string]agentformat.Operation{
		"update":      agentformat.OpUpdate,
		"unspecified": agentformat.OpUnspecified,
	} {
		t.Run(name, func(t *testing.T) {
			// Act
			result := applyWithOp(t, op)

			// Assert
			keys := reportKeys(result)
			if !keys[agentformat.CarriageMarkersKey] {
				t.Errorf("op %s did not carry the marker container of the deployed document", name)
			}
			if !keys["customField"] {
				t.Errorf("op %s did not carry the user-owned key customField", name)
			}
		})
	}
}
