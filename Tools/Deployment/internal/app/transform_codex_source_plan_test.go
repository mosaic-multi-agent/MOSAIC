package app_test

// transform_codex_source_plan_test.go specifies what Transform finds and reads when the SOURCE
// harness is Codex: agent files are the .toml files of the Codex agents folder, keys come from
// the file name without the extension, the orchestrator is recognised by orchestrator.toml,
// files that are not MOSAIC agents (other extensions, unstamped or undecodable TOML) are not
// planned as agents, and the source model is read from the decoded file. Runs are dry runs:
// the plan outcome is read from the result and nothing is written.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// cxOutcomeBySource returns the agent outcome whose source file has the given base name.
func cxOutcomeBySource(res app.TransformHarnessResult, base string) (app.TransformFileOutcome, bool) {
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactAgent && filepath.Base(filepath.ToSlash(f.SourcePath)) == base {
			return f, true
		}
	}
	return app.TransformFileOutcome{}, false
}

func TestTransformFromCodex_Plan_ListsTomlAgentsWithKeysAndRoles(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub, role: "subagent"}))
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator", model: cxSourceModelOrch}))
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	sub := trFile(t, res, "code-reviewer")
	if sub.Status != app.StatusTransformed || sub.Role != domain.RoleSubagent {
		t.Errorf("code-reviewer status %q (reason %q) role %q, want transformed subagent", sub.Status, sub.Reason, sub.Role)
	}
	if base := filepath.Base(filepath.ToSlash(sub.SourcePath)); base != "code-reviewer.toml" {
		t.Errorf("code-reviewer source file = %q, want code-reviewer.toml", base)
	}
	if got := filepath.ToSlash(sub.DestinationPath); got != r.dest("code-reviewer") {
		t.Errorf("code-reviewer destination = %q, want %q", got, r.dest("code-reviewer"))
	}
	orch := trFile(t, res, "orchestrator")
	if orch.Status != app.StatusTransformed || orch.Role != domain.RoleOrchestrator {
		t.Errorf("orchestrator.toml status %q (reason %q) role %q, want transformed orchestrator (file-name rule, no role stamp)",
			orch.Status, orch.Reason, orch.Role)
	}
}

func TestTransformFromCodex_Plan_OnlyOrchestratorTomlIsRecognisedAsOrchestratorByName(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator"}))
	cxPut(t, r, "helper.toml", codexSourceAgent(cxOpts{name: "helper"}))
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	if got := trFile(t, res, "helper").Role; got != domain.RoleSubagent {
		t.Errorf("helper.toml role = %q, want subagent", got)
	}
	if got := trFile(t, res, "orchestrator").Role; got != domain.RoleOrchestrator {
		t.Errorf("orchestrator.toml role = %q, want orchestrator", got)
	}
}

func TestTransformFromCodex_Plan_FilesThatAreNotTomlAgentsAreNotListedAsAgents(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))
	// A Markdown file is not an agent file of a Codex folder, even when it is a stamped agent.
	cxPut(t, r, "markdown-agent.md", trFixture(t, "subagent"))
	cxPut(t, r, "readme.txt", "notes\n")
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	for _, name := range []string{"markdown-agent.md", "readme.txt"} {
		if f, found := cxOutcomeBySource(res, name); found {
			t.Errorf("%s was planned as an agent outcome %+v, want it left out", name, f)
		}
	}
	if keys := trAgentKeys(res); len(keys) != 1 || keys[0] != "code-reviewer" {
		t.Errorf("agent keys = %v, want only code-reviewer", keys)
	}
}

func TestTransformFromCodex_Plan_TomlWithoutMosaicStampIsSkippedAsNotAnAgent(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "plain.toml", "name = \"plain\"\nsandbox_mode = \"read-only\"\ndeveloper_instructions = \"Plain notes.\\n\"\n")
	req := cxRequest(r)

	res := r.mustRun(req)

	f, found := cxOutcomeBySource(res, "plain.toml")
	if !found {
		t.Fatalf("no outcome for plain.toml; agent keys %v", trAgentKeys(res))
	}
	if f.Status != app.StatusSkipped || f.Skip != app.SkipNotAgent || f.Reason == "" {
		t.Errorf("plain.toml status %q skip %q reason %q, want skipped/not-agent with a reason", f.Status, f.Skip, f.Reason)
	}
	if r.exists(r.dest("plain")) {
		t.Errorf("plain.toml is not an agent but was written to the target")
	}
}

