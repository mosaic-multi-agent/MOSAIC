package app_test

// deployed_tools_orchestrator_test.go verifies the main orchestrator tool-list rule
// (complete placeholder expansion plus extras, versus a trimmed list) and the resolution of
// ambiguous reverse mappings by a catalog entry's generic tools.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

var claudeExpansion = []string{
	"Read", "Write", "Edit", "Bash", "Glob", "Grep", "Task", "TaskStop", "AskUserQuestion",
}

// concat returns a new slice holding the given slices in order.
func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestConvertDeployedTools_MainOrchestratorWithFullExpansionUsesTargetPlaceholder(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, claudeExpansion,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true for a complete source expansion")
	}
	assertStrings(t, "Generic", nil, got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "Unmapped", nil, got.Unmapped)
	for _, name := range claudeExpansion {
		assertRule(t, got, name, app.ToolRulePlaceholder)
	}
}

func TestConvertDeployedTools_MainOrchestratorExtrasAreConvertedByRule(t *testing.T) {
	tools := concat(claudeExpansion[:4], []string{"mcp__srv__*"}, claudeExpansion[4:], []string{"WebFetch"})

	got := convertTools(t, harnessClaude, harnessCLI, tools,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true when the expansion is complete and extras follow")
	}
	assertStrings(t, "Generic holds only the extras", []string{"web_fetch"}, got.Generic)
	assertStrings(t, "TargetTools holds the server in the target's form", []string{"srv/*"}, got.TargetTools)
	assertStrings(t, "Unmapped extras", []string{"web_fetch"}, got.Unmapped)
	assertRule(t, got, "mcp__srv__*", app.ToolRuleMCP)
	assertRule(t, got, "WebFetch", app.ToolRuleMapped)
	assertRule(t, got, "Read", app.ToolRulePlaceholder)
	assertConversionSources(t, got, tools)
}

func TestConvertDeployedTools_MainOrchestratorExtraOutsideTargetIsDroppedAndKeptExtraStays(t *testing.T) {
	// OpenCode universe tools with no Claude Code equivalent are extras that get dropped;
	// the OpenCode expansion itself is covered by the placeholder.
	ocExpansion := []string{"read", "write", "edit", "glob", "grep", "bash", "question", "task"}
	tools := concat(ocExpansion, []string{"patch", "my-server"})

	got := convertTools(t, harnessOpen, harnessClaude, tools,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true for a complete OpenCode expansion")
	}
	assertStrings(t, "Dropped extras", []string{"patch", "my-server"}, droppedNames(got))
	assertStrings(t, "Generic extras", nil, got.Generic)
	assertCustomTools(t, nil, got.CustomTools)
}

func TestConvertDeployedTools_MainOrchestratorVSCodeExpansionNamesWithoutMappingAreNotDropped(t *testing.T) {
	// The VS Code GHCP expansion contains edit/createDirectory, which no mapping names.
	vsExpansion := []string{
		"read/readFile", "edit/createFile", "edit/createDirectory", "edit/editFiles", "search/fileSearch",
		"search/textSearch", "search/listDirectory", "execute/runInTerminal", "agent", "vscode/askQuestions",
	}

	got := convertTools(t, harnessVSCode, harnessCLI, vsExpansion,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true for the complete VS Code GHCP expansion")
	}
	for _, d := range got.Dropped {
		if d.Name == "edit/createDirectory" {
			t.Errorf("edit/createDirectory is part of the expansion and must not be reported dropped")
		}
	}
	assertRule(t, got, "edit/createDirectory", app.ToolRulePlaceholder)
}

func TestConvertDeployedTools_MainOrchestratorWithTrimmedExpansionIsConvertedToolByTool(t *testing.T) {
	trimmed := []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep", "Task", "AskUserQuestion"} // no TaskStop

	got := convertTools(t, harnessClaude, harnessOpen, trimmed,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want false when part of the source expansion is missing")
	}
	assertStrings(t, "Generic", []string{
		"file_read", "file_write", "file_edit", "terminal", "file_search", "content_search",
		"subagent", "user_interaction",
	}, got.Generic)
	assertRule(t, got, "Read", app.ToolRuleMapped)
}

func TestConvertDeployedTools_FullExpansionWithoutPlaceholderAgentIsConvertedToolByTool(t *testing.T) {
	// The script orchestrator and every other agent always use the per-tool rules.
	got := convertTools(t, harnessClaude, harnessOpen, claudeExpansion, withRole(domain.RoleOrchestrator))

	if got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want false for an agent whose catalog tools are not a placeholder")
	}
	assertStrings(t, "Generic", []string{
		"file_read", "file_write", "file_edit", "terminal", "file_search", "content_search",
		"subagent", "user_interaction",
	}, got.Generic)
}

func TestConvertDeployedTools_AmbiguousToolWithoutCatalogUsesFirstDeclaredGeneric(t *testing.T) {
	got := convertTools(t, harnessCLI, harnessClaude, []string{"edit", "search", "read"},
		withRole(domain.RoleOrchestrator))

	assertStrings(t, "Generic", []string{"file_write", "file_search", "file_read"}, got.Generic)
}

