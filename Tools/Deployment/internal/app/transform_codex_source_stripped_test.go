package app_test

// transform_codex_source_stripped_test.go specifies the stripped-field report of a Transform
// from the Codex harness: source-harness-specific fields (reason unknown-field, with their
// rendered values) and user keys held in the Codex carriage container (reason carried-key)
// are reported per agent, and the carriage container never appears in the rebuilt agent.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
)

const (
	cxCarriedKey      = "user_note"
	cxCarriedValue    = "keep-me-please"
	cxCarriedIntKey   = "retry_budget"
	cxStrippedHarness = "sandbox_mode"
)

// codexSourceWithUserKeys returns a Codex source agent file that also holds two user-owned
// TOML keys, which decode into the carriage container.
func codexSourceWithUserKeys(name string) string {
	src := codexSourceAgent(cxOpts{name: name, model: cxSourceModelSub})
	extra := cxCarriedKey + " = \"" + cxCarriedValue + "\"\n" + cxCarriedIntKey + " = 3\n"
	return strings.Replace(src, "developer_instructions", extra+"developer_instructions", 1)
}

// cxStripped returns the stripped entry with the key and reason, and whether it exists.
func cxStripped(fields []app.StrippedField, key string, reason app.StripReason) (app.StrippedField, bool) {
	for _, f := range fields {
		if f.Key == key && f.Reason == reason {
			return f, true
		}
	}
	return app.StrippedField{}, false
}

// cxCountKey counts the stripped entries with the key.
func cxCountKey(fields []app.StrippedField, key string) int {
	n := 0
	for _, f := range fields {
		if f.Key == key {
			n++
		}
	}
	return n
}

func TestTransformFromCodex_Run_ReportsHarnessSpecificFieldAsStripped(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, "code-reviewer")
	got, ok := cxStripped(f.StrippedFields, cxStrippedHarness, app.StripReasonUnknownField)
	if !ok {
		t.Fatalf("StrippedFields = %+v, want an unknown-field entry for %s", f.StrippedFields, cxStrippedHarness)
	}
	if len(got.Values) != 1 || !strings.Contains(got.Values[0], "read-only") {
		t.Errorf("%s values = %v, want its rendered value read-only", cxStrippedHarness, got.Values)
	}
	if n := cxCountKey(f.StrippedFields, cxStrippedHarness); n != 1 {
		t.Errorf("%s reported %d times, want once", cxStrippedHarness, n)
	}
	if out := r.read(r.dest("code-reviewer")); strings.Contains(out, cxStrippedHarness) {
		t.Errorf("%s is reported as stripped but still present in the target:\n%s", cxStrippedHarness, out)
	}
}

func TestTransformFromCodex_Run_ReportsCarriedUserKeysAsStripped(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceWithUserKeys("code-reviewer"))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, "code-reviewer")
	for _, key := range []string{cxCarriedKey, cxCarriedIntKey} {
		if _, ok := cxStripped(f.StrippedFields, key, app.StripReasonCarriedKey); !ok {
			t.Errorf("StrippedFields = %+v, want a carried-key entry for %s", f.StrippedFields, key)
		}
		if n := cxCountKey(f.StrippedFields, key); n != 1 {
			t.Errorf("%s reported %d times, want once", key, n)
		}
	}
}

func TestTransformFromCodex_Run_RebuiltAgentHasNoCarriageContainerOrCarriedKeys(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceWithUserKeys("code-reviewer"))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, "code-reviewer")
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	out := r.read(r.dest("code-reviewer"))
	for _, key := range append(agentformat.CarriageKeys(), cxCarriedKey, cxCarriedIntKey, cxCarriedValue) {
		if strings.Contains(out, key) {
			t.Errorf("rebuilt agent contains %q; carried keys and the container must not reach the target:\n%s", key, out)
		}
	}
	for _, d := range f.DroppedFields {
		if agentformat.IsCarriageKey(d) {
			t.Errorf("DroppedFields names the carriage container key %q", d)
		}
	}
}

func TestTransformFromCodex_Run_StrippedFieldsAreReportedUnderDryRun(t *testing.T) {
	r := newCodexSourceRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceWithUserKeys("code-reviewer"))
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	f := trFile(t, res, "code-reviewer")
	if _, ok := cxStripped(f.StrippedFields, cxCarriedKey, app.StripReasonCarriedKey); !ok {
		t.Errorf("dry run StrippedFields = %+v, want the carried key reported", f.StrippedFields)
	}
	if r.exists(r.dest("code-reviewer")) {
		t.Errorf("a dry run must not write the target")
	}
}

