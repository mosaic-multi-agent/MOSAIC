package app

// transform_plan_skills_hooks_test.go specifies planning of skills and hook bundles: what is
// planned from the source deployment, the not-carried reasons, and the exclusion of hooks for
// the harness pair that shares one hooks folder.

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// tpPlaceSkill copies the fixture skill into the source skills folder of m.
func tpPlaceSkill(t *testing.T, ws string, m *tpModule) {
	t.Helper()
	dir := m.desc.Paths.Skills.Project + "/review-skill"
	tpWriteFile(t, ws, dir+"/SKILL.md", tpFixture(t, "skills/review-skill/SKILL.md"))
	tpWriteFile(t, ws, dir+"/refs/checklist.md", tpFixture(t, "skills/review-skill/refs/checklist.md"))
}

// tpSkill returns the single planned skill.
func tpSkill(t *testing.T, p TransformPlan) PlannedSkill {
	t.Helper()
	if len(p.Skills) != 1 {
		t.Fatalf("planned skills = %+v, want exactly one", p.Skills)
	}
	return p.Skills[0]
}

// tpHook returns the single planned hook bundle.
func tpHook(t *testing.T, p TransformPlan) PlannedHookBundle {
	t.Helper()
	if len(p.Hooks) != 1 {
		t.Fatalf("planned hook bundles = %+v, want exactly one", p.Hooks)
	}
	return p.Hooks[0]
}

func TestPlanTransform_SkillIsPlannedWithItsFilesAndDestination(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceSkill(t, ws, src)

	sk := tpSkill(t, tpPlan(t, ws, src, tgt, false))

	files := append([]string(nil), sk.Files...)
	for i := range files {
		files[i] = filepath.ToSlash(files[i])
	}
	sort.Strings(files)
	if want := []string{"SKILL.md", "refs/checklist.md"}; !reflect.DeepEqual(files, want) {
		t.Errorf("Files = %v, want %v", files, want)
	}
	if sk.Key != "review-skill" || filepath.ToSlash(sk.DestinationDir) != ".opencode/skills/review-skill" {
		t.Errorf("key %q destination %q, want review-skill at .opencode/skills/review-skill", sk.Key, sk.DestinationDir)
	}
	if sk.Action != PlanWriteCreate || sk.Version != "1.4.0" {
		t.Errorf("action %q version %q, want create and 1.4.0", sk.Action, sk.Version)
	}
	if want := filepath.Join(ws, ".claude", "skills", "review-skill"); filepath.Clean(sk.SourceDir) != want {
		t.Errorf("SourceDir = %q, want %q", sk.SourceDir, want)
	}
}

func TestPlanTransform_SkillVersionFallsBackToLegacyVersionKey(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpWriteFile(t, ws, ".claude/skills/old/SKILL.md", "---\nname: old\nversion: 0.9.0\n---\n\nOld skill.\n")

	sk := tpSkill(t, tpPlan(t, ws, src, tgt, false))

	if sk.Version != "0.9.0" {
		t.Errorf("Version = %q, want 0.9.0", sk.Version)
	}
}

func TestPlanTransform_LooseFilesInSkillsFolderAreNotSkills(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceSkill(t, ws, src)
	tpWriteFile(t, ws, ".claude/skills/README.md", "not a skill\n")

	p := tpPlan(t, ws, src, tgt, false)

	if sk := tpSkill(t, p); sk.Key != "review-skill" {
		t.Errorf("skill key = %q, want review-skill", sk.Key)
	}
}

func TestPlanTransform_SkillWithExistingDestinationFileIsSkippedUnlessOverwrite(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceSkill(t, ws, src)
	tpWriteFile(t, ws, ".opencode/skills/review-skill/refs/checklist.md", "kept\n")

	skipped := tpSkill(t, tpPlan(t, ws, src, tgt, false))
	rebuilt := tpSkill(t, tpPlan(t, ws, src, tgt, true))

	if skipped.Action != PlanSkip || skipped.Skip != SkipDestinationExists || skipped.Reason == "" {
		t.Errorf("without overwrite: action %q skip %q reason %q, want skip %q with a reason", skipped.Action, skipped.Skip, skipped.Reason, SkipDestinationExists)
	}
	if rebuilt.Action != PlanWriteOverwrite {
		t.Errorf("with overwrite: action %q, want overwrite", rebuilt.Action)
	}
}

func TestPlanTransform_SkillsOfAPairSharingTheSkillsFolderAreNeverWritten(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpCLI), tpHarness(t, tpVSCode)
	tpPlaceSkill(t, ws, src)

	sk := tpSkill(t, tpPlan(t, ws, src, tgt, true))

	if sk.Action != PlanSkip || sk.Skip != SkipDestinationIsSource || sk.Reason == "" {
		t.Errorf("action %q skip %q reason %q, want skip %q with a reason (also under overwrite)", sk.Action, sk.Skip, sk.Reason, SkipDestinationIsSource)
	}
}

