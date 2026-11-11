package plan_test

// build_reasons_legacy_test.go verifies that the legacy-rename reason of an agent migrating from
// X.md to X.agent.md is its own reason and composes with the item's other reasons.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	legacyTargetPath = "agents/corrupted-agent.md"
	legacyOldPath    = "agents/corrupted-agent.legacy.md"
)

// buildLegacyRenameItem plans the fixture agent (catalog version 2.0) whose deployed state was
// probed from its recorded legacy file, which carries deployedVersion.
func buildLegacyRenameItem(t *testing.T, deployedVersion string) domain.PlanItem {
	t.Helper()
	entry := makeManifestEntry(agentRef("corrupted-agent"), legacyOldPath, deployedVersion, "sha256:legacy")
	input := buildParseFailedInput(makeManifestWithEntry(entry), map[string]domain.DeployedArtifactState{
		legacyTargetPath: deployedState("sha256:legacy", deployedVersion, "1.0", "1.0"),
	})
	input.LegacyAgentPaths = map[string]string{legacyTargetPath: legacyOldPath}

	result, err := plan.New().Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	item, ok := findItem(result.Items, "corrupted-agent")
	if !ok {
		t.Fatal("plan item for corrupted-agent not found")
	}
	return item
}

func TestBuild_LegacyRenameWithStaleVersion_GivesRenameAndVersionAsSeparateReasons(t *testing.T) {
	// Act
	item := buildLegacyRenameItem(t, "1.0")

	// Assert
	if item.RenamedFrom != legacyOldPath {
		t.Fatalf("RenamedFrom = %q, want %q", item.RenamedFrom, legacyOldPath)
	}
	var rename, version string
	for _, r := range item.Reasons {
		switch {
		case r == "renamed from "+legacyOldPath:
			rename = r
		case r == `version changed from "1.0" to "2.0"`:
			version = r
		}
	}
	if rename == "" {
		t.Errorf("Reasons = %q, want the separate reason %q", item.Reasons, "renamed from "+legacyOldPath)
	}
	if version == "" {
		t.Errorf("Reasons = %q, want the item's own version reason kept next to the rename", item.Reasons)
	}
	for _, r := range item.Reasons {
		if strings.Contains(r, "renamed from") && strings.Contains(r, "changed from") {
			t.Errorf("reason %q joins the rename and the staleness into one string", r)
		}
		if strings.HasPrefix(r, "stale:") {
			t.Errorf("reason %q carries the old \"stale: \" prefix", r)
		}
	}
}

func TestBuild_LegacyRenameWithCurrentVersion_HasTheRenameReason(t *testing.T) {
	// Act
	item := buildLegacyRenameItem(t, "2.0")

	// Assert
	found := false
	for _, r := range item.Reasons {
		if r == "renamed from "+legacyOldPath {
			found = true
		}
	}
	if !found {
		t.Errorf("Reasons = %q, want %q", item.Reasons, "renamed from "+legacyOldPath)
	}
}
