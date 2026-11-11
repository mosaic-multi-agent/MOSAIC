package descriptor_test

// unknown_tools_test.go verifies UnknownToolNames: which of the tool names written for a
// harness that ignores unknown names are neither known tools nor valid MCP names.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func reportingDescriptor(report bool) *domain.HarnessDescriptor {
	return &domain.HarnessDescriptor{
		ID: "reporting",
		Tools: domain.ToolSpec{
			Shape:              domain.ShapeList,
			ReportUnknownNames: report,
			Universe:           []domain.HarnessTool{{Name: "read"}, {Name: "web/fetch"}},
			Mappings: []domain.ToolMapping{{
				Generic:      "content_search",
				Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"search/textSearch"}}},
			}},
			MCP: domain.MCPNameForms{ServerForm: "{server}/*", ToolForm: "{server}/{tool}"},
		},
	}
}

func TestUnknownToolNames_ReturnsOnlyNamesThatAreNeitherKnownNorMcp(t *testing.T) {
	names := []string{"read", "web/fetch", "search/textSearch", "srv/*", "srv/tool", "bogus", "other-bogus"}

	got := descriptor.UnknownToolNames(reportingDescriptor(true), names)

	want := []string{"bogus", "other-bogus"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestUnknownToolNames_KeepsInputOrderAndListsEachNameOnce(t *testing.T) {
	got := descriptor.UnknownToolNames(reportingDescriptor(true), []string{"zeta", "read", "alpha", "zeta"})

	want := []string{"zeta", "alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestUnknownToolNames_NilWhenDescriptorDoesNotReport(t *testing.T) {
	if got := descriptor.UnknownToolNames(reportingDescriptor(false), []string{"bogus"}); got != nil {
		t.Errorf("want nil without the report flag, got %v", got)
	}
}

func TestUnknownToolNames_NilWhenEveryNameIsKnown(t *testing.T) {
	if got := descriptor.UnknownToolNames(reportingDescriptor(true), []string{"read", "a/b"}); len(got) != 0 {
		t.Errorf("want no unknown names, got %v", got)
	}
}

func TestUnknownToolNames_PartialGlobOfSingleToolIsUnknown(t *testing.T) {
	// A glob inside the tool part is not a valid MCP form and no universe name matches it.
	got := descriptor.UnknownToolNames(reportingDescriptor(true), []string{"srv/se*"})
	if !reflect.DeepEqual(got, []string{"srv/se*"}) {
		t.Errorf("want [srv/se*], got %v", got)
	}
}

func TestUnknownToolNames_BuiltInGhcpTargetsKnowTheirOwnToolNames(t *testing.T) {
	for _, rel := range []string{"ghcpcli/ghcp-cli.yaml", "vscodeghcp/vscode-ghcp.yaml"} {
		d := loadBuiltinYAML(t, rel)
		var names []string
		for _, u := range d.Tools.Universe {
			names = append(names, u.Name)
		}
		if len(names) == 0 {
			t.Fatalf("%s declares no tools", rel)
		}
		if got := descriptor.UnknownToolNames(d, append(names, "definitely-not-a-tool")); !reflect.DeepEqual(got, []string{"definitely-not-a-tool"}) {
			t.Errorf("%s: want only the invented name reported, got %v", rel, got)
		}
	}
}

func TestUnknownToolNames_BuiltInNonGhcpHarnessesNeverReport(t *testing.T) {
	for _, rel := range []string{"claudecode/claude-code.yaml", "opencode/opencode.yaml"} {
		d := loadBuiltinYAML(t, rel)
		if got := descriptor.UnknownToolNames(d, []string{"definitely-not-a-tool"}); got != nil {
			t.Errorf("%s ignores nothing it must report; want nil, got %v", rel, got)
		}
	}
}
