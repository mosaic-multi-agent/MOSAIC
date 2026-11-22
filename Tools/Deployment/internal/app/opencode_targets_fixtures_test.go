package app_test

// opencode_targets_fixtures_test.go provides the workspace and registry builders for the
// OpenCode one-target tests. The fixture type and the flow drivers are those of the GHCP target
// tests (a fully wired service, a recording executor, the five deploy-family flows); only the
// workspace content and the registry differ. Nothing here is a test itself.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	ocV1Name = "OpenCode CLI"
	ocV2Name = "OpenCode V2 CLI"

	// Both OpenCode targets write identical paths, so the fixtures use the same ones for each.
	ocAgentRel  = ".opencode/agents/x.md"
	ocSkillRel  = ".opencode/skills/anchor/SKILL.md"
	ocPluginRel = ".opencode/plugins/mosaic-logger.ts"
	ocUserAgent = ".opencode/agents/mine.md"
)

// ocWorkspaceKind names the OpenCode state a fixture workspace is in. ocBothHeld cannot be
// produced from one harness's entries: its service is built with the test-only constructor.
type ocWorkspaceKind string

const (
	ocNone     ocWorkspaceKind = "none"
	ocOnlyV1   ocWorkspaceKind = "only opencode"
	ocOnlyV2   ocWorkspaceKind = "only opencode-v2"
	ocBothHeld ocWorkspaceKind = "both held"
)

// ocEntries returns manifest entries of one harness: an agent, a skill file and a hook file, all
// at the paths both OpenCode targets share.
func ocEntries(harnessID string) []domain.ManifestEntry {
	return []domain.ManifestEntry{
		ghcpAgentEntry(harnessID, "x", ocAgentRel),
		{Harness: harnessID, Ref: domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "anchor"}, TargetPath: ocSkillRel},
		{Harness: harnessID, Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "mosaic-logger"}, TargetPath: ocPluginRel},
	}
}

// ocManifest builds a manifest holding the entries of each named harness.
func ocManifest(harnessIDs ...string) domain.Manifest {
	var m domain.Manifest
	for _, id := range harnessIDs {
		m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: id})
		m.Entries = append(m.Entries, ocEntries(id)...)
	}
	return m
}

// newOCFixture builds the workspace for kind.
func newOCFixture(t *testing.T, kind ocWorkspaceKind) *ghcpFixture {
	t.Helper()
	switch kind {
	case ocOnlyV1:
		return newOCFixtureWith(t, manifest.StatePresent, ocManifest(hOpen), nil)
	case ocOnlyV2:
		return newOCFixtureWith(t, manifest.StatePresent, ocManifest(hOpenV2), nil)
	case ocBothHeld:
		// Entries of an unrelated harness only: the refusal can come from the forced held set alone.
		return newOCFixtureWith(t, manifest.StatePresent, ocManifest(nonGhcpHarness), &[2]bool{true, true})
	}
	return newOCFixtureWith(t, manifest.StatePresent, ocManifest(nonGhcpHarness), nil)
}

// newOCFixtureWith writes a file for every recorded entry, a hook file and an unrecorded user
// agent, wires a registry holding both OpenCode harnesses and one other harness, and serves the
// manifest in the given state. A non-nil held forces the detected OpenCode targets {opencode,
// opencode-v2} through the test-only constructor.
func newOCFixtureWith(t *testing.T, state manifest.State, m domain.Manifest, held *[2]bool) *ghcpFixture {
	t.Helper()
	stub := interactiontest.NewBuilder().Build()
	deps, ws := newBaseDeps(t, stub)
	for _, e := range m.Entries {
		writeGhcpFile(t, ws, e.TargetPath)
	}
	// An unrecorded user file must never count as a deployed target.
	writeGhcpFile(t, ws, ocUserAgent)

	v1 := ghcpModule(hOpen, ocV1Name)
	v2 := ghcpModule(hOpenV2, ocV2Name)
	other := newMinimalModule()
	deps.Registry = &stubRegistry{
		list: []domain.HarnessRef{v1.ref, v2.ref, other.ref},
		modules: map[string]domain.HarnessModule{
			hOpen: v1, hOpenV2: v2, nonGhcpHarness: other},
	}
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: state, Manifest: m}}
	exec := &ghcpRecordingExecutor{ws: ws}
	deps.Executor = exec
	svc := app.New(deps)
	if held != nil {
		svc = newHeldOpenCodeService(t, deps, held[0], held[1])
	}
	return &ghcpFixture{svc: svc, ws: ws, stub: stub, executor: exec}
}

// newHeldOpenCodeService builds a service through the test-only held-targets constructor. A
// missing production seam makes only the calling test fail instead of aborting the package.
func newHeldOpenCodeService(t *testing.T, deps app.Deps, v1, v2 bool) (svc app.Service) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("held-targets service unavailable: %v", r)
		}
	}()
	return app.NewWithOpenCodeTargetsHeld(deps, v1, v2)
}

// ocTransformCommand is the exact command the refusal message must name.
func ocTransformCommand(presentID, requestedID, ws string) string {
	return strings.Join([]string{"mosaic-deploy transform --harness", presentID,
		"--target-harness", requestedID, "--workspace", ws}, " ")
}
