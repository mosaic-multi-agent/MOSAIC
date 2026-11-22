package builtin_test

// Tests that the shared opencode-v2 descriptor, as Deploy reads it, carries the
// V2 tool set, the generic-to-V2 mappings and the V2 file conventions.

import (
	"sort"
	"testing"

	"mosaic-deploy/internal/domain"
)

// v2Universe is the V2 tool set: no list, no todoread, and todowrite only
// behind an experimental flag, so it is not part of the universe.
var v2Universe = []string{
	"read", "write", "edit", "patch", "glob", "grep", "shell",
	"webfetch", "websearch", "question", "skill", "subagent", "execute",
}

// v2Mappings are the generic tools with their V2 main-destination names.
var v2Mappings = map[string][]string{
	"file_read":        {"read"},
	"file_write":       {"write", "patch"},
	"file_edit":        {"edit", "patch"},
	"file_search":      {"glob"},
	"content_search":   {"grep"},
	"terminal":         {"shell"},
	"user_interaction": {"question"},
	"subagent":         {"subagent"},
	"skill":            {"skill"},
	"web_fetch":        {"webfetch"},
	"web_search":       {"websearch"},
}

func loadV2(t *testing.T) *domain.HarnessDescriptor {
	t.Helper()
	return loadEmbeddedDescriptor(t, "opencode-v2.yaml")
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestOpenCodeV2Descriptor_UniverseIsTheV2ToolSet(t *testing.T) {
	d := loadV2(t)

	var got []string
	for _, u := range d.Tools.Universe {
		got = append(got, u.Name)
	}

	if !slicesEqual(sortedCopy(got), sortedCopy(v2Universe)) {
		t.Errorf("universe = %v, want exactly %v", got, v2Universe)
	}
}

func TestOpenCodeV2Descriptor_UniverseDeniesEveryToolByDefault(t *testing.T) {
	d := loadV2(t)

	if len(d.Tools.Universe) == 0 {
		t.Fatal("universe is empty")
	}
	for _, u := range d.Tools.Universe {
		if u.Unused != domain.Deny {
			t.Errorf("%s: Unused = %q, want %q", u.Name, u.Unused, domain.Deny)
		}
		if u.ByConvention {
			t.Errorf("%s: ByConvention = true, want false", u.Name)
		}
	}
}

func TestOpenCodeV2Descriptor_UniverseExcludesRemovedAndFlaggedTools(t *testing.T) {
	d := loadV2(t)

	for _, absent := range []string{"list", "todoread", "todowrite", "lsp"} {
		if indexInUniverse(d.Tools.Universe, absent) >= 0 {
			t.Errorf("universe contains %q, which the V2 tool set does not grant", absent)
		}
	}
}

func TestOpenCodeV2Descriptor_GenericMappingsMatchTheV2ToolSet(t *testing.T) {
	d := loadV2(t)

	got := map[string][]string{}
	for _, m := range d.Tools.Mappings {
		for _, dest := range m.Destinations {
			if dest.Kind == domain.DestMain {
				got[m.Generic] = append(got[m.Generic], dest.Names...)
			}
		}
	}

	for generic, want := range v2Mappings {
		if !slicesEqual(sortedCopy(got[generic]), sortedCopy(want)) {
			t.Errorf("mapping %s = %v, want %v", generic, got[generic], want)
		}
	}
}

func TestOpenCodeV2Descriptor_MappingsOnlyNameUniverseTools(t *testing.T) {
	d := loadV2(t)

	for _, m := range d.Tools.Mappings {
		for _, dest := range m.Destinations {
			for _, name := range dest.Names {
				if indexInUniverse(d.Tools.Universe, name) < 0 {
					t.Errorf("mapping %s names %q, which is not a universe tool", m.Generic, name)
				}
			}
		}
	}
}

func TestOpenCodeV2Descriptor_NoMappingGrantsToolsOutsideTheDefaultSet(t *testing.T) {
	d := loadV2(t)

	for _, m := range d.Tools.Mappings {
		for _, dest := range m.Destinations {
			for _, name := range dest.Names {
				switch name {
				case "execute", "todowrite", "todoread", "list":
					t.Errorf("mapping %s grants %q by default", m.Generic, name)
				}
			}
		}
	}
}

func TestOpenCodeV2Descriptor_PlaceholderExpansionStaysInsideTheUniverse(t *testing.T) {
	d := loadV2(t)

	if len(d.Tools.PlaceholderExpansion) == 0 {
		t.Fatal("placeholder_expansion is empty")
	}
	for _, name := range d.Tools.PlaceholderExpansion {
		if indexInUniverse(d.Tools.Universe, name) < 0 {
			t.Errorf("placeholder_expansion names %q, which is not a universe tool", name)
		}
		if name == "execute" {
			t.Errorf("placeholder_expansion names %q, which stays denied", name)
		}
	}
}

func TestOpenCodeV2Descriptor_FileConventions(t *testing.T) {
	d := loadV2(t)

	if d.ID != "opencode-v2" {
		t.Errorf("ID = %q, want %q", d.ID, "opencode-v2")
	}
	if got := d.Paths.Agents.Project; got != ".opencode/agents" {
		t.Errorf("agents path = %q, want %q", got, ".opencode/agents")
	}
	if got := d.Paths.Skills.Project; got != ".opencode/skills" {
		t.Errorf("skills path = %q, want %q", got, ".opencode/skills")
	}
	if got := d.Paths.Hooks.Project; got != ".opencode/plugins" {
		t.Errorf("hooks path = %q, want %q", got, ".opencode/plugins")
	}
	if got := d.Extensions[domain.ArtifactAgent]; got != ".md" {
		t.Errorf("agent extension = %q, want %q", got, ".md")
	}
	if !d.Hooks.Supported || d.Hooks.VariantKey != "opencode-v2" {
		t.Errorf("hooks = %+v, want supported with variant key %q", d.Hooks, "opencode-v2")
	}
}

func TestOpenCodeV2Descriptor_FrontmatterUsesV2PermissionsKey(t *testing.T) {
	d := loadV2(t)

	if d.Frontmatter.ToolsKey != "permissions" {
		t.Errorf("ToolsKey = %q, want %q", d.Frontmatter.ToolsKey, "permissions")
	}
	if d.Frontmatter.ModelKey != "model" {
		t.Errorf("ModelKey = %q, want %q", d.Frontmatter.ModelKey, "model")
	}
	if !containsStr(d.Frontmatter.Drop, "permission") {
		t.Errorf("Drop = %v, want it to drop the V1 %q key", d.Frontmatter.Drop, "permission")
	}
}

func TestOpenCodeV2Descriptor_EveryAgentGetsAnExplicitModeByRole(t *testing.T) {
	d := loadV2(t)

	want := map[domain.AgentRole]string{
		domain.RoleSubagent:     "subagent",
		domain.RoleOrchestrator: "primary",
		domain.RoleUtility:      "primary",
		domain.RoleStandalone:   "primary",
	}
	var mode *domain.RoleConditionalField
	for i := range d.Frontmatter.RoleConditionalAdd {
		if d.Frontmatter.RoleConditionalAdd[i].Key == "mode" {
			mode = &d.Frontmatter.RoleConditionalAdd[i]
		}
	}
	if mode == nil {
		t.Fatalf("no role-conditional %q field; a missing mode makes a V2 agent primary", "mode")
	}
	for role, wantValue := range want {
		v, ok := mode.ValueByRole[role]
		if !ok {
			t.Errorf("mode has no value for role %q", role)
			continue
		}
		if v.Scalar != wantValue {
			t.Errorf("mode for role %q = %q, want %q", role, v.Scalar, wantValue)
		}
	}
}

func TestOpenCodeV2Descriptor_CarriesExperimentalReason(t *testing.T) {
	d := loadV2(t)

	if d.ExperimentalReason == "" {
		t.Error("ExperimentalReason is empty, want the descriptor's experimental reason")
	}
}
