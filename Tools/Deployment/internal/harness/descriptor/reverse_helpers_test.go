package descriptor_test

// reverse_helpers_test.go verifies the reverse lookup used to turn deployed harness tool
// names back into generic tools. MCP names are read by the mcpname package.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// loadBuiltinYAML loads a built-in harness descriptor from its YAML file.
func loadBuiltinYAML(t *testing.T, relPath string) *domain.HarnessDescriptor {
	t.Helper()
	d, err := descriptor.Load("../builtin/" + relPath)
	if err != nil {
		t.Fatalf("load %s: %v", relPath, err)
	}
	return d
}

func TestReverseMapToolCandidates_AmbiguousNameListsAllGenericsInDeclarationOrder(t *testing.T) {
	cli := loadBuiltinYAML(t, "ghcpcli/ghcp-cli.yaml")
	vs := loadBuiltinYAML(t, "vscodeghcp/vscode-ghcp.yaml")

	cases := []struct {
		name string
		d    *domain.HarnessDescriptor
		in   string
		want []string
	}{
		{"ghcp-cli edit", cli, "edit", []string{"file_write", "file_edit"}},
		{"ghcp-cli search", cli, "search", []string{"file_search", "content_search"}},
		{"vscode editFiles", vs, "edit/editFiles", []string{"file_write", "file_edit"}},
		{"vscode createFile", vs, "edit/createFile", []string{"file_write"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := descriptor.ReverseMapToolCandidates(tc.d, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("candidates for %q: want %v, got %v", tc.in, tc.want, got)
			}
		})
	}
}

func TestReverseMapToolCandidates_TaskAndTaskStopBothReverseToSubagent(t *testing.T) {
	cc := loadBuiltinYAML(t, "claudecode/claude-code.yaml")
	for _, name := range []string{"Task", "TaskStop"} {
		got := descriptor.ReverseMapToolCandidates(cc, name)
		if !reflect.DeepEqual(got, []string{"subagent"}) {
			t.Errorf("candidates for %q: want [subagent], got %v", name, got)
		}
	}
}

func TestReverseMapToolCandidates_UnmappedNameGivesEmptyNonNilSlice(t *testing.T) {
	vs := loadBuiltinYAML(t, "vscodeghcp/vscode-ghcp.yaml")
	for _, name := range []string{"edit/createDirectory", "not-a-tool", ""} {
		got := descriptor.ReverseMapToolCandidates(vs, name)
		if got == nil || len(got) != 0 {
			t.Errorf("candidates for %q: want empty non-nil slice, got %#v", name, got)
		}
	}
}

func TestReverseMapToolCandidates_GenericWithEmptyDestinationsIsNotACandidate(t *testing.T) {
	cc := loadBuiltinYAML(t, "claudecode/claude-code.yaml")
	if got := descriptor.ReverseMapToolCandidates(cc, "skill"); len(got) != 0 {
		t.Errorf("claude-code has no destination name for skill; want no candidates, got %v", got)
	}
	cli := loadBuiltinYAML(t, "ghcpcli/ghcp-cli.yaml")
	if got := descriptor.ReverseMapToolCandidates(cli, "skill"); !reflect.DeepEqual(got, []string{"skill"}) {
		t.Errorf("ghcp-cli skill: want [skill], got %v", got)
	}
}

func TestReverseMapToolCandidates_NameInTwoDestinationsOfOneMappingListedOnce(t *testing.T) {
	d := &domain.HarnessDescriptor{
		ID: "dup-harness",
		Tools: domain.ToolSpec{
			Shape: domain.ShapeList,
			Mappings: []domain.ToolMapping{
				{Generic: "alpha", Destinations: []domain.ToolDestination{
					{Kind: domain.DestMain, Names: []string{"x"}},
					{Kind: domain.DestField, Field: "other", Names: []string{"x"}},
				}},
				{Generic: "beta", Destinations: []domain.ToolDestination{
					{Kind: domain.DestMain, Names: []string{"x"}},
				}},
			},
		},
	}
	got := descriptor.ReverseMapToolCandidates(d, "x")
	if !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Errorf("want [alpha beta] without duplicates, got %v", got)
	}
}

func TestReverseMapToolCandidates_FirstElementEqualsReverseMapTool(t *testing.T) {
	cli := loadBuiltinYAML(t, "ghcpcli/ghcp-cli.yaml")
	for _, name := range []string{"edit", "search", "read", "execute"} {
		first, ok := descriptor.ReverseMapTool(cli, name)
		if !ok {
			t.Fatalf("ReverseMapTool(%q) unexpectedly unmapped", name)
		}
		got := descriptor.ReverseMapToolCandidates(cli, name)
		if len(got) == 0 || got[0] != first {
			t.Errorf("first candidate for %q: want %q, got %v", name, first, got)
		}
	}
}
