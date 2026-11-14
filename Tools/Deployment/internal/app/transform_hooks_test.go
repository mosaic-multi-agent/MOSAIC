package app_test

// transform_hooks_test.go specifies how the whole-deployment transform carries hook bundles:
// the bundles present in the source deployment are installed from the target harness's
// variant, registration steps reach the TODO file, and every bundle that is not carried is named
// with a reason. Source hook files stay untouched outside the GHCP pair; between the GHCP
// targets the source-only hook files are removed (transform_ghcp_pair_exec_test.go).

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trHookRun wires a Claude Code to OpenCode run whose source deployment holds the given bundles.
func trHookRun(t *testing.T, bundles ...string) *trRun {
	t.Helper()
	r := trCarryRun(t, hClaude, hOpen, nil)
	for _, b := range bundles {
		r.seedHookBundle(b)
	}
	return r
}

func TestTransformHooks_InstallsTheTargetVariantOfABundlePresentInTheSource(t *testing.T) {
	r := trHookRun(t, trHookKey)

	r.mustRun(r.request())

	dest := r.hooksDir(hOpen) + "/logger.ts"
	if !r.exists(dest) {
		t.Fatalf("target variant file %s was not written", dest)
	}
	if got, want := r.read(dest), trReadTestdata(t, "hooks/nested/logger.ts"); got != want {
		t.Error("target hook file is not the target variant's file")
	}
	nested := r.hooksDir(hOpen) + "/lib/util.ts"
	if !r.exists(nested) {
		t.Fatalf("nested target variant file %s was not written", nested)
	}
	if got, want := r.read(nested), trReadTestdata(t, "hooks/nested/lib/util.ts"); got != want {
		t.Error("nested target hook file is not the target variant's file")
	}
	if r.exists(r.hooksDir(hOpen) + "/logger.sh") {
		t.Error("the source variant's file was copied to the target")
	}
}

func TestTransformHooks_OutsideTheGhcpPair_SourceHookFilesAreLeftUntouched(t *testing.T) {
	r := trHookRun(t, trHookKey)
	src := r.hooksDir(hClaude) + "/logger.sh"
	before := r.read(src)

	r.mustRun(r.request())

	if !r.exists(r.hooksDir(hOpen) + "/logger.ts") {
		t.Fatal("the bundle was not carried")
	}
	if r.read(src) != before {
		t.Error("the source hook file was modified")
	}
}

func TestTransformHooks_BundleAbsentFromTheSourceDeploymentIsNotCarried(t *testing.T) {
	r := trHookRun(t)
	r.seedSkill()

	res := r.mustRun(r.request())

	if r.exists(r.hooksDir(hOpen) + "/logger.ts") {
		t.Error("a bundle that is not in the source deployment was installed")
	}
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactHook {
			t.Errorf("hook outcome %+v for a bundle absent from the source", f)
		}
	}
}

func TestTransformHooks_ManifestRecordsEachWrittenFileWithCatalogVersionAndRealHash(t *testing.T) {
	r := trHookRun(t, trHookKey)

	r.mustRun(r.request())

	entries := trEntryByRef(trLoadManifest(t, r.ws), domain.ArtifactHook, trHookKey)
	want := map[string]string{
		r.hooksDir(hOpen) + "/logger.ts":   r.read(r.hooksDir(hOpen) + "/logger.ts"),
		r.hooksDir(hOpen) + "/lib/util.ts": r.read(r.hooksDir(hOpen) + "/lib/util.ts"),
	}
	if len(entries) != len(want) {
		t.Fatalf("hook manifest entries = %+v, want exactly one per written file", entries)
	}
	for _, e := range entries {
		content, ok := want[filepath.ToSlash(e.TargetPath)]
		if !ok {
			t.Errorf("entry path %q is not a written hook file; want one of %v", e.TargetPath, want)
			continue
		}
		if e.Version != trHookVersion {
			t.Errorf("%s: version = %q, want the catalog bundle version %q", e.TargetPath, e.Version, trHookVersion)
		}
		if e.ContentHash != manifest.Hash([]byte(content)) {
			t.Errorf("%s: content hash = %q, want the hash of the written bytes", e.TargetPath, e.ContentHash)
		}
	}
}

