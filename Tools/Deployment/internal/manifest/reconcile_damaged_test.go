package manifest_test

// reconcile_damaged_test.go specifies the damaged-bundle mark of reconciliation: a hook bundle
// that is kept although some of its recorded files are missing is reported, per harness and
// bundle key, so planning can see it. It reuses the helpers of reconcile_test.go.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestReconcile_HookBundleWithSomeFilesMissing_IsMarkedDamaged(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger.json"))

	want := []manifest.DamagedBundle{{Harness: "ghcp-cli", Key: "logger"}}
	if !reflect.DeepEqual(r.Damaged, want) {
		t.Errorf("Damaged = %+v, want exactly %+v", r.Damaged, want)
	}
	if !r.IsDamaged("ghcp-cli", "logger") {
		t.Error("IsDamaged(ghcp-cli, logger) = false, want true")
	}
}

func TestReconcile_HookBundleWithEveryFilePresent_IsNotDamaged(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger.json", ".github/hooks/logger.sh"))

	if len(r.Damaged) != 0 || r.IsDamaged("ghcp-cli", "logger") {
		t.Errorf("Damaged = %+v, want none for a complete bundle", r.Damaged)
	}
}

func TestReconcile_HookBundleWithEveryFileMissing_IsNotMarkedDamaged(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
	)

	r := manifest.Reconcile(m, present())

	if len(r.Damaged) != 0 || r.IsDamaged("ghcp-cli", "logger") {
		t.Errorf("Damaged = %+v, want none: a bundle dropped whole is not kept damaged", r.Damaged)
	}
}

func TestReconcile_BundleWhoseOnlyPresentFileIsABundleLevelEntry_IsNotMarkedDamaged(t *testing.T) {
	m := rcManifest(
		rcBundleLevelHook("ghcp-cli", "logger", ".github/hooks/logger"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger"))

	if len(r.Damaged) != 0 || r.IsDamaged("ghcp-cli", "logger") {
		t.Errorf("Damaged = %+v, want none: a bundle-level entry does not count towards presence", r.Damaged)
	}
}

func TestReconcile_BundleLevelEntryAlone_IsNotMarkedDamaged(t *testing.T) {
	m := rcManifest(rcBundleLevelHook("ghcp-cli", "logger", ".github/hooks/logger"))

	r := manifest.Reconcile(m, present())

	if len(r.Damaged) != 0 || r.IsDamaged("ghcp-cli", "logger") {
		t.Errorf("Damaged = %+v, want none for a bundle-level entry", r.Damaged)
	}
}

func TestReconcile_DamagedBundleIsMarkedOnlyForItsOwnHarness(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
		rcEntry("opencode", domain.ArtifactHook, "logger", ".opencode/plugins/logger.ts"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger.json", ".opencode/plugins/logger.ts"))

	if !r.IsDamaged("ghcp-cli", "logger") {
		t.Error("the partly missing bundle of ghcp-cli is not marked damaged")
	}
	if r.IsDamaged("opencode", "logger") {
		t.Error("the complete bundle of opencode with the same key is marked damaged")
	}
	if len(r.Damaged) != 1 {
		t.Errorf("Damaged = %+v, want only the ghcp-cli bundle", r.Damaged)
	}
}

func TestReconcile_OnlyTheBundleWithMissingFilesIsMarkedDamaged(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "audit", ".github/hooks/audit.json"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger.json", ".github/hooks/audit.json"))

	if !r.IsDamaged("ghcp-cli", "logger") || r.IsDamaged("ghcp-cli", "audit") || len(r.Damaged) != 1 {
		t.Errorf("Damaged = %+v, want only logger", r.Damaged)
	}
}
