package app_test

// transform_carry_helpers_test.go holds the shared builders of the skill and hook carry tests
// of the whole-deployment transform: a stub catalog with hook bundles whose variants point at
// tool-local files under testdata/hooks, skill and hook seeding of the source deployment, and
// lookups of skill and hook outcomes and manifest entries. Nothing here reads the repository
// Catalog.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	// trHookKey is the hook bundle every harness has a variant for.
	trHookKey = "logger-hooks"
	// trHookVersion is the catalog version of trHookKey.
	trHookVersion = "2.1.0"
	// trLegacyHookKey is a bundle that only the Claude Code variant exists for.
	trLegacyHookKey = "legacy-hooks"
	// trSkillKey is the skill under testdata/transform/skills.
	trSkillKey = "review-skill"
	// trSkillVersion is the mosaic_version the fixture skill carries.
	trSkillVersion = "1.4.0"
	// trManualStepText is the instruction of the registration step the user must do by hand.
	trManualStepText = "Approve the logger hook in the editor settings"
	// trRegistrationFile is the file the performable registration step creates.
	trRegistrationFile = ".opencode/logger-registration.json"
)

// trAbs returns the absolute path of a file below testdata.
func trAbs(t *testing.T, rel string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("abs %s: %v", rel, err)
	}
	return p
}

// trReadTestdata returns the content of a file below testdata.
func trReadTestdata(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(trAbs(t, rel))
	if err != nil {
		t.Fatalf("read testdata %s: %v", rel, err)
	}
	return string(b)
}

// trHookVariant is a supported variant with one file.
func trHookVariant(t *testing.T, harness, source, target string, steps ...domain.RegistrationStep) domain.HookVariant {
	t.Helper()
	return domain.HookVariant{
		HarnessID: harness, Supported: true,
		Files:        []domain.HookFile{{SourcePath: trAbs(t, source), TargetName: target}},
		Registration: steps,
	}
}

// trHookBundles returns the catalog hook bundles: trHookKey with a variant for every harness
// (the OpenCode one with a performable and a manual registration step), and trLegacyHookKey
// with a Claude Code variant only.
func trHookBundles(t *testing.T) []domain.HookBundle {
	t.Helper()
	performable := domain.RegistrationStep{
		ID: "enable-logger-plugin", TargetPath: trRegistrationFile, Performable: true,
		Fragment: "{\"plugin\":[\"logger\"]}\n", Instruction: "Enable the logger plugin",
	}
	manual := domain.RegistrationStep{
		ID: "approve-logger", Performable: false, Instruction: trManualStepText,
		Fragment: "approve logger",
	}
	openVariant := trHookVariant(t, hOpen, "hooks/nested/logger.ts", "logger.ts", performable, manual)
	openVariant.Files = append(openVariant.Files, domain.HookFile{SourcePath: trAbs(t, "hooks/nested/lib/util.ts"), TargetName: "lib/util.ts"})
	return []domain.HookBundle{
		{Key: trHookKey, Version: trHookVersion, Variants: map[string]domain.HookVariant{
			hClaude: trHookVariant(t, hClaude, "hooks/flat/logger.sh", "logger.sh"),
			hOpen:   openVariant,
			hCLI:    trHookVariant(t, hCLI, "hooks/flat/logger.sh", "logger.sh"),
			hVSCode: trHookVariant(t, hVSCode, "hooks/flat/logger.sh", "logger.sh"),
		}},
		{Key: trLegacyHookKey, Version: "1.0.0", Variants: map[string]domain.HookVariant{
			hClaude: trHookVariant(t, hClaude, "hooks/flat-alt/alt-only.sh", "legacy.sh"),
		}},
	}
}

// trCarryCatalog returns a stub catalog that knows the hook bundles.
func trCarryCatalog(t *testing.T) *stubCatalog {
	t.Helper()
	cat := newMinimalCatalog()
	cat.hooks = trHookBundles(t)
	return cat
}

// trCarryRun wires a transform run whose catalog knows the hook bundles.
func trCarryRun(t *testing.T, src, tgt string, script *interactiontest.Stub) *trRun {
	t.Helper()
	cat := trCarryCatalog(t)
	return trNew(t, script, src, tgt, func(d *app.Deps) { d.Catalog = cat })
}

// skillsDir returns the workspace-relative skills folder of a harness.
func (r *trRun) skillsDir(id string) string {
	return r.module(id).Descriptor().Paths.Skills.Project
}

// seedSkill copies the fixture skill into the source harness's skills folder and returns the
// workspace-relative files with their content.
func (r *trRun) seedSkill() map[string]string {
	r.t.Helper()
	files := map[string]string{
		"SKILL.md":          trReadTestdata(r.t, "transform/skills/review-skill/SKILL.md"),
		"refs/checklist.md": trReadTestdata(r.t, "transform/skills/review-skill/refs/checklist.md"),
	}
	out := map[string]string{}
	for rel, content := range files {
		p := r.skillsDir(r.src) + "/" + trSkillKey + "/" + rel
		trWrite(r.t, r.ws, p, content)
		out[rel] = content
	}
	return out
}

// seedHookBundle installs the bundle's source-harness variant into the workspace, the way a
// deploy of the source harness leaves it.
func (r *trRun) seedHookBundle(key string) {
	r.t.Helper()
	for _, b := range trHookBundles(r.t) {
		if b.Key != key {
			continue
		}
		hp, err := r.module(r.src).HookPlan(domain.HookPlanRequest{Bundle: b, Scope: domain.ScopeProject})
		if err != nil || !hp.Supported {
			r.t.Fatalf("source hook plan for %s: %+v, %v", key, hp, err)
		}
		for _, f := range hp.Files {
			data, rerr := os.ReadFile(f.SourcePath)
			if rerr != nil {
				r.t.Fatalf("read %s: %v", f.SourcePath, rerr)
			}
			trWrite(r.t, r.ws, hp.TargetDir+"/"+f.TargetName, string(data))
		}
		return
	}
	r.t.Fatalf("unknown hook bundle %q", key)
}

// hooksDir returns the workspace-relative hooks folder of a harness.
func (r *trRun) hooksDir(id string) string {
	return r.module(id).Descriptor().Paths.Hooks.Project
}

// trOutcome returns the outcome of one kind and key, failing the test when it is absent.
func trOutcome(t *testing.T, res app.TransformHarnessResult, kind domain.ArtifactKind, key string) app.TransformFileOutcome {
	t.Helper()
	var seen []string
	for _, f := range res.Files {
		if f.Kind == kind && f.Key == key {
			return f
		}
		seen = append(seen, string(f.Kind)+":"+f.Key)
	}
	t.Fatalf("no %s outcome for key %q; outcomes: %v", kind, key, seen)
	return app.TransformFileOutcome{}
}

// trEntryByRef returns the manifest entries recorded for a ref.
func trEntryByRef(m domain.Manifest, kind domain.ArtifactKind, key string) []domain.ManifestEntry {
	var out []domain.ManifestEntry
	for _, e := range m.Entries {
		if e.Ref.Kind == kind && e.Ref.Key == key {
			out = append(out, e)
		}
	}
	return out
}

// trReadTodo returns the TODO file the run reported, failing the test when none was written.
func trReadTodo(t *testing.T, res app.TransformHarnessResult) string {
	t.Helper()
	if res.TodoPath == "" {
		t.Fatal("TodoPath is empty, want the TODO file the run wrote")
	}
	b, err := os.ReadFile(res.TodoPath)
	if err != nil {
		t.Fatalf("read TODO file: %v", err)
	}
	return string(b)
}
