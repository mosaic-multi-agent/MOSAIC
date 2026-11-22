package descriptor_test

// maptools_entries_test.go verifies how MapTools writes a custom tool answer made of several
// MCP entries (a whole server, or one tool of a server) in the descriptor's own name forms.
//
// Coverage:
//   - Each entry becomes one harness name, in answer order, each name once.
//   - A descriptor that declares only one of the two forms keeps the neutral name for an entry
//     that needs the missing form; a descriptor with no forms keeps the neutral name for all.
//   - The names reach every destination of the custom tool destinations fan-out, or a single
//     main destination.
//   - A single whole-server entry is written exactly as a single server name always was.
//   - Permission shape: mapped and custom main names outside the universe become allow pairs
//     after the universe pairs, each name once.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

var (
	claudeLikeForms = domain.MCPNameForms{ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}"}

	// mixedAnswer is the answer used throughout: two tools of one server and a whole server.
	mixedAnswer = []domain.MCPEntry{
		{Server: "github", Tool: "get_pr"},
		{Server: "github", Tool: "list_comments"},
		{Server: "jira"},
	}
)

// entriesListDescriptor is a list-shape descriptor with the given MCP forms and no
// custom tool destinations.
func entriesListDescriptor(forms domain.MCPNameForms) *domain.HarnessDescriptor {
	return &domain.HarnessDescriptor{
		SchemaVersion: "1",
		ID:            "entries-list-harness",
		DisplayName:   "Entries List Harness",
		Tools: domain.ToolSpec{
			Shape:    domain.ShapeList,
			Universe: []domain.HarnessTool{{Name: "read/readFile", Unused: domain.Deny}},
			MCP:      forms,
		},
		Frontmatter: domain.FrontmatterSpec{ToolsKey: "tools"},
	}
}

// mapAnswer maps one generic "terminal" with the given entries and returns its resolution.
func mapAnswer(t *testing.T, d *domain.HarnessDescriptor, entries []domain.MCPEntry) (domain.ToolResolution, domain.ToolResult) {
	t.Helper()
	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:    "test-agent",
		Generic:     []string{"terminal"},
		CustomNames: domain.CustomToolAnswers{"terminal": entries},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	if len(result.Resolutions) != 1 {
		t.Fatalf("want one resolution, got %d", len(result.Resolutions))
	}
	return result.Resolutions[0], result
}

