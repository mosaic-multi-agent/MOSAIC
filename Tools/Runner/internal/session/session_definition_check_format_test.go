package session_test

// Run start resolves and checks agent definitions through the harness's agent
// file data. A harness without a catalog entry gets format data only through
// the test-only seam (here: Markdown files with the .mdx extension); a harness
// with neither keeps today's Markdown resolution and no definition check. A
// used definition that cannot be loaded refuses the run at start, naming the
// agent and its file, before any invocation or artifact write; unused agent
// files do not matter.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentload"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/agentresolve"
)

const (
	seamAgentsRel = ".seam/agents"
	seamExtension = ".mdx"

	// undecodableMarkdown has frontmatter the Markdown reader rejects.
	undecodableMarkdown = "---\nname: broken\nmodel: [unclosed\n---\nbody\n"
)

// newSeamRig deploys the linear workflow under a harness that has no catalog
// entry, whose Markdown agent files carry the .mdx extension through the
// test-only format seam.
func newSeamRig(t *testing.T, harnessID, fixture string, infra []string) *defRig {
	t.Helper()
	unregister := agentfile.RegisterTestOnly(harnessID, agentfile.Format{
		AgentsDir: seamAgentsRel, FormatID: "markdown", Extension: seamExtension,
	})
	t.Cleanup(unregister)
	rig := newDefRig(t, cliLayout{harnessID: harnessID, agentsRel: seamAgentsRel}, fixture, infra)
	entries, err := os.ReadDir(rig.agentsDir)
	if err != nil {
		t.Fatalf("newSeamRig: read agents dir: %v", err)
	}
	for _, e := range entries {
		from := filepath.Join(rig.agentsDir, e.Name())
		to := strings.TrimSuffix(from, ".md") + seamExtension
		if err := os.Rename(from, to); err != nil {
			t.Fatalf("newSeamRig: rename %s: %v", e.Name(), err)
		}
	}
	rig.orchPath = strings.TrimSuffix(rig.orchPath, ".md") + seamExtension
	return rig
}

// seamPath is the deployed definition file of agent id in a seam rig.
func seamPath(r *defRig, id string) string {
	return filepath.Join(r.agentsDir, id+seamExtension)
}

// writeSeamFile replaces the definition of agent id in a seam rig.
func writeSeamFile(t *testing.T, r *defRig, id, content string) {
	t.Helper()
	if err := os.WriteFile(seamPath(r, id), []byte(content), 0o600); err != nil {
		t.Fatalf("writeSeamFile(%q): %v", id, err)
	}
}

func TestSession_Start_SeamHarness_ValidExtensionFiles_StartsNormally(t *testing.T) {
	rig := newSeamRig(t, "seam-run-valid", "linear-orch.md", nil)

	res := rig.start(t, rig.config())

	requireStarted(t, res)
}

func TestSession_Start_SeamHarness_AgentWithoutModel_RefusesRun(t *testing.T) {
	rig := newSeamRig(t, "seam-run-nomodel", "linear-orch.md", nil)
	writeSeamFile(t, rig, "agent-a", definitionText("seam-run-nomodel", "agent-a", true, withoutModel))

	res := rig.start(t, rig.config())

	requireDefinitionRefusal(t, res, "agent-a", seamPath(rig, "agent-a"), agentresolve.ErrDefinitionModelMissing)
}

func TestSession_Start_SeamHarness_UndecodableAgent_RefusesRunBeforeAnyInvocationOrWrite(t *testing.T) {
	rig := newSeamRig(t, "seam-run-undecodable", "linear-orch.md", nil)
	writeSeamFile(t, rig, "agent-b", undecodableMarkdown)

	res := rig.start(t, rig.config())

	requireDefinitionRefusal(t, res, "agent-b", seamPath(rig, "agent-b"), agentload.ErrUndecodable)
}

func TestSession_Start_SeamHarness_UndecodableInfrastructureAgent_RefusesRun(t *testing.T) {
	rig := newSeamRig(t, "seam-run-infra", multiInfraFixture, multiInfraAgents)
	writeSeamFile(t, rig, selectedCommit, undecodableMarkdown)

	res := rig.start(t, rig.config())

	requireDefinitionRefusal(t, res, selectedCommit, seamPath(rig, selectedCommit), agentload.ErrUndecodable)
}

func TestSession_Start_SeamHarness_UndecodableUnusedAgentFile_DoesNotAffectRun(t *testing.T) {
	rig := newSeamRig(t, "seam-run-unused", "linear-orch.md", nil)
	writeSeamFile(t, rig, "unused-agent", undecodableMarkdown)

	res := rig.start(t, rig.config())

	requireStarted(t, res)
}

func TestSession_Start_CatalogHarness_UndecodableAgent_RefusesWithLoaderCause(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, "linear-orch.md", nil)
			if err := os.WriteFile(rig.path("agent-b"), []byte(undecodableMarkdown), 0o600); err != nil {
				t.Fatal(err)
			}

			res := rig.start(t, rig.config())

			requireDefinitionRefusal(t, res, "agent-b", rig.path("agent-b"), agentload.ErrUndecodable)
		})
	}
}

func TestSession_Start_HarnessWithoutFormatData_ResolvesMarkdownAndSkipsDefinitionCheck(t *testing.T) {
	rig := newDefRig(t, cliLayout{harnessID: "no-format-data-harness", agentsRel: ".nf/agents"}, "linear-orch.md", nil)
	rig.mutate(t, "agent-a", withoutModel)

	res := rig.start(t, rig.config())

	requireStarted(t, res)
}
