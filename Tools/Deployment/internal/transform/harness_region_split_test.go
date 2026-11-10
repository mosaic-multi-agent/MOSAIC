package transform_test

// harness_region_split_test.go specifies per-region delivery and stamping of harness
// injection regions: a deployed orchestrator carries HarnessConstraints (shared file content,
// shared file version) and OrchestratorHarnessConstraints (orchestrator file content,
// orchestrator file version) as two separate regions; subagents keep one region.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

var orchestratorKeys = []string{"orchestrator", "orchestrator-script"}

func TestOrchestratorRegions_BothMarkers_EachRegionHoldsOwnContentAndPlainVersion(t *testing.T) {
	for _, key := range orchestratorKeys {
		t.Run(key, func(t *testing.T) {
			mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

			doc, _ := regionApply(t, mod, orchestratorSourceBothMarkers, key, domain.RoleOrchestrator, "")

			shared := mustRegion(t, doc, docformat.HarnessRegionName)
			orch := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName)
			if !bytes.Contains(shared.Content(), []byte(sharedRegionText)) || bytes.Contains(shared.Content(), []byte(orchRegionText)) {
				t.Errorf("HarnessConstraints content = %q, want only the shared text", shared.Content())
			}
			if !bytes.Contains(orch.Content(), []byte(orchRegionText)) || bytes.Contains(orch.Content(), []byte(sharedRegionText)) {
				t.Errorf("OrchestratorHarnessConstraints content = %q, want only the orchestrator text", orch.Content())
			}
			if got := shared.Version(); got != sharedRegionVersion {
				t.Errorf("HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
			}
			if got := orch.Version(); got != orchRegionVersion {
				t.Errorf("OrchestratorHarnessConstraints version = %q, want %q", got, orchRegionVersion)
			}
		})
	}
}

func TestOrchestratorRegions_EachSourceTextAppearsExactlyOnce(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	_, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, "")

	for _, text := range []string{sharedRegionText, orchRegionText} {
		if n := strings.Count(string(result.Output), text); n != 1 {
			t.Errorf("text %q appears %d times in the output, want exactly once", text, n)
		}
	}
}

func TestOrchestratorRegions_VersionTagsArePlain_NeverCompound(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	_, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, "")

	out := string(result.Output)
	if strings.Contains(out, sharedRegionVersion+"+") || strings.Contains(out, orchRegionVersion+"+") ||
		strings.Contains(out, "+"+sharedRegionVersion) || strings.Contains(out, "+"+orchRegionVersion) ||
		strings.Contains(out, sharedRegionVersion+","+orchRegionVersion) {
		t.Errorf("output carries a compound version tag:\n%s", out)
	}
}

func TestOrchestratorRegions_EmptyOrchestratorFile_SecondRegionEmptyAndTaggedWithItsVersion(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, "", true)

	doc, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, "")

	orch := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName)
	if strings.TrimSpace(string(orch.Content())) != "" {
		t.Errorf("OrchestratorHarnessConstraints content = %q, want empty", orch.Content())
	}
	if got := orch.Version(); got != orchRegionVersion {
		t.Errorf("empty OrchestratorHarnessConstraints version = %q, want %q", got, orchRegionVersion)
	}
	if got := mustRegion(t, doc, docformat.HarnessRegionName).Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
	}
	assertNoUndeliveredGap(t, result.Report.Gaps)
}

func TestOrchestratorRegions_NoOrchestratorFile_SecondRegionEmptyAndUntagged(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, "", false)
	doc, _ := regionApplyWithVersions(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, sharedRegionVersion, "")

	orch := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName)
	if strings.TrimSpace(string(orch.Content())) != "" {
		t.Errorf("OrchestratorHarnessConstraints content = %q, want empty", orch.Content())
	}
	if got := orch.Version(); got != "" {
		t.Errorf("OrchestratorHarnessConstraints version = %q, want no version attribute", got)
	}
	if got := mustRegion(t, doc, docformat.HarnessRegionName).Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q regardless of role", got, sharedRegionVersion)
	}
}

func TestSubagentRegions_SingleHarnessRegionWithSharedContentAndVersion(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	doc, result := regionApply(t, mod, sourceSingleMarker, "worker", domain.RoleSubagent, "")

	shared := mustRegion(t, doc, docformat.HarnessRegionName)
	if !bytes.Contains(shared.Content(), []byte(sharedRegionText)) {
		t.Errorf("HarnessConstraints content = %q, want the shared text", shared.Content())
	}
	if got := shared.Version(); got != sharedRegionVersion {
		t.Errorf("subagent HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
	}
	if _, ok := doc.Body().Deployed(docformat.OrchestratorHarnessRegionName); ok {
		t.Error("subagent output carries OrchestratorHarnessConstraints, want a single harness region")
	}
	if bytes.Contains(result.Output, []byte(orchRegionText)) {
		t.Error("subagent output carries orchestrator text")
	}
}

func TestSubagentRegions_ZeroRole_UsesSharedVersionOnly(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	doc, _ := regionApply(t, mod, sourceSingleMarker, "orchestrator", domain.AgentRole(""), "")

	if got := mustRegion(t, doc, docformat.HarnessRegionName).Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
	}
}

func TestOrchestratorRegions_UpdateOverLegacyMergedRegion_LeavesNoOldMergedText(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	doc, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, deployedLegacyMergedOrchestrator)

	for _, old := range []string{"OLD_SHARED_TEXT", "OLD_ORCH_TEXT"} {
		if bytes.Contains(result.Output, []byte(old)) {
			t.Errorf("output still carries old merged text %q", old)
		}
	}
	if got := mustRegion(t, doc, docformat.HarnessRegionName).Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q (not the legacy %q)", got, sharedRegionVersion, "1.2.1")
	}
	if got := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName).Version(); got != orchRegionVersion {
		t.Errorf("OrchestratorHarnessConstraints version = %q, want %q", got, orchRegionVersion)
	}
	for _, text := range []string{sharedRegionText, orchRegionText} {
		if n := strings.Count(string(result.Output), text); n != 1 {
			t.Errorf("text %q appears %d times after update, want exactly once", text, n)
		}
	}
}

func TestOrchestratorRegions_UpdateOverLegacyMergedRegion_PreservesNestedProjectRegion(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	_, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, deployedLegacyMergedOrchestrator)

	if !bytes.Contains(result.Output, []byte("NESTED_USER_NOTE kept by the user.")) {
		t.Errorf("nested user region content was lost:\n%s", result.Output)
	}
	if n := strings.Count(string(result.Output), "NESTED_USER_NOTE"); n != 1 {
		t.Errorf("nested user note appears %d times, want exactly once", n)
	}
}

func TestOrchestratorRegions_SubagentSourceWithSecondMarker_EmitsItEmptyWithoutOrchestratorText(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	doc, result := regionApply(t, mod, orchestratorSourceBothMarkers, "worker", domain.RoleSubagent, "")

	orch := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName)
	if strings.TrimSpace(string(orch.Content())) != "" {
		t.Errorf("subagent OrchestratorHarnessConstraints content = %q, want empty", orch.Content())
	}
	if got := orch.Version(); got != orchRegionVersion {
		t.Errorf("subagent OrchestratorHarnessConstraints version = %q, want %q", got, orchRegionVersion)
	}
	if bytes.Contains(result.Output, []byte(orchRegionText)) {
		t.Error("subagent output carries orchestrator text")
	}
}