func TestTransformHooks_OutcomeIsReportedAndCounted(t *testing.T) {
	r := trHookRun(t, trHookKey)

	res := r.mustRun(r.request())

	f := trOutcome(t, res, domain.ArtifactHook, trHookKey)
	if f.Status != app.StatusTransformed || f.Action != app.PlanWriteCreate {
		t.Errorf("status %q action %q, want transformed/create", f.Status, f.Action)
	}
	if res.Transformed < 1 {
		t.Errorf("Transformed = %d, want the hook counted", res.Transformed)
	}
}

func TestTransformHooks_PerformableRegistrationStepWritesItsFragment(t *testing.T) {
	r := trHookRun(t, trHookKey)

	r.mustRun(r.request())

	if !r.exists(trRegistrationFile) {
		t.Fatalf("registration file %s was not written", trRegistrationFile)
	}
	if got := r.read(trRegistrationFile); !strings.Contains(got, "logger") {
		t.Errorf("registration file content %q lacks the fragment", got)
	}
}

func TestTransformHooks_ManualRegistrationStepIsReportedInTheTodoFile(t *testing.T) {
	r := trHookRun(t, trHookKey)

	res := r.mustRun(r.request())

	if todo := trReadTodo(t, res); !strings.Contains(todo, trManualStepText) {
		t.Errorf("TODO file does not carry the registration step %q", trManualStepText)
	}
}

func TestTransformHooks_BundleWithoutTargetVariantIsNotCarriedAndNamedWithReason(t *testing.T) {
	r := trHookRun(t, trHookKey, trLegacyHookKey)

	res := r.mustRun(r.request())

	f := trOutcome(t, res, domain.ArtifactHook, trLegacyHookKey)
	if f.Status != app.StatusSkipped || f.Skip != app.SkipNoTargetVariant || f.Reason == "" {
		t.Errorf("status %q skip %q reason %q, want skipped/no-target-variant with a reason", f.Status, f.Skip, f.Reason)
	}
	if got := trEntryByRef(trLoadManifest(t, r.ws), domain.ArtifactHook, trLegacyHookKey); len(got) != 0 {
		t.Errorf("manifest records %d entries for a bundle that was not carried", len(got))
	}
	if todo := trReadTodo(t, res); !strings.Contains(todo, trLegacyHookKey) {
		t.Errorf("TODO file does not name the bundle %q that was not carried", trLegacyHookKey)
	}
}

func TestTransformHooks_GhcpPairCarriesTheBundleWithTheTargetVariantInsteadOfSkippingIt(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := trCarryRun(t, pair[0], pair[1], nil)
			r.seedHookBundle(trHookKey)
			dest := r.hooksDir(pair[1]) + "/logger.sh"
			req := r.request()

			res := r.mustRun(req)

			f := trOutcome(t, res, domain.ArtifactHook, trHookKey)
			if f.Status != app.StatusTransformed || f.Skip != "" || f.Action != app.PlanWriteOverwrite {
				t.Errorf("status %q skip %q action %q, want transformed/overwrite with no skip", f.Status, f.Skip, f.Action)
			}
			if got, want := r.read(dest), trReadTestdata(t, "hooks/flat/logger.sh"); got != want {
				t.Error("the shared hook file is not the target variant's file")
			}
			got := trEntryByRef(trLoadManifestIfAny(t, r.ws), domain.ArtifactHook, trHookKey)
			if len(got) != 1 {
				t.Fatalf("manifest records %d hook entries for the carried bundle, want 1", len(got))
			}
			if filepath.ToSlash(got[0].TargetPath) != dest || got[0].Version != trHookVersion ||
				got[0].ContentHash != manifest.Hash([]byte(r.read(dest))) {
				t.Errorf("hook entry = %+v, want the written file %s at version %s with the hash of its bytes", got[0], dest, trHookVersion)
			}
		})
	}
}

func TestTransformHooks_DryRunWritesNothingAndReportsTheBundle(t *testing.T) {
	r := trHookRun(t, trHookKey)
	before := r.snapshot()
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run changed the workspace: %s", diff)
	}
	if f := trOutcome(t, res, domain.ArtifactHook, trHookKey); f.Status != app.StatusTransformed {
		t.Errorf("dry-run hook status = %q, want transformed (would be written)", f.Status)
	}
}

func TestTransformHooks_ABundleAloneStillSaysHooksMayBeNewerThanTheSource(t *testing.T) {
	r := trHookRun(t, trHookKey)

	res := r.mustRun(r.request())

	if len(trNotes(res, app.NoteNewerTargetContent)) == 0 {
		t.Errorf("no %q note although a hook bundle was transformed; notes: %+v", app.NoteNewerTargetContent, res.Notes)
	}
}
