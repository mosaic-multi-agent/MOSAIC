package app_test

// transform_vscode_agent_naming_test.go specifies that vscode-ghcp names agents
// .github/agents/X.agent.md, as ghcp-cli does: a transform from another harness into vscode-ghcp
// writes and records X.agent.md, and a transform out of vscode-ghcp recognises both X.agent.md
// and the legacy X.md.

import (
	"testing"

	"mosaic-deploy/internal/app"
)

func TestTransformIntoVSCodeGhcp_WritesAndRecordsAgentMdNames(t *testing.T) {
	for _, src := range []string{hClaude, hOpen} {
		t.Run(src, func(t *testing.T) {
			r := trCarryRun(t, src, hVSCode, nil)
			r.seed("subagent")

			res := r.mustRun(r.request())

			f := trFile(t, res, "subagent")
			const want = ".github/agents/subagent.agent.md"
			if f.Status != app.StatusTransformed || f.DestinationPath != want {
				t.Fatalf("outcome %q at %q, want transformed at %s", f.Status, f.DestinationPath, want)
			}
			if !r.exists(want) || r.exists(".github/agents/subagent.md") {
				t.Errorf("written files = %v, want only %s", r.snapshot(), want)
			}
			if e, ok := trEntryAt(trLoadManifest(t, r.ws), want); !ok || e.Ref.Key != "subagent" {
				t.Errorf("manifest entry at %s = %+v (found %t), want the agent entry", want, e, ok)
			}
		})
	}
}

func TestTransformIntoVSCodeGhcp_DryRunReportsTheAgentMdDestination(t *testing.T) {
	r := trCarryRun(t, hClaude, hVSCode, nil)
	r.seed("subagent")
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if f := trFile(t, res, "subagent"); f.DestinationPath != ".github/agents/subagent.agent.md" {
		t.Errorf("destination %q, want .github/agents/subagent.agent.md", f.DestinationPath)
	}
}

func TestTransformOutOfVSCodeGhcp_RecognisesAgentMdAndLegacyMdAgents(t *testing.T) {
	r := trCarryRun(t, hVSCode, hClaude, nil)
	trWrite(t, r.ws, ".github/agents/current.agent.md", trFixture(t, "subagent"))
	trWrite(t, r.ws, ".github/agents/legacy.md", trFixture(t, "subagent"))

	res := r.mustRun(r.request())

	for _, key := range []string{"current", "legacy"} {
		f := trFile(t, res, key)
		want := ".claude/agents/" + key + ".md"
		if f.Status != app.StatusTransformed || f.DestinationPath != want {
			t.Errorf("%s: outcome %q at %q, want transformed at %s", key, f.Status, f.DestinationPath, want)
		}
		if !r.exists(want) {
			t.Errorf("%s was not written", want)
		}
	}
}
