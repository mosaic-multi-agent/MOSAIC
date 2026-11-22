package builtin_test

// tool_fields_regression_test.go pins the tool fields that every harness other than
// opencode-v2 renders from fixed requests, and what the permission and list shapes read back
// from them. The pinned text is the output from before the rules tool shape existed: it must
// stay byte-identical, so every shared tool code path keeps its existing branches' output.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// v1Harnesses are the descriptors whose tool output must not change.
var v1Harnesses = []string{"claude-code", "opencode", "ghcp-cli", "vscode-ghcp", "codex"}

var allGenerics = []string{
	"file_read", "file_write", "file_edit", "file_search", "content_search",
	"terminal", "user_interaction", "subagent", "skill", "web_fetch", "web_search",
}

// toolRequests are the fixed requests, by label.
func toolRequests() map[string]domain.ToolRequest {
	return map[string]domain.ToolRequest{
		"all-generics": {AgentKey: "a", Generic: allGenerics},
		"read-only":    {AgentKey: "a", Generic: []string{"file_read"}},
		"none":         {AgentKey: "a"},
		"custom": {
			AgentKey: "a", Generic: []string{"file_read", "ask_user"},
			CustomNames: domain.CustomToolAnswers{"ask_user": {{Server: "hitl"}}},
		},
		"placeholder": {AgentKey: "orchestrator", Placeholder: "{tool-permissions}"},
	}
}

// canonical renders a field value as one line: kind, style and content.
func canonical(v domain.FieldValue) string {
	switch v.Kind {
	case domain.KindScalar:
		return fmt.Sprintf("scalar(%s)%q", v.Quote, v.Scalar)
	case domain.KindList:
		items := make([]string, len(v.Items))
		for i, it := range v.Items {
			items[i] = canonical(it)
		}
		return fmt.Sprintf("list(%s)[%s]", v.List, strings.Join(items, ", "))
	case domain.KindMapping:
		pairs := make([]string, len(v.Pairs))
		for i, p := range v.Pairs {
			pairs[i] = p.Key + ": " + canonical(p.Value)
		}
		return "map{" + strings.Join(pairs, "; ") + "}"
	}
	return "empty"
}

// renderTools maps one request and renders every field and resolution on lines.
func renderTools(t *testing.T, harness string, req domain.ToolRequest) string {
	t.Helper()
	d := loadEmbeddedDescriptor(t, harness+".yaml")
	res, err := descriptor.MapTools(d, req)
	if err != nil {
		t.Fatalf("%s: MapTools: %v", harness, err)
	}
	var b strings.Builder
	for _, f := range res.Fields {
		fmt.Fprintf(&b, "field %s = %s\n", f.Key, canonical(f.Value))
	}
	for _, r := range res.Resolutions {
		fmt.Fprintf(&b, "resolution %s %s %v\n", r.Generic, r.Outcome, r.HarnessTools)
	}
	return b.String()
}

func TestToolFields_OtherHarnessesRenderTheirPinnedOutput(t *testing.T) {
	for _, harness := range v1Harnesses {
		for label, req := range toolRequests() {
			t.Run(harness+"/"+label, func(t *testing.T) {
				got := renderTools(t, harness, req)

				want, ok := pinnedToolFields[harness+"/"+label]
				if !ok {
					t.Fatalf("no pinned output for %s/%s", harness, label)
				}
				if got != want {
					t.Errorf("rendered tool output changed.\nwant:\n%s\ngot:\n%s", want, got)
				}
			})
		}
	}
}

func TestToolFields_PermissionAndListShapesReadBackAsBefore(t *testing.T) {
	for _, harness := range v1Harnesses {
		d := loadEmbeddedDescriptor(t, harness+".yaml")
		res, err := descriptor.MapTools(d, toolRequests()["all-generics"])
		if err != nil {
			t.Fatalf("%s: MapTools: %v", harness, err)
		}
		if d.Frontmatter.ToolsKey == "" {
			continue
		}
		var value domain.FieldValue
		for _, f := range res.Fields {
			if f.Key == d.Frontmatter.ToolsKey {
				value = f.Value
			}
		}

		entries := fmt.Sprint(descriptor.ExtractToolEntries(d, value))
		grants := fmt.Sprint(descriptor.ExtractToolGrants(d, value))

		if want := pinnedReadBack[harness]; entries+"|"+grants != want {
			t.Errorf("%s: reading the tools field back changed.\nwant %s\ngot  %s", harness, want, entries+"|"+grants)
		}
	}
}
