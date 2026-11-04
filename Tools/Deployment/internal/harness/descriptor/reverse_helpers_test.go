package descriptor_test

// reverse_helpers_test.go verifies the reverse lookups used to turn deployed harness tool
// names back into generic tools and custom tool servers.

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

// templateDescriptor returns a minimal descriptor declaring the given custom tool template.
func templateDescriptor(template string) *domain.HarnessDescriptor {
	return &domain.HarnessDescriptor{
		ID:    "tmpl-harness",
		Tools: domain.ToolSpec{Shape: domain.ShapeList, CustomToolTemplate: template},
	}
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

func TestReverseCustomTool_ClaudeTemplateYieldsServer(t *testing.T) {
	cc := loadBuiltinYAML(t, "claudecode/claude-code.yaml")
	cases := []struct {
		in         string
		wantServer string
		wantOK     bool
	}{
		{"mcp__srv__*", "srv", true},
		{"mcp__a__b__*", "a__b", true},
		{"mcp__my-server__*", "my-server", true},
		{"mcp__srv__tool", "", false}, // narrower entry does not fit the wildcard template
		{"mcp____*", "", false},       // empty server
		{"mcp__srv__", "", false},     // missing suffix
		{"mcp__*", "", false},         // prefix and suffix overlap, no room for a server
		{"mcp__", "", false},          // prefix only
		{"__*", "", false},            // suffix only
		{"srv__*", "", false},         // missing prefix
		{"Read", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			server, ok := descriptor.ReverseCustomTool(cc, tc.in)
			if server != tc.wantServer || ok != tc.wantOK {
				t.Errorf("ReverseCustomTool(%q): want (%q, %v), got (%q, %v)",
					tc.in, tc.wantServer, tc.wantOK, server, ok)
			}
		})
	}
}

func TestReverseCustomTool_DescriptorWithoutTemplateNeverMatches(t *testing.T) {
	oc := loadBuiltinYAML(t, "opencode/opencode.yaml")
	if server, ok := descriptor.ReverseCustomTool(oc, "mcp__srv__*"); ok || server != "" {
		t.Errorf("opencode declares no template; want (\"\", false), got (%q, %v)", server, ok)
	}
}

func TestReverseCustomTool_TemplateNeedsExactlyOnePlaceholder(t *testing.T) {
	// Each name would be accepted by an implementation that cuts at the first "%s" and
	// treats the remainder literally or ignores the missing placeholder.
	cases := []struct{ name, tmpl, in string }{
		{"no placeholder", "mcp__", "mcp__x"},
		{"two placeholders", "%s__%s", "a__b"},
		{"two placeholders with longer middle", "%s__%s", "a____b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, ok := descriptor.ReverseCustomTool(templateDescriptor(tc.tmpl), tc.in)
			if ok || server != "" {
				t.Errorf("template %q, name %q: want (\"\", false), got (%q, %v)", tc.tmpl, tc.in, server, ok)
			}
		})
	}
}

func TestReverseCustomTool_SinglePlaceholderTemplateStillMatchesBesideTwoPlaceholderRejection(t *testing.T) {
	// Positive control for the rejection rows above: one "%s" with the same literal text works.
	server, ok := descriptor.ReverseCustomTool(templateDescriptor("%s__x"), "a__b__x")
	if !ok || server != "a__b" {
		t.Errorf("want (a__b, true), got (%q, %v)", server, ok)
	}
}

func TestReverseCustomTool_PlaceholderAtEdgesOfTemplate(t *testing.T) {
	server, ok := descriptor.ReverseCustomTool(templateDescriptor("%s-mcp"), "abc-mcp")
	if !ok || server != "abc" {
		t.Errorf("suffix-only template: want (abc, true), got (%q, %v)", server, ok)
	}
	server, ok = descriptor.ReverseCustomTool(templateDescriptor("mcp:%s"), "mcp:abc")
	if !ok || server != "abc" {
		t.Errorf("prefix-only template: want (abc, true), got (%q, %v)", server, ok)
	}
}
