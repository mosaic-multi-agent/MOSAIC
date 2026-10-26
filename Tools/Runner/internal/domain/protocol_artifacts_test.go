package domain_test

// Tests that a subagent request always serializes its artifact lists as JSON
// arrays: a nil or empty list is "[]", never "null", and non-empty lists, key
// order and omitempty behaviour are unchanged.

import (
	"encoding/json"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// wireFields serializes v and returns the raw JSON value of each top-level key.
func wireFields(t *testing.T, v any) map[string]string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
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

func requireArtifactFieldsAre(t *testing.T, fields map[string]string, wantInput, wantOutput string) {
	t.Helper()
	if got := fields["input_artifacts"]; got != wantInput {
		t.Errorf("want input_artifacts %s, got %s", wantInput, got)
	}
	if got := fields["output_artifacts"]; got != wantOutput {
		t.Errorf("want output_artifacts %s, got %s", wantOutput, got)
	}
}

func TestProtocolRequest_NilArtifactLists_SerializeAsEmptyArrays(t *testing.T) {
	req := domain.ProtocolRequest{AgentInstanceID: "agent-a#1", TaskDescription: "do it"}

	fields := wireFields(t, req)

	requireArtifactFieldsAre(t, fields, "[]", "[]")
}

func TestProtocolRequest_EmptyNonNilArtifactLists_SerializeAsEmptyArrays(t *testing.T) {
	req := domain.ProtocolRequest{
		AgentInstanceID: "agent-a#1",
		InputArtifacts:  []string{},
		OutputArtifacts: []string{},
	}

	fields := wireFields(t, req)

	requireArtifactFieldsAre(t, fields, "[]", "[]")
}

func TestProtocolRequest_OnlyOneListNil_ThatListIsAnEmptyArrayAndTheOtherIsKept(t *testing.T) {
	cases := map[string]struct {
		req                    domain.ProtocolRequest
		wantInput, wantOutput string
	}{
		"nil input": {
			req:        domain.ProtocolRequest{OutputArtifacts: []string{"out.md"}},
			wantInput:  "[]",
			wantOutput: `["out.md"]`,
		},
		"nil output": {
			req:        domain.ProtocolRequest{InputArtifacts: []string{"a.md", "b.md"}},
			wantInput:  `["a.md","b.md"]`,
			wantOutput: "[]",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fields := wireFields(t, tc.req)

			requireArtifactFieldsAre(t, fields, tc.wantInput, tc.wantOutput)
		})
	}
}

func TestProtocolRequest_PointerReceiver_SerializesNilListsAsEmptyArrays(t *testing.T) {
	req := &domain.ProtocolRequest{AgentInstanceID: "agent-a#1"}

	fields := wireFields(t, req)

	requireArtifactFieldsAre(t, fields, "[]", "[]")
}

func TestProtocolRequest_NestedInOtherValues_SerializesNilListsAsEmptyArrays(t *testing.T) {
	type envelope struct {
		Requests []domain.ProtocolRequest `json:"requests"`
	}
	data, err := json.Marshal(envelope{Requests: []domain.ProtocolRequest{{AgentInstanceID: "agent-a#1"}}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(data)

	if strings.Contains(got, "null") {
		t.Errorf("want no null in the nested request, got %s", got)
	}
	if !strings.Contains(got, `"input_artifacts":[]`) || !strings.Contains(got, `"output_artifacts":[]`) {
		t.Errorf("want both artifact lists as empty arrays, got %s", got)
	}
}

func TestProtocolRequest_Marshal_DoesNotModifyTheRequest(t *testing.T) {
	req := domain.ProtocolRequest{AgentInstanceID: "agent-a#1"}

	if _, err := json.Marshal(req); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if req.InputArtifacts != nil || req.OutputArtifacts != nil {
		t.Errorf("want the request's nil lists left nil, got %#v and %#v", req.InputArtifacts, req.OutputArtifacts)
	}
}

// The complete wire form of a request with every field set, pinned so that key
// names, key order and list contents of existing consumers stay unchanged.
func TestProtocolRequest_FullyPopulated_SerializesExactlyAsBefore(t *testing.T) {
	req := domain.ProtocolRequest{
		AgentInstanceID:      "agent-a#3",
		RunID:                "run-1",
		TaskDescription:      "write the plan",
		InputArtifacts:       []string{"in-1.md", "in-2.md"},
		OutputArtifacts:      []string{"out.md"},
		InputFiles:           []string{"src/a.go"},
		OutputFiles:          []string{"src/b.go"},
		Constraints:          "keep it short",
		IncludeResultSummary: true,
		HumanInTheLoop:       true,
	}
	const want = `{"agent_instance_id":"agent-a#3","run_id":"run-1","task_description":"write the plan",` +
		`"input_artifacts":["in-1.md","in-2.md"],"output_artifacts":["out.md"],` +
		`"input_files":["src/a.go"],"output_files":["src/b.go"],"constraints":"keep it short",` +
		`"include_result_summary":true,"human_in_the_loop":true}`

	data, err := json.Marshal(req)

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != want {
		t.Errorf("want\n%s\ngot\n%s", want, data)
	}
}

// A minimal request keeps omitting the optional keys; only the two artifact
// lists change from null to arrays.
func TestProtocolRequest_Minimal_OmitsOptionalKeysAndKeepsKeyOrder(t *testing.T) {
	req := domain.ProtocolRequest{AgentInstanceID: "agent-a#1", TaskDescription: "t"}
	const want = `{"agent_instance_id":"agent-a#1","task_description":"t",` +
		`"input_artifacts":[],"output_artifacts":[],` +
		`"include_result_summary":false,"human_in_the_loop":false}`

	data, err := json.Marshal(req)

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != want {
		t.Errorf("want\n%s\ngot\n%s", want, data)
	}
}

func TestProtocolRequest_SerializedForm_DecodesBackWithTheSameNonEmptyLists(t *testing.T) {
	req := domain.ProtocolRequest{
		AgentInstanceID: "agent-a#1",
		InputArtifacts:  []string{"in.md"},
		OutputArtifacts: []string{"out.md"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got domain.ProtocolRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}

	if len(got.InputArtifacts) != 1 || got.InputArtifacts[0] != "in.md" ||
		len(got.OutputArtifacts) != 1 || got.OutputArtifacts[0] != "out.md" {
		t.Errorf("want lists kept through a round trip, got %#v and %#v", got.InputArtifacts, got.OutputArtifacts)
	}
}
