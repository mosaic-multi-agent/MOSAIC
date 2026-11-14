package app_test

// deployed_tools_skill_test.go verifies explicit skill access: a source that grants skills
// without writing them (Claude Code, VS Code GHCP) converted to a target that writes skill
// access gives every non-orchestrator agent the skill tool, and orchestrators never.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestConvertDeployedTools_ImplicitSkillSourceGivesNonOrchestratorsExplicitSkill(t *testing.T) {
	sources := []string{harnessClaude, harnessVSCode}
	targets := []string{harnessOpen, harnessCLI}
	roles := []domain.AgentRole{domain.RoleSubagent, domain.RoleUtility, domain.RoleStandalone}
	readByHarness := map[string]string{harnessClaude: "Read", harnessVSCode: "read/readFile"}

	for _, src := range sources {
		for _, tgt := range targets {
			for _, role := range roles {
				name := src + " to " + tgt + " as " + string(role)
				t.Run(name, func(t *testing.T) {
					got := convertTools(t, src, tgt, []string{readByHarness[src]}, withRole(role))

					assertStrings(t, "Generic", []string{"file_read", "skill"}, got.Generic)
					assertStrings(t, "Unmapped", nil, got.Unmapped)
				})
			}
		}
	}
}

func TestConvertDeployedTools_ImplicitSkillSourceNeverGivesOrchestratorsSkill(t *testing.T) {
	for _, src := range []string{harnessClaude, harnessVSCode} {
		for _, tgt := range []string{harnessOpen, harnessCLI} {
			read := "Read"
			if src == harnessVSCode {
				read = "read/readFile"
			}
			got := convertTools(t, src, tgt, []string{read}, withRole(domain.RoleOrchestrator))

			assertStrings(t, "Generic "+src+" to "+tgt, []string{"file_read"}, got.Generic)
		}
	}
}

func TestConvertDeployedTools_SkillAccessNotAddedWhenTargetWritesNoSkillName(t *testing.T) {
	// Claude Code and VS Code GHCP both map skill to an empty destination.
	got := convertTools(t, harnessClaude, harnessVSCode, []string{"Read"})
	assertStrings(t, "Generic claude to vscode", []string{"file_read"}, got.Generic)

	got = convertTools(t, harnessVSCode, harnessClaude, []string{"read/readFile"})
	assertStrings(t, "Generic vscode to claude", []string{"file_read"}, got.Generic)
}

func TestConvertDeployedTools_SkillAccessNotAddedWhenSourceWritesItExplicitly(t *testing.T) {
	// OpenCode and GHCP CLI write skill into the agent file: absence there means no access.
	got := convertTools(t, harnessOpen, harnessCLI, []string{"read"})
	assertStrings(t, "Generic opencode to ghcp-cli", []string{"file_read"}, got.Generic)

	got = convertTools(t, harnessCLI, harnessOpen, []string{"read"})
	assertStrings(t, "Generic ghcp-cli to opencode", []string{"file_read"}, got.Generic)
}

func TestConvertDeployedTools_ExplicitSkillNameInSourceListIsNotDuplicated(t *testing.T) {
	// The source writes skill itself, so this is regression coverage for de-duplication only,
	// not for the "added if absent" skill-access guard.
	got := convertTools(t, harnessCLI, harnessOpen, []string{"read", "skill"}, withRole(domain.RoleSubagent))

	assertStrings(t, "Generic", []string{"file_read", "skill"}, got.Generic)
}

func TestConvertDeployedTools_SkillAccessNotAddedWhenTargetHasNoSkillMappingEntry(t *testing.T) {
	src := loadToolHarness(t, harnessClaude)
	tgt := syntheticHarness("tgt-h", []string{"r"}, map[string][]string{"file_read": {"r"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"Read"})
	in.Source, in.Target = src, tgt

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
}

func TestConvertDeployedTools_SkillAccessAddedToAgentWithOnlyCustomTools(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"mcp__srv__*"}, withRole(domain.RoleUtility))

	assertStrings(t, "Generic", []string{"skill"}, got.Generic)
	assertStrings(t, "TargetTools", []string{"srv_*"}, got.TargetTools)
}

func TestConvertDeployedTools_SkillAccessAddsNoAuditEntry(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, []string{"Read"})

	assertConversionSources(t, got, []string{"Read"})
	assertStrings(t, "TargetTools", nil, got.TargetTools)
}

func TestConvertDeployedTools_SkillAccessAddedToAgentWithNoTools(t *testing.T) {
	got := convertTools(t, harnessClaude, harnessOpen, nil, withRole(domain.RoleSubagent))

	assertStrings(t, "Generic", []string{"skill"}, got.Generic)
}

func TestConvertDeployedTools_SkillAccessNotAddedWhenSourceHasNoSkillMappingEntry(t *testing.T) {
	src := syntheticHarness("src-h", []string{"r"}, map[string][]string{"file_read": {"r"}}, []string{"file_read"})
	in := newToolInput(t, harnessClaude, harnessOpen, []string{"r"})
	in.Source = src

	got := app.ConvertDeployedTools(in)

	assertStrings(t, "Generic", []string{"file_read"}, got.Generic)
}