func TestConvertDeployedTools_AmbiguousToolResolvedToCatalogListedGenerics(t *testing.T) {
	catalogTools := []string{"file_read", "file_edit", "content_search", "terminal"}

	got := convertTools(t, harnessCLI, harnessClaude, []string{"edit", "search", "read"},
		withRole(domain.RoleOrchestrator), withCatalogTools(catalogTools))

	assertStrings(t, "Generic", []string{"file_edit", "content_search", "file_read"}, got.Generic)
	cv := findConversion(t, got, "edit")
	if len(cv.Generic) != 1 || cv.Generic[0] != "file_edit" {
		t.Errorf("edit conversion generics: want [file_edit], got %v", cv.Generic)
	}
}

func TestConvertDeployedTools_AmbiguousToolGetsEveryCandidateTheCatalogLists(t *testing.T) {
	got := convertTools(t, harnessCLI, harnessClaude, []string{"edit"},
		withCatalogTools([]string{"file_edit", "file_write"}))

	assertStrings(t, "Generic in candidate order", []string{"file_write", "file_edit"}, got.Generic)
}

func TestConvertDeployedTools_AmbiguousVSCodeEditFilesResolvedByCatalog(t *testing.T) {
	got := convertTools(t, harnessVSCode, harnessClaude, []string{"edit/editFiles"},
		withCatalogTools([]string{"file_edit"}))

	assertStrings(t, "Generic", []string{"file_edit"}, got.Generic)
}

func TestConvertDeployedTools_CatalogListingNoCandidateFallsBackToFirstDeclared(t *testing.T) {
	for name, catalog := range map[string][]string{
		"unrelated generics": {"terminal"},
		"empty non-nil list": {},
	} {
		t.Run(name, func(t *testing.T) {
			got := convertTools(t, harnessCLI, harnessClaude, []string{"edit"}, withCatalogTools(catalog))

			assertStrings(t, "Generic", []string{"file_write"}, got.Generic)
		})
	}
}

func TestConvertDeployedTools_CatalogDoesNotRestrictUnambiguousMappings(t *testing.T) {
	got := convertTools(t, harnessCLI, harnessClaude, []string{"read", "execute"},
		withCatalogTools([]string{"file_edit"}))

	assertStrings(t, "Generic", []string{"file_read", "terminal"}, got.Generic)
}

func TestConvertDeployedTools_MainOrchestratorKeepsTargetNativeExtra(t *testing.T) {
	tgt := syntheticHarness("tgt-h", []string{"native-x"}, map[string][]string{"file_read": {"r"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, concat(claudeExpansion, []string{"native-x"}),
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())
	in.Target = tgt

	got := app.ConvertDeployedTools(in)

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true for a complete expansion followed by an extra")
	}
	assertStrings(t, "TargetTools holds the kept extra", []string{"native-x"}, got.TargetTools)
	assertStrings(t, "Generic", nil, got.Generic)
	assertRule(t, got, "native-x", app.ToolRuleKept)
}

func TestConvertDeployedTools_MainOrchestratorWithGhcpCliFullExpansionUsesTargetPlaceholder(t *testing.T) {
	cliExpansion := []string{"read", "edit", "search", "execute", "ask_user", "agent"}

	got := convertTools(t, harnessCLI, harnessClaude, cliExpansion,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true for the complete GHCP CLI expansion")
	}
	assertStrings(t, "Generic", nil, got.Generic)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	for _, name := range cliExpansion {
		assertRule(t, got, name, app.ToolRulePlaceholder)
	}
}

func TestConvertDeployedTools_PlaceholderRuleAppliesToMainOrchestratorOnly(t *testing.T) {
	for _, role := range []domain.AgentRole{domain.RoleSubagent, domain.RoleUtility, domain.RoleStandalone} {
		got := convertTools(t, harnessClaude, harnessOpen, claudeExpansion, withRole(role), withPlaceholderAgent())

		if got.UsePlaceholder {
			t.Errorf("role %s: UsePlaceholder must stay false for a non-orchestrator", role)
		}
		if len(got.Generic) == 0 {
			t.Errorf("role %s: want per-tool generics, got none", role)
		}
	}
}

func TestConvertDeployedTools_SourceWithoutDeclaredExpansionUsesUniverseWithoutByConventionNames(t *testing.T) {
	src := syntheticHarness("src-h", []string{"u1", "u2", "bc"}, nil, nil)
	src.Tools.Universe[2].ByConvention = true

	in := newToolInput(t, harnessClaude, harnessOpen, []string{"u1", "u2"},
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())
	in.Source = src

	got := app.ConvertDeployedTools(in)

	if !got.UsePlaceholder {
		t.Fatal("UsePlaceholder: want true when the list holds the whole universe minus by-convention names")
	}
}
