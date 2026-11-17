package app

// transform_opencode_pair_hooks_plan_test.go specifies how transform plans hook bundles between
// the two OpenCode targets. Both targets deploy into the shared .opencode/plugins folder, and V2
// drops a V1-form plugin with a warning, so no source-variant file may linger: every source
// variant file (subfolder files included) and performable registration target that the target
// variant does not write is planned for removal, also when the target has no variant. The plans
// come from the bundle variants, so the tests use fixed plans with arbitrary file names.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	tpOCPluginsDir = ".opencode/plugins"
	tpOCRegFile    = ".opencode/opencode.json"
)

// tpOCHooks builds an OpenCode pair whose hook plans write the given files (plugins-folder
// relative) into the shared plugins folder, and places the given files in the workspace.
func tpOCHooks(t *testing.T, c tpOCCase, srcFiles, tgtFiles, placed []string) (string, *tpModule, *tpModule) {
	t.Helper()
	ws := t.TempDir()
	src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
	src.hookPlan = tpHookPlan(tpOCPluginsDir, srcFiles...)
	tgt.hookPlan = tpHookPlan(tpOCPluginsDir, tgtFiles...)
	for _, f := range placed {
		tpWriteFile(t, ws, tpOCPluginsDir+"/"+f, "export default {}\n")
	}
	return ws, src, tgt
}

func tpOCHookRef(path string) string { return "hook:" + tpHookKey + " " + path }

func TestPlanOpenCodePair_HookBundleIsCarriedWithTheTargetVariantEvenWhenSameNamedFilesExist(t *testing.T) {
	for _, c := range tpOCCases {
		for _, overwrite := range []bool{false, true} {
			t.Run(c.name+map[bool]string{false: " plain", true: " overwrite"}[overwrite], func(t *testing.T) {
				ws, src, tgt := tpOCHooks(t, c, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"})

				h := tpHook(t, tpPlan(t, ws, src, tgt, overwrite))

				if h.Key != tpHookKey || h.Action != PlanWriteOverwrite || h.Skip != "" {
					t.Errorf("key %q action %q skip %q, want %s carried as overwrite with no skip: never shared-hooks-folder", h.Key, h.Action, h.Skip, tpHookKey)
				}
				if len(h.Plan.Files) != 1 || h.Plan.Files[0].TargetName != "mosaic-logger.ts" || h.Plan.TargetDir != tpOCPluginsDir {
					t.Errorf("Plan = %+v, want the target variant plan", h.Plan)
				}
			})
		}
	}
}

func TestPlanOpenCodePair_BundleWhoseTargetFilesAreAbsentIsPlannedAsCreateAndOldFilesAreRemoved(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c, []string{"old.ts"}, []string{"new.ts"}, []string{"old.ts"})

			p := tpPlan(t, ws, src, tgt, false)

			if h := tpHook(t, p); h.Action != PlanWriteCreate || h.Skip != "" {
				t.Errorf("action %q skip %q, want create", h.Action, h.Skip)
			}
			if got, want := tpOCRemovals(p), []string{tpOCHookRef(tpOCPluginsDir + "/old.ts")}; !tpEqualStrings(got, want) {
				t.Errorf("Removals = %v, want %v", got, want)
			}
			if got := tpReplacements(p); len(got) != 0 {
				t.Errorf("Replacements = %v, want none: no target file exists yet", got)
			}
		})
	}
}

func TestPlanOpenCodePair_SourceFilesTheTargetDoesNotWriteAreRemovedSubfoldersIncluded(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c,
				[]string{"mosaic-logger.ts", "lib/a.ts", "lib/b.ts", "gone.ts"}, []string{"mosaic-logger.ts"},
				[]string{"mosaic-logger.ts", "lib/a.ts", "lib/b.ts", "user-plugin.ts", "lib/user-helper.ts"})

			p := tpPlan(t, ws, src, tgt, false)

			// lib/a.ts and lib/b.ts exist and the target does not write them; gone.ts is not on
			// disk; user-plugin.ts and lib/user-helper.ts are listed by no variant.
			wantRm := []string{
				tpOCHookRef(tpOCPluginsDir + "/lib/a.ts"),
				tpOCHookRef(tpOCPluginsDir + "/lib/b.ts"),
			}
			if got := tpOCRemovals(p); !tpEqualStrings(got, wantRm) {
				t.Errorf("Removals = %v, want %v", got, wantRm)
			}
			wantRp := []string{tpOCHookRef(tpOCPluginsDir + "/mosaic-logger.ts")}
			if got := tpReplacements(p); !tpEqualStrings(got, wantRp) {
				t.Errorf("Replacements = %v, want %v", got, wantRp)
			}
		})
	}
}

func TestPlanOpenCodePair_TargetVariantFilesThatExistAreReplacedWithoutRemoval(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c,
				[]string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts", "lib/host.ts"},
				[]string{"mosaic-logger.ts", "lib/host.ts"})

			p := tpPlan(t, ws, src, tgt, false)

			want := []string{
				tpOCHookRef(tpOCPluginsDir + "/lib/host.ts"),
				tpOCHookRef(tpOCPluginsDir + "/mosaic-logger.ts"),
			}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want %v", got, want)
			}
			if got := tpOCRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none: the target writes every source file", got)
			}
		})
	}
}

