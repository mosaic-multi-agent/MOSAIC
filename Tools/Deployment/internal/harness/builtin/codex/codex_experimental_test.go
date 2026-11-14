package codex_test

// Tests that the built-in Codex descriptor declares the harness experimental with a brief
// reason covering what works and what does not, and that the declaration reaches the ref the
// module reports and the ref the registry lists.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// codexReason returns the experimental reason of the Codex descriptor, failing the test when
// the descriptor does not declare one.
func codexReason(t *testing.T) string {
	t.Helper()
	reason := newModuleFromFrozen(t).Descriptor().ExperimentalReason
	if reason == "" {
		t.Fatal("Codex descriptor ExperimentalReason is empty, want the descriptor to declare Codex experimental")
	}
	return reason
}

func TestCodexDescriptor_DeclaresExperimental(t *testing.T) {
	// Arrange / Act
	reason := codexReason(t)

	// Assert
	if strings.TrimSpace(reason) != reason {
		t.Errorf("reason = %q, want no leading or trailing whitespace", reason)
	}
}

func TestCodexDescriptor_ReasonStatesWhatWorks(t *testing.T) {
	// Arrange / Act
	reason := codexReason(t)

	// Assert: the deploy target of agents is named.
	if !strings.Contains(reason, ".codex/agents") {
		t.Errorf("reason %q does not name the .codex/agents deploy target", reason)
	}
}

func TestCodexDescriptor_ReasonStatesWhatDoesNotWork(t *testing.T) {
	// Arrange / Act
	reason := codexReason(t)

	// Assert: the missing mosaic-run support (no orchestration) is named.
	if !strings.Contains(reason, "mosaic-run") {
		t.Errorf("reason %q does not name the missing mosaic-run support", reason)
	}
	if !strings.Contains(strings.ToLower(reason), "orchestrat") {
		t.Errorf("reason %q does not state that orchestration is unavailable", reason)
	}
}

func TestCodexDescriptor_ReasonPointsToNoFileOrDoc(t *testing.T) {
	// Arrange / Act
	reason := codexReason(t)

	// Assert
	if strings.Contains(reason, "FEASIBILITY") {
		t.Errorf("reason %q references the feasibility document", reason)
	}
	if md := regexp.MustCompile(`(?i)\.md\b`).FindString(reason); md != "" {
		t.Errorf("reason %q references a markdown file (%q), want no file or doc path", reason, md)
	}
}

func TestCodexModule_RefIsExperimentalWithDescriptorReason(t *testing.T) {
	// Arrange
	mod := newModuleFromFrozen(t)

	// Act
	ref := mod.Ref()

	// Assert
	if !ref.IsExperimental() {
		t.Fatal("Codex module ref IsExperimental() = false, want true")
	}
	if ref.ExperimentalReason != mod.Descriptor().ExperimentalReason {
		t.Errorf("ref reason = %q, want the descriptor reason %q",
			ref.ExperimentalReason, mod.Descriptor().ExperimentalReason)
	}
	if !ref.Usable {
		t.Errorf("Codex ref Usable = false (%q), want experimental status to leave it usable", ref.UnusableReason)
	}
}

func TestCodexRegistry_ListedRefIsExperimentalWithSameReason(t *testing.T) {
	// Arrange
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "MosaicDeploy", "harnesses"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reg, err := registry.Discover(registry.Options{MosaicRoot: root, BuiltinContentDir: frozenContentDirs(t)})
	if err != nil {
		t.Fatalf("registry.Discover: %v", err)
	}

	// Act
	var listed *domain.HarnessRef
	for _, r := range reg.List() {
		if r.ID == "codex" {
			r := r
			listed = &r
		}
	}

	// Assert
	if listed == nil {
		t.Fatal("registry does not list the codex harness")
	}
	if !listed.IsExperimental() {
		t.Fatal("listed codex ref IsExperimental() = false, want true")
	}
	if want := codexReason(t); listed.ExperimentalReason != want {
		t.Errorf("listed reason = %q, want %q", listed.ExperimentalReason, want)
	}
}
