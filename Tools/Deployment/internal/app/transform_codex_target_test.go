package app_test

// transform_codex_target_test.go specifies whole-deployment Transform from a Markdown harness
// to the Codex harness: the run writes .codex/agents/<key>.toml, the written files decode to
// the expected agent with per-region versions on harness region tags, the run result reports
// tools and dropped fields, and an existing destination is overwritten without an operation
// error while its user-owned TOML keys are preserved.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const hCodex = "codex"

// withCodexTarget registers the real Codex module in the run's registry.
func withCodexTarget(t *testing.T) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		m := codexAppModule(t)
		reg.list = append(reg.list, m.Ref())
		reg.modules[hCodex] = m
	}
}

// newCodexTransform wires a Claude Code to Codex run seeded with the fully stamped fixture
// as the deployed agent code-reviewer.
func newCodexTransform(t *testing.T) *trRun {
	t.Helper()
	r := trNew(t, nil, hClaude, hCodex, withCodexTarget(t))
	r.put(hClaude, "code-reviewer", string(rbFixture(t, "claude-code-full.md")))
	return r
}

// codexRequest is the pre-answered request of a Codex-target run.
func codexRequest(r *trRun) app.TransformHarnessRequest {
	req := r.request()
	req.TargetModel = codexAppModel.ModelID
	return req
}

func TestTransformToCodex_Create_WritesTomlAtKeyToml(t *testing.T) {
	r := newCodexTransform(t)

	res := r.mustRun(codexRequest(r))

	f := trFile(t, res, "code-reviewer")
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	want := ".codex/agents/code-reviewer.toml"
	if got := filepath.ToSlash(f.DestinationPath); got != want || !r.exists(want) {
		t.Fatalf("destination = %q (exists %v), want %s written", got, r.exists(want), want)
	}
	out := []byte(r.read(want))
	if got := codexInstructions(t, out); !strings.Contains(got, "BODY-IDENTITY-TEXT") {
		t.Errorf("developer_instructions lacks the source body:\n%s", got)
	}
}

func TestTransformToCodex_Create_HarnessRegionTagCarriesVersion(t *testing.T) {
	r := newCodexTransform(t)

	r.mustRun(codexRequest(r))

	doc, err := docformat.Parse(decodeCodexOutput(t, []byte(r.read(".codex/agents/code-reviewer.toml"))))
	if err != nil {
		t.Fatalf("parse decoded canonical: %v", err)
	}
	region, ok := doc.Body().Deployed(docformat.HarnessRegionName)
	if !ok {
		t.Fatal("HarnessConstraints region absent from the written Codex file")
	}
	want := codexAppModule(t).Descriptor().InjectionsVersion
	if want == "" || region.Version() != want {
		t.Errorf("HarnessConstraints version = %q, want the Codex module's %q", region.Version(), want)
	}
	if strings.Contains(string(region.Content()), "SOURCE-HARNESS-CONSTRAINT-TEXT") {
		t.Errorf("the source harness's constraint text must be replaced by the Codex content: %q", region.Content())
	}
}

func TestTransformToCodex_Create_ResultReportsToolsAndDroppedFields(t *testing.T) {
	r := newCodexTransform(t)

	res := r.mustRun(codexRequest(r))

	f := trFile(t, res, "code-reviewer")
	if len(f.Tools) != 0 {
		t.Errorf("Tools = %v, want none for a target without a tools key", f.Tools)
	}
	if !containsString(f.DroppedFields, "tools") {
		t.Errorf("DroppedFields = %v, want tools (not carried by a Codex file)", f.DroppedFields)
	}
	if f.Role != domain.RoleSubagent || f.Model != codexAppModel.ModelID {
		t.Errorf("role %q model %q, want subagent and %q", f.Role, f.Model, codexAppModel.ModelID)
	}
}

