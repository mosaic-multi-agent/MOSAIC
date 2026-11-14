package main

// e2e_hook_entries_test.go runs the built mosaic-deploy binary to verify that every flow that
// writes hooks (deploy, update, deploy hooks, transform) records one manifest entry per deployed
// hook file, that an edited hook file is backed up before it is overwritten, and that the
// pre-change bundle-level record is replaced. Fixtures are built in temporary folders; nothing
// reads the shipped catalog.

import (
	"strings"
	"testing"
)

// requirePerFileEntries fails unless the manifest holds exactly one entry per given path under
// the harness, each with the real content hash of the file on disk, and no entry of the
// registration file or with the empty-content hash.
func requirePerFileEntries(t *testing.T, h hkWorkspace, harness string, paths ...string) {
	t.Helper()
	entries := h.hookEntries(t)
	own := 0
	for key, e := range entries {
		if !strings.HasPrefix(key, harness+"|") {
			continue
		}
		own++
		if e.TargetPath == hkRegRel {
			t.Errorf("the registration file %q has a manifest entry", e.TargetPath)
		}
		if e.ContentHash == hkEmptyHash {
			t.Errorf("%s: entry carries the empty-content hash", e.TargetPath)
		}
		if !h.exists(e.TargetPath) {
			t.Errorf("%s: entry records a path that does not exist", e.TargetPath)
		}
	}
	if own != len(paths) {
		t.Errorf("%s hook entries = %d (%v); want %d", harness, own, entries, len(paths))
	}
	for _, p := range paths {
		e, ok := entries[harness+"|"+p]
		if !ok {
			t.Errorf("no %s hook entry for %q (entries: %v)", harness, p, entries)
			continue
		}
		if want := hkHash(h.read(t, p)); e.ContentHash != want {
			t.Errorf("%s: content_hash = %q; want the hash of the file on disk %q", p, e.ContentHash, want)
		}
	}
}

func TestBinary_DeployWithHook_RecordsOneEntryPerHookFileAndKeepsAgentActions(t *testing.T) {
	h := newHkWorkspace(t)

	s := h.deployWithHook(t)

	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
	if !h.exists(hkRegRel) {
		t.Error("the registration file was not created")
	}
	if got := s.takenOf("agent"); len(got) == 0 {
		t.Error("the run summary has no agent actions")
	} else {
		for _, taken := range got {
			if taken != "created" {
				t.Errorf("agent action Taken = %q; want created", taken)
			}
		}
	}
}

func TestBinary_HooksSubcommand_RecordsOneEntryPerHookFile(t *testing.T) {
	h := newHkWorkspace(t)

	h.run(t, "hooks", "--hooks", hkKey, "--yes", "--output", "json")

	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_UpdateAfterDeploy_LeavesUneditedHookFilesAlone(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)

	s := h.run(t, "update", "--auto-confirm", "--output", "json")

	for _, taken := range s.takenOf("hook") {
		if taken != "unchanged" {
			t.Errorf("hook action Taken = %q on an unedited deployment; want unchanged", taken)
		}
	}
	if b := h.backups(); len(b) != 0 {
		t.Errorf("backups = %v; an unedited deployment makes none", b)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_UpdateWithEditedHookFile_BacksUpThenRestoresAndRerecords(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)
	h.write(t, hkFlatRel, hkEditedText)

	s := h.run(t, "update", "--auto-confirm", "--output", "json")

	if got := h.read(t, hkFlatRel); got != hkFlatSrc {
		t.Errorf("edited hook file = %q; want it restored to the source", got)
	}
	backups := h.backups()
	if len(backups) != 1 || h.readAbs(t, backups[0]) != hkEditedText {
		t.Errorf("backups = %v; want exactly one holding the hand-edited text", backups)
	}
	if got := s.takenOf("hook"); !containsString(got, "backed-up-and-overwritten") {
		t.Errorf("hook actions = %v; want one backed-up-and-overwritten", got)
	}
	if got := h.read(t, hkNestedRel); got != hkNestedSrc {
		t.Errorf("untouched hook file changed: %q", got)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_UpdateAfterCatalogBump_RewritesHookFilesWithoutBackup(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)
	hkWriteBundle(t, h.mosaicRoot, "1.1.0")

	h.run(t, "update", "--auto-confirm", "--output", "json")

	if b := h.backups(); len(b) != 0 {
		t.Errorf("backups = %v; a plain version update of unedited files makes none", b)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_HooksSubcommandOnUnrecordedFile_BacksUpWhenAskedTo(t *testing.T) {
	h := newHkWorkspace(t)
	h.write(t, hkFlatRel, hkEditedText)

	h.run(t, "hooks", "--hooks", hkKey, "--yes", "--conflict", "backup", "--output", "json")

	if got := h.read(t, hkFlatRel); got != hkFlatSrc {
		t.Errorf("unrecorded hook file = %q; want it replaced by the source", got)
	}
	backups := h.backups()
	if len(backups) != 1 || h.readAbs(t, backups[0]) != hkEditedText {
		t.Errorf("backups = %v; want exactly one holding the unrecorded file", backups)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_HooksSubcommandOnUnrecordedFile_SkipKeepsFile(t *testing.T) {
	h := newHkWorkspace(t)
	h.write(t, hkFlatRel, hkEditedText)

	h.run(t, "hooks", "--hooks", hkKey, "--yes", "--conflict", "skip", "--output", "json")

	if got := h.read(t, hkFlatRel); got != hkEditedText {
		t.Errorf("skipped hook file = %q; want it kept", got)
	}
	if _, ok := h.hookEntries(t)["claude-code|"+hkFlatRel]; ok {
		t.Error("a skipped file has an entry for bytes Deploy did not write")
	}
}

func TestBinary_PreChangeBundleEntry_ReplacedAtNextHookDeployWithoutConflict(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)
	h.useBundleLevelRecord(t)
	h.write(t, hkFlatRel, hkEditedText)

	h.run(t, "hooks", "--hooks", hkKey, "--yes", "--conflict", "skip", "--output", "json")

	if got := h.read(t, hkFlatRel); got != hkFlatSrc {
		t.Errorf("hook file under a pre-change record = %q; want it overwritten without a conflict prompt", got)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_DamagedHookBundle_RewrittenByNextUpdate(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)
	if err := removeWorkspaceFile(h.ws, hkNestedRel); err != nil {
		t.Fatalf("remove hook file: %v", err)
	}

	h.run(t, "update", "--auto-confirm", "--output", "json")

	if !h.exists(hkNestedRel) {
		t.Fatal("the missing hook file was not rewritten")
	}
	if got := h.read(t, hkNestedRel); got != hkNestedSrc {
		t.Errorf("rewritten hook file = %q; want the source", got)
	}
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}

func TestBinary_TransformWithHooks_RecordsPerFileEntriesForTarget(t *testing.T) {
	h := newHkWorkspace(t)
	h.deployWithHook(t)

	run := runInWorkspace(t, h.mosaicRoot, h.ws, "transform", "--target-harness", "opencode", "--yes")
	requireRanOrSkipped(t, run)

	requirePerFileEntries(t, h, "opencode", hkPluginRel)
	requirePerFileEntries(t, h, "claude-code", hkFlatRel, hkNestedRel)
}
