package app_test

// ghcp_agent_migration_helpers_test.go holds the builders and assertions shared by the GHCP
// agent naming and legacy migration tests: a legacy workspace (a real deploy whose agent files
// and manifest entries are renamed back to X.md), the four deploy flows driven headlessly over
// the tool-local parity catalog, and readers for the manifest and the backup folder.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// gamAgentsDir is the project agents folder of both GHCP targets.
const gamAgentsDir = ".github/agents"

// gamTargets are the two GHCP targets.
var gamTargets = []string{hCLI, hVSCode}

// gamOpts adjusts one flow run.
type gamOpts struct {
	dry      bool
	conflict domain.ConflictDecision
	deps     []func(*app.Deps)  // adjustments to the service dependencies
	out      *domain.RunSummary // receives the run summary when set
}

// gamFlow is one of the four agent-writing deploy flows with the agents it deploys on a
// workspace holding all four parity sample agents.
type gamFlow struct {
	name     string
	subject  string   // an agent this flow always deploys
	migrates []string // agents the flow deploys, so migrates
	keeps    []string // agents the flow does not deploy, so leaves alone
	run      func(t *testing.T, ws, harness string, o gamOpts) error
}

func gamModels() map[string]string {
	models := map[string]string{}
	for _, a := range parityAgents {
		models[a.key] = parityModel
	}
	return models
}

// gamKeep hands the run summary to the caller that asked for it.
func gamKeep(o gamOpts, sum domain.RunSummary) {
	if o.out != nil {
		*o.out = sum
	}
}

func gamConflict(o gamOpts) domain.ConflictDecision {
	if o.conflict != "" {
		return o.conflict
	}
	return domain.DecisionSkip
}

// gamFlows returns the four flows. The conflict decision defaults to skip, so a replacement
// that happens anyway is not a conflict resolution.
func gamFlows() []gamFlow {
	ctx := context.Background()
	return []gamFlow{
		{
			name: "DeployNew", subject: "parity-runner",
			migrates: []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"},
			run: func(t *testing.T, ws, h string, o gamOpts) error {
				sum, err := parityServiceWith(t, o.deps...).DeployNew(ctx, app.DeployRequest{
					HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
					WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
					AgentModels: gamModels(), AutoConfirmPlan: true, DryRun: o.dry,
					ConflictDefault: gamConflict(o)})
				gamKeep(o, sum)
				return err
			},
		},
		{
			name: "DeployAgents", subject: "parity-runner",
			migrates: []string{"parity-runner"},
			keeps:    []string{"parity-helper", "orchestrator", "orchestrator-script"},
			run: func(t *testing.T, ws, h string, o gamOpts) error {
				sum, err := parityServiceWith(t, o.deps...).DeployAgents(ctx, app.DeployAgentsRequest{
					HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
					SubagentIDs: []string{"parity-runner"}, UtilityAgentIDs: []string{},
					InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
					AgentModels: gamModels(), AutoConfirmPlan: true, DryRun: o.dry,
					ConflictDefault: gamConflict(o)})
				gamKeep(o, sum)
				return err
			},
		},
		{
			name: "Update", subject: "parity-runner",
			migrates: []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"},
			run: func(t *testing.T, ws, h string, o gamOpts) error {
				sum, err := parityServiceWith(t, o.deps...).Update(ctx, app.UpdateRequest{
					HarnessID: h, WorkspacePath: ws, AgentModels: gamModels(),
					AutoConfirmPlan: true, DryRun: o.dry, ConflictDefault: gamConflict(o)})
				gamKeep(o, sum)
				return err
			},
		},
		{
			name: "UpdateWorkflows", subject: "orchestrator",
			migrates: []string{"orchestrator", "orchestrator-script"},
			keeps:    []string{"parity-runner", "parity-helper"},
			run: func(t *testing.T, ws, h string, o gamOpts) error {
				sum, err := parityServiceWith(t, o.deps...).UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
					HarnessID: h, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
					AutoConfirmPlan: true, DryRun: o.dry, ConflictDefault: gamConflict(o)})
				gamKeep(o, sum)
				return err
			},
		},
	}
}

