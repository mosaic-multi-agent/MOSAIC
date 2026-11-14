package transform_test

// tools_field_merge_test.go specifies transform.MergeToolsField, the pure decision of the tools
// field a deployed agent is written with: built-in tools (exactly the descriptor's tool
// universe) follow the catalog, custom tools (every other name, MCP forms included) are kept as
// deployed, and custom tools the run produces are added when absent. The change of built-in
// names is reported as added and removed.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// mergeDescriptor returns a descriptor with the universe read, edit, bash, a mapping whose
// destination name "dest-only" is outside the universe, and the given shape.
func mergeDescriptor(shape domain.ToolShape) *domain.HarnessDescriptor {
	universe := []domain.HarnessTool{{Name: "read"}, {Name: "edit"}, {Name: "bash"}}
	return &domain.HarnessDescriptor{
		ID: "merge-harness",
		Tools: domain.ToolSpec{
			Shape:    shape,
			Universe: universe,
			Mappings: []domain.ToolMapping{{
				Generic: "generic_x",
				Destinations: []domain.ToolDestination{
					{Kind: domain.DestMain, Names: []string{"dest-only"}},
				},
			}},
		},
		Frontmatter: domain.FrontmatterSpec{ToolsKey: "tools"},
	}
}

// mergeList builds a list-shaped tools value.
func mergeList(names ...string) domain.FieldValue {
	items := make([]domain.FieldValue, len(names))
	for i, n := range names {
		items[i] = domain.ScalarValue(n, domain.QuotePlain)
	}
	return domain.ListValue(items, domain.ListFlow)
}

// grant is one name with its disposition in a permission-shaped value.
type grant struct {
	name string
	disp domain.Disposition
}

// mergePerm builds a permission-shaped tools value.
func mergePerm(grants ...grant) domain.FieldValue {
	pairs := make([]domain.FieldPair, len(grants))
	for i, g := range grants {
		pairs[i] = domain.FieldPair{Key: g.name, Value: domain.ScalarValue(string(g.disp), domain.QuotePlain)}
	}
	return domain.MappingValue(pairs)
}

// listNames returns the scalar items of a list value, nil when it is not a list.
func listNames(v domain.FieldValue) []string {
	if v.Kind != domain.KindList {
		return nil
	}
	var out []string
	for _, it := range v.Items {
		out = append(out, it.Scalar)
	}
	return out
}

// permGrants returns the pairs of a mapping value as grants, nil when it is not a mapping.
func permGrants(v domain.FieldValue) []grant {
	if v.Kind != domain.KindMapping {
		return nil
	}
	var out []grant
	for _, p := range v.Pairs {
		out = append(out, grant{p.Key, domain.Disposition(p.Value.Scalar)})
	}
	return out
}

func assertNames(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestMergeToolsField_List_BuiltInFollowsCatalogAddedAndRemoved(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("read", "edit"), mergeList("read", "bash"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "edit"})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"edit"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"bash"})
}

func TestMergeToolsField_List_CatalogChangeOnlyChangesBuiltIn_NoChangeReportedWhenEqual(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("read", "edit"), mergeList("edit", "read"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "edit"})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, nil)
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_List_CustomNamesKeptWhenCatalogDoesNotProduceThem(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("read"), mergeList("read", "my-tool", "bash"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "my-tool"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"bash"})
}

func TestMergeToolsField_List_McpFormsAreCustomAndKeptInDeployedOrder(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	deployed := mergeList("mcp__srv__*", "read", "srv/tool", "srv_*", "mcp__other__lookup")

	got := transform.MergeToolsField(d, mergeList("read", "edit"), deployed, true)

	assertNames(t, "Value", listNames(got.Value),
		[]string{"read", "edit", "mcp__srv__*", "srv/tool", "srv_*", "mcp__other__lookup"})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"edit"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_List_NamesOutsideUniverseAreCustom(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	// A mapping destination name and a hierarchical name that is not in the universe.
	deployed := mergeList("read", "dest-only", "read/readFile")

	got := transform.MergeToolsField(d, mergeList("read"), deployed, true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "dest-only", "read/readFile"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_List_ByConventionToolOutsideUniverseIsCustom(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	d.Tools.Universe = append(d.Tools.Universe, domain.HarnessTool{Name: "conv-universe", ByConvention: true})

	got := transform.MergeToolsField(d, mergeList("read"), mergeList("read", "conv-other"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "conv-other"})
}

func TestMergeToolsField_List_ByConventionUniverseToolIsBuiltInAndRemovedWhenCatalogOmitsIt(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	d.Tools.Universe = append(d.Tools.Universe, domain.HarnessTool{Name: "conv-universe", ByConvention: true})

	got := transform.MergeToolsField(d, mergeList("read"), mergeList("read", "conv-universe"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"conv-universe"})
}

func TestMergeToolsField_List_ByConventionUniverseToolIsBuiltInAndAddedWhenCatalogGrantsIt(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	d.Tools.Universe = append(d.Tools.Universe, domain.HarnessTool{Name: "conv-universe", ByConvention: true})

	got := transform.MergeToolsField(d, mergeList("read", "conv-universe"), mergeList("read"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "conv-universe"})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"conv-universe"})
}

func TestMergeToolsField_List_RunProducedCustomAddedWhenAbsent(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("read", "new-server"), mergeList("read", "mine"), true)

	// Order: built-in of generated, custom of generated absent from deployed, custom of deployed.
	assertNames(t, "Value", listNames(got.Value), []string{"read", "new-server", "mine"})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, nil)
}

