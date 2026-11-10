package registry_test

// descriptoronly_orchestrator_region_test.go specifies per-region injection lookup of the
// descriptor-only runtime module: each region name returns one source file only.

import (
	"fmt"
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestDescriptorOnly_UnmigratedOrchestratorRegionName_ServedOnlyUnderOrchestratorRegion
// verifies that an orchestrator injection file whose region still uses the HarnessConstraints
// name has its content served under OrchestratorHarnessConstraints only: the shared region
// name returns no orchestrator text, so nothing is lost or duplicated.
func TestDescriptorOnly_UnmigratedOrchestratorRegionName_ServedOnlyUnderOrchestratorRegion(t *testing.T) {
	id := "descriptor-unmigrated-orch-test"
	yamlContent := fmt.Sprintf("schema_version: \"1\"\nid: %q\ndisplay_name: \"Unmigrated Orch Test\"\n", id)
	injectionsMd := "---\nversion: \"1.0.0\"\n---\n<HarnessConstraints type=\"managed\">\nSHARED_LINE\n</HarnessConstraints>\n"
	orchMd := "---\nversion: \"2.0.0\"\n---\n<HarnessConstraints type=\"managed\">\nLEGACY_ORCH_LINE\n</HarnessConstraints>\n"
	root := makeDescriptorRootWithOrchestratorContent(t, id, yamlContent, injectionsMd, orchMd)
	m := resolveDescriptorOnly(t, root, id)

	shared, sharedOK := m.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	orch, orchOK := m.Injection(domain.InjectionRequest{Name: "OrchestratorHarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})

	if !sharedOK || shared != "SHARED_LINE" {
		t.Errorf("HarnessConstraints for the orchestrator = %q (ok=%v), want exactly %q", shared, sharedOK, "SHARED_LINE")
	}
	if !orchOK || orch != "LEGACY_ORCH_LINE" {
		t.Errorf("OrchestratorHarnessConstraints for the orchestrator = %q (ok=%v), want %q", orch, orchOK, "LEGACY_ORCH_LINE")
	}
}

// TestDescriptorOnly_OrchestratorRegion_EachNameServesOneFile verifies that when both files
// declare content, each region name returns only its own file's text for the orchestrator
// and the orchestrator region name returns nothing for other roles.
func TestDescriptorOnly_OrchestratorRegion_EachNameServesOneFile(t *testing.T) {
	id := "descriptor-eachname-test"
	yamlContent := fmt.Sprintf("schema_version: \"1\"\nid: %q\ndisplay_name: \"Each Name Test\"\n", id)
	injectionsMd := "---\nversion: \"1.0.0\"\n---\n<HarnessConstraints type=\"managed\">\nSHARED_LINE\n</HarnessConstraints>\n"
	orchMd := "---\nversion: \"2.0.0\"\n---\n<OrchestratorHarnessConstraints type=\"managed\">\nORCH_LINE\n</OrchestratorHarnessConstraints>\n"
	root := makeDescriptorRootWithOrchestratorContent(t, id, yamlContent, injectionsMd, orchMd)
	m := resolveDescriptorOnly(t, root, id)

	shared, _ := m.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	subShared, _ := m.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "worker", Role: domain.RoleSubagent})
	_, subOrchOK := m.Injection(domain.InjectionRequest{Name: "OrchestratorHarnessConstraints", AgentKey: "worker", Role: domain.RoleSubagent})

	if shared != "SHARED_LINE" || subShared != "SHARED_LINE" {
		t.Errorf("HarnessConstraints: orchestrator %q, subagent %q; both want exactly %q", shared, subShared, "SHARED_LINE")
	}
	if subOrchOK {
		t.Error("OrchestratorHarnessConstraints served to a subagent, want ok=false")
	}
}
