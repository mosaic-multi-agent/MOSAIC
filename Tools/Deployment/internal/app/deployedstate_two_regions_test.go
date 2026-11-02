package app

// deployedstate_two_regions_test.go specifies how probeDeployedArtifact reads the two harness
// injection versions of a deployed agent: the version tag of the HarnessConstraints region
// (shared file) and the version tag of the OrchestratorHarnessConstraints region (orchestrator
// file), each with its own region-presence signal, independent of body order and of agent role.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	trFrontmatter = "---\nmosaic_version: \"2.0\"\nmosaic_harness_version: \"4.0\"\n---\n\n"
	trSharedOpen  = "<HarnessConstraints type=\"managed\" version=\"1.4.0\">\nshared text\n</HarnessConstraints>\n"
	trOrchOpen    = "<OrchestratorHarnessConstraints type=\"managed\" version=\"1.2.1\">\norchestrator text\n</OrchestratorHarnessConstraints>\n"
)

func probeTwoRegion(t *testing.T, body string) domain.DeployedArtifactState {
	t.Helper()
	return probeTempAgent(t, []byte(trFrontmatter+body))
}

func TestProbeDeployedArtifact_TwoRegionOrchestrator_ReadsBothVersions(t *testing.T) {
	state := probeTwoRegion(t, trSharedOpen+"\n"+trOrchOpen)

	if state.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want 1.4.0", state.InjectionsVersion)
	}
	if state.OrchestratorInjectionsVersion != "1.2.1" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want 1.2.1", state.OrchestratorInjectionsVersion)
	}
	if !state.HasInjectionRegion || !state.HasOrchestratorInjectionRegion {
		t.Errorf("HasInjectionRegion = %v, HasOrchestratorInjectionRegion = %v, want both true",
			state.HasInjectionRegion, state.HasOrchestratorInjectionRegion)
	}
}

func TestProbeDeployedArtifact_TwoRegionOrchestrator_BodyOrderDoesNotMatter(t *testing.T) {
	state := probeTwoRegion(t, trOrchOpen+"\n"+trSharedOpen)

	if state.InjectionsVersion != "1.4.0" || state.OrchestratorInjectionsVersion != "1.2.1" {
		t.Errorf("versions = (%q, %q), want (1.4.0, 1.2.1) with the second region first",
			state.InjectionsVersion, state.OrchestratorInjectionsVersion)
	}
	if !state.HasInjectionRegion || !state.HasOrchestratorInjectionRegion {
		t.Errorf("presence signals = (%v, %v), want both true with the second region first",
			state.HasInjectionRegion, state.HasOrchestratorInjectionRegion)
	}
}

func TestProbeDeployedArtifact_SubagentWithSharedRegionOnly_YieldsSharedVersionOnly(t *testing.T) {
	state := probeTwoRegion(t, trSharedOpen)

	if state.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want 1.4.0", state.InjectionsVersion)
	}
	if state.OrchestratorInjectionsVersion != "" || state.HasOrchestratorInjectionRegion {
		t.Errorf("orchestrator version %q / presence %v, want empty / false for a file without the second region",
			state.OrchestratorInjectionsVersion, state.HasOrchestratorInjectionRegion)
	}
	if !state.HasInjectionRegion {
		t.Error("HasInjectionRegion = false, want true")
	}
}

func TestProbeDeployedArtifact_EmptyTaggedSecondRegion_ReadsItsTag(t *testing.T) {
	state := probeTwoRegion(t, trSharedOpen+"\n<OrchestratorHarnessConstraints type=\"managed\" version=\"1.2.1\">\n</OrchestratorHarnessConstraints>\n")

	if state.OrchestratorInjectionsVersion != "1.2.1" || !state.HasOrchestratorInjectionRegion {
		t.Errorf("orchestrator version %q / presence %v, want 1.2.1 / true for an empty tagged second region",
			state.OrchestratorInjectionsVersion, state.HasOrchestratorInjectionRegion)
	}
}

func TestProbeDeployedArtifact_UntaggedSecondRegion_PresentWithEmptyVersion(t *testing.T) {
	// The frontmatter carries a legacy orchestrator injection version that must not be used.
	fm := "---\nmosaic_version: \"2.0\"\nmosaic_harness_version: \"4.0\"\nmosaic_orchestrator_injections_version: \"9.9.9\"\n---\n\n"
	state := probeTempAgent(t, []byte(fm+trSharedOpen+"\n<OrchestratorHarnessConstraints type=\"managed\">\n</OrchestratorHarnessConstraints>\n"))

	if !state.HasOrchestratorInjectionRegion {
		t.Error("HasOrchestratorInjectionRegion = false, want true for a present untagged second region")
	}
	if state.OrchestratorInjectionsVersion != "" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want empty for an untagged region (no frontmatter fallback)",
			state.OrchestratorInjectionsVersion)
	}
	if state.InjectionsVersion != "1.4.0" {
		t.Errorf("InjectionsVersion = %q, want 1.4.0; the second region's missing tag must not affect the shared value",
			state.InjectionsVersion)
	}
}

