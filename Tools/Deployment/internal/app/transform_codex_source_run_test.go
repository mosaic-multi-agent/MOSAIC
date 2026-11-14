package app_test

// transform_codex_source_run_test.go specifies running a Transform from the Codex harness to a
// Markdown harness: the written agents carry the Codex file's instructions as their body, the
// name, description and mapped model as frontmatter, and the role; the Codex source files are
// never changed. The rebuild of a single Codex source file (role rule, undecodable input) and
// the same-harness rejection are pinned here too. The target tool list is deliberately not
// asserted: a Codex source records none.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTransformFromCodex_Run_WritesMarkdownAgentWithInstructionsAndFrontmatter(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub, role: "subagent"}))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, "code-reviewer")
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	if !r.exists(r.dest("code-reviewer")) {
		t.Fatalf("target %s was not written", r.dest("code-reviewer"))
	}
	out := r.read(r.dest("code-reviewer"))
	if strings.HasPrefix(strings.TrimSpace(out), "name =") || strings.Contains(out, "developer_instructions") {
		t.Errorf("target must be Markdown with the Codex instructions as body, not TOML:\n%s", out)
	}
	doc := cxParse(t, out)
	if !strings.Contains(out, cxBodyText) {
		t.Errorf("target body lacks the Codex developer_instructions text %q:\n%s", cxBodyText, out)
	}
	for key, want := range map[string]string{
		"name":        "code-reviewer",
		"description": "Codex source agent code-reviewer",
		"model":       cxTargetModelSub,
	} {
		if got, ok := cxFrontmatterScalar(doc, key); !ok || got != want {
			t.Errorf("frontmatter %s = %q (present %v), want %q", key, got, ok, want)
		}
	}
	if f.Role != domain.RoleSubagent || f.Model != cxTargetModelSub {
		t.Errorf("outcome role %q model %q, want subagent and %q", f.Role, f.Model, cxTargetModelSub)
	}
}

func TestTransformFromCodex_Run_OrchestratorTomlIsWrittenAsOrchestrator(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator", model: cxSourceModelOrch}))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, "orchestrator")
	if f.Status != app.StatusTransformed || f.Role != domain.RoleOrchestrator {
		t.Fatalf("orchestrator.toml status %q (reason %q) role %q, want transformed orchestrator", f.Status, f.Reason, f.Role)
	}
	out := r.read(r.dest("orchestrator"))
	doc := cxParse(t, out)
	if got, _ := cxFrontmatterScalar(doc, "model"); got != cxTargetModelOrch {
		t.Errorf("model = %q, want %q", got, cxTargetModelOrch)
	}
	if !strings.Contains(out, cxBodyText) {
		t.Errorf("target body lacks the Codex instructions text %q", cxBodyText)
	}
}

func TestTransformFromCodex_Run_EveryTomlAgentIsWrittenAndSourcesStayUnchanged(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator", model: cxSourceModelOrch}))
	cxPut(t, r, "plain.toml", "name = \"plain\"\n")
	before := r.snapshot()

	res := r.mustRun(cxRequest(r))

	for _, key := range []string{"code-reviewer", "orchestrator"} {
		if !r.exists(r.dest(key)) {
			t.Errorf("agent %q was not written to %s", key, r.dest(key))
		}
	}
	if res.Transformed != 2 {
		t.Errorf("Transformed = %d, want 2 (the unstamped plain.toml is not an agent)", res.Transformed)
	}
	after := r.snapshot()
	for path, content := range before {
		if got, ok := after[path]; !ok || got != content {
			t.Errorf("source file %s was changed or deleted by the transform", path)
		}
	}
}

func TestTransformFromCodex_Run_UndecodableSourceDoesNotStopOtherAgents(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "broken.toml", "model = \"a\"\nmodel = \"b\"\n")
	cxPut(t, r, "good.toml", codexSourceAgent(cxOpts{name: "good", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	if !r.exists(r.dest("good")) || trFile(t, res, "good").Status != app.StatusTransformed {
		t.Errorf("good agent not transformed: %+v", res.Files)
	}
	if r.exists(r.dest("broken")) {
		t.Errorf("an undecodable source file must not be written to the target")
	}
}

func TestTransformFromCodexToCodex_IsRejectedAsSameHarness(t *testing.T) {
	r := trNew(t, nil, hCodex, hCodex, withCodexTarget(t))
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))
	before := r.snapshot()

	_, err := r.run(r.request())

	if !errors.Is(err, app.ErrTransformSameHarness) {
		t.Fatalf("error = %v, want ErrTransformSameHarness", err)
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("a rejected transform changed the workspace: %s", diff)
	}
}

