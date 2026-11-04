package app_test

// deployed_tools_custom_test.go verifies the custom tool template reverse and the forward
// and backward round trip between Claude Code (template mcp__%s__*) and harnesses with no
// template.

import (
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func TestConvertDeployedTools_ClaudeTemplateToolBecomesServerOnNoTemplateTargets(t *testing.T) {
	for _, target := range []string{harnessOpen, harnessCLI, harnessVSCode} {
		got := convertTools(t, harnessClaude, target, []string{"mcp__server__*"})

		wantGeneric := withSkill("server")
		if target == harnessVSCode {
			wantGeneric = []string{"server"} // the VS Code target writes no skill name
		}
		assertStrings(t, "Generic for "+target, wantGeneric, got.Generic)
		assertCustomTools(t, map[string]string{"server": "server"}, got.CustomTools)
		assertStrings(t, "TargetTools for "+target, nil, got.TargetTools)
		assertStrings(t, "Dropped for "+target, nil, droppedNames(got))
		cv := findConversion(t, got, "mcp__server__*")
		if cv.Rule != app.ToolRuleCustomTemplate || cv.Server != "server" {
			t.Errorf("target %s conversion: want custom-template with server, got %+v", target, cv)
		}
	}
}

func TestConvertDeployedTools_PlainServerOnNoTemplateSourceIsKeyedForClaudeCodeTemplate(t *testing.T) {
	// The forward mapping writes mcp__server__* for this key; the conversion keys it as a server.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"server"})

	assertStrings(t, "Generic", []string{"server"}, got.Generic)
	assertCustomTools(t, map[string]string{"server": "server"}, got.CustomTools)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_RoundTripKeepsEveryCustomServer(t *testing.T) {
	original := []string{"Read", "mcp__alpha__*", "mcp__beta__*", "mcp__a__b__*"}

	forward := convertTools(t, harnessClaude, harnessOpen, original, withRole(domain.RoleOrchestrator))
	wantServers := map[string]string{"alpha": "alpha", "beta": "beta", "a__b": "a__b"}
	assertCustomTools(t, wantServers, forward.CustomTools)

	written := writtenToolNames(t, harnessOpen, forward.Generic, forward.CustomTools)
	back := convertTools(t, harnessOpen, harnessClaude, written, withRole(domain.RoleOrchestrator))

	assertCustomTools(t, wantServers, back.CustomTools)
	gotGeneric := append([]string(nil), back.Generic...)
	sort.Strings(gotGeneric)
	assertStrings(t, "Generic back (sorted)", []string{"a__b", "alpha", "beta", "file_read"}, gotGeneric)
	assertStrings(t, "Unmapped back", nil, back.Unmapped)
	assertStrings(t, "Dropped back", nil, droppedNames(back))

	// The Claude Code forward mapping must write the original custom names again.
	finalNames := writtenToolNames(t, harnessClaude, back.Generic, back.CustomTools)
	var customFinal []string
	for _, n := range finalNames {
		if strings.HasPrefix(n, "mcp__") {
			customFinal = append(customFinal, n)
		}
	}
	sort.Strings(customFinal)
	assertStrings(t, "Claude Code custom names written", []string{"mcp__a__b__*", "mcp__alpha__*", "mcp__beta__*"}, customFinal)
}

// writtenToolNames runs the forward tool mapping for a harness and extracts the tool names it
// would write into the agent file, as the reverse side reads them back.
func writtenToolNames(t *testing.T, rel string, generic []string, custom map[string]string) []string {
	t.Helper()
	d := loadToolHarness(t, rel)
	res, err := descriptor.MapTools(d, domain.ToolRequest{Generic: generic, CustomNames: custom})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	var names []string
	for _, f := range res.Fields {
		if f.Key == d.Frontmatter.ToolsKey {
			names = descriptor.ExtractToolEntries(d, f.Value)
		}
	}
	if len(names) == 0 {
		t.Fatalf("forward mapping wrote no tool names for %v", generic)
	}
	return names
}

func TestConvertDeployedTools_NarrowerClaudeEntryDoesNotFitTheWildcardTemplate(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"mcp__server__tool"})

	// It falls through to the later rules and, being no known tool, is kept as a custom tool.
	assertRule(t, got, "mcp__server__tool", app.ToolRuleCustom)
	assertStrings(t, "Generic", withSkill("mcp__server__tool"), got.Generic)
	assertCustomTools(t, map[string]string{"mcp__server__tool": "mcp__server__tool"}, got.CustomTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_NameAlreadyInTargetTemplateFormIsKeptNotRewrapped(t *testing.T) {
	// A name already written in the Claude Code custom form (found in another harness's
	// file) is in the target's own custom format and goes through unchanged.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"mcp__srv__*"})

	assertStrings(t, "TargetTools", []string{"mcp__srv__*"}, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
}

func TestConvertDeployedTools_NarrowerTargetFormNameIsKeptForNonClaudeSources(t *testing.T) {
	// mcp__server__tool starts with the Claude Code template prefix, so it is already in the
	// target's custom form and must not be wrapped into mcp__mcp__server__tool__*.
	for _, src := range []string{harnessCLI, harnessOpen} {
		got := convertTools(t, src, harnessClaude, []string{"mcp__server__tool"})

		assertRule(t, got, "mcp__server__tool", app.ToolRuleKept)
		assertStrings(t, "TargetTools", []string{"mcp__server__tool"}, got.TargetTools)
		assertCustomTools(t, nil, got.CustomTools)
	}
}

func TestConvertDeployedTools_TemplateNameAndMappedToolConvertSideBySide(t *testing.T) {
	// Mixed list: the template name becomes a server while the mapped tool follows its mapping.
	got := convertTools(t, harnessClaude, harnessCLI, []string{"WebFetch", "mcp__srv__*"})

	assertRule(t, got, "mcp__srv__*", app.ToolRuleCustomTemplate)
	assertRule(t, got, "WebFetch", app.ToolRuleMapped)
	assertStrings(t, "Generic", withSkill("web_fetch", "srv"), got.Generic)
	// Custom server keys are never listed as unmapped generics; web_fetch has no ghcp-cli mapping.
	assertStrings(t, "Unmapped", []string{"web_fetch"}, got.Unmapped)
}

// templateOrderSource returns a source whose universe holds a name fitting its custom tool
// template, with no mappings, so only rules 2 to 4 can claim the name.
func templateOrderSource() *domain.HarnessDescriptor {
	src := syntheticHarness("src-h", []string{"mcp__x__*"}, nil, nil)
	src.Tools.CustomToolTemplate = "mcp__%s__*"
	return src
}

func TestConvertDeployedTools_TemplateRuleBeatsDroppedRuleForTemplateFittingUniverseName(t *testing.T) {
	// The name is in the source universe and absent from the target: rule 3 would drop it.
	tgt := syntheticHarness("tgt-h", []string{"other"}, nil, nil)
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"mcp__x__*"})
	in.Source, in.Target = templateOrderSource(), tgt

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, "mcp__x__*", app.ToolRuleCustomTemplate)
	assertStrings(t, "Generic", []string{"x"}, got.Generic)
	assertCustomTools(t, map[string]string{"x": "x"}, got.CustomTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_TemplateRuleBeatsKeptRuleForTemplateFittingUniverseName(t *testing.T) {
	// The name is also valid in the target: rule 4 would keep it as a target tool.
	tgt := syntheticHarness("tgt-h", []string{"mcp__x__*"}, nil, nil)
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"mcp__x__*"})
	in.Source, in.Target = templateOrderSource(), tgt

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, "mcp__x__*", app.ToolRuleCustomTemplate)
	assertStrings(t, "Generic", []string{"x"}, got.Generic)
	assertCustomTools(t, map[string]string{"x": "x"}, got.CustomTools)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}
