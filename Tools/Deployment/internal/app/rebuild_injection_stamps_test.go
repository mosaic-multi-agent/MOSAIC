package app_test

// rebuild_injection_stamps_test.go specifies the version stamps a rebuild result records for
// the two injection files: every agent carries HarnessInjections.md's version in the
// injections field, and only an orchestrator additionally carries
// HarnessInjectionsOrchestrator.md's version in its own field. The two are never substituted
// for one another. The target module serves explicit, distinct versions.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestRebuild_OrchestratorResult_StampsEachInjectionVersionInItsOwnField(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}

	in := rbInput(t, claudeToClaude, false, withFileName("orchestrator.md"), withoutRoleStamp, withSecondHarnessRegion)
	in.TargetModule = mod
	res, _ := rbRebuild(t, in)

	if res.Role != domain.RoleOrchestrator {
		t.Fatalf("fixture rebuilt as role %v, want orchestrator", res.Role)
	}
	if res.Stamps.InjectionsVersion != stampSharedVersion {
		t.Errorf("Stamps.InjectionsVersion = %q, want the HarnessInjections.md version %q", res.Stamps.InjectionsVersion, stampSharedVersion)
	}
	if res.Stamps.OrchestratorInjectionsVersion != stampOrchVersion {
		t.Errorf("Stamps.OrchestratorInjectionsVersion = %q, want the HarnessInjectionsOrchestrator.md version %q",
			res.Stamps.OrchestratorInjectionsVersion, stampOrchVersion)
	}
}

func TestRebuild_OrchestratorResult_StampsMatchRegionTagsOfTheOutput(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}

	in := rbInput(t, claudeToClaude, false, withFileName("orchestrator.md"), withoutRoleStamp, withSecondHarnessRegion)
	in.TargetModule = mod
	res, out := rbRebuild(t, in)

	assertRegionStamps(t, "rebuild", []byte(out))
	if res.Stamps.InjectionsVersion != stampSharedVersion || res.Stamps.OrchestratorInjectionsVersion != stampOrchVersion {
		t.Errorf("Stamps = {%q, %q}, want {%q, %q} to match the region tags",
			res.Stamps.InjectionsVersion, res.Stamps.OrchestratorInjectionsVersion, stampSharedVersion, stampOrchVersion)
	}
}

func TestRebuild_OrchestratorResult_WithoutOrchestratorFile_OrchestratorStampEmpty(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: "", orchOK: false, orchVersion: ""}

	in := rbInput(t, claudeToClaude, false, withFileName("orchestrator.md"), withoutRoleStamp, withSecondHarnessRegion)
	in.TargetModule = mod
	res, _ := rbRebuild(t, in)

	if res.Stamps.InjectionsVersion != stampSharedVersion {
		t.Errorf("Stamps.InjectionsVersion = %q, want %q even when the harness has no orchestrator file", res.Stamps.InjectionsVersion, stampSharedVersion)
	}
	if res.Stamps.OrchestratorInjectionsVersion != "" {
		t.Errorf("Stamps.OrchestratorInjectionsVersion = %q, want empty", res.Stamps.OrchestratorInjectionsVersion)
	}
}

func TestRebuild_SubagentResult_StampsSharedVersionAndNoOrchestratorVersion(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}

	in := rbInput(t, claudeToClaude, false)
	in.TargetModule = mod
	res, _ := rbRebuild(t, in)

	if res.Role == domain.RoleOrchestrator {
		t.Fatalf("fixture rebuilt as an orchestrator; want a subagent")
	}
	if res.Stamps.InjectionsVersion != stampSharedVersion {
		t.Errorf("Stamps.InjectionsVersion = %q, want the HarnessInjections.md version %q", res.Stamps.InjectionsVersion, stampSharedVersion)
	}
	if res.Stamps.OrchestratorInjectionsVersion != "" {
		t.Errorf("Stamps.OrchestratorInjectionsVersion = %q for a subagent, want empty", res.Stamps.OrchestratorInjectionsVersion)
	}
}
