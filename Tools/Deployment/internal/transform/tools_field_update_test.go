package transform_test

// tools_field_update_test.go covers the tools field a deployed agent is written with by
// transform.Apply when the catalog's tool mapping changes, when the run produces custom tools,
// and for the permission shape: a mapping change reaches the written agent, run-produced custom
// tools are added when absent and never removed, and the catalog's allow/deny wins for built-in
// tools while custom tools keep their deployed disposition.

import (
	"reflect"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// mappingChangeDescriptorYAML maps generic_a to beta (alpha before the mapping change).
const mappingChangeDescriptorYAML = `schema_version: "1"
id: "mapping-change-harness"
display_name: "Mapping Change Harness"
tools:
  shape: list
  universe:
    - name: "alpha"
      unused: deny
      by_convention: false
    - name: "beta"
      unused: deny
      by_convention: false
  mappings:
    - generic: "generic_a"
      destinations:
        - to: main
          names:
            - "beta"
frontmatter:
  tools_key: "tools"
`

const mappingChangeSource = `---
id: 300
version: 1.0.0
description: Mapping change test agent
tools: [generic_a]
---
Body.
`

// mappingChangeDeployed was written when generic_a mapped to alpha.
const mappingChangeDeployed = `---
mosaic_id: 300
version: 1.0.0
mosaic_tool_mappings_version: oldhash
tools: [alpha, user-extra]
---
Body.
`

func applyTools(t *testing.T, descYAML, source, deployed string, mutate func(*transform.Request)) *docformat.Document {
	t.Helper()
	mod := newDescriptorModule(t, descYAML, "inline:tools-field-update")
	req := transform.Request{
		Source:   []byte(source),
		Deployed: []byte(deployed),
		Kind:     domain.ArtifactAgent,
		Key:      "tools-field-update-agent",
		Module:   mod,
		Model:    domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:    domain.ScopeProject,
	}
	if mutate != nil {
		mutate(&req)
	}
	result, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	doc, err := docformat.Parse(result.Output)
	if err != nil {
		t.Fatalf("Parse output: %v", err)
	}
	return doc
}

func TestApplyToolsField_MappingChange_ReachesWrittenAgentAndKeepsCustomTool(t *testing.T) {
	doc := applyTools(t, mappingChangeDescriptorYAML, mappingChangeSource, mappingChangeDeployed,
		func(r *transform.Request) { r.ToolMappingsVersion = "newhash" })

	want := []string{"beta", "user-extra"}
	if got := listFieldScalars(t, doc, "tools"); !reflect.DeepEqual(got, want) {
		t.Errorf("output tools = %v, want %v (alpha is no longer produced; beta replaces it; the custom tool stays)", got, want)
	}
	v, ok := doc.Frontmatter().Get("mosaic_tool_mappings_version")
	if !ok || v.Scalar != "newhash" {
		t.Errorf("mosaic_tool_mappings_version = %+v (present %v), want the new stamp %q", v, ok, "newhash")
	}
}

func TestApplyToolsField_AppendTools_AddedWhenAbsentFromDeployed(t *testing.T) {
	doc := applyTools(t, preservationListDescriptorYAML, preservationSourceAB, deployedListABWithUserExtra,
		func(r *transform.Request) { r.AppendTools = []string{"run-server"} })

	want := []string{"alpha", "beta", "run-server", "user-extra"}
	if got := listFieldScalars(t, doc, "tools"); !reflect.DeepEqual(got, want) {
		t.Errorf("output tools = %v, want %v", got, want)
	}
}

func TestApplyToolsField_AppendTools_AlreadyDeployedIsNotDuplicated(t *testing.T) {
	const deployed = `---
mosaic_id: 100
version: 1.0.0
tools: [alpha, beta, run-server]
---
Body.
`
	doc := applyTools(t, preservationListDescriptorYAML, preservationSourceAB, deployed,
		func(r *transform.Request) { r.AppendTools = []string{"run-server"} })

	items := listFieldScalars(t, doc, "tools")
	if n := countOccurrences(items, "run-server"); n != 1 {
		t.Errorf("run-server appears %d times in %v, want once", n, items)
	}
}

func TestApplyToolsField_RunWithoutCustomAnswer_NeverRemovesDeployedCustomTool(t *testing.T) {
	doc := applyTools(t, preservationListDescriptorYAML, preservationSourceAB, deployedListABWithUserExtra, nil)

	if !toolListContains(doc, "tools", "user-extra") {
		t.Errorf("custom tool %q removed; Deploy never removes custom tools; got %v",
			"user-extra", listFieldScalars(t, doc, "tools"))
	}
}

// permDispositions returns the pairs of the output's permission-shaped tools field by name.
func permDispositions(t *testing.T, doc *docformat.Document, key string) map[string]string {
	t.Helper()
	v, ok := doc.Frontmatter().Get(key)
	if !ok || v.Kind != domain.KindMapping {
		t.Fatalf("field %q absent or not a mapping in output", key)
	}
	out := map[string]string{}
	for _, p := range v.Pairs {
		out[p.Key] = p.Value.Scalar
	}
	return out
}

func TestApplyToolsField_PermissionShape_CatalogWinsForBuiltInCustomKeepsDeployedDeny(t *testing.T) {
	// Catalog grants alpha only; beta is built-in and unused (deny). The user had flipped beta to
	// allow and added a denied custom server entry and an allowed one.
	const deployed = `---
mosaic_id: 103
version: 1.0.0
tools:
  alpha: deny
  beta: allow
  srv_*: deny
  mine: allow
---
Body.
`
	const source = `---
id: 103
version: 1.0.0
description: Permission update test agent
tools: [generic_a]
---
Body.
`
	doc := applyTools(t, preservationPermissionDescriptorYAML, source, deployed, nil)

	got := permDispositions(t, doc, "tools")
	want := map[string]string{"alpha": "allow", "beta": "deny", "srv_*": "deny", "mine": "allow"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("permission tools = %v, want %v (built-ins from the catalog, custom as deployed)", got, want)
	}
}

func TestApplyToolsField_PermissionShape_AppendedCustomAddedAsAllowAndDeployedDenyKept(t *testing.T) {
	const deployed = `---
mosaic_id: 103
version: 1.0.0
tools:
  alpha: allow
  beta: allow
  keep_*: deny
---
Body.
`
	doc := applyTools(t, preservationPermissionDescriptorYAML, preservationPermissionSource, deployed,
		func(r *transform.Request) { r.AppendTools = []string{"new_*"} })

	got := permDispositions(t, doc, "tools")
	if got["new_*"] != "allow" {
		t.Errorf("new_* = %q, want %q for a run-produced custom tool", got["new_*"], "allow")
	}
	if got["keep_*"] != "deny" {
		t.Errorf("keep_* = %q, want the deployed %q kept", got["keep_*"], "deny")
	}
}
