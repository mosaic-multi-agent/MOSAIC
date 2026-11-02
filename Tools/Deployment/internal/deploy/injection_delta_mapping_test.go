package deploy_test

// injection_delta_mapping_test.go specifies that on an update item the executor maps each
// injection delta to its own manifest field only: the shared-file delta never reaches the
// orchestrator field and the orchestrator-file delta never reaches the injections field.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func executeUpdateWithDeltas(t *testing.T, key string, stamp domain.VersionStamp, deltas ...domain.VersionDelta) domain.ManifestEntry {
	t.Helper()
	target := "agents/" + key + ".md"
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
		SourcePath: "/fake/" + target,
		TargetPath: target,
		Action:     domain.ActionUpdate,
		Stale:      deltas,
	}
	req := deploy.ExecRequest{
		Plan:          newPlan(t.TempDir(), item),
		MosaicRoot:    t.TempDir(),
		Content:       fixedContent([]byte("# updated")),
		VersionStamps: map[string]domain.VersionStamp{target: stamp},
	}
	result, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	entry, found := result.Manifest.Lookup(item.Ref)
	if !found {
		t.Fatalf("manifest has no entry for %s", key)
	}
	return entry
}

func TestExecutor_UpdateItem_SharedDeltaOnly_LeavesOrchestratorFieldAtItsOwnStamp(t *testing.T) {
	entry := executeUpdateWithDeltas(t, "orchestrator",
		domain.VersionStamp{InjectionsVersion: "1.4.0", OrchestratorInjectionsVersion: "2.3.0"},
		domain.VersionDelta{Field: "injections_version", Deployed: "1.2.1", Source: "1.4.0"},
	)

	if entry.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want the delta source 1.4.0", entry.InjectionsVersion)
	}
	if entry.OrchestratorInjectionsVersion != "2.3.0" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want its own stamp 2.3.0", entry.OrchestratorInjectionsVersion)
	}
}

func TestExecutor_UpdateItem_OrchestratorDeltaOnly_LeavesInjectionsFieldAtItsOwnStamp(t *testing.T) {
	entry := executeUpdateWithDeltas(t, "orchestrator",
		domain.VersionStamp{InjectionsVersion: "1.4.0", OrchestratorInjectionsVersion: "2.3.0"},
		domain.VersionDelta{Field: "orchestrator_injections_version", Deployed: "2.0.0", Source: "2.3.0"},
	)

	if entry.OrchestratorInjectionsVersion != "2.3.0" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want the delta source 2.3.0", entry.OrchestratorInjectionsVersion)
	}
	if entry.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want its own stamp 1.4.0", entry.InjectionsVersion)
	}
}

func TestExecutor_UpdateItem_SubagentSharedDelta_StampsSharedVersionAndNoOrchestratorVersion(t *testing.T) {
	entry := executeUpdateWithDeltas(t, "test-writer",
		domain.VersionStamp{InjectionsVersion: "1.4.0"},
		domain.VersionDelta{Field: "injections_version", Deployed: "1.2.1", Source: "1.4.0"},
	)

	if entry.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want 1.4.0", entry.InjectionsVersion)
	}
	if entry.OrchestratorInjectionsVersion != "" {
		t.Errorf("OrchestratorInjectionsVersion = %q for a subagent, want empty", entry.OrchestratorInjectionsVersion)
	}
}