func TestTransformFromCodex_Plan_UndecodableTomlIsSkippedWithDecodeReason(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "good.toml", codexSourceAgent(cxOpts{name: "good", model: cxSourceModelSub}))
	cxPut(t, r, "broken.toml", "model = \"a\"\nmodel = \"duplicate-key\"\n")
	req := cxRequest(r)

	res := r.mustRun(req)

	f, found := cxOutcomeBySource(res, "broken.toml")
	if !found {
		t.Fatalf("no outcome for broken.toml; agent keys %v", trAgentKeys(res))
	}
	if f.Status != app.StatusSkipped || f.Skip != app.SkipNotAgent {
		t.Errorf("broken.toml status %q skip %q, want skipped/not-agent", f.Status, f.Skip)
	}
	if !strings.Contains(f.Reason, "decode") {
		t.Errorf("reason %q does not say the file could not be decoded", f.Reason)
	}
	if g := trFile(t, res, "good"); g.Status != app.StatusTransformed {
		t.Errorf("good.toml status %q (reason %q), want transformed: a broken sibling must not stop the run", g.Status, g.Reason)
	}
}

func TestTransformFromCodex_Plan_SourceModelIsReadFromDecodedContent(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator", model: cxSourceModelOrch}))
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	if got := trFile(t, res, "code-reviewer").Model; got != cxTargetModelSub {
		t.Errorf("code-reviewer model = %q, want %q (source model %q mapped)", got, cxTargetModelSub, cxSourceModelSub)
	}
	if got := trFile(t, res, "orchestrator").Model; got != cxTargetModelOrch {
		t.Errorf("orchestrator model = %q, want %q (source model %q mapped)", got, cxTargetModelOrch, cxSourceModelOrch)
	}
	if got := res.AppliedModelMap[cxSourceModelSub]; got != cxTargetModelSub {
		t.Errorf("AppliedModelMap = %v, want source model %q resolved to %q", res.AppliedModelMap, cxSourceModelSub, cxTargetModelSub)
	}
}

func TestTransformFromCodex_Plan_SourceModelGroupsAreAskedOncePerDistinctModel(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "a.toml", codexSourceAgent(cxOpts{name: "a", model: cxSourceModelSub}))
	cxPut(t, r, "b.toml", codexSourceAgent(cxOpts{name: "b", model: cxSourceModelSub}))
	cxPut(t, r, "c.toml", codexSourceAgent(cxOpts{name: "c", model: cxSourceModelOrch}))
	req := r.request()
	req.DryRun = true

	r.mustRun(req)

	subjects := r.spy.modelSubjects()
	if len(subjects) != 2 {
		t.Fatalf("model questions asked for %q, want one per distinct source model (%q, %q)", subjects, cxSourceModelSub, cxSourceModelOrch)
	}
	for _, want := range []string{cxSourceModelSub, cxSourceModelOrch} {
		if !containsString(subjects, want) {
			t.Errorf("no model question for source model %q; asked for %q", want, subjects)
		}
	}
}

func TestReadSourceModel_CodexSource_ReadsModelFromDecodedToml(t *testing.T) {
	desc := *codexAppModule(t).Descriptor()
	content := []byte(codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))

	got := app.ReadSourceModel(content, desc)

	if got != cxSourceModelSub {
		t.Errorf("ReadSourceModel = %q, want %q", got, cxSourceModelSub)
	}
}

func TestReadSourceModel_CodexSource_NoModelKeyOrUndecodableYieldsUnset(t *testing.T) {
	desc := *codexAppModule(t).Descriptor()
	cases := map[string][]byte{
		"no model key": []byte(codexSourceAgent(cxOpts{name: "code-reviewer"})),
		"undecodable":  []byte("model = \"a\"\nmodel = \"b\"\n"),
		"empty":        nil,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if got := app.ReadSourceModel(content, desc); got != app.UnsetSourceModel {
				t.Errorf("ReadSourceModel = %q, want unset", got)
			}
		})
	}
}
