package app_test

// transform_ghcp_pair_result_hooks_test.go specifies that the run result and the JSON document
// of a transform between the two GHCP targets list the hook-file removals and the replaced
// registration file, in addition to the agent removals, with agents listed before hooks.

import (
	"encoding/json"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	// trPairHookKey is a bundle whose source variant writes a file the target variant does not.
	trPairHookKey = "pair-hooks"
	// trPairSourceOnlyFile is the hook file only the source variant writes.
	trPairSourceOnlyFile = "source-only.sh"
	// trPairRegistrationFile is the file the performable registration step of the target variant creates.
	trPairRegistrationFile = ".github/hooks/pair-registration.json"
)

// trPairHookBundle is a bundle whose variants for both GHCP targets write logger.sh, whose source
// variant writes one more file, and whose target variant registers through trPairRegistrationFile.
func trPairHookBundle(t *testing.T, src, tgt string) domain.HookBundle {
	t.Helper()
	step := domain.RegistrationStep{
		ID: "register-pair", TargetPath: trPairRegistrationFile, Performable: true,
		Fragment: "{\"hooks\":[\"logger\"]}\n", Instruction: "Register the logger hook",
	}
	srcVariant := trHookVariant(t, src, "hooks/flat/logger.sh", "logger.sh")
	srcVariant.Files = append(srcVariant.Files,
		domain.HookFile{SourcePath: trAbs(t, "hooks/flat-alt/alt-only.sh"), TargetName: trPairSourceOnlyFile})
	return domain.HookBundle{Key: trPairHookKey, Version: trHookVersion, Variants: map[string]domain.HookVariant{
		src: srcVariant,
		tgt: trHookVariant(t, tgt, "hooks/flat/logger.sh", "logger.sh", step),
	}}
}

// trPairHookRun is a pair run whose workspace holds a recorded source agent, the source hook
// files (one of them absent from the target variant) and an existing registration file.
func trPairHookRun(t *testing.T, src, tgt string) (*trRun, string) {
	t.Helper()
	return trPairHookRunWithAgent(t, src, tgt, trPairInPlaceAgent)
}

// trPairHookRunWithAgent is trPairHookRun with the recorded source agent at agentRel.
func trPairHookRunWithAgent(t *testing.T, src, tgt, agentRel string, opts ...func(*app.Deps)) (*trRun, string) {
	t.Helper()
	cat := newMinimalCatalog()
	cat.hooks = []domain.HookBundle{trPairHookBundle(t, src, tgt)}
	all := append([]func(*app.Deps){func(d *app.Deps) { d.Catalog = cat }}, opts...)
	r := trNew(t, nil, src, tgt, all...)
	rel := agentRel
	trWrite(t, r.ws, rel, trFixture(t, "subagent"))
	trSaveManifest(t, r.ws, src, rel)
	trWrite(t, r.ws, r.hooksDir(src)+"/logger.sh", trReadTestdata(t, "hooks/flat/logger.sh"))
	trWrite(t, r.ws, r.hooksDir(src)+"/"+trPairSourceOnlyFile, trReadTestdata(t, "hooks/flat-alt/alt-only.sh"))
	trWrite(t, r.ws, trPairRegistrationFile, "{\"hooks\":[\"edited by the user\"]}\n")
	return r, rel
}

func TestTransformGhcpPair_DryRunListsSourceOnlyHookFileRemovalAndRegistrationReplacement(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, _ := trPairHookRun(t, pair[0], pair[1])
			before := r.snapshot()
			req := r.request()
			req.DryRun = true

			res := r.mustRun(req)

			if diff := trSameTree(before, r.snapshot()); diff != "" {
				t.Errorf("dry run changed the workspace: %s", diff)
			}
			rm := trChangeAt(t, res.Removals, r.hooksDir(pair[0])+"/"+trPairSourceOnlyFile)
			if rm.Kind != domain.ArtifactHook || rm.Key != trPairHookKey {
				t.Errorf("hook removal = %+v, want kind hook, key %s", rm, trPairHookKey)
			}
			rp := trChangeAt(t, res.Replacements, trPairRegistrationFile)
			if rp.Kind != domain.ArtifactHook || rp.Key != trPairHookKey {
				t.Errorf("registration replacement = %+v, want kind hook, key %s", rp, trPairHookKey)
			}
			trChangeAt(t, res.Replacements, r.hooksDir(pair[1])+"/logger.sh")
		})
	}
}

func TestTransformGhcpPair_RemovalsListLegacyAgentsBeforeHookFiles(t *testing.T) {
	r, legacyRel := trPairHookRunWithAgent(t, hCLI, hVSCode, trPairLegacyAgent)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if len(res.Removals) != 2 {
		t.Fatalf("Removals = %+v, want the legacy agent and the source-only hook file", res.Removals)
	}
	if res.Removals[0].Kind != domain.ArtifactAgent || res.Removals[0].Path != legacyRel {
		t.Errorf("first removal = %+v, want the agent %s", res.Removals[0], legacyRel)
	}
	if res.Removals[1].Kind != domain.ArtifactHook {
		t.Errorf("second removal = %+v, want the hook file", res.Removals[1])
	}
}

func TestTransformGhcpPair_ReplacementsListInPlaceAgentsBeforeHookFilesAndRegistration(t *testing.T) {
	r, agentRel := trPairHookRun(t, hVSCode, hCLI)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if len(res.Replacements) != 3 {
		t.Fatalf("Replacements = %+v, want the agent, the hook file and the registration file", res.Replacements)
	}
	if got := res.Replacements[0]; got.Kind != domain.ArtifactAgent || got.Path != agentRel {
		t.Errorf("first replacement = %+v, want the agent %s", got, agentRel)
	}
	if got := res.Replacements[2]; got.Path != trPairRegistrationFile {
		t.Errorf("last replacement = %+v, want the registration file %s", got, trPairRegistrationFile)
	}
	if len(res.Removals) != 1 || res.Removals[0].Kind != domain.ArtifactHook {
		t.Errorf("Removals = %+v, want only the source-only hook file", res.Removals)
	}
}

func TestTransformGhcpPair_JSONListsHookFileRemovalAndRegistrationReplacement(t *testing.T) {
	r, _ := trPairHookRun(t, hVSCode, hCLI)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Removals     []map[string]any `json:"removals"`
		Replacements []map[string]any `json:"replacements"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !jsonListHas(doc.Removals, "hook", r.hooksDir(hVSCode)+"/"+trPairSourceOnlyFile) {
		t.Errorf("JSON removals = %v, want the source-only hook file", doc.Removals)
	}
	if !jsonListHas(doc.Replacements, "hook", trPairRegistrationFile) {
		t.Errorf("JSON replacements = %v, want the registration file", doc.Replacements)
	}
}

// jsonListHas reports whether a decoded change list holds an entry of the kind and path.
func jsonListHas(list []map[string]any, kind, path string) bool {
	for _, e := range list {
		if e["kind"] == kind && e["path"] == path {
			return true
		}
	}
	return false
}
