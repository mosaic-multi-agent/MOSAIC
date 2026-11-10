package harness_test

// Tests that harness.MarshalRequest, the serializer every production adapter
// uses, emits both artifact lists of a request as JSON arrays.

import (
	"encoding/json"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

func marshalRequestFields(t *testing.T, req domain.ProtocolRequest) map[string]string {
	t.Helper()
	data, err := harness.MarshalRequest(req)
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = string(v)
	}
	return out
}

func TestMarshalRequest_NilArtifactLists_AreEmptyArrays(t *testing.T) {
	fields := marshalRequestFields(t, domain.ProtocolRequest{AgentInstanceID: "agent-a#1", TaskDescription: "t"})

	if got := fields["input_artifacts"]; got != "[]" {
		t.Errorf("want input_artifacts [], got %s", got)
	}
	if got := fields["output_artifacts"]; got != "[]" {
		t.Errorf("want output_artifacts [], got %s", got)
	}
}

func TestMarshalRequest_NonEmptyArtifactLists_AreKeptInOrder(t *testing.T) {
	fields := marshalRequestFields(t, domain.ProtocolRequest{
		AgentInstanceID: "agent-a#1",
		InputArtifacts:  []string{"b.md", "a.md"},
		OutputArtifacts: []string{"out.md"},
	})

	if got := fields["input_artifacts"]; got != `["b.md","a.md"]` {
		t.Errorf(`want input_artifacts ["b.md","a.md"], got %s`, got)
	}
	if got := fields["output_artifacts"]; got != `["out.md"]` {
		t.Errorf(`want output_artifacts ["out.md"], got %s`, got)
	}
}