func TestMapToolsEntries_EachEntryIsWrittenInItsOwnFormInAnswerOrder(t *testing.T) {
	d := entriesListDescriptor(claudeLikeForms)

	res, _ := mapAnswer(t, d, mixedAnswer)

	if res.Outcome != domain.ToolCustom {
		t.Fatalf("outcome = %q, want %q", res.Outcome, domain.ToolCustom)
	}
	want := []string{"mcp__github__get_pr", "mcp__github__list_comments", "mcp__jira__*"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_AllNamesGoToOneMainDestination(t *testing.T) {
	d := entriesListDescriptor(claudeLikeForms)

	res, result := mapAnswer(t, d, mixedAnswer)

	want := []string{"mcp__github__get_pr", "mcp__github__list_comments", "mcp__jira__*"}
	if len(res.Destinations) != 1 || res.Destinations[0].Kind != domain.DestMain ||
		!reflect.DeepEqual(res.Destinations[0].Names, want) {
		t.Errorf("Destinations = %+v, want one main destination with %v", res.Destinations, want)
	}
	f := findField(t, result.Fields, "tools")
	if f == nil {
		t.Fatal("tools field absent")
	}
	for _, n := range want {
		if !fieldContainsName(f, n) {
			t.Errorf("tools field lacks %q: %+v", n, f.Value)
		}
	}
}

func TestMapToolsEntries_SameEntryTwiceIsWrittenOnce(t *testing.T) {
	d := entriesListDescriptor(claudeLikeForms)

	res, _ := mapAnswer(t, d, []domain.MCPEntry{
		{Server: "github", Tool: "get_pr"}, {Server: "jira"}, {Server: "github", Tool: "get_pr"},
	})

	want := []string{"mcp__github__get_pr", "mcp__jira__*"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_ServerNameNormalisationAppliesToToolForm(t *testing.T) {
	forms := claudeLikeForms
	forms.NormalizeServerName = true
	d := entriesListDescriptor(forms)

	res, _ := mapAnswer(t, d, []domain.MCPEntry{{Server: "my.server", Tool: "get_pr"}, {Server: "my.server"}})

	want := []string{"mcp__my_server__get_pr", "mcp__my_server__*"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_SingleWholeServerEntryIsWrittenAsBefore(t *testing.T) {
	d := entriesListDescriptor(claudeLikeForms)

	res, _ := mapAnswer(t, d, []domain.MCPEntry{{Server: "my-server"}})

	if !reflect.DeepEqual(res.HarnessTools, []string{"mcp__my-server__*"}) {
		t.Errorf("HarnessTools = %v, want [mcp__my-server__*]", res.HarnessTools)
	}
	want := []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"mcp__my-server__*"}}}
	if !reflect.DeepEqual(res.Destinations, want) {
		t.Errorf("Destinations = %+v, want %+v", res.Destinations, want)
	}
}

func TestMapToolsEntries_OnlyServerFormDeclaredKeepsNeutralNameForSingleToolEntry(t *testing.T) {
	d := entriesListDescriptor(domain.MCPNameForms{ServerForm: "{server}/*"})

	res, _ := mapAnswer(t, d, []domain.MCPEntry{{Server: "github", Tool: "get_pr"}, {Server: "jira"}})

	want := []string{"github/get_pr", "jira/*"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_OnlyToolFormDeclaredKeepsNeutralNameForWholeServerEntry(t *testing.T) {
	d := entriesListDescriptor(domain.MCPNameForms{ToolForm: "mcp__{server}__{tool}"})

	res, _ := mapAnswer(t, d, []domain.MCPEntry{{Server: "github", Tool: "get_pr"}, {Server: "jira"}})

	want := []string{"mcp__github__get_pr", "jira"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_NoMCPFormsKeepsOneNeutralNamePerEntry(t *testing.T) {
	d := entriesListDescriptor(domain.MCPNameForms{})

	res, _ := mapAnswer(t, d, mixedAnswer)

	if res.Outcome != domain.ToolCustom {
		t.Fatalf("outcome = %q, want %q", res.Outcome, domain.ToolCustom)
	}
	want := []string{"github/get_pr", "github/list_comments", "jira"}
	if !reflect.DeepEqual(res.HarnessTools, want) {
		t.Errorf("HarnessTools = %v, want %v", res.HarnessTools, want)
	}
}

func TestMapToolsEntries_NoMCPFormsSingleWholeServerKeepsTodaysRawName(t *testing.T) {
	d := entriesListDescriptor(domain.MCPNameForms{})

	res, _ := mapAnswer(t, d, []domain.MCPEntry{{Server: "my-server"}})

	if !reflect.DeepEqual(res.HarnessTools, []string{"my-server"}) {
		t.Errorf("HarnessTools = %v, want [my-server]", res.HarnessTools)
	}
}

func TestMapToolsEntries_AllNamesReachEveryCustomToolDestination(t *testing.T) {
	d := makeListCustomDestMainAndFieldDescriptor()
	d.Tools.MCP = claudeLikeForms

	res, result := mapAnswer(t, d, mixedAnswer)

	want := []string{"mcp__github__get_pr", "mcp__github__list_comments", "mcp__jira__*"}
	if len(res.Destinations) != 2 {
		t.Fatalf("Destinations = %+v, want the two declared destinations", res.Destinations)
	}
	if res.Destinations[0].Kind != domain.DestMain || res.Destinations[1].Kind != domain.DestField ||
		res.Destinations[1].Field != "mcpServers" || res.Destinations[1].Format != domain.FormatListBlock {
		t.Errorf("destination kinds, field or format were not preserved: %+v", res.Destinations)
	}
	for i, dest := range res.Destinations {
		if !reflect.DeepEqual(dest.Names, want) {
			t.Errorf("Destinations[%d].Names = %v, want %v", i, dest.Names, want)
		}
	}
	if f := findField(t, result.Fields, "mcpServers"); f == nil || !fieldContainsName(f, "mcp__jira__*") {
		t.Errorf("mcpServers field must carry every name; got %+v", f)
	}
}

func TestMapToolsEntries_AnswerWinsOverMappingEntryAndUsesOneMainDestination(t *testing.T) {
	d := makeListCustomDestMainAndFieldDescriptor()
	d.Tools.MCP = claudeLikeForms
	d.Tools.Mappings = append(d.Tools.Mappings, domain.ToolMapping{
		Generic:      "terminal",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"read/readFile"}}},
	})

	res, _ := mapAnswer(t, d, mixedAnswer)

	if res.Outcome != domain.ToolCustom {
		t.Fatalf("outcome = %q, want %q", res.Outcome, domain.ToolCustom)
	}
	want := []domain.ToolDestination{{
		Kind:  domain.DestMain,
		Names: []string{"mcp__github__get_pr", "mcp__github__list_comments", "mcp__jira__*"},
	}}
	if !reflect.DeepEqual(res.Destinations, want) {
		t.Errorf("Destinations = %+v, want %+v", res.Destinations, want)
	}
}

func TestMapToolsEntries_SkippedStillWinsOverAnswer(t *testing.T) {
	d := entriesListDescriptor(claudeLikeForms)

	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:     "test-agent",
		Generic:      []string{"terminal"},
		CustomNames:  domain.CustomToolAnswers{"terminal": mixedAnswer},
		SkippedTools: map[string]bool{"terminal": true},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	if result.Resolutions[0].Outcome != domain.ToolSkipped {
		t.Errorf("outcome = %q, want %q", result.Resolutions[0].Outcome, domain.ToolSkipped)
	}
}

func TestMapToolsEntries_PermissionShapeAllowsEachNameAfterUniversePairs(t *testing.T) {
	d := makePermNoCustomDestDescriptor()
	d.Tools.MCP = claudeLikeForms

	_, result := mapAnswer(t, d, mixedAnswer)
	pairs := permPairs(t, result)

	if len(pairs) != 5 {
		t.Fatalf("want 2 universe pairs and 3 allow pairs, got %v", pairKeys(pairs))
	}
	wantPair(t, pairs, 2, "mcp__github__get_pr", "allow")
	wantPair(t, pairs, 3, "mcp__github__list_comments", "allow")
	wantPair(t, pairs, 4, "mcp__jira__*", "allow")
}

func TestMapToolsEntries_PermissionShapeWritesAnEntryNamedByTwoGenericsOnce(t *testing.T) {
	d := makePermNoCustomDestDescriptor()
	d.Tools.MCP = claudeLikeForms

	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey: "test-agent",
		Generic:  []string{"terminal", "web_fetch"},
		CustomNames: domain.CustomToolAnswers{
			"terminal":  {{Server: "jira"}, {Server: "github", Tool: "get_pr"}},
			"web_fetch": {{Server: "github", Tool: "get_pr"}},
		},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	pairs := permPairs(t, result)

	if len(pairs) != 4 {
		t.Fatalf("want 2 universe pairs and 2 allow pairs, got %v", pairKeys(pairs))
	}
	wantPair(t, pairs, 2, "mcp__jira__*", "allow")
	wantPair(t, pairs, 3, "mcp__github__get_pr", "allow")
}

func TestMapToolsEntries_PermissionShapeMappedNameOutsideUniverseBecomesAllowPair(t *testing.T) {
	d := makePermNoCustomDestDescriptor()
	d.Tools.Mappings = append(d.Tools.Mappings, domain.ToolMapping{
		Generic:      "lookup",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"srv_lookup"}}},
	})

	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey: "test-agent", Generic: []string{"file_read", "lookup"},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	pairs := permPairs(t, result)

	if len(pairs) != 3 {
		t.Fatalf("want 2 universe pairs and 1 allow pair, got %v", pairKeys(pairs))
	}
	wantPair(t, pairs, 0, "read", "allow")
	wantPair(t, pairs, 1, "write", "deny")
	wantPair(t, pairs, 2, "srv_lookup", "allow")
}

func TestMapToolsEntries_PermissionShapeUniverseOnlyMappingsRenderAsBefore(t *testing.T) {
	d := makePermNoCustomDestDescriptor()

	result, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "test-agent", Generic: []string{"file_read"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	pairs := permPairs(t, result)

	if len(pairs) != 2 {
		t.Fatalf("want the two universe pairs only, got %v", pairKeys(pairs))
	}
	wantPair(t, pairs, 0, "read", "allow")
	wantPair(t, pairs, 1, "write", "deny")
}
