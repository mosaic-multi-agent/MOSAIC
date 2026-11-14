package app_test

// deployed_tools_mcp_split_test.go verifies how an ambiguous OpenCode single-tool name
// (a-b_c_d can split after a-b or after a-b_c) is resolved, in this order and never by a naming
// heuristic: a tool mapping naming the OpenCode tool, then the server names declared in the
// OpenCode MCP configuration (longest declared name followed by "_"), then an answered split.
// A name still unresolved is reported ambiguous and is not written.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

const splitName = "a-b_c_d"

// openCodeNamingTool returns the OpenCode descriptor with an extra destination name on the
// mapping of the generic tool.
func openCodeNamingTool(t *testing.T, generic, name string) *domain.HarnessDescriptor {
	t.Helper()
	d := loadToolHarness(t, harnessOpen)
	for i := range d.Tools.Mappings {
		m := &d.Tools.Mappings[i]
		if m.Generic != generic {
			continue
		}
		for j := range m.Destinations {
			if m.Destinations[j].Kind == domain.DestMain {
				m.Destinations[j].Names = append(m.Destinations[j].Names, name)
				return d
			}
		}
	}
	t.Fatalf("opencode has no main destination for %q", generic)
	return nil
}

func TestConvertDeployedTools_AmbiguousNameWithNothingToDecideIsReportedNotWritten(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName})

	want := []app.AmbiguousName{{
		Source:      splitName,
		Disposition: domain.Allow,
		Splits:      []mcpname.Name{{Server: "a-b", Tool: "c_d"}, {Server: "a-b_c", Tool: "d"}},
	}}
	if !reflect.DeepEqual(got.Ambiguous, want) {
		t.Errorf("Ambiguous: want %+v, got %+v", want, got.Ambiguous)
	}
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertRule(t, got, splitName, app.ToolRuleAmbiguous)
}

func TestConvertDeployedTools_AmbiguousDenyKeepsItsDisposition(t *testing.T) {
	got := convertGrants(t, harnessOpen, harnessCLI, []domain.ToolGrant{denyGrant(splitName)})

	if len(got.Ambiguous) != 1 || got.Ambiguous[0].Disposition != domain.Deny {
		t.Fatalf("want one ambiguous deny, got %+v", got.Ambiguous)
	}
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_RepeatedAmbiguousNameIsReportedOnce(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName, splitName, "x-y_p_q"})

	if len(got.Ambiguous) != 2 || got.Ambiguous[0].Source != splitName || got.Ambiguous[1].Source != "x-y_p_q" {
		t.Errorf("want the two distinct names once each in document order, got %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_NameWithOneSplitIsNeverAmbiguous(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{"contact-user_*", "contact-user*", "srv_lookup"})

	assertStrings(t, "TargetTools", []string{"contact-user/*", "srv/lookup"}, got.TargetTools)
	if len(got.Ambiguous) != 0 {
		t.Errorf("Ambiguous: want none, got %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_ToolMappingResolvesAnAmbiguousNameThroughItsGeneric(t *testing.T) {
	in := newToolInput(t, harnessOpen, harnessCLI, []string{splitName})
	in.Source = openCodeNamingTool(t, "file_read", splitName)

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, splitName, app.ToolRuleMapped)
	if !containsString(got.Generic, "file_read") {
		t.Errorf("Generic must carry file_read, got %v", got.Generic)
	}
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	if len(got.Ambiguous) != 0 {
		t.Errorf("a mapped name is not ambiguous, got %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_ToolMappingBeatsDeclaredServersAndAnswers(t *testing.T) {
	in := newToolInput(t, harnessOpen, harnessCLI, []string{splitName},
		withDeclaredServers("a-b"), withSplitAnswers(map[string]string{splitName: "a-b_c"}))
	in.Source = openCodeNamingTool(t, "file_read", splitName)

	got := app.ConvertDeployedTools(in)

	assertRule(t, got, splitName, app.ToolRuleMapped)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_DeclaredServerDecidesTheSplit(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName}, withDeclaredServers("a-b"))

	assertStrings(t, "TargetTools", []string{"a-b/c_d"}, got.TargetTools)
	assertRuleOf(t, got, splitName, app.ToolRuleMCP, "a-b", "c_d")
	if len(got.Ambiguous) != 0 {
		t.Errorf("Ambiguous: want none, got %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_LongestDeclaredServerFollowedByUnderscoreWins(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName}, withDeclaredServers("a-b", "a-b_c", "a"))

	assertStrings(t, "TargetTools", []string{"a-b_c/d"}, got.TargetTools)
	assertRuleOf(t, got, splitName, app.ToolRuleMCP, "a-b_c", "d")
}

func TestConvertDeployedTools_DeclaredServerNotFollowedByUnderscoreDoesNotDecide(t *testing.T) {
	// "a" is a prefix of the name but is followed by "-", and "other" is no prefix at all.
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName}, withDeclaredServers("a", "other", "a-b_c_d"))

	if len(got.Ambiguous) != 1 {
		t.Fatalf("the name must stay ambiguous, got Ambiguous %+v TargetTools %v", got.Ambiguous, got.TargetTools)
	}
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_DeclaredServerAppliesToDeniedNamesToo(t *testing.T) {
	target := permissionTarget(t)
	in := newToolInput(t, harnessOpen, harnessOpen, nil, withDeclaredServers("a-b_c"))
	in.Target = target
	in.Tools = []domain.ToolGrant{denyGrant(splitName)}

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "TargetDenied", []string{"a-b_c.d"}, got.TargetDenied)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_AnsweredSplitIsWrittenInTargetForm(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName},
		withSplitAnswers(map[string]string{splitName: "a-b_c"}))

	assertStrings(t, "TargetTools", []string{"a-b_c/d"}, got.TargetTools)
	assertRuleOf(t, got, splitName, app.ToolRuleMCP, "a-b_c", "d")
	if len(got.Ambiguous) != 0 {
		t.Errorf("Ambiguous: want none, got %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_AnswerNamingNoOfferedSplitLeavesTheNameAmbiguous(t *testing.T) {
	got := convertTools(t, harnessOpen, harnessCLI, []string{splitName},
		withSplitAnswers(map[string]string{splitName: "zzz"}))

	if len(got.Ambiguous) != 1 {
		t.Errorf("Ambiguous: want the name still reported, got %+v", got.Ambiguous)
	}
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_ExactSourceNameIsNeverSplitByANamingHeuristic(t *testing.T) {
	// Even a declared server that merely looks like the first segment of another name decides
	// nothing for that other name.
	got := convertTools(t, harnessOpen, harnessCLI, []string{"x-y_p_q"}, withDeclaredServers("a-b"))

	if len(got.Ambiguous) != 1 || got.Ambiguous[0].Source != "x-y_p_q" {
		t.Errorf("want x-y_p_q ambiguous, got %+v", got.Ambiguous)
	}
}
