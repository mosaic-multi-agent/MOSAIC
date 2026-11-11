package app_test

// deployed_tools_custom_test.go verifies how MCP names found in a deployed source agent are
// converted: parsed with the source harness's forms and written with the target harness's
// forms (never carried as plain text), names already in the Claude Code form kept verbatim,
// and the round trip between Claude Code and OpenCode.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestConvertDeployedTools_ClaudeWholeServerNameIsWrittenInTargetForm(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{harnessOpen, "server_*"},
		{harnessCLI, "server/*"},
		{harnessVSCode, "server/*"},
	}
	for _, tc := range cases {
		got := convertTools(t, harnessClaude, tc.target, []string{"mcp__server__*"})

		assertStrings(t, "TargetTools for "+tc.target, []string{tc.want}, got.TargetTools)
		assertStrings(t, "Dropped for "+tc.target, nil, droppedNames(got))
		assertCustomTools(t, nil, got.CustomTools)
		for _, g := range got.Generic {
			if g == "server" {
				t.Errorf("target %s: the server must not be a generic tool, got Generic %v", tc.target, got.Generic)
			}
		}
		cv := findConversion(t, got, "mcp__server__*")
		if cv.Rule != app.ToolRuleMCP || cv.Server != "server" || cv.Tool != "" {
			t.Errorf("target %s conversion: want mcp rule for the whole server, got %+v", tc.target, cv)
		}
	}
}

func TestConvertDeployedTools_PlainNameOnNoFormSourceIsUnknownNotCarried(t *testing.T) {
	// A bare name is no MCP name in any harness's forms: it is dropped, never written as text.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"server"})

	assertStrings(t, "Dropped", []string{"server"}, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
	assertRule(t, got, "server", app.ToolRuleUnknown)
	want := []app.DroppedTool{{Name: "server", Reason: "not a known opencode tool or MCP name"}}
	if !reflect.DeepEqual(got.Dropped, want) {
		t.Errorf("Dropped: want %+v, got %+v", want, got.Dropped)
	}
}

func TestConvertDeployedTools_RoundTripKeepsEveryServerBetweenClaudeAndOpenCode(t *testing.T) {
	original := []string{"Read", "mcp__alpha__*", "mcp__beta__*", "mcp__a__b__*"}

	forward := convertTools(t, harnessClaude, harnessOpen, original)
	assertStrings(t, "forward TargetTools", []string{"alpha_*", "beta_*", "a__b_*"}, forward.TargetTools)
	assertStrings(t, "forward Dropped", nil, droppedNames(forward))

	back := convertTools(t, harnessOpen, harnessClaude, forward.TargetTools)

	assertStrings(t, "back TargetTools", []string{"mcp__alpha__*", "mcp__beta__*", "mcp__a__b__*"}, back.TargetTools)
	assertStrings(t, "back Dropped", nil, droppedNames(back))
	assertStrings(t, "back Unmapped", nil, back.Unmapped)
}

func TestConvertDeployedTools_NarrowerClaudeEntryBecomesSingleToolForm(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"mcp__server__tool"})

	assertStrings(t, "TargetTools", []string{"server_tool"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	cv := findConversion(t, got, "mcp__server__tool")
	if cv.Rule != app.ToolRuleMCP || cv.Server != "server" || cv.Tool != "tool" {
		t.Errorf("conversion: want mcp rule with server and tool, got %+v", cv)
	}
}

func TestConvertDeployedTools_NameAlreadyInTargetTemplateFormIsKeptNotRewrapped(t *testing.T) {
	// A name already written in the Claude Code form (found in another harness's file) is in
	// the target's own MCP syntax and goes through unchanged.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"mcp__srv__*"})

	assertStrings(t, "TargetTools", []string{"mcp__srv__*"}, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
}

func TestConvertDeployedTools_NarrowerTargetFormNameIsKeptForNonClaudeSources(t *testing.T) {
	// mcp__server__tool starts with the Claude Code form prefix, so it is already in the
	// target's MCP syntax and must not be wrapped into mcp__mcp__server__tool__*.
	for _, src := range []string{harnessCLI, harnessOpen} {
		got := convertTools(t, src, harnessClaude, []string{"mcp__server__tool"})

		assertRule(t, got, "mcp__server__tool", app.ToolRuleKept)
		assertStrings(t, "TargetTools", []string{"mcp__server__tool"}, got.TargetTools)
		assertCustomTools(t, nil, got.CustomTools)
	}
}

func TestConvertDeployedTools_ServerNameAndMappedToolConvertSideBySide(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessCLI, []string{"WebFetch", "mcp__srv__*"})

	assertRule(t, got, "mcp__srv__*", app.ToolRuleMCP)
	assertRule(t, got, "WebFetch", app.ToolRuleMapped)
	assertStrings(t, "Generic", withSkill("web_fetch"), got.Generic)
	assertStrings(t, "TargetTools", []string{"srv/*"}, got.TargetTools)
	// web_fetch has no ghcp-cli mapping; the server is never listed as unmapped.
	assertStrings(t, "Unmapped", []string{"web_fetch"}, got.Unmapped)
}

// mcpFormUniverseSource returns a source whose universe holds a name that also fits its MCP
// server form, with no mappings.
func mcpFormUniverseSource() *domain.HarnessDescriptor {
	src := syntheticHarness("src-h", []string{"mcp__x__*"}, nil, nil)
	src.Tools.MCP = domain.MCPNameForms{ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}", NormalizeServerName: true}
	return src
}

func TestConvertDeployedTools_SourceUniverseRuleBeatsMCPRuleForFormFittingUniverseName(t *testing.T) {
	// The name is in the source universe and absent from the target: it is dropped, not read as MCP.
	tgt := syntheticHarness("tgt-h", []string{"other"}, nil, nil)
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"mcp__x__*"})
	in.Source, in.Target = mcpFormUniverseSource(), tgt

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, "mcp__x__*", app.ToolRuleDropped)
	assertStrings(t, "Dropped", []string{"mcp__x__*"}, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_TargetVerbatimRuleBeatsMCPRuleForFormFittingUniverseName(t *testing.T) {
	// The name is also valid in the target verbatim: it is kept as a target tool.
	tgt := syntheticHarness("tgt-h", []string{"mcp__x__*"}, nil, nil)
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"mcp__x__*"})
	in.Source, in.Target = mcpFormUniverseSource(), tgt

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, "mcp__x__*", app.ToolRuleKept)
	assertStrings(t, "TargetTools", []string{"mcp__x__*"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}
