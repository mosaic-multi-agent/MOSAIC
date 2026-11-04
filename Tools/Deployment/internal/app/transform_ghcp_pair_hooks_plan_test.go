package app

// transform_ghcp_pair_hooks_plan_test.go specifies how transform plans hook bundles between
// the two GHCP targets: every source bundle is carried with the target's variant (same-named
// files notwithstanding), the source variant's files that the target does not write are planned
// for removal, and the shared registration file is planned for replacement.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	tpHooksDir = ".github/hooks"
	tpRegFile  = ".github/hooks/mosaic-logger.json"
)

// tpPairHooks builds a GHCP pair whose hook plans write the given files into the shared hooks
// folder, and places the given files (hooks-folder relative) in the workspace.
func tpPairHooks(t *testing.T, c tpPairCase, srcFiles, tgtFiles, placed []string) (string, *tpModule, *tpModule) {
	t.Helper()
	ws := t.TempDir()
	src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
	src.hookPlan = tpHookPlan(tpHooksDir, srcFiles...)
	tgt.hookPlan = tpHookPlan(tpHooksDir, tgtFiles...)
	for _, f := range placed {
		tpWriteFile(t, ws, tpHooksDir+"/"+f, "#!/bin/sh\n")
	}
	return ws, src, tgt
}

func tpRegistration(path string, performable bool) domain.RegistrationStep {
	return domain.RegistrationStep{
		ID: "register-logger", TargetPath: path, Performable: performable,
		Fragment: "{\"hooks\":[]}\n", Instruction: "register the logger",
	}
}

func TestPlanGhcpPair_HookBundleIsCarriedWithTheTargetVariantEvenWhenSameNamedFilesExist(t *testing.T) {
	for _, c := range tpPairCases {
		for _, overwrite := range []bool{false, true} {
			t.Run(c.name+map[bool]string{false: " plain", true: " overwrite"}[overwrite], func(t *testing.T) {
				ws, src, tgt := tpPairHooks(t, c, []string{"logger.sh"}, []string{"logger.sh"}, []string{"logger.sh"})

				h := tpHook(t, tpPlan(t, ws, src, tgt, overwrite))

				if h.Key != tpHookKey || h.Action != PlanWriteOverwrite || h.Skip != "" {
					t.Errorf("key %q action %q skip %q, want %s carried as overwrite with no skip", h.Key, h.Action, h.Skip, tpHookKey)
				}
				if len(h.Plan.Files) != 1 || h.Plan.Files[0].TargetName != "logger.sh" || h.Plan.TargetDir != tpHooksDir {
					t.Errorf("Plan = %+v, want the target variant plan", h.Plan)
				}
			})
		}
	}
}

func TestPlanGhcpPair_BundleWhoseTargetFilesAreAbsentIsPlannedAsCreate(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpPairHooks(t, c, []string{"old.sh"}, []string{"new.sh"}, []string{"old.sh"})

			p := tpPlan(t, ws, src, tgt, false)

			if h := tpHook(t, p); h.Action != PlanWriteCreate || h.Skip != "" {
				t.Errorf("action %q skip %q, want create", h.Action, h.Skip)
			}
			if got, want := tpRemovals(p), []string{"hook:" + tpHookKey + " " + tpHooksDir + "/old.sh drop=false"}; !tpEqualStrings(got, want) {
				t.Errorf("Removals = %v, want %v", got, want)
			}
			if got := tpReplacements(p); len(got) != 0 {
				t.Errorf("Replacements = %v, want none: no target file exists yet", got)
			}
		})
	}
}

func TestPlanGhcpPair_OnlySourceVariantFilesTheTargetDoesNotWriteAreRemoved(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpPairHooks(t, c,
				[]string{"logger.sh", "extra.sh", "gone.sh"}, []string{"logger.sh"},
				[]string{"logger.sh", "extra.sh", "unrelated.txt"})

			p := tpPlan(t, ws, src, tgt, false)

			// extra.sh exists and the target does not write it; gone.sh is not on disk;
			// unrelated.txt is not part of any variant.
			wantRm := []string{"hook:" + tpHookKey + " " + tpHooksDir + "/extra.sh drop=false"}
			if got := tpRemovals(p); !tpEqualStrings(got, wantRm) {
				t.Errorf("Removals = %v, want %v", got, wantRm)
			}
			wantRp := []string{"hook:" + tpHookKey + " " + tpHooksDir + "/logger.sh"}
			if got := tpReplacements(p); !tpEqualStrings(got, wantRp) {
				t.Errorf("Replacements = %v, want %v", got, wantRp)
			}
		})
	}
}

