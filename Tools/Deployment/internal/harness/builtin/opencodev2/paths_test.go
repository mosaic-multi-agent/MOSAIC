package opencodev2_test

// Tests for where the OpenCode V2 module places agents, skills and hook files, and that the
// module satisfies the shared harness module contract suite.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/contracttest"
)

func targetPath(t *testing.T, req domain.TargetPathRequest) string {
	t.Helper()
	req.Scope = domain.ScopeProject
	req.GOOS = "linux"
	got, err := newModule(t).TargetPath(req)
	if err != nil {
		t.Fatalf("TargetPath(%+v): %v", req, err)
	}
	return got
}

func TestTargetPath_AgentGoesToOpenCodeAgentsFolder(t *testing.T) {
	got := targetPath(t, domain.TargetPathRequest{Kind: domain.ArtifactAgent, Key: "test-runner"})

	if want := ".opencode/agents/test-runner.md"; got != want {
		t.Errorf("TargetPath = %q, want %q", got, want)
	}
}

func TestTargetPath_SkillGetsItsOwnFolderWithSkillMd(t *testing.T) {
	got := targetPath(t, domain.TargetPathRequest{Kind: domain.ArtifactSkill, Key: "lean-tdd", FileName: "SKILL.md"})

	if want := ".opencode/skills/lean-tdd/SKILL.md"; got != want {
		t.Errorf("TargetPath = %q, want %q", got, want)
	}
}

func TestTargetPath_TwoSkillsDoNotCollide(t *testing.T) {
	a := targetPath(t, domain.TargetPathRequest{Kind: domain.ArtifactSkill, Key: "skill-a", FileName: "SKILL.md"})
	b := targetPath(t, domain.TargetPathRequest{Kind: domain.ArtifactSkill, Key: "skill-b", FileName: "SKILL.md"})

	if a == b {
		t.Errorf("both skills resolve to %q; each skill needs its own folder", a)
	}
}

func TestTargetPath_HookFilesGoToThePluginsFolderIncludingSubfolders(t *testing.T) {
	cases := map[string]string{
		"plugin.ts":     ".opencode/plugins/plugin.ts",
		"lib/helper.ts": ".opencode/plugins/lib/helper.ts",
	}
	for fileName, want := range cases {
		t.Run(fileName, func(t *testing.T) {
			got := targetPath(t, domain.TargetPathRequest{Kind: domain.ArtifactHook, Key: "plugin-bundle", FileName: fileName})

			if got != want {
				t.Errorf("TargetPath = %q, want %q", got, want)
			}
		})
	}
}

func TestContract_OpenCodeV2(t *testing.T) {
	mod := newModule(t)
	bundle := loadFixtureBundle(t, "plugin-bundle")

	contracttest.Run(t, mod, contracttest.Fixtures{
		TargetPathCases: []contracttest.TargetPathCase{
			{
				Name:     "agent_project_scope",
				Request:  domain.TargetPathRequest{Kind: domain.ArtifactAgent, Scope: domain.ScopeProject, Key: "my-agent"},
				Expected: ".opencode/agents/my-agent.md",
			},
			{
				Name: "skill_project_scope_with_key_subdir",
				Request: domain.TargetPathRequest{
					Kind: domain.ArtifactSkill, Scope: domain.ScopeProject, Key: "lean-tdd", FileName: "SKILL.md",
				},
				Expected: ".opencode/skills/lean-tdd/SKILL.md",
			},
		},
		HookPlanCases: []contracttest.HookPlanCase{
			{
				Name:      "plugin_files_with_helper_subfolder",
				Request:   domain.HookPlanRequest{Bundle: bundle, Scope: domain.ScopeProject},
				Supported: true,
				TargetDir: ".opencode/plugins",
				FileNames: []string{"plugin.ts", "lib/helper.ts"},
			},
		},
	})
}