func TestTransformToCodex_Overwrite_UpdatesExistingDestinationKeepingUserKey(t *testing.T) {
	r := newCodexTransform(t)
	dest := ".codex/agents/code-reviewer.toml"
	trWrite(t, r.ws, dest, string(existingCodexDestination()))
	req := codexRequest(r)
	req.Overwrite = true

	res := r.mustRun(req)

	f := trFile(t, res, "code-reviewer")
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	out := []byte(r.read(dest))
	if got := codexInstructions(t, out); !strings.Contains(got, "BODY-IDENTITY-TEXT") || strings.Contains(got, codexOldInstructions) {
		t.Errorf("the overwrite must carry the source instructions:\n%s", got)
	}
	if got := parseTomlMap(t, out)[codexUserKey]; got != codexUserValue {
		t.Errorf("user-owned key %q = %v, want %q preserved\n%s", codexUserKey, got, codexUserValue, out)
	}
}

func TestTransformToCodex_Overwrite_UndecodableDestinationFailsThatAgentOnly(t *testing.T) {
	r := newCodexTransform(t)
	r.put(hClaude, "second-reviewer", string(rbFixture(t, "claude-code-full.md")))
	dest := ".codex/agents/code-reviewer.toml"
	garbage := "this is = = not [valid toml\n"
	trWrite(t, r.ws, dest, garbage)
	req := codexRequest(r)
	req.Overwrite = true

	res := r.mustRun(req)

	f := trFile(t, res, "code-reviewer")
	if f.Status != app.StatusFailed {
		t.Fatalf("status = %q, want failed for an undecodable Codex destination", f.Status)
	}
	if !strings.Contains(f.Reason, "code-reviewer.toml") {
		t.Errorf("reason %q does not name the destination", f.Reason)
	}
	if got := r.read(dest); got != garbage {
		t.Errorf("the undecodable destination must stay unchanged:\n%s", got)
	}
	if o := trFile(t, res, "second-reviewer"); o.Status != app.StatusTransformed {
		t.Errorf("other agent status = %q (reason %q), want transformed", o.Status, o.Reason)
	}
}

func TestTransformToCodex_Overwrite_DestinationDecodeGapReachesTodoFile(t *testing.T) {
	r := newCodexTransform(t)
	// A destination without developer_instructions decodes with a missing-instructions gap.
	trWrite(t, r.ws, ".codex/agents/code-reviewer.toml", "sandbox_mode = \"read-only\"\n")
	req := codexRequest(r)
	req.Overwrite = true

	res := r.mustRun(req)

	if f := trFile(t, res, "code-reviewer"); f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	if todo := trReadTodo(t, res); !strings.Contains(todo, "developer_instructions") {
		t.Errorf("TODO file does not carry the destination's decode gap:\n%s", todo)
	}
}

func TestTransformToCodex_DryRun_ReportsFieldsAndWritesNothing(t *testing.T) {
	r := newCodexTransform(t)
	before := r.snapshot()
	req := codexRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	f := trFile(t, res, "code-reviewer")
	if !containsString(f.DroppedFields, "tools") || f.Model != codexAppModel.ModelID {
		t.Errorf("dry run outcome dropped %v model %q, want tools dropped and model %q", f.DroppedFields, f.Model, codexAppModel.ModelID)
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run changed the workspace: %s", diff)
	}
}

func TestTransformToCodex_ExistingDestinationWithoutOverwrite_IsKept(t *testing.T) {
	r := newCodexTransform(t)
	dest := ".codex/agents/code-reviewer.toml"
	trWrite(t, r.ws, dest, string(existingCodexDestination()))

	res := r.mustRun(codexRequest(r))

	if f := trFile(t, res, "code-reviewer"); f.Status != app.StatusSkipped {
		t.Errorf("status = %q, want skipped", f.Status)
	}
	if b, _ := os.ReadFile(filepath.Join(r.ws, filepath.FromSlash(dest))); string(b) != string(existingCodexDestination()) {
		t.Errorf("an existing destination without overwrite must stay unchanged:\n%s", b)
	}
}
