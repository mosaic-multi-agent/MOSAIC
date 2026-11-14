package app_test

// ghcp_targets_fixtures_test.go provides the shared workspace, registry and flow-driver
// helpers for the GHCP target refusal and regression tests. Nothing here is a test itself.

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	ghcpCLIID      = "ghcp-cli"
	ghcpVSCodeID   = "vscode-ghcp"
	ghcpCLIName    = "GHCP CLI / VS Code GHCP Copilot"
	ghcpVSCodeName = "VS Code GHCP Local"
	nonGhcpHarness = "stub-harness"
	cliAgentRel    = ".github/agents/x.md"
	vscodeAgentRel = ".github/agents/y.md"
	hookFileRel    = ".github/hooks/mosaic-logger.json"
	userAgentRel   = ".github/agents/mine.md"
)

// ghcpWorkspaceKind names the GHCP state a fixture workspace is in. A workspace holds the
// target its manifest harness_id names; agent file names do not matter. ghcpBoth cannot be
// produced by a manifest: its service is built with the test-only both-targets constructor.
type ghcpWorkspaceKind string

const (
	ghcpNone       ghcpWorkspaceKind = "none"
	ghcpOnlyCLI    ghcpWorkspaceKind = "only ghcp-cli"
	ghcpOnlyVSCode ghcpWorkspaceKind = "only vscode-ghcp"
	ghcpBoth       ghcpWorkspaceKind = "both"
)

// ghcpRecordingExecutor records Execute calls and returns a successful minimal result.
type ghcpRecordingExecutor struct {
	called bool
	ws     string
}

func (e *ghcpRecordingExecutor) Execute(_ context.Context, _ deploy.ExecRequest) (deploy.ExecResult, error) {
	e.called = true
	return newMinimalExecResult(e.ws), nil
}

// ghcpModule returns a stub harness module with the given id and display name.
func ghcpModule(id, name string) *stubHarnessModule {
	m := newMinimalModule()
	m.ref = domain.HarnessRef{ID: id, DisplayName: name, Tier: domain.TierBuiltin, Usable: true}
	m.descriptor.ID = id
	m.descriptor.DisplayName = name
	return m
}

// writeGhcpFile creates a workspace-relative file with fixed content.
func writeGhcpFile(t *testing.T, ws, rel string) {
	t.Helper()
	p := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte("deployed"), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func ghcpAgentEntry(harnessID, key, rel string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:    harnessID,
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
		TargetPath: rel,
	}
}

// ghcpFixture is a fully wired service over a workspace in the requested GHCP state.
type ghcpFixture struct {
	svc      app.Service
	ws       string
	stub     *interactiontest.Stub
	executor *ghcpRecordingExecutor
}

// newGhcpFixture builds the workspace for kind. A GHCP workspace is deployed for the target
// its manifest harness_id names and records one plain .md agent (as a deployed ghcp-cli
// workspace does today). ghcpBoth is a ghcp-cli workspace with no recorded agents and a
// service forced to report both targets.
func newGhcpFixture(t *testing.T, kind ghcpWorkspaceKind) *ghcpFixture {
	t.Helper()
	switch kind {
	case ghcpOnlyCLI:
		return newGhcpFixtureFor(t, ghcpCLIID, false, cliAgentRel)
	case ghcpOnlyVSCode:
		return newGhcpFixtureFor(t, ghcpVSCodeID, false, vscodeAgentRel)
	case ghcpBoth:
		// Deliberately no recorded agents: no detection rule over the manifest can classify
		// this workspace as both, so the refusal can only come from the forced held set.
		return newGhcpFixtureFor(t, ghcpCLIID, true)
	}
	return newGhcpFixtureFor(t, nonGhcpHarness, false)
}

// newGhcpFixtureFor writes the recorded agent files and a hook file, records the agents in the
// manifest snapshot under harnessID and wires a registry holding both GHCP harnesses and one
// non-GHCP harness. With heldBoth the service reports both GHCP targets for the workspace.
func newGhcpFixtureFor(t *testing.T, harnessID string, heldBoth bool, agentRels ...string) *ghcpFixture {
	t.Helper()
	if heldBoth {
		return newGhcpFixtureHeld(t, harnessID, &[2]bool{true, true}, agentRels...)
	}
	return newGhcpFixtureHeld(t, harnessID, nil, agentRels...)
}

