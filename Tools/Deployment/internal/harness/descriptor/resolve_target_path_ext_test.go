package descriptor_test

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// extDescriptor builds a minimal descriptor with an agents directory and, when ext is
// non-empty, a declared agent extension. Skills get their own directory.
func extDescriptor(agentExt string) *domain.HarnessDescriptor {
	d := &domain.HarnessDescriptor{
		Paths: domain.PathSpec{
			Agents: domain.ScopedPaths{Supported: true, Project: ".h/agents"},
			Skills: domain.ScopedPaths{Supported: true, Project: ".h/skills"},
		},
	}
	if agentExt != "" {
		d.Extensions = map[domain.ArtifactKind]string{domain.ArtifactAgent: agentExt}
	}
	return d
}

func TestResolveTargetPath_AgentFileNameAcrossExtensions(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		fileName string
		key      string
		want     string
	}{
		{"ghcp keeps agent md name verbatim", ".agent.md", "X.agent.md", "X", ".h/agents/X.agent.md"},
		{"ghcp adds agent md to plain md name", ".agent.md", "X.md", "X", ".h/agents/X.agent.md"},
		{"ghcp from key only", ".agent.md", "", "X", ".h/agents/X.agent.md"},
		{"md harness keeps plain md name", ".md", "X.md", "X", ".h/agents/X.md"},
		{"md harness from key only", ".md", "", "X", ".h/agents/X.md"},
		{"codex converts md name to toml", ".toml", "X.md", "X", ".h/agents/X.toml"},
		{"codex keeps toml name", ".toml", "X.toml", "X", ".h/agents/X.toml"},
		{"codex from key only", ".toml", "", "X", ".h/agents/X.toml"},
		{"dotted key keeps its dots under ghcp", ".agent.md", "a.b.agent.md", "a.b", ".h/agents/a.b.agent.md"},
		{"no declared extension keeps the file name", "", "X.md", "X", ".h/agents/X.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := descriptor.ResolveTargetPath(extDescriptor(tc.ext), domain.TargetPathRequest{
				Kind:     domain.ArtifactAgent,
				Key:      tc.key,
				FileName: tc.fileName,
				Scope:    domain.ScopeProject,
				GOOS:     "linux",
			})
			if err != nil {
				t.Fatalf("ResolveTargetPath: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveTargetPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveTargetPath_SkillIgnoresAgentExtension(t *testing.T) {
	d := extDescriptor(".agent.md")
	d.Extensions[domain.ArtifactSkill] = ""

	got, err := descriptor.ResolveTargetPath(d, domain.TargetPathRequest{
		Kind:     domain.ArtifactSkill,
		Key:      "myskill",
		FileName: "SKILL.md",
		Scope:    domain.ScopeProject,
		GOOS:     "linux",
	})

	if err != nil {
		t.Fatalf("ResolveTargetPath: %v", err)
	}
	if want := ".h/skills/myskill/SKILL.md"; got != want {
		t.Errorf("ResolveTargetPath = %q, want %q", got, want)
	}
}
