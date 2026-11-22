package app_test

// deployed_tools_v2_rules_test.go verifies the deployed tool conversion with the opencode-v2
// harness, whose tools field is a rule list. As a target, a name the source denies is only
// withheld from the grants: the target's leading deny-all rule already denies it, so no deny is
// written and nothing is reported dropped. As a source, the allowed actions read back as the
// generic tools they grant.

import (
	"slices"
	"testing"

	"mosaic-deploy/internal/domain"
)

const harnessOpenV2 = "opencode-v2.yaml"

func TestConvertDeployedTools_RulesTargetWithholdsDeniedMCPNamesWithoutWritingOrReportingThem(t *testing.T) {
	got := convertGrants(t, harnessOpen, harnessOpenV2, []domain.ToolGrant{
		allowGrant("keep_*"), denyGrant("srv_*"), denyGrant("other_lookup"),
	})

	assertStrings(t, "TargetTools", []string{"keep_*"}, got.TargetTools)
	assertStrings(t, "TargetDenied", nil, got.TargetDenied)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_RulesTargetNeverGrantsANameTheSourceAlsoDenies(t *testing.T) {
	got := convertGrants(t, harnessOpen, harnessOpenV2, []domain.ToolGrant{
		allowGrant("srv_lookup"), denyGrant("srv_lookup"), allowGrant("keep_*"),
	})

	if slices.Contains(got.TargetTools, "srv_lookup") {
		t.Errorf("a denied name must not be granted, got TargetTools %v", got.TargetTools)
	}
	assertStrings(t, "TargetDenied", nil, got.TargetDenied)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	assertStrings(t, "TargetTools", []string{"keep_*"}, got.TargetTools)
}

func TestConvertDeployedTools_PermissionTargetStillWritesDeniedMCPNames(t *testing.T) {
	// The V1 permission shape is unchanged by the rules shape.
	got := convertGrants(t, harnessOpen, harnessOpen, []domain.ToolGrant{allowGrant("keep_*"), denyGrant("srv_*")})

	assertStrings(t, "TargetDenied", []string{"srv_*"}, got.TargetDenied)
}

func TestConvertDeployedTools_RulesSourceReadsFileWritingActionsBackAsBothFileTools(t *testing.T) {
	// An allowed edit action reads back as write, edit and patch.
	got := convertGrants(t, harnessOpenV2, harnessClaude, []domain.ToolGrant{
		allowGrant("read"), allowGrant("write"), allowGrant("edit"), allowGrant("patch"), allowGrant("shell"),
	})

	for _, want := range []string{"file_read", "file_write", "file_edit", "terminal"} {
		if !slices.Contains(got.Generic, want) {
			t.Errorf("Generic = %v, want it to contain %q", got.Generic, want)
		}
	}
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_RulesSourceMCPServerIsWrittenInTheTargetForm(t *testing.T) {
	got := convertGrants(t, harnessOpenV2, harnessClaude, []domain.ToolGrant{allowGrant("read"), allowGrant("hitl_*")})

	assertStrings(t, "TargetTools", []string{"mcp__hitl__*"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}

func TestConvertDeployedTools_RulesToRulesKeepsMCPServersAndDeniedNamesStayOut(t *testing.T) {
	got := convertGrants(t, harnessOpenV2, harnessOpenV2, []domain.ToolGrant{
		allowGrant("read"), allowGrant("hitl_*"), denyGrant("blocked_*"),
	})

	assertStrings(t, "TargetTools", []string{"hitl_*"}, got.TargetTools)
	assertStrings(t, "TargetDenied", nil, got.TargetDenied)
	assertStrings(t, "Dropped", nil, droppedNames(got))
}
