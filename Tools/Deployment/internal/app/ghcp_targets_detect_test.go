package app

// ghcp_targets_detect_test.go verifies which GHCP targets a workspace is classified as
// holding. The only signal is the manifest harness_id of a usable manifest; agent file names
// (X.md or X.agent.md, recorded or not) never change the result.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// ghcpSnap builds a manifest snapshot with the given state, harness id and agent entries.
func ghcpSnap(state manifest.State, harnessID string, agentPaths ...string) manifest.Snapshot {
	m := domain.Manifest{HarnessID: harnessID}
	for i, p := range agentPaths {
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "k" + string(rune('a'+i))},
			TargetPath: p,
		})
	}
	return manifest.Snapshot{State: state, Manifest: m}
}

type ghcpDetectCase struct {
	name string
	snap manifest.Snapshot
	want ghcpTargets
}

func runGhcpDetectCases(t *testing.T, cases []ghcpDetectCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := detectGhcpTargets(tc.snap)

			// Assert
			if got != tc.want {
				t.Errorf("detectGhcpTargets = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDetectGhcpTargets_HarnessIDDecides covers the harness_id rule, whatever the agent files
// recorded in the manifest are called.
func TestDetectGhcpTargets_HarnessIDDecides(t *testing.T) {
	present := manifest.StatePresent
	const (
		plainMd = ".github/agents/x.md"
		agentMd = ".github/agents/x.agent.md"
	)
	var cases []ghcpDetectCase
	for _, id := range []struct {
		harness string
		want    ghcpTargets
	}{
		{"ghcp-cli", ghcpTargets{CLI: true}},
		{"vscode-ghcp", ghcpTargets{VSCode: true}},
	} {
		for _, v := range []struct {
			name   string
			agents []string
		}{
			{"no agents", nil},
			{"X.md agents", []string{plainMd, ".github/agents/y.md"}},
			{"X.agent.md agents", []string{agentMd, ".github/agents/y.agent.md"}},
			{"mixed X.md and X.agent.md agents", []string{plainMd, ".github/agents/y.agent.md"}},
			{"upper-case extension", []string{".github/agents/x.AGENT.MD", ".github/agents/z.MD"}},
			{"backslash target paths", []string{`.github\agents\x.md`, `.github\agents\y.agent.md`}},
		} {
			cases = append(cases, ghcpDetectCase{
				name: id.harness + " with " + v.name,
				snap: ghcpSnap(present, id.harness, v.agents...),
				want: id.want,
			})
		}
	}
	runGhcpDetectCases(t, cases)
}

// TestDetectGhcpTargets_NonGhcpHarnessHoldsNoTarget verifies that GHCP-looking agent files do
// not make a workspace whose manifest names another harness hold a GHCP target.
func TestDetectGhcpTargets_NonGhcpHarnessHoldsNoTarget(t *testing.T) {
	present := manifest.StatePresent
	runGhcpDetectCases(t, []ghcpDetectCase{
		{"claude-code without agents", ghcpSnap(present, "claude-code"), ghcpTargets{}},
		{"claude-code with X.md agents in .github/agents",
			ghcpSnap(present, "claude-code", ".github/agents/x.md"), ghcpTargets{}},
		{"claude-code with X.agent.md agents in .github/agents",
			ghcpSnap(present, "claude-code", ".github/agents/x.agent.md"), ghcpTargets{}},
		{"empty harness id with mixed agents",
			ghcpSnap(present, "", ".github/agents/x.md", ".github/agents/y.agent.md"), ghcpTargets{}},
	})
}

// TestDetectGhcpTargets_HookEntriesAreNotASignal verifies that hook entries recorded in the
// manifest do not change the result: only the harness_id decides.
func TestDetectGhcpTargets_HookEntriesAreNotASignal(t *testing.T) {
	withHook := func(id string) manifest.Snapshot {
		s := ghcpSnap(manifest.StatePresent, id)
		s.Manifest.Entries = append(s.Manifest.Entries, domain.ManifestEntry{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "mosaic-logger"},
			TargetPath: ".github/hooks/mosaic-logger.json",
		})
		return s
	}
	runGhcpDetectCases(t, []ghcpDetectCase{
		{"ghcp-cli with a hook entry", withHook("ghcp-cli"), ghcpTargets{CLI: true}},
		{"vscode-ghcp with a hook entry", withHook("vscode-ghcp"), ghcpTargets{VSCode: true}},
		{"claude-code with a hook entry", withHook("claude-code"), ghcpTargets{}},
	})
}

// TestDetectGhcpTargets_HarnessIDMatchIsExact pins that the harness_id must match exactly.
func TestDetectGhcpTargets_HarnessIDMatchIsExact(t *testing.T) {
	present := manifest.StatePresent
	runGhcpDetectCases(t, []ghcpDetectCase{
		{"upper-case ghcp-cli", ghcpSnap(present, "GHCP-CLI", ".github/agents/x.md"), ghcpTargets{}},
		{"upper-case vscode-ghcp", ghcpSnap(present, "VSCODE-GHCP", ".github/agents/x.md"), ghcpTargets{}},
	})
}

// TestDetectGhcpTargets_NoUsableManifestHoldsNoTarget covers manifests that cannot be trusted.
func TestDetectGhcpTargets_NoUsableManifestHoldsNoTarget(t *testing.T) {
	agents := []string{".github/agents/x.md", ".github/agents/y.agent.md"}
	runGhcpDetectCases(t, []ghcpDetectCase{
		{"absent manifest", ghcpSnap(manifest.StateAbsent, ""), ghcpTargets{}},
		{"absent manifest naming ghcp-cli", ghcpSnap(manifest.StateAbsent, "ghcp-cli", agents...), ghcpTargets{}},
		{"empty manifest naming ghcp-cli", ghcpSnap(manifest.StateEmpty, "ghcp-cli", agents...), ghcpTargets{}},
		{"corrupt manifest naming ghcp-cli", ghcpSnap(manifest.StateCorrupt, "ghcp-cli", agents...), ghcpTargets{}},
		{"future-schema manifest naming vscode-ghcp", ghcpSnap(manifest.StateFuture, "vscode-ghcp", agents...), ghcpTargets{}},
	})
}