// cxRebuildInput builds a rebuild of a Codex source file into the real Claude Code module.
func cxRebuildInput(t *testing.T, fileName, content string) app.RebuildInput {
	t.Helper()
	return app.RebuildInput{
		Source:       []byte(content),
		FileName:     fileName,
		AgentKey:     agentkey.FromFileNameExt(fileName, ".toml"),
		SourceModule: codexAppModule(t),
		TargetModule: rbModule(t, hClaude),
		Model:        domain.ModelSelection{ModelID: rbModelID, Origin: domain.OriginHarnessList},
		Timestamp:    "2026-10-31T12:00:00Z",
		Op:           agentformat.OpCreate,
	}
}

func TestRebuildFromCodex_BuildsMarkdownFromDecodedSource(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", role: "subagent"}))

	res, out := rbRebuild(t, in)

	if !strings.Contains(out, cxBodyText) || strings.Contains(out, "developer_instructions") {
		t.Errorf("rebuilt output must carry the decoded instructions as Markdown body:\n%s", out)
	}
	if got, ok := rbScalar(t, out, "name"); !ok || got != "code-reviewer" {
		t.Errorf("name = %q (present %v), want code-reviewer", got, ok)
	}
	if got, _ := rbScalar(t, out, "model"); got != rbModelID {
		t.Errorf("model = %q, want %q", got, rbModelID)
	}
	if res.Role != domain.RoleSubagent {
		t.Errorf("role = %q, want subagent", res.Role)
	}
}

func TestRebuildFromCodex_OrchestratorTomlFileNameInfersOrchestratorRole(t *testing.T) {
	content := codexSourceAgent(cxOpts{name: "orchestrator"})

	cases := map[string]domain.AgentRole{
		"orchestrator.toml": domain.RoleOrchestrator,
		"helper.toml":       domain.RoleSubagent,
	}
	for fileName, want := range cases {
		t.Run(fileName, func(t *testing.T) {
			in := cxRebuildInput(t, fileName, content)

			res, _ := rbRebuild(t, in)
			insp, err := app.InspectDeployedAgent(in)

			if res.Role != want {
				t.Errorf("RebuildDeployedAgent role = %q, want %q", res.Role, want)
			}
			if err != nil || insp.Role != want {
				t.Errorf("InspectDeployedAgent role = %q (err %v), want %q", insp.Role, err, want)
			}
		})
	}
}

func TestRebuildFromCodex_UndecodableSourceFailsAsMalformedDeployed(t *testing.T) {
	in := cxRebuildInput(t, "broken.toml", "model = \"a\"\nmodel = \"b\"\n")

	_, rerr := app.RebuildDeployedAgent(in)
	_, ierr := app.InspectDeployedAgent(in)

	if !errors.Is(rerr, agentformat.ErrMalformedDeployed) {
		t.Errorf("RebuildDeployedAgent error = %v, want ErrMalformedDeployed", rerr)
	}
	if !errors.Is(ierr, agentformat.ErrMalformedDeployed) {
		t.Errorf("InspectDeployedAgent error = %v, want ErrMalformedDeployed", ierr)
	}
}

// The skill folder of a Codex-source run is Markdown like for every harness: its version is
// still read from SKILL.md.
func TestTransformFromCodex_Run_SkillVersionStillReadFromMarkdownSkillFile(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))
	skillDir := codexAppModule(t).Descriptor().Paths.Skills.Project + "/review-skill"
	src := filepath.Join("testdata", "transform", "skills", "review-skill", "SKILL.md")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read skill fixture: %v", err)
	}
	trWrite(t, r.ws, skillDir+"/SKILL.md", string(b))

	res := r.mustRun(cxRequest(r))

	if f := trFile(t, res, "code-reviewer"); f.Status != app.StatusTransformed {
		t.Errorf("agent status %q (reason %q), want transformed alongside a Markdown skill", f.Status, f.Reason)
	}
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactSkill && f.Status == app.StatusFailed {
			t.Errorf("skill %q failed: %s", f.Key, f.Reason)
		}
	}
}
