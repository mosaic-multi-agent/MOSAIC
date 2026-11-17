package opencodev2_test

// Tests that the OpenCode V2 module injects no content for any region and any role, while
// both injection versions are still stamped from the injection files, and that the frozen
// copy of the two injection files carries the required rationale and watch-list item.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/injectionfile"
)

func TestInjection_NoRegionHasContentForAnyRole(t *testing.T) {
	mod := newModule(t)
	regions := []string{
		docformat.HarnessRegionName, docformat.OrchestratorHarnessRegionName,
		"HarnessConstraints", "OrchestratorHarnessConstraints",
		"IdentityExtension", "ProtocolExtension", "CodebaseContext", "LanguagePatterns",
		"OutputArtifactTemplate", "CustomConstraints", "ErrorHandlingExtension", "ContextLimits",
	}
	roles := []domain.AgentRole{"", domain.RoleSubagent, domain.RoleOrchestrator, domain.RoleUtility, domain.RoleStandalone}

	for _, region := range regions {
		for _, role := range roles {
			content, _ := mod.Injection(domain.InjectionRequest{Name: region, AgentKey: "agent", Role: role})
			if content != "" {
				t.Errorf("Injection(%q, role %q) = %q, want no content", region, role, content)
			}
		}
	}
}

func TestInjection_VersionsAreStampedFromTheInjectionFilesEvenWithoutContent(t *testing.T) {
	d := newModule(t).Descriptor()

	if d.InjectionsVersion == "" {
		t.Error("InjectionsVersion is empty, want the HarnessInjections.md frontmatter version")
	}
	if d.OrchestratorInjectionsVersion == "" {
		t.Error("OrchestratorInjectionsVersion is empty, want the HarnessInjectionsOrchestrator.md frontmatter version")
	}
}

func frozenInjectionFile(t *testing.T, name string) string {
	t.Helper()
	dir := catalogpaths.HarnessInjectionsDir(filepath.Join(deploymentTestdata(t, "frozen-catalog"), "Catalog"),
		catalogpaths.HarnessDirNameOpenCodeV2)
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read frozen injection file: %v", err)
	}
	return string(b)
}

func TestFrozenInjectionFiles_DeclareTheV2HarnessAndTheStandardSections(t *testing.T) {
	for _, name := range []string{injectionfile.SharedFileName, injectionfile.OrchestratorFileName} {
		t.Run(name, func(t *testing.T) {
			src := frozenInjectionFile(t, name)

			for _, want := range []string{"harness: opencode-v2", "## Design Rationale", "## Changelog"} {
				if !strings.Contains(src, want) {
					t.Errorf("%s does not contain %q", name, want)
				}
			}
		})
	}
}

func TestFrozenInjectionFiles_InjectNoContent(t *testing.T) {
	dir := catalogpaths.HarnessInjectionsDir(filepath.Join(deploymentTestdata(t, "frozen-catalog"), "Catalog"),
		catalogpaths.HarnessDirNameOpenCodeV2)

	content, err := injectionfile.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir(%s): %v", dir, err)
	}

	if len(content.Shared) != 0 {
		t.Errorf("HarnessInjections.md carries regions %v, want none", content.Shared)
	}
	if len(content.Orchestrator) != 0 {
		t.Errorf("HarnessInjectionsOrchestrator.md carries regions %v, want none", content.Orchestrator)
	}
}

func TestFrozenInjectionFiles_RationaleSaysV2ConstraintsAreAddedOnlyOnceObserved(t *testing.T) {
	for _, name := range []string{injectionfile.SharedFileName, injectionfile.OrchestratorFileName} {
		t.Run(name, func(t *testing.T) {
			src := strings.ToLower(frozenInjectionFile(t, name))

			if !strings.Contains(src, "observed") {
				t.Errorf("%s rationale does not say that V2-specific constraints are added once observed in use", name)
			}
		})
	}
}

func TestFrozenOrchestratorInjectionFile_ListsTheRunningChildSessionWatchItem(t *testing.T) {
	src := frozenInjectionFile(t, injectionfile.OrchestratorFileName)

	for _, want := range []string{"sessionID", "subagent"} {
		if !strings.Contains(src, want) {
			t.Errorf("orchestrator injection file does not mention %q; the watch list names the orchestrator "+
				"re-calling subagent with a running child's sessionID", want)
		}
	}
}
