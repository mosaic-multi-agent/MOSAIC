package opencodev2_test

// Tests for the hook plan the OpenCode V2 module resolves from a hook bundle. Only top-level
// files in .opencode/plugins/ load as V2 plugins and a lib subfolder is importable, so the
// plan places plugin files at the top level and helpers in a subfolder. The bundles come from
// the tool-local fixture catalog testdata/catalog/hook-plugin-v2, which has an opencode-v2
// variant (the live logger bundle has none yet).

import (
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func planFor(t *testing.T, bundleID string) domain.HookPlan {
	t.Helper()
	plan, err := newModule(t).HookPlan(domain.HookPlanRequest{Bundle: loadFixtureBundle(t, bundleID), Scope: domain.ScopeProject})
	if err != nil {
		t.Fatalf("HookPlan(%s): %v", bundleID, err)
	}
	return plan
}

func targetNames(files []domain.HookFile) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.TargetName)
	}
	return out
}

func TestHookPlan_UsesTheOpenCodeV2VariantAndTheV2PluginsFolder(t *testing.T) {
	plan := planFor(t, "plugin-bundle")

	if !plan.Supported {
		t.Fatalf("Supported = false (%q), want true", plan.Reason)
	}
	if plan.TargetDir != ".opencode/plugins" {
		t.Errorf("TargetDir = %q, want %q", plan.TargetDir, ".opencode/plugins")
	}
}

func TestHookPlan_PlacesThePluginAtTheTopLevelAndHelpersInASubfolder(t *testing.T) {
	plan := planFor(t, "plugin-bundle")

	got := targetNames(plan.Files)
	want := map[string]bool{"plugin.ts": true, "lib/helper.ts": true}
	if len(got) != len(want) {
		t.Fatalf("plan files = %v, want exactly %v", got, []string{"plugin.ts", "lib/helper.ts"})
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected plan file %q; want only the opencode-v2 variant files", name)
		}
		if strings.Contains(name, "v1-plugin") {
			t.Errorf("plan file %q comes from the V1 opencode variant", name)
		}
	}
}

func TestHookPlan_FilesPointAtExistingSourceFiles(t *testing.T) {
	plan := planFor(t, "plugin-bundle")

	if len(plan.Files) == 0 {
		t.Fatal("plan has no files")
	}
	for _, f := range plan.Files {
		if info, err := os.Stat(f.SourcePath); err != nil || info.IsDir() {
			t.Errorf("source %q of %q is not a readable file (err %v)", f.SourcePath, f.TargetName, err)
		}
	}
}

func TestHookPlan_NeedsNoRegistrationSteps(t *testing.T) {
	plan := planFor(t, "plugin-bundle")

	if len(plan.Registration) != 0 {
		t.Errorf("Registration = %v, want none: a plugin file in the folder is enough", plan.Registration)
	}
}

func TestHookPlan_BundleWithoutAnOpenCodeV2Variant_HasNoFilesAndNoError(t *testing.T) {
	// The bundle carries opencode (V1) and claude-code variants only.
	plan := planFor(t, "no-v2-bundle")

	if len(plan.Files) != 0 {
		t.Errorf("plan files = %v, want none: neither the V1 opencode nor the claude-code variant may be used", targetNames(plan.Files))
	}
	if !plan.Supported {
		t.Errorf("Supported = false (%q), want true: the harness supports hooks, the bundle has just no variant", plan.Reason)
	}
	if plan.TargetDir != ".opencode/plugins" {
		t.Errorf("TargetDir = %q, want %q", plan.TargetDir, ".opencode/plugins")
	}
}