func TestPlanOpenCodePair_RegistrationFileIsReplacedAndSourceOnlyRegistrationIsRemoved(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"})
			src.hookPlan.Registration = []domain.RegistrationStep{
				tpRegistration(tpOCRegFile, true), tpRegistration(".opencode/source-only.json", true)}
			tgt.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpOCRegFile, true)}
			tpWriteFile(t, ws, tpOCRegFile, "{\"edited\":true}\n")
			tpWriteFile(t, ws, ".opencode/source-only.json", "{}\n")

			p := tpPlan(t, ws, src, tgt, false)

			wantRp := []string{tpOCHookRef(tpOCPluginsDir + "/mosaic-logger.ts"), tpOCHookRef(tpOCRegFile)}
			if got := tpReplacements(p); !tpEqualStrings(got, wantRp) {
				t.Errorf("Replacements = %v, want %v", got, wantRp)
			}
			wantRm := []string{tpOCHookRef(".opencode/source-only.json")}
			if got := tpOCRemovals(p); !tpEqualStrings(got, wantRm) {
				t.Errorf("Removals = %v, want %v", got, wantRm)
			}
		})
	}
}

func TestPlanOpenCodePair_BundleWithoutTargetVariantStillRemovesTheSourceFiles(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c,
				[]string{"mosaic-logger.ts", "lib/a.ts"}, nil,
				[]string{"mosaic-logger.ts", "lib/a.ts", "user-plugin.ts"})

			p := tpPlan(t, ws, src, tgt, true)

			if h := tpHook(t, p); h.Action != PlanSkip || h.Skip != SkipNoTargetVariant {
				t.Errorf("action %q skip %q, want skip %q", h.Action, h.Skip, SkipNoTargetVariant)
			}
			want := []string{
				tpOCHookRef(tpOCPluginsDir + "/lib/a.ts"),
				tpOCHookRef(tpOCPluginsDir + "/mosaic-logger.ts"),
			}
			if got := tpOCRemovals(p); !tpEqualStrings(got, want) {
				t.Errorf("Removals = %v, want %v: V2 drops a V1-form plugin, so none may linger", got, want)
			}
			if got := tpReplacements(p); len(got) != 0 {
				t.Errorf("Replacements = %v, want none: the target writes nothing", got)
			}
		})
	}
}

func TestPlanOpenCodePair_BundleWhoseTargetHookPlanIsUnsupportedStillRemovesTheSourceFiles(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"}, []string{"mosaic-logger.ts"})
			tgt.hookPlan = domain.HookPlan{Supported: false, Reason: "no hooks"}

			p := tpPlan(t, ws, src, tgt, true)

			if h := tpHook(t, p); h.Skip != SkipUnsupported {
				t.Errorf("skip %q, want %q", h.Skip, SkipUnsupported)
			}
			want := []string{tpOCHookRef(tpOCPluginsDir + "/mosaic-logger.ts")}
			if got := tpOCRemovals(p); !tpEqualStrings(got, want) {
				t.Errorf("Removals = %v, want %v", got, want)
			}
		})
	}
}

func TestPlanOpenCodePair_EveryRemovalAndReplacementCarriesASingleLineReason(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpOCHooks(t, c,
				[]string{"mosaic-logger.ts", "lib/a.ts"}, []string{"mosaic-logger.ts"},
				[]string{"mosaic-logger.ts", "lib/a.ts"})
			src.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpOCRegFile, true)}
			tgt.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpOCRegFile, true)}
			tpWriteFile(t, ws, tpOCRegFile, "{}\n")
			tpPlaceAgent(t, ws, tpOCAgentsDir, "a.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpOCAgentsDir+"/a.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if len(p.Removals) != 1 || len(p.Replacements) != 3 {
				t.Fatalf("setup: Removals %v / Replacements %v, want 1 and 3", tpOCRemovals(p), tpReplacements(p))
			}
			for _, r := range p.Removals {
				assertSingleLineReason(t, "removal "+r.Path, r.Reason)
			}
			for _, r := range p.Replacements {
				assertSingleLineReason(t, "replacement "+r.Path, r.Reason)
			}
		})
	}
}

func TestPlanTransform_OpenCodeWithAnotherHarnessPlansNoHookRemovalOrReplacement(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpenV2)
	tgt.hookPlan = tpHookPlan(tpOCPluginsDir, "mosaic-logger.ts")
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")
	tpWriteFile(t, ws, tpOCPluginsDir+"/mosaic-logger.ts", "export default {}\n")

	p := tpPlan(t, ws, src, tgt, true)

	if p.Removals != nil || p.Replacements != nil {
		t.Errorf("Removals %v / Replacements %v, want none outside the OpenCode pair", tpOCRemovals(p), tpReplacements(p))
	}
}
