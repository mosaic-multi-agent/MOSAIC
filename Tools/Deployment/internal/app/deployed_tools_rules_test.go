package app_test

// deployed_tools_rules_test.go verifies the ordered rules of ConvertDeployedTools for each
// built-in source harness: mapped, custom by template, dropped, kept and custom.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
)

func TestConvertDeployedTools_ClaudeCodeSourceMappedToGenerics(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen,
		[]string{"Read", "Edit", "Bash", "Glob", "Grep", "AskUserQuestion", "Task"})

	assertStrings(t, "Generic", withSkill(
		"file_read", "file_edit", "terminal", "file_search", "content_search", "user_interaction", "subagent",
	), got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "Unmapped", nil, got.Unmapped)
	assertRule(t, got, "Read", app.ToolRuleMapped)
	if cv := findConversion(t, got, "Edit"); len(cv.Generic) != 1 || cv.Generic[0] != "file_edit" {
		t.Errorf("Edit conversion generics: want [file_edit], got %v", cv.Generic)
	}
}

func TestConvertDeployedTools_OpenCodePermissionSourceMappedToGenerics(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessClaude,
		[]string{"read", "write", "edit", "glob", "grep", "bash", "question", "task"})

	assertStrings(t, "Generic", []string{
		"file_read", "file_write", "file_edit", "file_search", "content_search", "terminal",
		"user_interaction", "subagent",
	}, got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_GhcpCliSourceMappedToGenerics(t *testing.T) {
	got := convertTools(t, harnessCLI, harnessClaude,
		[]string{"read", "execute", "ask_user", "agent", "skill"})

	assertStrings(t, "Generic", []string{
		"file_read", "terminal", "user_interaction", "subagent", "skill",
	}, got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_VSCodeSourceMappedToGenerics(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessCLI, []string{
		"read/readFile", "edit/createFile", "edit/editFiles", "search/fileSearch", "search/textSearch",
		"execute/runInTerminal", "agent", "vscode/askQuestions",
	})

	assertStrings(t, "Generic", withSkill(
		"file_read", "file_write", "file_search", "content_search", "terminal", "subagent", "user_interaction",
	), got.Generic)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertRule(t, got, "edit/editFiles", app.ToolRuleMapped)
}

func TestConvertDeployedTools_ConfigMergedMappingConvertsThroughItsGeneric(t *testing.T) {
	// A mapping merged into the module at registry discovery is an ordinary mapping.
	src := syntheticHarness("src-h", []string{"alpha"}, map[string][]string{"file_read": {"alpha"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"alpha"})
	in.Source = src

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
	assertRule(t, got, "alpha", app.ToolRuleMapped)
}

func TestConvertDeployedTools_MappedRuleBeatsKeptWhenNameIsAlsoValidInTarget(t *testing.T) {
	// "read" is both a mapped OpenCode name and a valid ghcp-cli name: mapped wins.
	got := convertTools(t, harnessOpen, harnessCLI, []string{"read"})

	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertRule(t, got, "read", app.ToolRuleMapped)
}

func TestConvertDeployedTools_NameValidInTargetUniverseIsKeptVerbatim(t *testing.T) {
	// "patch" is not in ghcp-cli's universe; it is in OpenCode's universe and unmapped there.
	got := convertTools(t, harnessCLI, harnessOpen, []string{"patch", "list"})

	assertStrings(t, "TargetTools", []string{"patch", "list"}, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertRule(t, got, "patch", app.ToolRuleKept)
}

func TestConvertDeployedTools_NameValidOnlyAsTargetMappingDestinationIsKept(t *testing.T) {
	target := syntheticHarness("tgt-h", []string{"universe-only"},
		map[string][]string{"file_read": {"dest-only"}}, []string{"file_read"})
	in := newToolInput(t, harnessCLI, harnessOpen, []string{"patch", "universe-only", "dest-only"})
	in.Target = target

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "TargetTools", []string{"universe-only", "dest-only"}, got.TargetTools)
	assertRule(t, got, "dest-only", app.ToolRuleKept)
	assertRule(t, got, "universe-only", app.ToolRuleKept)
	// "patch" is neither in the synthetic target nor in ghcp-cli's universe: a custom tool.
	assertRule(t, got, "patch", app.ToolRuleCustom)
}

func TestConvertDeployedTools_SourceUniverseNameValidInTargetIsKeptNotDropped(t *testing.T) {
	src := syntheticHarness("src-h", []string{"shared"}, nil, nil)
	tgt := syntheticHarness("tgt-h", []string{"shared"}, nil, nil)
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"shared"})
	in.Source, in.Target = src, tgt

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "TargetTools", []string{"shared"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertRule(t, got, "shared", app.ToolRuleKept)
}

func TestConvertDeployedTools_SourceUniverseNameValidOnlyAsTargetMappingDestinationIsKeptNotDropped(t *testing.T) {
	// "dest-only" is in the source universe (unmapped there) and in the target only as a
	// mapping destination name, not in the target universe: the kept rule must see it.
	src := syntheticHarness("src-h", []string{"dest-only"}, nil, nil)
	tgt := syntheticHarness("tgt-h", []string{"other"}, map[string][]string{"file_read": {"dest-only"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"dest-only"})
	in.Source, in.Target = src, tgt

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "TargetTools", []string{"dest-only"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertRule(t, got, "dest-only", app.ToolRuleKept)
}

func TestConvertDeployedTools_SourceUniverseNameInNeitherTargetListIsDropped(t *testing.T) {
	src := syntheticHarness("src-h", []string{"src-only"}, nil, nil)
	tgt := syntheticHarness("tgt-h", []string{"other"}, map[string][]string{"file_read": {"dest"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"src-only"})
	in.Source, in.Target = src, tgt

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "Dropped", []string{"src-only"}, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertRule(t, got, "src-only", app.ToolRuleDropped)
}

func TestConvertDeployedTools_MappingRuleBeatsTemplateRuleWhenNameIsBoth(t *testing.T) {
	// "mcp__x__*" is a source mapping name and also fits the source's custom template.
	src := syntheticHarness("src-h", []string{"mcp__x__*"}, map[string][]string{"file_read": {"mcp__x__*"}}, []string{"file_read"})
	src.Tools.CustomToolTemplate = "mcp__%s__*"
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"mcp__x__*"})
	in.Source = src

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, "mcp__x__*", app.ToolRuleMapped)
	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
}

func TestConvertDeployedTools_SameServerReachedTwoWaysIsListedOnce(t *testing.T) {
	// Source Claude Code: "mcp__a__*" via the template, "a" via the custom rule.
	got := convertTools(t, harnessClaude, harnessOpen, []string{"mcp__a__*", "a"})

	assertStrings(t, "Generic", withSkill("a"), got.Generic)
	assertCustomTools(t, map[string]string{"a": "a"}, got.CustomTools)
}

func TestConvertDeployedTools_UnknownNameIsACustomToolKeyedForForwardMapping(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessClaude, []string{"my-server"})

	assertStrings(t, "Generic", []string{"my-server"}, got.Generic)
	assertCustomTools(t, map[string]string{"my-server": "my-server"}, got.CustomTools)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Unmapped", nil, got.Unmapped)
	cv := findConversion(t, got, "my-server")
	if cv.Rule != app.ToolRuleCustom || cv.Server != "my-server" {
		t.Errorf("conversion: want custom rule with server my-server, got %+v", cv)
	}
}

func TestConvertDeployedTools_PlainServerNamesStayCustomOnNoTemplateSourcesAndTargets(t *testing.T) {
	// A plain name on a no-template source is indistinguishable from an unknown name.
	for _, src := range []string{harnessOpen, harnessVSCode} {
		got := convertTools(t, src, harnessCLI, []string{"context7"})
		assertCustomTools(t, map[string]string{"context7": "context7"}, got.CustomTools)
		wantGeneric := []string{"context7"}
		if src == harnessVSCode {
			wantGeneric = withSkill("context7") // implicit-skill source to an explicit-skill target
		}
		assertStrings(t, "Generic for "+src, wantGeneric, got.Generic)
		assertStrings(t, "TargetTools for "+src, nil, got.TargetTools)
	}
}

func TestConvertDeployedTools_RulesApplyInDocumentOrderAcrossMixedList(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessCLI, []string{
		"edit/createDirectory", "read/readFile", "context7", "agent",
	})

	assertConversionSources(t, got, []string{"edit/createDirectory", "read/readFile", "context7", "agent"})
	assertRule(t, got, "edit/createDirectory", app.ToolRuleDropped)
	assertRule(t, got, "read/readFile", app.ToolRuleMapped)
	assertRule(t, got, "context7", app.ToolRuleCustom)
	assertRule(t, got, "agent", app.ToolRuleMapped)
	assertStrings(t, "Generic", withSkill("file_read", "context7", "subagent"), got.Generic)
}

func TestConvertDeployedTools_IsDeterministic(t *testing.T) {
	in := newToolInput(t, harnessVSCode, harnessCLI,
		[]string{"edit/createDirectory", "read/readFile", "context7", "edit/editFiles"})

	first := app.ConvertDeployedTools(in)
	if len(first.Generic) == 0 || len(first.CustomTools) == 0 || len(first.Conversions) == 0 {
		t.Fatal("conversion produced no result; determinism check is vacuous")
	}
	for i := 0; i < 20; i++ {
		if again := app.ConvertDeployedTools(in); !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d differs: want %+v, got %+v", i, first, again)
		}
	}
}