func TestPlanGhcpPair_RegistrationFileIsReplacedEvenWhenTheUserEditedIt(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws, src, tgt := tpPairHooks(t, c, []string{"logger.sh"}, []string{"logger.sh", "a.sh"}, []string{"logger.sh", "a.sh"})
			src.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpRegFile, true)}
			tgt.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpRegFile, true)}
			tpWriteFile(t, ws, tpRegFile, "{\"edited\":true}\n")

			p := tpPlan(t, ws, src, tgt, false)

			// Hook files by name first, then the bundle's registration targets.
			want := []string{
				"hook:" + tpHookKey + " " + tpHooksDir + "/a.sh",
				"hook:" + tpHookKey + " " + tpHooksDir + "/logger.sh",
				"hook:" + tpHookKey + " " + tpRegFile,
			}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want %v", got, want)
			}
			if got := tpRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none: the target writes the registration file", got)
			}
		})
	}
}

func TestPlanGhcpPair_RegistrationTargetIsReplacedOnlyWhenItExistsAndTheStepIsPerformable(t *testing.T) {
	cases := []struct {
		name       string
		performed  bool
		fileExists bool
	}{
		{"missing file", true, false},
		{"manual step", false, true},
	}
	for _, k := range cases {
		t.Run(k.name, func(t *testing.T) {
			ws, src, tgt := tpPairHooks(t, tpPairCases[0], []string{"logger.sh"}, []string{"logger.sh"}, []string{"logger.sh"})
			tgt.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(tpRegFile, k.performed)}
			if k.fileExists {
				tpWriteFile(t, ws, tpRegFile, "{}\n")
			}

			p := tpPlan(t, ws, src, tgt, false)

			want := []string{"hook:" + tpHookKey + " " + tpHooksDir + "/logger.sh"}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want only the hook file %v", got, want)
			}
		})
	}
}

func TestPlanGhcpPair_SourceRegistrationTargetTheTargetDoesNotWriteIsRemoved(t *testing.T) {
	ws, src, tgt := tpPairHooks(t, tpPairCases[0], []string{"logger.sh"}, []string{"logger.sh"}, []string{"logger.sh"})
	src.hookPlan.Registration = []domain.RegistrationStep{tpRegistration(".github/hooks/old-registration.json", true)}
	tpWriteFile(t, ws, ".github/hooks/old-registration.json", "{}\n")

	p := tpPlan(t, ws, src, tgt, false)

	want := []string{"hook:" + tpHookKey + " .github/hooks/old-registration.json drop=false"}
	if got := tpRemovals(p); !tpEqualStrings(got, want) {
		t.Errorf("Removals = %v, want %v", got, want)
	}
}

func TestPlanGhcpPair_BundleNotCarriedPlansNoRemovalOrReplacement(t *testing.T) {
	t.Run("target has no variant", func(t *testing.T) {
		ws, src, tgt := tpPairHooks(t, tpPairCases[0], []string{"logger.sh", "extra.sh"}, nil, []string{"logger.sh", "extra.sh"})

		p := tpPlan(t, ws, src, tgt, true)

		if h := tpHook(t, p); h.Action != PlanSkip || h.Skip != SkipNoTargetVariant {
			t.Errorf("action %q skip %q, want skip %q", h.Action, h.Skip, SkipNoTargetVariant)
		}
		if len(p.Removals) != 0 || len(p.Replacements) != 0 {
			t.Errorf("Removals %v / Replacements %v, want none for a bundle that is not carried", tpRemovals(p), tpReplacements(p))
		}
	})
	t.Run("target does not support hooks", func(t *testing.T) {
		ws, src, tgt := tpPairHooks(t, tpPairCases[0], []string{"logger.sh"}, []string{"logger.sh"}, []string{"logger.sh"})
		tgt.hookPlan = domain.HookPlan{Supported: false, Reason: "no hooks"}

		p := tpPlan(t, ws, src, tgt, true)

		if h := tpHook(t, p); h.Skip != SkipUnsupported {
			t.Errorf("skip %q, want %q", h.Skip, SkipUnsupported)
		}
		if len(p.Removals) != 0 || len(p.Replacements) != 0 {
			t.Errorf("Removals %v / Replacements %v, want none", tpRemovals(p), tpReplacements(p))
		}
	})
}

func TestPlanTransform_OtherPairsPlanNoHookRemovalOrReplacement(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tgt.hookPlan = tpHookPlan(".opencode/plugins", "logger.ts")
	tpWriteFile(t, ws, ".claude/hooks/logger.sh", "#!/bin/sh\n")
	tpWriteFile(t, ws, ".opencode/plugins/logger.ts", "export {}\n")

	p := tpPlan(t, ws, src, tgt, true)

	if p.Removals != nil || p.Replacements != nil {
		t.Errorf("Removals %v / Replacements %v, want none outside the GHCP pair", tpRemovals(p), tpReplacements(p))
	}
}
