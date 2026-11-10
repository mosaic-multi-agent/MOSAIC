package app_test

// transform_skills_test.go specifies how the whole-deployment transform carries skills: the
// source bytes and version are kept, the files land in the target skills folder, the manifest
// records real content hashes, and a dry run writes nothing.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestTransformSkills_CarriesEverySkillFileUnchangedToTheTargetSkillsFolder(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	want := r.seedSkill()

	r.mustRun(r.request())

	for rel, content := range want {
		dest := r.skillsDir(hOpen) + "/" + trSkillKey + "/" + rel
		if !r.exists(dest) {
			t.Errorf("skill file %s was not written", dest)
			continue
		}
		if got := r.read(dest); got != content {
			t.Errorf("%s differs from the source bytes", dest)
		}
	}
}

func TestTransformSkills_TargetSkillsFolderFollowsTheTargetHarness(t *testing.T) {
	cases := []struct{ name, src, tgt, folder string }{
		{"to-claude-code", hOpen, hClaude, ".claude/skills"},
		{"to-opencode", hClaude, hOpen, ".opencode/skills"},
		{"to-ghcp-cli", hClaude, hCLI, ".github/skills"},
		{"to-vscode-ghcp", hOpen, hVSCode, ".github/skills"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := trCarryRun(t, c.src, c.tgt, nil)
			r.seedSkill()

			r.mustRun(r.request())

			if dest := c.folder + "/" + trSkillKey + "/SKILL.md"; !r.exists(dest) {
				t.Errorf("skill was not written to %s", dest)
			}
		})
	}
}

func TestTransformSkills_LegacyVersionKeyAndLineEndingsAreCarriedByteForByte(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	legacy := "---\r\nname: old-skill\r\nversion: 0.9.0\r\n---\r\n\r\nOld skill.\r\n"
	trWrite(t, r.ws, r.skillsDir(hClaude)+"/old-skill/SKILL.md", legacy)

	r.mustRun(r.request())

	dest := r.skillsDir(hOpen) + "/old-skill/SKILL.md"
	if !r.exists(dest) {
		t.Fatalf("legacy skill was not written to %s", dest)
	}
	if got := r.read(dest); got != legacy {
		t.Errorf("legacy skill bytes changed:\n got %q\nwant %q", got, legacy)
	}
	if strings.Contains(r.read(dest), "mosaic_version") {
		t.Error("the legacy version key was renamed; the skill must be copied unchanged")
	}
}

func TestTransformSkills_CatalogVersionNeverReplacesTheSourceSkill(t *testing.T) {
	cat := trCarryCatalog(t)
	cat.skills = []domain.Skill{{
		Key: trSkillKey, Version: "9.9.0", SourceDir: trAbs(t, "hooks/flat"), EntryFile: "logger.sh",
		Files: []string{"logger.sh"},
	}}
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat })
	want := r.seedSkill()

	r.mustRun(r.request())

	dest := r.skillsDir(hOpen) + "/" + trSkillKey + "/SKILL.md"
	if !r.exists(dest) {
		t.Fatalf("skill was not written to %s", dest)
	}
	if got := r.read(dest); got != want["SKILL.md"] {
		t.Errorf("target skill is not the source skill (catalog version must not be used)")
	}
}

func TestTransformSkills_ManifestRecordsEachFileWithRealHashAndSourceVersion(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	want := r.seedSkill()

	r.mustRun(r.request())

	m := trLoadManifest(t, r.ws)
	for rel, content := range want {
		path := r.skillsDir(hOpen) + "/" + trSkillKey + "/" + rel
		e, ok := trEntryAt(m, path)
		if !ok {
			t.Errorf("no manifest entry at %s", path)
			continue
		}
		if e.Ref != (domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: trSkillKey}) {
			t.Errorf("%s: ref = %+v, want skill %s", path, e.Ref, trSkillKey)
		}
		if e.ContentHash != manifest.Hash([]byte(content)) {
			t.Errorf("%s: content hash %q is not the hash of the carried bytes", path, e.ContentHash)
		}
		if e.Version != trSkillVersion {
			t.Errorf("%s: version = %q, want the source version %q", path, e.Version, trSkillVersion)
		}
	}
}

func TestTransformSkills_OutcomeIsReportedAndCounted(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	r.seedSkill()

	res := r.mustRun(r.request())

	f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey)
	if f.Status != app.StatusTransformed || f.Action != app.PlanWriteCreate {
		t.Errorf("status %q action %q, want transformed/create", f.Status, f.Action)
	}
	if f.DestinationPath != r.skillsDir(hOpen)+"/"+trSkillKey {
		t.Errorf("DestinationPath = %q, want the target skill folder", f.DestinationPath)
	}
	if res.Transformed < 1 {
		t.Errorf("Transformed = %d, want the skill counted", res.Transformed)
	}
}

func TestTransformSkills_DryRunWritesNothingAndReportsWhatWouldBeWritten(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	r.seedSkill()
	before := r.snapshot()
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run changed the workspace: %s", diff)
	}
	if f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey); f.Status != app.StatusTransformed {
		t.Errorf("dry-run skill status = %q, want transformed (would be written)", f.Status)
	}
}

func TestTransformSkills_ExistingDestinationIsKeptWithoutOverwrite(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	r.seedSkill()
	dest := r.skillsDir(hOpen) + "/" + trSkillKey + "/SKILL.md"
	trWrite(t, r.ws, dest, "locally edited\n")

	res := r.mustRun(r.request())

	if got := r.read(dest); got != "locally edited\n" {
		t.Errorf("an existing skill file was replaced without Overwrite")
	}
	if f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey); f.Status != app.StatusSkipped || f.Skip != app.SkipDestinationExists {
		t.Errorf("status %q skip %q, want skipped/destination-exists", f.Status, f.Skip)
	}
}

func TestTransformSkills_ExistingDestinationIsReplacedWithOverwrite(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	want := r.seedSkill()
	dest := r.skillsDir(hOpen) + "/" + trSkillKey + "/SKILL.md"
	trWrite(t, r.ws, dest, "locally edited\n")
	req := r.request()
	req.Overwrite = true

	r.mustRun(req)

	if got := r.read(dest); got != want["SKILL.md"] {
		t.Errorf("with Overwrite the skill file is not the source bytes")
	}
}

func TestTransformSkills_GhcpPairSharingTheSkillsFolderWritesNothingAndSaysWhy(t *testing.T) {
	r := trCarryRun(t, hCLI, hVSCode, nil)
	want := r.seedSkill()
	req := r.request()
	req.Overwrite = true

	res := r.mustRun(req)

	f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey)
	if f.Status != app.StatusSkipped || f.Skip != app.SkipDestinationIsSource || f.Reason == "" {
		t.Errorf("status %q skip %q reason %q, want skipped/destination-is-source with a reason", f.Status, f.Skip, f.Reason)
	}
	if got := r.read(".github/skills/" + trSkillKey + "/SKILL.md"); got != want["SKILL.md"] {
		t.Error("the source skill file was changed")
	}
	if got := trEntryByRef(trLoadManifestIfAny(t, r.ws), domain.ArtifactSkill, trSkillKey); len(got) != 0 {
		t.Errorf("manifest records %d skill entries for a skill that was not written", len(got))
	}
}

// trLoadManifestIfAny loads the manifest; a missing manifest is an empty one.
func trLoadManifestIfAny(t *testing.T, ws string) domain.Manifest {
	t.Helper()
	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return snap.Manifest
}