func TestProbeDeployedArtifact_SecondRegionOnly_DoesNotSetSharedPresence(t *testing.T) {
	state := probeTwoRegion(t, trOrchOpen)

	if state.HasInjectionRegion {
		t.Error("HasInjectionRegion = true with only the second region present; it reports HarnessConstraints only")
	}
	if state.InjectionsVersion != "" {
		t.Errorf("InjectionsVersion = %q, want empty; the second region's tag is not the shared version", state.InjectionsVersion)
	}
	if !state.HasOrchestratorInjectionRegion || state.OrchestratorInjectionsVersion != "1.2.1" {
		t.Errorf("second region read as (%v, %q), want (true, 1.2.1)",
			state.HasOrchestratorInjectionRegion, state.OrchestratorInjectionsVersion)
	}
}

// A file deployed with today's single merged region carries the orchestrator file's version on
// HarnessConstraints and no second region: read against the new stamps it is stale.
func TestProbeDeployedArtifact_LegacyMergedOrchestrator_ComparesStaleAgainstNewStamps(t *testing.T) {
	state := probeTwoRegion(t, "<HarnessConstraints type=\"managed\" version=\"1.2.1\">\nmerged text\n</HarnessConstraints>\n")
	agent := domain.Agent{Key: "orchestrator", Version: "2.0", Role: domain.RoleOrchestrator}
	stamps := domain.VersionStamps{
		Version: "2.0", HarnessVersion: "4.0",
		InjectionsVersion: "1.4.0", OrchestratorInjectionsVersion: "1.2.1",
	}

	deltas := plan.AgentStaleness(state, agent, stamps)

	if state.HasOrchestratorInjectionRegion {
		t.Error("HasOrchestratorInjectionRegion = true for a single merged region")
	}
	if len(deltas) != 1 || deltas[0].Field != "injections_version" ||
		deltas[0].Deployed != "1.2.1" || deltas[0].Source != "1.4.0" {
		t.Errorf("deltas = %v, want one injections_version delta 1.2.1 -> 1.4.0", deltas)
	}
}

// A two-region orchestrator compares clean against matching stamps, and a bump of only the
// orchestrator version produces only the orchestrator delta.
func TestProbeDeployedArtifact_TwoRegionOrchestrator_StalenessFollowsEachVersion(t *testing.T) {
	state := probeTwoRegion(t, trSharedOpen+"\n"+trOrchOpen)
	agent := domain.Agent{Key: "orchestrator", Version: "2.0", Role: domain.RoleOrchestrator}
	stamps := domain.VersionStamps{
		Version: "2.0", HarnessVersion: "4.0",
		InjectionsVersion: "1.4.0", OrchestratorInjectionsVersion: "1.2.1",
	}

	if deltas := plan.AgentStaleness(state, agent, stamps); len(deltas) != 0 {
		t.Errorf("matching stamps: deltas = %v, want none", deltas)
	}

	stamps.OrchestratorInjectionsVersion = "1.3.0"
	deltas := plan.AgentStaleness(state, agent, stamps)
	if len(deltas) != 1 || deltas[0].Field != "orchestrator_injections_version" ||
		deltas[0].Deployed != "1.2.1" || deltas[0].Source != "1.3.0" {
		t.Errorf("orchestrator bump: deltas = %v, want one orchestrator_injections_version delta 1.2.1 -> 1.3.0", deltas)
	}
}

// A shared region without a version tag keeps the legacy frontmatter fallback for the shared
// version while the second region, present and tagged, is read untouched.
func TestProbeDeployedArtifact_UntaggedSharedRegionWithSecondRegion_SharedFromFrontmatter(t *testing.T) {
	fm := "---\nmosaic_version: \"2.0\"\nmosaic_harness_version: \"4.0\"\nmosaic_injections_version: \"1.1.0\"\n---\n\n"
	body := "<HarnessConstraints type=\"managed\">\nshared text\n</HarnessConstraints>\n\n" + trOrchOpen

	state := probeTempAgent(t, []byte(fm+body))

	if state.InjectionsVersion != "1.1.0" || !state.HasInjectionRegion {
		t.Errorf("shared version %q / presence %v, want 1.1.0 / true from the frontmatter fallback",
			state.InjectionsVersion, state.HasInjectionRegion)
	}
	if state.OrchestratorInjectionsVersion != "1.2.1" || !state.HasOrchestratorInjectionRegion {
		t.Errorf("second region read as (%q, %v), want (1.2.1, true)",
			state.OrchestratorInjectionsVersion, state.HasOrchestratorInjectionRegion)
	}
}
