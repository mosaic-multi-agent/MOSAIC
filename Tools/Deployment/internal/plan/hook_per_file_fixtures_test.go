package plan_test

// hook_per_file_fixtures_test.go holds the builders for the per-file hook planning tests: one
// two-file hook bundle (a nested file and a flat file, plus one registration step) planned on a
// module whose hook plan names those files. Nothing in this file is a test itself.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	pfHookKey     = "mosaic-logger"
	pfHookVersion = "1.0"
	pfHookDir     = ".github/hooks"
	pfFlatPath    = ".github/hooks/logger.json"
	pfNestedPath  = ".github/hooks/lib/logger.sh"
	pfRegTarget   = ".github/settings.json"

	// Fixed reasons of the per-file hook rule.
	pfNewFileReason = "new hook file; no file at target path"
	pfDamagedReason = "recorded hook file is missing; the bundle is completed"
	pfLegacyReason  = "hook bundle was recorded before per-file records; the file is rewritten and recorded"
)

// pfModule returns a module whose hook plan for the bundle has the flat and the nested file and
// one performable registration step.
func pfModule() *fakeModule {
	mod := newFakeModule()
	mod.HookPlanFn = func(req domain.HookPlanRequest) (domain.HookPlan, error) {
		return domain.HookPlan{
			Supported: true,
			TargetDir: pfHookDir,
			Files: []domain.HookFile{
				{SourcePath: "/fake/hooks/" + req.Bundle.Key + "/logger.json", TargetName: "logger.json"},
				{SourcePath: "/fake/hooks/" + req.Bundle.Key + "/lib/logger.sh", TargetName: "lib/logger.sh"},
			},
			Registration: []domain.RegistrationStep{{
				ID: "register-logger", TargetPath: pfRegTarget, Fragment: "{}", Performable: true,
			}},
		}, nil
	}
	return mod
}

// pfInput returns a plan input for the bundle at catalog version bundleVersion.
func pfInput(snap manifest.Snapshot, deployed map[string]domain.DeployedArtifactState, bundleVersion string) plan.Input {
	return plan.Input{
		Catalog: &fakeCatalog{
			orchestrator: makeOrchestrator(),
			hooks:        []domain.HookBundle{makeHookBundle(pfHookKey, bundleVersion)},
		},
		Module:        pfModule(),
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      snap,
		HookIDs:       []string{pfHookKey},
		Models: map[string]domain.ModelSelection{
			"orchestrator": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
		DeployedState: deployed,
	}
}

// pfBuild plans the bundle and returns the whole plan.
func pfBuild(t *testing.T, snap manifest.Snapshot, deployed map[string]domain.DeployedArtifactState, bundleVersion string) domain.Plan {
	t.Helper()
	p, err := plan.New().Build(context.Background(), pfInput(snap, deployed, bundleVersion))
	must(t, err)
	return p
}

// pfHookItems returns the hook plan items by target path.
func pfHookItems(p domain.Plan) map[string]domain.PlanItem {
	out := map[string]domain.PlanItem{}
	for _, it := range p.Items {
		if it.Ref.Kind == domain.ArtifactHook {
			out[it.TargetPath] = it
		}
	}
	return out
}

// pfEntry returns the per-file entry of a deployed hook file at the bundle version.
func pfEntry(path string, owned *domain.OwnedContent) domain.ManifestEntry {
	e := makeManifestEntry(hookRef(pfHookKey), path, pfHookVersion, "sha256:written-"+path)
	e.Owned = owned
	return e
}

// pfBundleEntry returns the bundle-level entry recorded before per-file entries: the bundle
// path (which exists on no disk) and the empty-content hash.
func pfBundleEntry() domain.ManifestEntry {
	return makeManifestEntry(hookRef(pfHookKey), pfHookDir+"/"+pfHookKey, pfHookVersion, domain.EmptyContentHash)
}

// pfDeployed returns the deployed state of a present hook file with the given record.
func pfDeployed(owned *domain.OwnedContent) domain.DeployedArtifactState {
	return domain.DeployedArtifactState{Present: true, ContentHash: "sha256:on-disk", Owned: owned}
}

// pfSnap returns a usable manifest snapshot holding the entries.
func pfSnap(entries ...domain.ManifestEntry) manifest.Snapshot {
	return presentSnapshot(domain.Manifest{Entries: entries})
}