func TestPlanTransform_SkillIsSkippedWhenEitherHarnessHasNoSkillsFolder(t *testing.T) {
	for _, side := range []string{"source", "target"} {
		t.Run(side+" unsupported", func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
			tpPlaceSkill(t, ws, src)
			if side == "source" {
				src.desc.Paths.Skills.Supported = false
			} else {
				tgt.desc.Paths.Skills.Supported = false
			}

			p := tpPlan(t, ws, src, tgt, false)

			if len(p.Skills) != 1 {
				t.Fatalf("planned skills = %+v, want the one skill listed as skipped", p.Skills)
			}
			sk := p.Skills[0]
			if sk.Action != PlanSkip || sk.Skip != SkipUnsupported || sk.Reason == "" {
				t.Errorf("skill %s: action %q skip %q reason %q, want skip %q with a reason", sk.Key, sk.Action, sk.Skip, sk.Reason, SkipUnsupported)
			}
		})
	}
}

func TestPlanTransform_HookBundleDeployedInSourceIsPlannedWithTargetVariant(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tgt.hookPlan = tpHookPlan(".opencode/plugins", "logger.ts")
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")

	h := tpHook(t, tpPlan(t, ws, src, tgt, false))

	if h.Key != tpHookKey || h.Action != PlanWriteCreate {
		t.Errorf("key %q action %q, want %s and create", h.Key, h.Action, tpHookKey)
	}
	if !reflect.DeepEqual(h.Plan, tgt.hookPlan) {
		t.Errorf("Plan = %+v, want the target variant plan %+v", h.Plan, tgt.hookPlan)
	}
}

func TestPlanTransform_HookBundleAbsentFromSourceDeploymentIsNotPlanned(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)

	p := tpPlan(t, ws, src, tgt, false)

	if len(p.Hooks) != 0 {
		t.Errorf("planned hook bundles = %+v, want none", p.Hooks)
	}
}

func TestPlanTransform_HookBundleWithoutTargetVariantIsNotCarriedWithReason(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tgt.hookPlan = domain.HookPlan{Supported: true, TargetDir: ".opencode/plugins"}
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")

	h := tpHook(t, tpPlan(t, ws, src, tgt, false))

	if h.Action != PlanSkip || h.Skip != SkipNoTargetVariant || h.Reason == "" || h.Key != tpHookKey {
		t.Errorf("key %q action %q skip %q reason %q, want %s skipped as %q with a reason", h.Key, h.Action, h.Skip, h.Reason, tpHookKey, SkipNoTargetVariant)
	}
}

func TestPlanTransform_HookBundleOfUnsupportedTargetIsNotCarriedWithThePlanReason(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tgt.hookPlan = domain.HookPlan{Supported: false, Reason: "provisioned at runtime by the host"}
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")

	h := tpHook(t, tpPlan(t, ws, src, tgt, false))

	if h.Action != PlanSkip || h.Skip != SkipUnsupported || !strings.Contains(h.Reason, "provisioned at runtime") {
		t.Errorf("action %q skip %q reason %q, want skip %q carrying the plan reason", h.Action, h.Skip, h.Reason, SkipUnsupported)
	}
}

func TestPlanTransform_HookBundleAlreadyInTargetIsSkippedUnlessOverwrite(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tgt.hookPlan = tpHookPlan(".opencode/plugins", "logger.ts")
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")
	tpWriteFile(t, ws, ".opencode/plugins/logger.ts", "export {}\n")

	skipped := tpHook(t, tpPlan(t, ws, src, tgt, false))
	rebuilt := tpHook(t, tpPlan(t, ws, src, tgt, true))

	if skipped.Action != PlanSkip || skipped.Skip != SkipDestinationExists {
		t.Errorf("without overwrite: action %q skip %q, want skip %q", skipped.Action, skipped.Skip, SkipDestinationExists)
	}
	if rebuilt.Action != PlanWriteOverwrite {
		t.Errorf("with overwrite: action %q, want overwrite", rebuilt.Action)
	}
}

func TestPlanTransform_HooksOfTheGhcpPairAreCarriedWithTheTargetVariantNotSkipped(t *testing.T) {
	for _, pair := range [][2]string{{tpCLI, tpVSCode}, {tpVSCode, tpCLI}} {
		for _, overwrite := range []bool{false, true} {
			t.Run(pair[0]+"->"+pair[1]+map[bool]string{false: "", true: " overwrite"}[overwrite], func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, pair[0]), tpHarness(t, pair[1])
				tpWriteFile(t, ws, ".github/hooks/logger.sh", "#!/bin/sh\n")

				h := tpHook(t, tpPlan(t, ws, src, tgt, overwrite))

				if h.Action != PlanWriteOverwrite || h.Skip != "" {
					t.Errorf("action %q skip %q, want overwrite with no skip (same-named files are replaced, with or without Overwrite)", h.Action, h.Skip)
				}
				if !reflect.DeepEqual(h.Plan, tgt.hookPlan) {
					t.Errorf("Plan = %+v, want the target variant plan %+v", h.Plan, tgt.hookPlan)
				}
			})
		}
	}
}