func TestMergeToolsField_List_RunProducedCustomAlreadyDeployedAppearsOnce(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("read", "shared"), mergeList("shared", "read"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "shared"})
}

func TestMergeToolsField_List_BuiltInOrderFollowsGenerated(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)

	got := transform.MergeToolsField(d, mergeList("bash", "read"), mergeList("read", "bash", "edit"), true)

	assertNames(t, "Value", listNames(got.Value), []string{"bash", "read"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"edit"})
}

func TestMergeToolsField_DeployedAbsent_ReturnsGeneratedUnchanged(t *testing.T) {
	d := mergeDescriptor(domain.ShapeList)
	generated := mergeList("read", "edit", "run-server")

	got := transform.MergeToolsField(d, generated, domain.FieldValue{}, false)

	assertNames(t, "Value", listNames(got.Value), []string{"read", "edit", "run-server"})
}

func TestMergeToolsField_Permission_CatalogAllowDenyWinsForBuiltIn(t *testing.T) {
	d := mergeDescriptor(domain.ShapePermission)
	generated := mergePerm(grant{"read", domain.Allow}, grant{"edit", domain.Deny}, grant{"bash", domain.Deny})
	deployed := mergePerm(grant{"read", domain.Deny}, grant{"edit", domain.Allow}, grant{"bash", domain.Allow})

	got := transform.MergeToolsField(d, generated, deployed, true)

	want := []grant{{"read", domain.Allow}, {"edit", domain.Deny}, {"bash", domain.Deny}}
	if !reflect.DeepEqual(permGrants(got.Value), want) {
		t.Errorf("Value = %v, want %v", permGrants(got.Value), want)
	}
	// Granted means allow: read becomes granted, edit and bash stop being granted.
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"read"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"edit", "bash"})
}

func TestMergeToolsField_Permission_CustomKeptExactlyWithAllowAndDeny(t *testing.T) {
	d := mergeDescriptor(domain.ShapePermission)
	generated := mergePerm(grant{"read", domain.Allow})
	deployed := mergePerm(grant{"read", domain.Allow}, grant{"srv_*", domain.Deny},
		grant{"mine", domain.Allow}, grant{"mcp__x__y", domain.Deny})

	got := transform.MergeToolsField(d, generated, deployed, true)

	want := []grant{{"read", domain.Allow}, {"srv_*", domain.Deny}, {"mine", domain.Allow}, {"mcp__x__y", domain.Deny}}
	if !reflect.DeepEqual(permGrants(got.Value), want) {
		t.Errorf("Value = %v, want %v", permGrants(got.Value), want)
	}
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, nil)
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_Permission_DeployedDispositionWinsForCustomPresentInBoth(t *testing.T) {
	d := mergeDescriptor(domain.ShapePermission)
	generated := mergePerm(grant{"read", domain.Allow}, grant{"srv_*", domain.Allow})
	deployed := mergePerm(grant{"srv_*", domain.Deny})

	got := transform.MergeToolsField(d, generated, deployed, true)

	want := []grant{{"read", domain.Allow}, {"srv_*", domain.Deny}}
	if !reflect.DeepEqual(permGrants(got.Value), want) {
		t.Errorf("Value = %v, want %v", permGrants(got.Value), want)
	}
}

func TestMergeToolsField_Permission_RunProducedCustomAddedWithGeneratedDispositionWhenAbsent(t *testing.T) {
	d := mergeDescriptor(domain.ShapePermission)
	generated := mergePerm(grant{"read", domain.Allow}, grant{"new_*", domain.Allow})
	deployed := mergePerm(grant{"read", domain.Allow}, grant{"mine", domain.Deny})

	got := transform.MergeToolsField(d, generated, deployed, true)

	want := []grant{{"read", domain.Allow}, {"new_*", domain.Allow}, {"mine", domain.Deny}}
	if !reflect.DeepEqual(permGrants(got.Value), want) {
		t.Errorf("Value = %v, want %v", permGrants(got.Value), want)
	}
}

func TestMergeToolsField_Permission_BuiltInAbsentFromGeneratedIsNotCarried(t *testing.T) {
	d := mergeDescriptor(domain.ShapePermission)
	generated := mergePerm(grant{"read", domain.Allow})
	deployed := mergePerm(grant{"read", domain.Allow}, grant{"bash", domain.Allow})

	got := transform.MergeToolsField(d, generated, deployed, true)

	want := []grant{{"read", domain.Allow}}
	if !reflect.DeepEqual(permGrants(got.Value), want) {
		t.Errorf("Value = %v, want %v", permGrants(got.Value), want)
	}
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"bash"})
}
