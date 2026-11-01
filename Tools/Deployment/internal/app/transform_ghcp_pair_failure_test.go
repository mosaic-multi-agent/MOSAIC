package app_test

// transform_ghcp_pair_failure_test.go specifies that a failure during a real transform between
// the two GHCP targets (a hook file or agent that cannot be replaced, a file that cannot be
// removed, a manifest that cannot be saved) returns the reverted-run error and leaves the
// source deployment usable: agents, hook files, registration and manifest as they were.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// trBlock replaces the file at rel (if any) with a folder holding a file, so the path can
// neither be overwritten nor removed.
func trBlock(t *testing.T, ws, rel string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(ws, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("remove %s: %v", rel, err)
	}
	trWrite(t, ws, rel+"/keep.txt", "blocker")
}

// trAssertSourceUsable fails the test unless the run returned the reverted-run error and the
// workspace (backups aside) and the manifest are exactly as they were.
func trAssertSourceUsable(t *testing.T, r *trRun, before map[string]string, res app.TransformHarnessResult, err error) {
	t.Helper()
	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("error = %v, want ErrRunReverted", err)
	}
	if res.Cancelled || len(res.Files) != 0 || len(res.Removals) != 0 || len(res.Replacements) != 0 {
		t.Errorf("result = %+v, want the identifiers only after a reverted run", res)
	}
	if diff := trSameTree(trWithoutBackups(before), trWithoutBackups(r.snapshot())); diff != "" {
		t.Errorf("the reverted run left a trace: %s", diff)
	}
	if m := trLoadManifest(t, r.ws); m.HarnessID != r.src {
		t.Errorf("manifest harness_id = %q after the revert, want the source %q", m.HarnessID, r.src)
	}
}

// The manifest save is the last step of the executor, after it removed the source-only files
// and replaced the shared ones. A file removal alone cannot be made to fail on both operating
// systems (the planner leaves a non-empty folder alone), so the failing manifest save, observed
// after the removals and replacements ran, stands in for a failed removal.

func TestTransformGhcpPair_ManifestSaveFailureRevertsEveryChange(t *testing.T) {
	trPairEach2(t, func(t *testing.T, c trPairExecCase, src, tgt string) (*trRun, func(*trRun)) {
		var seen map[string]string
		r := trPairExecRun(t, src, tgt, c, trFailManifestSave(t, &seen))
		return r, func(r *trRun) { trAssertChangesAppliedBeforeSave(t, r, c, seen) }
	})
}

// trAssertChangesAppliedBeforeSave fails the test unless the workspace seen by the failing save
// already held the target deployment: removals and replacements ran before the revert.
func trAssertChangesAppliedBeforeSave(t *testing.T, r *trRun, c trPairExecCase, seen map[string]string) {
	t.Helper()
	if seen == nil {
		t.Fatalf("the manifest save was never reached")
	}
	hooks := r.hooksDir(r.src)
	if _, ok := seen[hooks+"/"+trPairSourceOnlyFile]; ok {
		t.Errorf("source-only hook file was still there at the save: the removal had not run")
	}
	if got, want := seen[hooks+"/logger.sh"], trReadTestdata(t, "hooks/flat/logger.sh"); got != want {
		t.Errorf("logger.sh held %q at the save, want the replaced target file %q", got, want)
	}
	if got, want := seen[trPairRegistrationFile], "{\"hooks\":[\"logger\"]}\n"; got != want {
		t.Errorf("registration held %q at the save, want the replaced target fragment %q", got, want)
	}
	if c.agentRel == trPairLegacyAgent {
		if _, ok := seen[trPairLegacyAgent]; ok {
			t.Errorf("legacy %s was still there at the save: the removal had not run", trPairLegacyAgent)
		}
	}
}

func TestTransformGhcpPair_BackupFailureRevertsEveryChange(t *testing.T) {
	trPairEach2(t, func(t *testing.T, c trPairExecCase, src, tgt string) (*trRun, func(*trRun)) {
		r := trPairExecRun(t, src, tgt, c)
		// A file where the backup folder must be makes every backup fail.
		trWrite(t, r.ws, ".mosaic/backups", "not a folder")
		return r, nil
	})
}

func TestTransformGhcpPair_BlockedTargetHookFileLeavesTheSourceUsable(t *testing.T) {
	trPairEach2(t, func(t *testing.T, c trPairExecCase, src, tgt string) (*trRun, func(*trRun)) {
		r := trPairExecRun(t, src, tgt, c)
		trBlock(t, r.ws, r.hooksDir(tgt)+"/logger.sh")
		return r, nil
	})
}

// trPairEach2 runs a failure scenario per direction and recorded-deployment shape: build wires
// the run, injects the failure and may return an extra check to run after the run; the real
// run must revert.
func trPairEach2(t *testing.T, build func(t *testing.T, c trPairExecCase, src, tgt string) (*trRun, func(*trRun))) {
	t.Helper()
	for _, c := range trPairExecCases {
		for _, pair := range trPairDirections {
			t.Run(c.name+" "+pair[0]+"->"+pair[1], func(t *testing.T) {
				r, check := build(t, c, pair[0], pair[1])
				before := r.snapshot()

				res, err := r.run(r.request())

				trAssertSourceUsable(t, r, before, res, err)
				if check != nil {
					check(r)
				}
			})
		}
	}
}

func TestTransformGhcpPair_RevertedRunKeepsTheSourceAgentsAndHookFilesReadable(t *testing.T) {
	r := trPairExecRun(t, hCLI, hVSCode, trPairExecCases[1], trFailManifestSave(t, nil))
	agent, edited := r.read(trPairLegacyAgent), r.read(r.hooksDir(hCLI)+"/logger.sh")
	registration := r.read(trPairRegistrationFile)

	_, err := r.run(r.request())

	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("error = %v, want ErrRunReverted", err)
	}
	if r.exists(trPairInPlaceAgent) {
		t.Errorf("%s was left behind by the reverted run", trPairInPlaceAgent)
	}
	if got := r.read(trPairLegacyAgent); got != agent {
		t.Errorf("legacy agent changed to %q", got)
	}
	if got := r.read(r.hooksDir(hCLI) + "/logger.sh"); got != edited {
		t.Errorf("hook file changed to %q", got)
	}
	if got := r.read(trPairRegistrationFile); got != registration {
		t.Errorf("registration file changed to %q", got)
	}
	if e, ok := trEntryAt(trLoadManifest(t, r.ws), trPairLegacyAgent); !ok || e.Ref.Kind != domain.ArtifactAgent {
		t.Errorf("manifest lost the entry of %s", trPairLegacyAgent)
	}
}
