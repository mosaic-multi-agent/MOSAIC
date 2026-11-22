package external_test

// external_custom_tools_test.go verifies that a custom tool answer made of MCP entries reaches
// an external module over the wire and comes back written in the module's own forms, and that
// the protocol version marks the new wire shape.
//
// Coverage:
//   - A module that still answers protocol 1.0 is refused at handshake (the version constant
//     itself is pinned in external_test.go).
//   - The tools request carries "custom_tools" (generic -> entries with server and optional
//     tool) and no longer carries "custom_names"; with no answers the key is absent.
//   - The bundled OpenCode module writes every entry in OpenCode's whole-server or single-tool
//     form and resolves the generic as custom.
//   - The shared contract suite passes for the bundled module with a multi-entry answer.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/contracttest"
	"mosaic-deploy/internal/harness/external"
)

// runFakeHarnessCustomToolsReflect answers the handshake and descriptor requests and, for a
// tools request, returns the raw params it received as the single harness tool name of one
// custom resolution, so the test can inspect the exact wire shape.
func runFakeHarnessCustomToolsReflect() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		var req map[string]any
		if err := dec.Decode(&req); err != nil {
			return
		}
		id, _ := req["id"].(string)
		method, _ := req["method"].(string)
		reply := func(result map[string]any) {
			_ = enc.Encode(map[string]any{"protocol": external.ProtocolVersion, "id": id, "result": result})
		}
		switch method {
		case "handshake":
			reply(map[string]any{
				"protocol": external.ProtocolVersion,
				"harness":  map[string]any{"id": "custom-tools-reflect", "tier": "external", "usable": true},
			})
		case "descriptor":
			reply(map[string]any{"descriptor": map[string]any{
				"schema_version": "1", "id": "custom-tools-reflect", "display_name": "Custom Tools Reflect",
			}})
		case "tools":
			raw, _ := json.Marshal(req["params"])
			reply(map[string]any{
				"fields": []any{},
				"resolutions": []any{map[string]any{
					"generic": "terminal", "outcome": "custom", "harness_tools": []string{string(raw)},
				}},
			})
		default:
			_ = enc.Encode(map[string]any{
				"protocol": external.ProtocolVersion, "id": id,
				"error": map[string]any{"code": "unsupported_method", "message": fmt.Sprintf("unsupported method %s", method)},
			})
		}
	}
}

// runFakeHarnessProtocolOnePointZero answers the handshake as a module built for protocol 1.0.
func runFakeHarnessProtocolOnePointZero() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var req map[string]any
	if err := dec.Decode(&req); err != nil {
		return
	}
	id, _ := req["id"].(string)
	_ = enc.Encode(map[string]any{
		"protocol": "1.0",
		"id":       id,
		"result": map[string]any{
			"protocol": "1.0",
			"harness":  map[string]any{"id": "old-module", "tier": "external", "usable": true},
		},
	})
}

