package opencodev2_test

// Tests that the OpenCode V2 module declares itself experimental with a non-empty reason,
// that the reason reaches the harness ref the module reports and the ref the registry
// lists, and that the module is still usable.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

func TestModuleRef_IsExperimentalWithTheDescriptorReason(t *testing.T) {
	// Arrange
	mod := newModule(t)

	// Act
	ref := mod.Ref()

	// Assert
	if !ref.IsExperimental() {
		t.Fatal("OpenCode V2 module ref IsExperimental() = false, want true")
	}
	if strings.TrimSpace(ref.ExperimentalReason) == "" {
		t.Error("ref ExperimentalReason is blank, want the reason the descriptor declares")
	}
	if ref.ExperimentalReason != mod.Descriptor().ExperimentalReason {
		t.Errorf("ref reason = %q, want the descriptor reason %q", ref.ExperimentalReason, mod.Descriptor().ExperimentalReason)
	}
	if !ref.Usable {
		t.Errorf("ref Usable = false (%q), want experimental status to leave the harness usable", ref.UnusableReason)
	}
	if ref.Tier != domain.TierBuiltin {
		t.Errorf("ref Tier = %q, want %q", ref.Tier, domain.TierBuiltin)
	}
}

func TestRegistry_ListsTheModuleAsExperimental(t *testing.T) {
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
		if r.ID == "opencode-v2" {
			r := r
			listed = &r
		}
	}

	// Assert
	if listed == nil {
		t.Fatal("registry does not list the opencode-v2 harness; the package must register itself on import")
	}
	if !listed.IsExperimental() {
		t.Error("listed opencode-v2 ref IsExperimental() = false, want true")
	}
	if want := newModule(t).Descriptor().ExperimentalReason; listed.ExperimentalReason != want {
		t.Errorf("listed reason = %q, want %q", listed.ExperimentalReason, want)
	}
}
