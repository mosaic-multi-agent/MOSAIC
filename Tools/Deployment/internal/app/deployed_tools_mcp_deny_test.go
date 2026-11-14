package app_test

// deployed_tools_mcp_deny_test.go verifies that allow and deny survive the tool conversion: a
// name denied in the source is never granted in the target, is written as a deny where the
// target can express one, and is omitted (reported dropped) where it cannot.

import (
	"slices"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// permissionTarget returns an OpenCode-shaped permission descriptor with its own MCP forms, so a
// written name is distinguishable from the source's.
func permissionTarget(t *testing.T) *domain.HarnessDescriptor {
	t.Helper()
	d := loadToolHarness(t, harnessOpen)
	d.ID = "opencode-copy"
	d.Tools.MCP = domain.MCPNameForms{ServerForm: "{server}.*", ToolForm: "{server}.{tool}"}
	return d
}

func TestConvertDeployedTools_DeniedUniverseNamesAddNothing(t *testing.T) {
	// A permission map lists every tool of the harness; a deny there is listing, not intent.
	got := convertGrants(t, harnessOpen, harnessClaude, []domain.ToolGrant{
		allowGrant("read"), denyGrant("bash"), denyGrant("edit"), denyGrant("patch"),
	})

	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
	assertStrings(t, "TargetTools", nil, got.TargetTools)
	assertStrings(t, "TargetDenied", nil, got.TargetDenied)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_DeniedMCPNameIsOmittedWhereTheTargetCannotExpressDeny(t *testing.T) {
	grants := []domain.ToolGrant{allowGrant("keep_*"), denyGrant("srv_*"), denyGrant("other_lookup")}
	for _, tgt := range []string{harnessClaude, harnessCLI, harnessVSCode} {
		got := convertGrants(t, harnessOpen, tgt, grants)

		for _, name := range append(append([]string(nil), got.TargetTools...), got.TargetDenied...) {
			if slices.Contains([]string{"srv/*", "mcp__srv__*", "other/lookup", "mcp__other__lookup"}, name) {
				t.Errorf("target %s: denied name written as %q", tgt, name)
			}
		}
		assertStrings(t, "TargetDenied for "+tgt, nil, got.TargetDenied)
		assertStrings(t, "Dropped for "+tgt, []string{"srv_*", "other_lookup"}, droppedNames(got))
		wantReason := "denied in opencode; " + loadToolHarness(t, tgt).ID + " cannot express deny"
		for _, d := range got.Dropped {
			if d.Reason != wantReason {
				t.Errorf("target %s: drop reason for %s: want %q, got %q", tgt, d.Name, wantReason, d.Reason)
			}
		}
		if len(got.TargetTools) != 1 {
			t.Errorf("target %s: only the allowed server may be written, got %v", tgt, got.TargetTools)
		}
	}
}

func TestConvertDeployedTools_DeniedMCPNameIsWrittenAsDenyForPermissionTargets(t *testing.T) {
	in := newToolInput(t, harnessOpen, harnessOpen, nil)
	in.Target = permissionTarget(t)
	in.Tools = []domain.ToolGrant{allowGrant("keep_*"), denyGrant("srv_*"), denyGrant("other_lookup")}

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "TargetTools", []string{"keep.*"}, got.TargetTools)
	assertStrings(t, "TargetDenied", []string{"srv.*", "other.lookup"}, got.TargetDenied)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertRuleOf(t, got, "srv_*", app.ToolRuleMCP, "srv", "")
}

func TestConvertDeployedTools_DeniedNameValidInTargetVerbatimFollowsTheTargetShape(t *testing.T) {
	// "native" is not a source tool; it is a tool the target declares.
	list := syntheticHarness("list-target", []string{"native"}, nil, nil)
	perm := syntheticHarness("perm-target", []string{"native"}, nil, nil)
	perm.Tools.Shape = domain.ShapePermission

	inList := newToolInput(t, harnessOpen, harnessClaude, nil)
	inList.Target, inList.Tools = list, []domain.ToolGrant{denyGrant("native")}
	gotList := app.ConvertDeployedTools(inList)
	assertStrings(t, "list TargetTools", nil, gotList.TargetTools)
	assertStrings(t, "list Dropped", []string{"native"}, droppedNames(gotList))

	inPerm := newToolInput(t, harnessOpen, harnessClaude, nil)
	inPerm.Target, inPerm.Tools = perm, []domain.ToolGrant{denyGrant("native")}
	gotPerm := app.ConvertDeployedTools(inPerm)
	assertStrings(t, "permission TargetTools", nil, gotPerm.TargetTools)
	assertStrings(t, "permission TargetDenied", []string{"native"}, gotPerm.TargetDenied)
}

func TestConvertDeployedTools_DeniedMappedToolIsNeverGrantedInTarget(t *testing.T) {
	// A deny on a name the source reverse mapping claims adds no generic tool.
	got := convertGrants(t, harnessOpen, harnessCLI, []domain.ToolGrant{denyGrant("read"), denyGrant("grep")})

	for _, g := range got.Generic {
		if g == "file_read" || g == "content_search" {
			t.Errorf("a denied mapped tool must not become generic %q, got %v", g, got.Generic)
		}
	}
	assertStrings(t, "Generic", nil, got.Generic)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_DenyDoesNotCountTowardThePlaceholderExpansion(t *testing.T) {
	expansion := []string{"read", "write", "edit", "glob", "grep", "bash", "question", "task"}
	grants := allowToolGrants(expansion[:7])
	grants = append(grants, denyGrant("task"))

	got := convertGrants(t, harnessOpen, harnessClaude, grants,
		withRole(domain.RoleOrchestrator), withPlaceholderAgent())

	if got.UsePlaceholder {
		t.Error("UsePlaceholder: a denied expansion name must not complete the expansion")
	}
}
