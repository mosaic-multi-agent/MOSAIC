package app_test

// rebuild_mcp_names_test.go covers the wiring of MCP names into the rebuilt agent file: names
// are written in the target harness's forms, a denied name is never granted, ambiguous
// OpenCode names are resolved by the declared servers or an answered split, and an unresolved
// one is reported unmapped with a gap and written nowhere.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// openCodeToGhcpCli is an OpenCode source rebuilt for GHCP CLI, whose tools field is a list.
var openCodeToGhcpCli = rbPair{"opencode-to-ghcp-cli", hOpen, hCLI}

// withMCPPermissions adds a whole-server allow, an ambiguous single-tool allow and a denied
// server to the OpenCode fixture's permission map.
var withMCPPermissions = withSourceEdit(replaceText("  bash: allow\n",
	"  bash: allow\n  contact-user*: allow\n  tracker_get_issue: allow\n  blocked_*: deny\n"))

// withDeclared sets the servers declared in the source harness's MCP configuration.
func withDeclared(servers ...string) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.DeclaredServers = servers }
}

// withMCPSplits sets the answered splits (source name -> server).
func withMCPSplits(splits map[string]string) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.MCPSplits = splits }
}

func unmappedSubjectGap(gaps []domain.Gap, subject string) bool {
	for _, g := range gaps {
		if g.Kind == domain.GapUnmappedTool && g.Subject == subject {
			return true
		}
	}
	return false
}

func TestRebuild_McpNamesAreWrittenInTargetFormsAndNeverAsSourceText(t *testing.T) {
	res, out := rbRebuild(t, rbInput(t, openCodeToGhcpCli, false, withMCPPermissions, withDeclared("tracker")))

	names := rbToolNames(t, out, "tools")
	for _, want := range []string{"contact-user/*", "tracker/get_issue"} {
		if countName(names, want) != 1 {
			t.Errorf("%q must be written exactly once, got %v", want, names)
			continue
		}
		if countName(res.Tools, want) != 1 {
			t.Errorf("RebuildResult.Tools must list %q exactly once, got %v", want, res.Tools)
		}
	}
	for _, bad := range []string{"contact-user*", "tracker_get_issue", "blocked_*", "blocked/*"} {
		if containsString(names, bad) {
			t.Errorf("%q must not be written into the GHCP CLI agent, got %v", bad, names)
		}
	}
	if unmappedSubjectGap(res.Gaps, "tracker_get_issue") || containsString(res.UnmappedTools, "tracker_get_issue") {
		t.Errorf("a name resolved by the declared server must not be unmapped: %v %+v", res.UnmappedTools, res.Gaps)
	}
}

func TestRebuild_DeniedMcpNameIsReportedDroppedWhereTargetCannotExpressDeny(t *testing.T) {
	res, _ := rbRebuild(t, rbInput(t, openCodeToGhcpCli, false, withMCPPermissions, withDeclared("tracker")))

	found := false
	for _, d := range res.Dropped {
		if d.Name == "blocked_*" {
			found = true
		}
	}
	if !found {
		t.Errorf("Dropped must list the denied blocked_*, got %+v", res.Dropped)
	}
}

func TestRebuild_AmbiguousNameWithoutDeclaredServerOrAnswerIsUnmappedAndNotWritten(t *testing.T) {
	res, out := rbRebuild(t, rbInput(t, openCodeToGhcpCli, false, withMCPPermissions))

	names := rbToolNames(t, out, "tools")
	for _, bad := range []string{"tracker_get_issue", "tracker/get_issue", "tracker_get/issue"} {
		if containsString(names, bad) {
			t.Errorf("an unresolved ambiguous name must not be written, found %q in %v", bad, names)
		}
	}
	if !containsString(res.UnmappedTools, "tracker_get_issue") {
		t.Errorf("UnmappedTools must list the ambiguous name, got %v", res.UnmappedTools)
	}
	if !unmappedSubjectGap(res.Gaps, "tracker_get_issue") {
		t.Errorf("an unmapped-tool gap must name tracker_get_issue, got %+v", res.Gaps)
	}
	if !containsString(names, "contact-user/*") {
		t.Errorf("the unambiguous whole-server name must still be written, got %v", names)
	}
}

func TestRebuild_AnsweredSplitIsWrittenInTargetForm(t *testing.T) {
	splits := withMCPSplits(map[string]string{"tracker_get_issue": "tracker_get"})

	res, out := rbRebuild(t, rbInput(t, openCodeToGhcpCli, false, withMCPPermissions, splits))

	if names := rbToolNames(t, out, "tools"); countName(names, "tracker_get/issue") != 1 {
		t.Errorf("the answered split must be written once as tracker_get/issue, got %v", names)
	}
	if containsString(res.UnmappedTools, "tracker_get_issue") {
		t.Errorf("an answered name must not be unmapped, got %v", res.UnmappedTools)
	}
}

func TestRebuild_DeclaredServerBeatsAnAnswerForTheSameName(t *testing.T) {
	// The resolution order is mapping, declared servers, answer: a declared server decides first.
	splits := withMCPSplits(map[string]string{"tracker_get_issue": "tracker_get"})

	_, out := rbRebuild(t, rbInput(t, openCodeToGhcpCli, false, withMCPPermissions, withDeclared("tracker"), splits))

	if names := rbToolNames(t, out, "tools"); countName(names, "tracker/get_issue") != 1 {
		t.Errorf("the declared server must decide the split, got %v", names)
	}
}
