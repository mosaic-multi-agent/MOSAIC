package builtin_test

// injection_regions_test.go specifies the per-region injection lookup every built-in harness
// module serves: the HarnessConstraints region returns only HarnessInjections.md content and
// the OrchestratorHarnessConstraints region returns only HarnessInjectionsOrchestrator.md
// content, for the orchestrator role only. No returned text mixes the two files.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/injectionfile"
)

const (
	sharedRegion       = "HarnessConstraints"
	orchestratorRegion = "OrchestratorHarnessConstraints"
)

func lookup(m domain.HarnessModule, name, key string, role domain.AgentRole) (string, bool) {
	return m.Injection(domain.InjectionRequest{Name: name, AgentKey: key, Role: role})
}

func orchestratorFile(version, regionName, content string) string {
	return "---\nversion: \"" + version + "\"\n---\n<" + regionName + " type=\"managed\">\n" + content + "\n</" + regionName + ">\n"
}

func TestBuiltinInjection_OrchestratorRole_EachRegionNameServesOneFile(t *testing.T) {
	for _, tc := range contentDirCases() {
		for _, key := range []string{"orchestrator", "orchestrator-script"} {
			t.Run(tc.name+"/"+key, func(t *testing.T) {
				dir := t.TempDir()
				writeInjectionContent(t, dir, "A", "4.0.0")
				mod, err := tc.build(fixedContentDir(dir))
				if err != nil {
					t.Fatalf("construct: %v", err)
				}

				shared, sharedOK := lookup(mod, sharedRegion, key, domain.RoleOrchestrator)
				orch, orchOK := lookup(mod, orchestratorRegion, key, domain.RoleOrchestrator)

				if !sharedOK || shared != "SHARED_A" {
					t.Errorf("HarnessConstraints for the orchestrator = %q (ok=%v), want exactly %q", shared, sharedOK, "SHARED_A")
				}
				if !orchOK || orch != "ORCH_A" {
					t.Errorf("OrchestratorHarnessConstraints for the orchestrator = %q (ok=%v), want exactly %q", orch, orchOK, "ORCH_A")
				}
			})
		}
	}
}

func TestBuiltinInjection_NonOrchestratorRoles_SharedOnlyAndNoOrchestratorRegion(t *testing.T) {
	roles := map[string]domain.AgentRole{"subagent": domain.RoleSubagent, "zero": domain.AgentRole("")}
	for _, tc := range contentDirCases() {
		for roleName, role := range roles {
			t.Run(tc.name+"/"+roleName, func(t *testing.T) {
				dir := t.TempDir()
				writeInjectionContent(t, dir, "A", "4.0.0")
				mod, err := tc.build(fixedContentDir(dir))
				if err != nil {
					t.Fatalf("construct: %v", err)
				}

				shared, sharedOK := lookup(mod, sharedRegion, "orchestrator", role)
				orch, orchOK := lookup(mod, orchestratorRegion, "orchestrator", role)

				if !sharedOK || shared != "SHARED_A" {
					t.Errorf("HarnessConstraints = %q (ok=%v), want exactly %q", shared, sharedOK, "SHARED_A")
				}
				if orchOK || orch != "" {
					t.Errorf("OrchestratorHarnessConstraints = %q (ok=%v), want no content for a non-orchestrator role", orch, orchOK)
				}
			})
		}
	}
}

func TestBuiltinInjection_UnmigratedOrchestratorFile_ContentNeitherLostNorDuplicated(t *testing.T) {
	// An orchestrator injection file whose region still carries the HarnessConstraints name.
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInjectionContent(t, dir, "A", "4.0.0")
			writeContentFile(t, dir, injectionfile.OrchestratorFileName, orchestratorFile("4.0.0", sharedRegion, "LEGACY_ORCH_A"))
			mod, err := tc.build(fixedContentDir(dir))
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			shared, _ := lookup(mod, sharedRegion, "orchestrator", domain.RoleOrchestrator)
			orch, orchOK := lookup(mod, orchestratorRegion, "orchestrator", domain.RoleOrchestrator)

			if shared != "SHARED_A" {
				t.Errorf("HarnessConstraints for the orchestrator = %q, want exactly %q (no legacy orchestrator text)", shared, "SHARED_A")
			}
			if !orchOK || orch != "LEGACY_ORCH_A" {
				t.Errorf("OrchestratorHarnessConstraints = %q (ok=%v), want the legacy-named region content %q", orch, orchOK, "LEGACY_ORCH_A")
			}
		})
	}
}

func TestBuiltinInjection_EmptyOrchestratorRegion_DeclaredEmptyForOrchestratorOnly(t *testing.T) {
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInjectionContent(t, dir, "A", "4.0.0")
			writeContentFile(t, dir, injectionfile.OrchestratorFileName, orchestratorFile("1.0.0", orchestratorRegion, ""))
			mod, err := tc.build(fixedContentDir(dir))
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			orch, orchOK := lookup(mod, orchestratorRegion, "orchestrator", domain.RoleOrchestrator)
			_, subOK := lookup(mod, orchestratorRegion, "orchestrator", domain.RoleSubagent)

			if !orchOK || orch != "" {
				t.Errorf("empty OrchestratorHarnessConstraints for the orchestrator = %q (ok=%v), want (\"\", true)", orch, orchOK)
			}
			if subOK {
				t.Error("OrchestratorHarnessConstraints served to a subagent, want ok=false")
			}
			if got := mod.Descriptor().OrchestratorInjectionsVersion; got != "1.0.0" {
				t.Errorf("OrchestratorInjectionsVersion = %q, want %q", got, "1.0.0")
			}
		})
	}
}
