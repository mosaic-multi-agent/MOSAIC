package app_test

// deployed_tools_dropped_test.go verifies dropped source-only tools, de-duplicated reports
// and the generics the target cannot map.
//
// Only VS Code GHCP and OpenCode sources are exercised for dropped tools: Claude Code and
// GHCP CLI declare no universe names without a mapping. If a universe name without a mapping
// is ever added to either, add dropped-tool cases for that source.

import (
	"testing"

	"mosaic-deploy/internal/app"
)

func TestConvertDeployedTools_VSCodeCreateDirectoryIsDroppedForGhcpCli(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessCLI, []string{"edit/createDirectory", "read/readFile"})

	assertStrings(t, "Dropped", []string{"edit/createDirectory"}, droppedNames(got))
	assertStrings(t, "Generic", withSkill("file_read"), got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertRule(t, got, "edit/createDirectory", app.ToolRuleDropped)
	if want := "built into vscode-ghcp with no ghcp-cli equivalent"; got.Dropped[0].Reason != want {
		t.Errorf("drop reason: want %q, got %q", want, got.Dropped[0].Reason)
	}
}

func TestConvertDeployedTools_VSCodeByConventionListDirectoryIsDropped(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessClaude, []string{"search/listDirectory", "edit/createDirectory"})

	assertStrings(t, "Dropped", []string{"search/listDirectory", "edit/createDirectory"}, droppedNames(got))
}

func TestConvertDeployedTools_OpenCodeSourceOnlyToolsAreDropped(t *testing.T) {
	builtIns := []string{"patch", "webfetch", "lsp", "todowrite", "todoread", "list"}
	for _, target := range []string{harnessClaude, harnessCLI, harnessVSCode} {
		got := convertTools(t, harnessOpen, target, append([]string{"read"}, builtIns...))

		assertStrings(t, "Dropped for "+target, builtIns, droppedNames(got))
		assertStrings(t, "Generic for "+target, []string{"file_read"}, got.Generic)
		assertStrings(t, "TargetTools for "+target, nil, got.TargetTools)
	}
}

func TestConvertDeployedTools_DropReasonNamesSourceAndTargetHarness(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessClaude, []string{"lsp"})

	if len(got.Dropped) != 1 {
		t.Fatalf("want one dropped tool, got %+v", got.Dropped)
	}
	want := app.DroppedTool{Name: "lsp", Reason: "built into opencode with no claude-code equivalent"}
	if got.Dropped[0] != want {
		t.Errorf("dropped entry: want %+v, got %+v", want, got.Dropped[0])
	}
}

func TestConvertDeployedTools_DroppedToolIsNeverAlsoGenericOrCustom(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessCLI, []string{"edit/createDirectory"})

	assertStrings(t, "Dropped", []string{"edit/createDirectory"}, droppedNames(got))

	// The dropped tool contributes nothing; only the implicit skill access remains.
	assertStrings(t, "Generic", []string{"skill"}, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
	assertStrings(t, "Unmapped", nil, got.Unmapped)
}

func TestConvertDeployedTools_RepeatedSourceNamesAreReportedOnce(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessCLI, []string{
		"edit/createDirectory", "read/readFile", "edit/createDirectory", "read/readFile", "context7", "context7",
	})

	assertStrings(t, "Dropped", []string{"edit/createDirectory", "context7"}, droppedNames(got))
	assertStrings(t, "Generic", withSkill("file_read"), got.Generic)
	assertConversionSources(t, got, []string{"edit/createDirectory", "read/readFile", "context7"})
}

func TestConvertDeployedTools_RepeatedKeptNamesAppearOnceInTargetTools(t *testing.T) {
	got := convertTools(t, harnessCLI, harnessOpen, []string{"patch", "patch", "list"})

	assertStrings(t, "TargetTools", []string{"patch", "list"}, got.TargetTools)
}

func TestConvertDeployedTools_SourceNamesSharingAGenericListThatGenericOnce(t *testing.T) {
	cases := []struct {
		name     string
		src, tgt string
		tools    []string
		want     []string
	}{
		{"claude Task and TaskStop", harnessClaude, harnessOpen, []string{"Task", "TaskStop"}, withSkill("subagent")},
		{"vscode createFile and editFiles", harnessVSCode, harnessCLI,
			[]string{"edit/createFile", "edit/editFiles"}, withSkill("file_write")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertTools(t, tc.src, tc.tgt, tc.tools)

			assertStrings(t, "Generic", tc.want, got.Generic)
			// Every distinct source name still has its own audit entry.
			assertConversionSources(t, got, tc.tools)
		})
	}
}

func TestConvertDeployedTools_GenericWithoutTargetMappingIsReportedUnmapped(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessCLI, []string{"Read", "WebFetch", "WebSearch"})

	assertStrings(t, "Generic", withSkill("file_read", "web_fetch", "web_search"), got.Generic)
	assertStrings(t, "Unmapped", []string{"web_fetch", "web_search"}, got.Unmapped)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_UnmappedListsEachGenericOnceInFirstSeenOrder(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"WebSearch", "Read", "WebFetch", "WebSearch"})

	assertStrings(t, "Unmapped", []string{"web_search", "web_fetch"}, got.Unmapped)
}

func TestConvertDeployedTools_CustomServerKeysAreNotUnmapped(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"mcp__srv__*", "WebSearch"})

	assertStrings(t, "Unmapped", []string{"web_search"}, got.Unmapped)
}

func TestConvertDeployedTools_GenericMappedToNoDestinationNamesIsNotUnmapped(t *testing.T) {
	// OpenCode skill reverses to generic skill; Claude Code maps skill to an empty destination
	// list, which is a declared mapping, so deploy would not ask a custom tool question.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"skill"})

	assertStrings(t, "Generic", []string{"skill"}, got.Generic)
	assertStrings(t, "Unmapped", nil, got.Unmapped)
}
