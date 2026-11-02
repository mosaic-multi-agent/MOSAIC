package app_test

// transform_carry_edges_test.go specifies the edge outcomes of carrying skills and hooks in a
// whole-deployment transform: a target that cannot hold them is reported as unsupported, a hook
// bundle that already exists in the target is kept unless overwriting, and a write failure while
// carrying a skill or hook reverts every file written by the run.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// trNoSkillsModule is a harness module whose descriptor says skills are not supported.
type trNoSkillsModule struct{ domain.HarnessModule }

func (m trNoSkillsModule) Descriptor() *domain.HarnessDescriptor {
	d := *m.HarnessModule.Descriptor()
	d.Paths.Skills.Supported = false
	return &d
}

// trNoHooksModule is a harness module whose hook plan is unsupported with a reason.
type trNoHooksModule struct{ domain.HarnessModule }

const trNoHooksReason = "hooks are provisioned at runtime by the host"

func (m trNoHooksModule) HookPlan(domain.HookPlanRequest) (domain.HookPlan, error) {
	return domain.HookPlan{Supported: false, Reason: trNoHooksReason}, nil
}

// trWrapTarget replaces the module of a harness in the run's registry.
func trWrapTarget(harnessID string, wrap func(domain.HarnessModule) domain.HarnessModule) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		reg.modules[harnessID] = wrap(reg.modules[harnessID])
	}
}

// trFullDeployment wires a Claude Code to OpenCode run whose source holds agents, a skill and a hook bundle.
func trFullDeployment(t *testing.T) *trRun {
	t.Helper()
	cat := trCarryCatalog(t)
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat })
	r.seed(trAgentFixtures...)
	r.seedSkill()
	r.seedHookBundle(trHookKey)
	return r
}

func TestTransformSkills_TargetWithoutSkillsSupportSkipsTheSkillWithReason(t *testing.T) {
	cat := trCarryCatalog(t)
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat },
		trWrapTarget(hOpen, func(m domain.HarnessModule) domain.HarnessModule { return trNoSkillsModule{m} }))
	r.seedSkill()

	res := r.mustRun(r.request())

	f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey)
	if f.Status != app.StatusSkipped || f.Skip != app.SkipUnsupported || f.Reason == "" {
		t.Errorf("status %q skip %q reason %q, want skipped/unsupported with a reason", f.Status, f.Skip, f.Reason)
	}
	if got := trEntryByRef(trLoadManifestIfAny(t, r.ws), domain.ArtifactSkill, trSkillKey); len(got) != 0 {
		t.Errorf("manifest records %d skill entries for a skill that was not carried", len(got))
	}
}

func TestTransformHooks_TargetWithoutHookSupportSkipsTheBundleWithThePlanReason(t *testing.T) {
	cat := trCarryCatalog(t)
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat },
		trWrapTarget(hOpen, func(m domain.HarnessModule) domain.HarnessModule { return trNoHooksModule{m} }))
	r.seedHookBundle(trHookKey)

	res := r.mustRun(r.request())

	f := trOutcome(t, res, domain.ArtifactHook, trHookKey)
	if f.Status != app.StatusSkipped || f.Skip != app.SkipUnsupported || !strings.Contains(f.Reason, trNoHooksReason) {
		t.Errorf("status %q skip %q reason %q, want skipped/unsupported carrying the plan reason", f.Status, f.Skip, f.Reason)
	}
	if got := trEntryByRef(trLoadManifestIfAny(t, r.ws), domain.ArtifactHook, trHookKey); len(got) != 0 {
		t.Errorf("manifest records %d hook entries for a bundle that was not carried", len(got))
	}
}

func TestTransformHooks_BundleAlreadyInTheTargetIsKeptWithoutOverwrite(t *testing.T) {
	r := trHookRun(t, trHookKey)
	dest := r.hooksDir(hOpen) + "/logger.ts"
	trWrite(t, r.ws, dest, "// locally edited\n")

	res := r.mustRun(r.request())

	if got := r.read(dest); got != "// locally edited\n" {
		t.Error("an existing target hook file was replaced without Overwrite")
	}
	f := trOutcome(t, res, domain.ArtifactHook, trHookKey)
	if f.Status != app.StatusSkipped || f.Skip != app.SkipDestinationExists {
		t.Errorf("status %q skip %q, want skipped/destination-exists", f.Status, f.Skip)
	}
}

func TestTransformHooks_BundleAlreadyInTheTargetIsReplacedWithOverwrite(t *testing.T) {
	r := trHookRun(t, trHookKey)
	dest := r.hooksDir(hOpen) + "/logger.ts"
	trWrite(t, r.ws, dest, "// locally edited\n")
	req := r.request()
	req.Overwrite = true

	r.mustRun(req)

	if got, want := r.read(dest), trReadTestdata(t, "hooks/nested/logger.ts"); got != want {
		t.Error("with Overwrite the hook file is not the target variant's file")
	}
}

func TestTransformSkills_WriteFailureRevertsAgentSkillAndHookWrites(t *testing.T) {
	r := trFullDeployment(t)
	// A non-empty folder where a skill file must be written cannot be overwritten.
	trWrite(t, r.ws, r.skillsDir(hOpen)+"/"+trSkillKey+"/refs/checklist.md/keep.txt", "blocker")
	before := r.snapshot()
	req := r.request()
	req.Overwrite = true

	_, err := r.run(req)

	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("error = %v, want ErrRunReverted", err)
	}
	assertRunLeftNoTrace(t, r, before)
}

func TestTransformHooks_WriteFailureRevertsAgentSkillAndHookWrites(t *testing.T) {
	r := trFullDeployment(t)
	// A non-empty folder where a hook file must be written cannot be overwritten.
	trWrite(t, r.ws, r.hooksDir(hOpen)+"/logger.ts/keep.txt", "blocker")
	before := r.snapshot()
	req := r.request()
	req.Overwrite = true

	_, err := r.run(req)

	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("error = %v, want ErrRunReverted", err)
	}
	assertRunLeftNoTrace(t, r, before)
}

// assertRunLeftNoTrace checks that a reverted run restored the workspace and wrote no manifest
// entry of any kind.
func assertRunLeftNoTrace(t *testing.T, r *trRun, before map[string]string) {
	t.Helper()
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("the reverted run left a trace: %s", diff)
	}
	for _, key := range trAgentFixtures {
		if r.exists(r.dest(key)) {
			t.Errorf("agent %q is still written after the revert", key)
		}
	}
	if r.exists(r.skillsDir(hOpen) + "/" + trSkillKey + "/SKILL.md") {
		t.Error("the skill is still written after the revert")
	}
	if n := len(trLoadManifestIfAny(t, r.ws).Entries); n != 0 {
		t.Errorf("manifest holds %d entries after the revert, want none", n)
	}
}