// newGhcpFixtureHeld is newGhcpFixtureFor with an explicit held set {ghcp-cli, vscode-ghcp}.
// A nil held leaves detection to the manifest; otherwise the service is built with the
// test-only constructor, which fixes the reported targets whatever the manifest holds.
func newGhcpFixtureHeld(t *testing.T, harnessID string, held *[2]bool, agentRels ...string) *ghcpFixture {
	t.Helper()
	stub := interactiontest.NewBuilder().Build()
	deps, ws := newBaseDeps(t, stub)

	m := domain.Manifest{Harnesses: []domain.HarnessSection{{HarnessID: harnessID}}}
	for _, rel := range agentRels {
		key := strings.TrimSuffix(strings.TrimSuffix(path.Base(rel), ".agent.md"), ".md")
		m.Entries = append(m.Entries, ghcpAgentEntry(harnessID, key, rel))
		writeGhcpFile(t, ws, rel)
	}
	if len(agentRels) == 0 {
		// A workspace is held through its entries, so a fixture without agents records a skill.
		const anchorRel = ".github/skills/anchor/SKILL.md"
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Harness:    harnessID,
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "anchor"},
			TargetPath: anchorRel,
		})
		writeGhcpFile(t, ws, anchorRel)
	}
	if harnessID != nonGhcpHarness {
		writeGhcpFile(t, ws, hookFileRel)
	}
	// An unrecorded user file must never count as a deployed target.
	writeGhcpFile(t, ws, userAgentRel)

	cli := ghcpModule(ghcpCLIID, ghcpCLIName)
	vscode := ghcpModule(ghcpVSCodeID, ghcpVSCodeName)
	other := newMinimalModule()
	deps.Registry = &stubRegistry{
		list: []domain.HarnessRef{cli.ref, vscode.ref, other.ref},
		modules: map[string]domain.HarnessModule{
			ghcpCLIID: cli, ghcpVSCodeID: vscode, nonGhcpHarness: other},
	}
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: manifest.StatePresent, Manifest: m}}
	exec := &ghcpRecordingExecutor{ws: ws}
	deps.Executor = exec
	svc := app.New(deps)
	if held != nil {
		svc = newHeldGhcpService(t, deps, held[0], held[1])
	}
	return &ghcpFixture{svc: svc, ws: ws, stub: stub, executor: exec}
}

// newHeldGhcpService builds a service through the test-only both-targets constructor. A
// missing production seam makes only the calling test fail instead of aborting the package.
func newHeldGhcpService(t *testing.T, deps app.Deps, cli, vscode bool) (svc app.Service) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("held-targets service unavailable: %v", r)
		}
	}()
	return app.NewWithGhcpTargetsHeld(deps, cli, vscode)
}

// ghcpFlow runs one of the five deploy flows headlessly for the given harness.
type ghcpFlow struct {
	name string
	run  func(f *ghcpFixture, harnessID string) error
}

func ghcpFlows() []ghcpFlow {
	ctx := context.Background()
	return []ghcpFlow{
		{"DeployNew", func(f *ghcpFixture, id string) error {
			_, err := f.svc.DeployNew(ctx, app.DeployRequest{
				HarnessID: id, WorkspacePath: f.ws, Scope: domain.ScopeProject,
				WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true})
			return err
		}},
		{"DeployAgents", func(f *ghcpFixture, id string) error {
			_, err := f.svc.DeployAgents(ctx, app.DeployAgentsRequest{
				HarnessID: id, WorkspacePath: f.ws, Scope: domain.ScopeProject,
				SubagentIDs: []string{}, UtilityAgentIDs: []string{},
				InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
				AutoConfirmPlan: true})
			return err
		}},
		{"DeployHooks", func(f *ghcpFixture, id string) error {
			_, err := f.svc.DeployHooks(ctx, app.DeployHooksRequest{
				HarnessID: id, WorkspacePath: f.ws, Scope: domain.ScopeProject,
				HookIDs: []string{}, AutoConfirmPlan: true})
			return err
		}},
		{"Update", func(f *ghcpFixture, id string) error {
			_, err := f.svc.Update(ctx, app.UpdateRequest{
				HarnessID: id, WorkspacePath: f.ws, AutoConfirmPlan: true})
			return err
		}},
		{"UpdateWorkflows", func(f *ghcpFixture, id string) error {
			_, err := f.svc.UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
				HarnessID: id, WorkspacePath: f.ws, WorkflowIDs: []string{"quick-fix"},
				AutoConfirmPlan: true})
			return err
		}},
	}
}
