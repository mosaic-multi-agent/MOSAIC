package codex_test

// Tests that each Codex injection region is served from exactly one file: the shared
// file serves every region except the orchestrator region, which only the orchestrator
// file serves, and only to orchestrator-role requests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/docformat"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/codex"
	"mosaic-deploy/internal/harness/registry"
)

// TestInjection_OrchestratorRequest_SharedRegionGetsSharedContentOnly verifies that an
// orchestrator-role request for the shared region gets the shared file's content and
// nothing from the orchestrator file.
func TestInjection_OrchestratorRequest_SharedRegionGetsSharedContentOnly(t *testing.T) {
	mod := newModuleWithLocalFixtures(t)

	content, ok := mod.Injection(domain.InjectionRequest{
		Name:     docformat.HarnessRegionName,
		AgentKey: "orchestrator",
		Role:     domain.RoleOrchestrator,
	})

	if !ok {
		t.Fatal("shared region returned ok=false for an orchestrator request")
	}
	if content != codexLocalHarnessConstraints {
		t.Errorf("shared region for orchestrator:\n  got:  %q\n  want: %q", content, codexLocalHarnessConstraints)
	}
	if strings.Contains(content, codexLocalOrchestratorConstraints) {
		t.Errorf("shared region content contains orchestrator file text: %q", content)
	}
}

// TestInjection_OrchestratorRegion_ServedFromOrchestratorFileOnly verifies that the
// orchestrator region for an orchestrator-role request is exactly the orchestrator file's
// content, with no shared file text.
func TestInjection_OrchestratorRegion_ServedFromOrchestratorFileOnly(t *testing.T) {
	mod := newModuleWithLocalFixtures(t)

	content, ok := mod.Injection(domain.InjectionRequest{
		Name:     docformat.OrchestratorHarnessRegionName,
		AgentKey: "orchestrator",
		Role:     domain.RoleOrchestrator,
	})

	if !ok {
		t.Fatal("orchestrator region returned ok=false for an orchestrator request")
	}
	if content != codexLocalOrchestratorConstraints {
		t.Errorf("orchestrator region:\n  got:  %q\n  want: %q", content, codexLocalOrchestratorConstraints)
	}
	if strings.Contains(content, codexLocalHarnessConstraints) {
		t.Errorf("orchestrator region content contains shared file text: %q", content)
	}
}

// TestInjection_OrchestratorRegion_NotServedToOtherRoles verifies that the orchestrator
// region is not served to subagent requests or requests without a role.
func TestInjection_OrchestratorRegion_NotServedToOtherRoles(t *testing.T) {
	mod := newModuleWithLocalFixtures(t)

	for _, role := range []domain.AgentRole{domain.RoleSubagent, ""} {
		content, ok := mod.Injection(domain.InjectionRequest{
			Name:     docformat.OrchestratorHarnessRegionName,
			AgentKey: "some-agent",
			Role:     role,
		})
		if ok {
			t.Errorf("orchestrator region served to role %q: %q", role, content)
		}
	}
}

// codexLocalOrchestratorFileSharedNameText is the text of the unmigrated HarnessConstraints
// region that the module-local orchestrator fixture carries next to its
// OrchestratorHarnessConstraints region. It differs from the shared file's text so that any
// concatenation of the two files is observable.
const codexLocalOrchestratorFileSharedNameText = "Unmigrated orchestrator-file copy of the shared region; " +
	"it must never be served for the shared region name."

// TestInjection_SharedNameOnlyInOrchestratorFile_NotServedForSharedRegion verifies that
// a region name declared only in the orchestrator file is not served for the shared
// region name, while the orchestrator region request is served from the orchestrator file.
func TestInjection_SharedNameOnlyInOrchestratorFile_NotServedForSharedRegion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, catalogpaths.MosaicRelHarnessInjectionsDir, "Codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	shared := "---\nversion: \"1.0.0\"\nharness: codex\n---\n\n" +
		"<SomeOtherRegion type=\"managed\">\nother text\n</SomeOtherRegion>\n"
	orch := "---\nversion: \"1.0.0\"\nharness: codex\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\n" + codexLocalOrchestratorFileSharedNameText + "\n</HarnessConstraints>\n"
	writeFixture(t, filepath.Join(dir, "HarnessInjections.md"), shared)
	writeFixture(t, filepath.Join(dir, "HarnessInjectionsOrchestrator.md"), orch)
	mod, err := codex.New(registry.BuiltinOptions{ContentDir: contentDirsUnder(root)})
	if err != nil {
		t.Fatalf("codex.New() with temp fixtures: %v", err)
	}

	sharedContent, sharedOK := mod.Injection(domain.InjectionRequest{
		Name:     docformat.HarnessRegionName,
		AgentKey: "orchestrator",
		Role:     domain.RoleOrchestrator,
	})
	orchContent, orchOK := mod.Injection(domain.InjectionRequest{
		Name:     docformat.OrchestratorHarnessRegionName,
		AgentKey: "orchestrator",
		Role:     domain.RoleOrchestrator,
	})

	if sharedOK {
		t.Errorf("shared region served from the orchestrator file only: %q", sharedContent)
	}
	if !orchOK {
		t.Fatal("orchestrator region returned ok=false although the orchestrator file declares only the unmigrated name")
	}
	if orchContent != codexLocalOrchestratorFileSharedNameText {
		t.Errorf("orchestrator region:\n  got:  %q\n  want: %q", orchContent, codexLocalOrchestratorFileSharedNameText)
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