// sentToolsParams sends the request through the reflecting fake module and returns the tools
// params object the module received.
func sentToolsParams(t *testing.T, req domain.ToolRequest) map[string]any {
	t.Helper()
	m, err := external.New(fakeHarnessExe(t, "custom-tools-reflect"), minimalDescriptor(t, "custom-tools-reflect"),
		external.Options{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("external.New: %v", err)
	}
	defer m.Close() //nolint:errcheck

	result, err := m.Tools(req)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(result.Resolutions) != 1 || len(result.Resolutions[0].HarnessTools) != 1 {
		t.Fatalf("want one resolution carrying the reflected params, got %+v", result.Resolutions)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(result.Resolutions[0].HarnessTools[0]), &params); err != nil {
		t.Fatalf("reflected params are not JSON: %v", err)
	}
	return params
}

func TestCustomTools_ModuleAnsweringProtocolOnePointZeroIsRefusedAtHandshake(t *testing.T) {
	_, err := external.New(fakeHarnessExe(t, "protocol-1-0"), minimalDescriptor(t, "old-module"),
		external.Options{Timeout: 5 * time.Second})

	if !errors.Is(err, external.ErrProtocolMismatch) {
		t.Errorf("external.New with a 1.0 module: got %v, want errors.Is(err, ErrProtocolMismatch)", err)
	}
}

func TestCustomTools_RequestCarriesEntriesPerGenericInAnswerOrder(t *testing.T) {
	params := sentToolsParams(t, domain.ToolRequest{
		AgentKey: "a",
		Generic:  []string{"terminal"},
		CustomNames: domain.CustomToolAnswers{"terminal": {
			{Server: "github", Tool: "get_pr"}, {Server: "jira"},
		}},
	})

	custom, ok := params["custom_tools"].(map[string]any)
	if !ok {
		t.Fatalf("tools params lack custom_tools; got %v", params)
	}
	entries, _ := custom["terminal"].([]any)
	if len(entries) != 2 {
		t.Fatalf("custom_tools[terminal] = %v, want two entries", custom["terminal"])
	}
	first, _ := entries[0].(map[string]any)
	second, _ := entries[1].(map[string]any)
	if first["server"] != "github" || first["tool"] != "get_pr" {
		t.Errorf("first entry = %v, want server github, tool get_pr", first)
	}
	if second["server"] != "jira" {
		t.Errorf("second entry = %v, want server jira", second)
	}
	if tool, present := second["tool"]; present && tool != "" {
		t.Errorf("a whole-server entry must carry no tool; got %v", tool)
	}
}

func TestCustomTools_RequestNoLongerCarriesCustomNames(t *testing.T) {
	params := sentToolsParams(t, domain.ToolRequest{
		AgentKey:    "a",
		Generic:     []string{"terminal"},
		CustomNames: domain.CustomToolAnswers{"terminal": {{Server: "jira"}}},
	})

	if _, present := params["custom_names"]; present {
		t.Errorf("tools params still carry custom_names: %v", params)
	}
}

func TestCustomTools_RequestWithoutAnswersOmitsTheKey(t *testing.T) {
	params := sentToolsParams(t, domain.ToolRequest{AgentKey: "a", Generic: []string{"terminal"}})

	if _, present := params["custom_tools"]; present {
		t.Errorf("tools params carry custom_tools without any answer: %v", params)
	}
}

// openReferenceModule starts the bundled OpenCode module as an external module.
func openReferenceModule(t *testing.T) domain.HarnessModule {
	t.Helper()
	m, err := external.New(buildReferenceModule(t), openCodeDescriptor(t), external.Options{
		Timeout:    10 * time.Second,
		MosaicRoot: frozenCatalogRoot(t),
	})
	if err != nil {
		t.Fatalf("external.New with reference module binary: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestReferenceModule_MixedEntriesAreWrittenInOpenCodeForms(t *testing.T) {
	m := openReferenceModule(t)

	result, err := m.Tools(domain.ToolRequest{
		AgentKey: "mcp-user",
		Generic:  []string{"user_feedback"},
		CustomNames: domain.CustomToolAnswers{"user_feedback": {
			{Server: "github", Tool: "get_pr"}, {Server: "github", Tool: "list_comments"}, {Server: "jira"},
		}},
	})
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	if len(result.Resolutions) != 1 || result.Resolutions[0].Outcome != domain.ToolCustom {
		t.Fatalf("want one custom resolution, got %+v", result.Resolutions)
	}
	want := []string{"github_get_pr", "github_list_comments", "jira_*"}
	if got := result.Resolutions[0].HarnessTools; !reflect.DeepEqual(got, want) {
		t.Errorf("HarnessTools = %v, want %v", got, want)
	}
	allowed := map[string]string{}
	for _, f := range result.Fields {
		for _, p := range f.Value.Pairs {
			allowed[p.Key] = p.Value.Scalar
		}
	}
	for _, n := range want {
		if allowed[n] != "allow" {
			t.Errorf("permission map must carry %s: allow; got %v", n, allowed)
		}
	}
}

func TestReferenceModule_PassesTheContractSuiteForAMultiEntryAnswer(t *testing.T) {
	m := openReferenceModule(t)

	pairs := permissionPairs(map[string]bool{"read": true})
	for _, n := range []string{"github_get_pr", "jira_*"} {
		pairs = append(pairs, domain.FieldPair{Key: n, Value: domain.ScalarValue("allow", domain.QuotePlain)})
	}

	contracttest.Run(t, m, contracttest.Fixtures{
		ToolCases: []contracttest.ToolCase{{
			Name: "multi_entry_custom_answer",
			Request: domain.ToolRequest{
				AgentKey: "mcp-user",
				Generic:  []string{"file_read", "user_feedback"},
				CustomNames: domain.CustomToolAnswers{"user_feedback": {
					{Server: "github", Tool: "get_pr"}, {Server: "jira"},
				}},
			},
			Fields: []domain.FrontmatterField{{Key: "permission", Value: domain.MappingValue(pairs)}},
			Resolutions: []domain.ToolResolution{
				{Generic: "file_read", Outcome: domain.ToolMapped, HarnessTools: []string{"read"}},
				{Generic: "user_feedback", Outcome: domain.ToolCustom, HarnessTools: []string{"github_get_pr", "jira_*"}},
			},
		}},
	})
}