func gamAbs(ws, rel string) string { return filepath.Join(ws, filepath.FromSlash(rel)) }

func gamRel(key, ext string) string { return gamAgentsDir + "/" + key + ext }

func gamExists(ws, rel string) bool {
	_, err := os.Stat(gamAbs(ws, rel))
	return err == nil
}

func gamRead(t *testing.T, ws, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(gamAbs(ws, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

func gamWrite(t *testing.T, ws, rel, content string) {
	t.Helper()
	p := gamAbs(ws, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func gamManifest(t *testing.T, ws string) domain.Manifest {
	t.Helper()
	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return snap.Manifest
}

func gamSaveManifest(t *testing.T, ws string, m domain.Manifest) {
	t.Helper()
	sort.SliceStable(m.Entries, func(i, j int) bool { return m.Entries[i].TargetPath < m.Entries[j].TargetPath })
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// gamEntryPaths returns the recorded target paths of an agent key.
func gamEntryPaths(m domain.Manifest, key string) []string {
	var out []string
	for _, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
			out = append(out, e.TargetPath)
		}
	}
	return out
}

// gamBackups returns the content of every backup of a workspace-relative file.
func gamBackups(t *testing.T, ws, rel string) [][]byte {
	t.Helper()
	dir := filepath.Join(ws, manifest.Dir, manifest.BackupDir, filepath.Dir(filepath.FromSlash(rel)))
	matches, err := filepath.Glob(filepath.Join(dir, filepath.Base(rel)+".*.bak"))
	if err != nil {
		t.Fatalf("glob backups of %s: %v", rel, err)
	}
	var out [][]byte
	for _, m := range matches {
		b, rerr := os.ReadFile(m)
		if rerr != nil {
			t.Fatalf("read backup %s: %v", m, rerr)
		}
		out = append(out, b)
	}
	return out
}

func gamHasBackup(t *testing.T, ws, rel string, want []byte) bool {
	t.Helper()
	for _, b := range gamBackups(t, ws, rel) {
		if string(b) == string(want) {
			return true
		}
	}
	return false
}

// gamLegacyWorkspace deploys the parity sample to a fresh workspace and puts every agent back
// into the form an earlier deploy recorded: file X.md, manifest entry X.md. It returns the
// workspace and the legacy bytes of each agent by key.
func gamLegacyWorkspace(t *testing.T, harness string) (string, map[string][]byte) {
	t.Helper()
	ws := parityDeploy(t, harness)
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind != domain.ArtifactAgent || !strings.HasSuffix(e.TargetPath, ".agent.md") {
			continue
		}
		legacy := strings.TrimSuffix(e.TargetPath, ".agent.md") + ".md"
		if err := os.Rename(gamAbs(ws, e.TargetPath), gamAbs(ws, legacy)); err != nil {
			t.Fatalf("rename %s to legacy name: %v", e.TargetPath, err)
		}
		m.Entries[i].TargetPath = legacy
	}
	gamSaveManifest(t, ws, m)
	bytesByKey := map[string][]byte{}
	for _, a := range parityAgents {
		bytesByKey[a.key] = gamRead(t, ws, gamRel(a.key, ".md"))
	}
	return ws, bytesByKey
}

// gamRecordAgentAt adds a manifest entry for key at rel whose hash matches content.
func gamRecordAgentAt(t *testing.T, ws, key, rel, content string) {
	t.Helper()
	m := gamManifest(t, ws)
	m.Entries = append(m.Entries, domain.ManifestEntry{
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
		TargetPath:  rel,
		ContentHash: manifest.Hash([]byte(content)),
	})
	gamSaveManifest(t, ws, m)
}

// gamDropEntry removes the manifest entries of an agent key at rel.
func gamDropEntry(t *testing.T, ws, key, rel string) {
	t.Helper()
	m := gamManifest(t, ws)
	kept := m.Entries[:0]
	for _, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key && e.TargetPath == rel {
			continue
		}
		kept = append(kept, e)
	}
	m.Entries = kept
	gamSaveManifest(t, ws, m)
}

// gamAssertMigrated checks one agent ended as a single recorded X.agent.md with the legacy
// file recoverable from backup.
func gamAssertMigrated(t *testing.T, ws, key string, legacy []byte) {
	t.Helper()
	newRel, oldRel := gamRel(key, ".agent.md"), gamRel(key, ".md")
	if !gamExists(ws, newRel) {
		t.Errorf("%s: %s was not written", key, newRel)
	}
	if gamExists(ws, oldRel) {
		t.Errorf("%s: legacy %s still exists", key, oldRel)
	}
	paths := gamEntryPaths(gamManifest(t, ws), key)
	if len(paths) != 1 || paths[0] != newRel {
		t.Errorf("%s: manifest entries = %v, want exactly [%s]", key, paths, newRel)
	}
	if !gamHasBackup(t, ws, oldRel, legacy) {
		t.Errorf("%s: no backup of %s holds the legacy bytes", key, oldRel)
	}
}

// gamAssertKept checks one agent still is its legacy X.md, recorded and byte-identical,
// with no X.agent.md beside it and no backup made.
func gamAssertKept(t *testing.T, ws, key string, legacy []byte) {
	t.Helper()
	newRel, oldRel := gamRel(key, ".agent.md"), gamRel(key, ".md")
	if !gamExists(ws, oldRel) || string(gamRead(t, ws, oldRel)) != string(legacy) {
		t.Errorf("%s: %s was removed or changed", key, oldRel)
	}
	if gamExists(ws, newRel) {
		t.Errorf("%s: %s was written although the run does not deploy the agent", key, newRel)
	}
	paths := gamEntryPaths(gamManifest(t, ws), key)
	if len(paths) != 1 || paths[0] != oldRel {
		t.Errorf("%s: manifest entries = %v, want exactly [%s]", key, paths, oldRel)
	}
	if n := len(gamBackups(t, ws, oldRel)); n != 0 {
		t.Errorf("%s: %d backups of %s made for an agent the run does not deploy", key, n, oldRel)
	}
}

// gamAssertNoAgentInBothForms checks no agent key has both an X.md and an X.agent.md file.
func gamAssertNoAgentInBothForms(t *testing.T, ws string) {
	t.Helper()
	for _, a := range parityAgents {
		if gamExists(ws, gamRel(a.key, ".md")) && gamExists(ws, gamRel(a.key, ".agent.md")) {
			t.Errorf("%s exists as both X.md and X.agent.md", a.key)
		}
	}
}

// gamAssertOnlyAgentExt checks every agent file in the agents folder is X.agent.md and every
// recorded agent entry names an existing X.agent.md.
func gamAssertOnlyAgentExt(t *testing.T, ws string) {
	t.Helper()
	files, err := os.ReadDir(gamAbs(ws, gamAgentsDir))
	if err != nil {
		t.Fatalf("read agents folder: %v", err)
	}
	for _, f := range files {
		if !f.IsDir() && !strings.HasSuffix(f.Name(), ".agent.md") {
			t.Errorf("agent file %s is not named X.agent.md", f.Name())
		}
	}
	for _, e := range gamManifest(t, ws).Entries {
		if e.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		if !strings.HasSuffix(e.TargetPath, ".agent.md") || !gamExists(ws, e.TargetPath) {
			t.Errorf("agent %s recorded as %s: not an existing X.agent.md", e.Ref.Key, e.TargetPath)
		}
	}
}

// gamAssertAgentsRecorded checks each key has a file and one entry named X.agent.md.
func gamAssertAgentsRecorded(t *testing.T, ws string, keys []string) {
	t.Helper()
	m := gamManifest(t, ws)
	for _, key := range keys {
		want := gamRel(key, ".agent.md")
		if paths := gamEntryPaths(m, key); len(paths) != 1 || paths[0] != want {
			t.Errorf("%s: manifest entries = %v, want exactly [%s]", key, paths, want)
		}
		if !gamExists(ws, want) {
			t.Errorf("%s: %s was not written", key, want)
		}
	}
}