func TestTransformFromMarkdown_Run_ReportsNoStrippedFields(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent")

	res := r.mustRun(r.request())

	for _, f := range res.Files {
		if len(f.StrippedFields) != 0 {
			t.Errorf("%s: a Markdown source without carriage reports no stripped fields, got %+v", f.Key, f.StrippedFields)
		}
	}
}

func TestRebuildFromCodex_ReportsStrippedHarnessFieldWithValues(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: "code-reviewer"}))

	res, _ := rbRebuild(t, in)

	got, ok := cxStripped(res.StrippedFields, cxStrippedHarness, app.StripReasonUnknownField)
	if !ok || len(got.Values) != 1 || !strings.Contains(got.Values[0], "read-only") {
		t.Errorf("StrippedFields = %+v, want %s unknown-field with value read-only", res.StrippedFields, cxStrippedHarness)
	}
	if !containsString(res.DroppedFields, cxStrippedHarness) {
		t.Errorf("DroppedFields = %v, want %s still listed as dropped", res.DroppedFields, cxStrippedHarness)
	}
	// Only keys absent from the written output are reported: model, description and name are kept.
	for _, kept := range []string{"model", "description", "name"} {
		if n := cxCountKey(res.StrippedFields, kept); n != 0 {
			t.Errorf("%s is present in the output and must not be reported as stripped: %+v", kept, res.StrippedFields)
		}
	}
	if len(res.StrippedFields) != 1 {
		t.Errorf("StrippedFields = %+v, want only the %s entry for the default fixture", res.StrippedFields, cxStrippedHarness)
	}
}

// codexSourceWithMarkerKeys returns a Codex source agent that also holds user keys the carriage
// holds by marker: one with an attached TOML comment and one with a non-bare (quoted) name.
func codexSourceWithMarkerKeys(name string) string {
	src := codexSourceAgent(cxOpts{name: name, model: cxSourceModelSub})
	extra := "commented_key = \"with-comment\" # attached comment\n\"odd key\" = 7\n"
	return strings.Replace(src, "developer_instructions", extra+"developer_instructions", 1)
}

func TestRebuildFromCodex_MarkerCarriedKeysAreReportedOnceAndAbsentFromOutput(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceWithMarkerKeys("code-reviewer"))

	res, out := rbRebuild(t, in)

	for _, key := range []string{"commented_key", "odd key"} {
		if _, ok := cxStripped(res.StrippedFields, key, app.StripReasonCarriedKey); !ok {
			t.Errorf("StrippedFields = %+v, want carried-key entry for %q", res.StrippedFields, key)
		}
		if n := cxCountKey(res.StrippedFields, key); n != 1 {
			t.Errorf("%q reported %d times, want once", key, n)
		}
		if strings.Contains(out, key) {
			t.Errorf("output contains carried key %q:\n%s", key, out)
		}
	}
}

func TestRebuildFromCodex_CarriedKeysAndHarnessFieldAreEachReportedOnceUnderOwnReason(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceWithUserKeys("code-reviewer"))

	res, _ := rbRebuild(t, in)

	if _, ok := cxStripped(res.StrippedFields, cxStrippedHarness, app.StripReasonUnknownField); !ok {
		t.Errorf("StrippedFields = %+v, want %s under unknown-field", res.StrippedFields, cxStrippedHarness)
	}
	for _, key := range []string{cxStrippedHarness, cxCarriedKey, cxCarriedIntKey} {
		if n := cxCountKey(res.StrippedFields, key); n != 1 {
			t.Errorf("%s reported %d times, want once: %+v", key, n, res.StrippedFields)
		}
	}
}

func TestRebuildFromCodex_CarriedKeysAreReportedAndAbsentFromOutputAndDroppedFields(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceWithUserKeys("code-reviewer"))

	res, out := rbRebuild(t, in)

	for _, key := range []string{cxCarriedKey, cxCarriedIntKey} {
		if _, ok := cxStripped(res.StrippedFields, key, app.StripReasonCarriedKey); !ok {
			t.Errorf("StrippedFields = %+v, want carried-key entry for %s", res.StrippedFields, key)
		}
		if containsString(res.DroppedFields, key) {
			t.Errorf("DroppedFields lists carried key %s; carried keys are reported only as stripped", key)
		}
		if strings.Contains(out, key) {
			t.Errorf("output contains carried key %s:\n%s", key, out)
		}
	}
	for _, key := range agentformat.CarriageKeys() {
		if strings.Contains(out, key) || containsString(res.DroppedFields, key) {
			t.Errorf("carriage container key %q reached the output or DroppedFields", key)
		}
	}
}

func TestRebuildFromMarkdown_ReportsNoStrippedFields(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, _ := rbRebuild(t, rbInput(t, p, false))

			if res.StrippedFields != nil {
				t.Errorf("StrippedFields = %+v, want nil for a Markdown source without carriage", res.StrippedFields)
			}
		})
	}
}
