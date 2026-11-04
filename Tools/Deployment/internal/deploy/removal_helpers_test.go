package deploy_test

// removal_helpers_test.go provides helpers shared by the removal, replacement and atomic
// removal tests. Nothing here is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// agentRefFor returns the agent ArtifactRef whose key is the base name of rel without its
// extension, matching what seedAgents records.
func agentRefFor(rel string) domain.ArtifactRef {
	base := filepath.Base(rel)
	return domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: strings.TrimSuffix(base, filepath.Ext(base))}
}

// seedAgents deploys one agent per entry (relative path -> content) through a plain executor
// run, so the workspace holds the files and a real manifest that records them with matching
// content hashes.
func seedAgents(t *testing.T, workspace string, files map[string]string) {
	t.Helper()
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	items := make([]domain.PlanItem, 0, len(paths))
	for _, p := range paths {
		items = append(items, newAgentItem(agentRefFor(p).Key, p, domain.ActionCreate))
	}
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, items...),
		MosaicRoot: t.TempDir(),
		Content: func(it domain.PlanItem) ([]byte, error) {
			return []byte(files[it.TargetPath]), nil
		},
	}
	ex := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector())
	if _, err := ex.Execute(context.Background(), req); err != nil {
		t.Fatalf("seedAgents: %v", err)
	}
	// The seeding run may leave a TODO file behind; tests assert on TODO files written by the
	// run under test, so start from a workspace without any.
	stale, _ := filepath.Glob(filepath.Join(workspace, "MOSAIC-DEPLOYMENT-TODO*"))
	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			t.Fatalf("seedAgents: remove seeded TODO file: %v", err)
		}
	}
}

// writeWorkspaceFile writes content at the workspace-relative path, creating directories.
func writeWorkspaceFile(t *testing.T, workspace, rel, content string) string {
	t.Helper()
	abs := filepath.Join(workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %q: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", rel, err)
	}
	return abs
}

// manifestTargetPaths returns the slash-normalised target paths of the manifest saved on disk.
func manifestTargetPaths(t *testing.T, workspace string) []string {
	t.Helper()
	snap, err := manifest.NewStore().Load(workspace)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	var out []string
	for _, e := range snap.Manifest.Entries {
		out = append(out, filepath.ToSlash(e.TargetPath))
	}
	sort.Strings(out)
	return out
}

// hasPath reports whether paths contains want.
func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// backupFilesFor lists the backups of the workspace-relative file rel under .mosaic/backups.
func backupFilesFor(t *testing.T, workspace, rel string) []string {
	t.Helper()
	dir := filepath.Join(workspace, ".mosaic", "backups", filepath.Dir(filepath.FromSlash(rel)))
	matches, err := filepath.Glob(filepath.Join(dir, filepath.Base(rel)+".*.bak"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	return matches
}

// assertBackupHolds fails unless exactly one backup of rel exists and holds want.
func assertBackupHolds(t *testing.T, workspace, rel, want string) string {
	t.Helper()
	backups := backupFilesFor(t, workspace, rel)
	if len(backups) != 1 {
		t.Fatalf("backups of %q: got %d, want 1 (%v)", rel, len(backups), backups)
	}
	if got := string(readFile(t, backups[0])); got != want {
		t.Errorf("backup of %q holds %q; want original %q", rel, got, want)
	}
	return backups[0]
}

// assertFileHolds fails unless the workspace-relative file exists and holds want.
func assertFileHolds(t *testing.T, workspace, rel, want string) {
	t.Helper()
	abs := filepath.Join(workspace, filepath.FromSlash(rel))
	if !fileExistsAt(t, abs) {
		t.Fatalf("file %q does not exist; want it to hold %q", rel, want)
	}
	if got := string(readFile(t, abs)); got != want {
		t.Errorf("file %q holds %q; want %q", rel, got, want)
	}
}

// agentRemoval builds a Removal of an agent file that also drops its manifest entry.
func agentRemoval(rel string) domain.Removal {
	return domain.Removal{Ref: agentRefFor(rel), Path: rel, DropEntry: true, Reason: "moved"}
}

// execWithPlan runs the production executor over plan with the standard test request shape.
func execWithPlan(t *testing.T, plan domain.Plan, atomic, dryRun bool) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:       plan,
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("# new")),
		Atomic:     atomic,
		DryRun:     dryRun,
	}
	ex := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector())
	res, err := ex.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

// hookPlanFor builds a supported hook plan writing one file with the given content.
func hookPlanFor(t *testing.T, dir, name, content string, steps ...domain.RegistrationStep) domain.HookPlan {
	t.Helper()
	return domain.HookPlan{
		Supported:    true,
		TargetDir:    dir,
		Files:        []domain.HookFile{{SourcePath: makeHookSourceFile(t, content), TargetName: name}},
		Registration: steps,
	}
}

// registrationStep builds a performable registration step for target with the fragment.
func registrationStep(target, fragment string) domain.RegistrationStep {
	return domain.RegistrationStep{ID: "reg", TargetPath: target, Fragment: fragment, Performable: true, Instruction: "register"}
}
