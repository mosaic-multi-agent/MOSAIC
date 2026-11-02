package app_test

// transform_custom_tool_test.go specifies the custom-tool questions of the whole-deployment
// transform: a generic tool the target harness cannot map is asked once for the agents that
// will be written; an answer names the server, a skip or skip-all leaves the tool unmapped in
// the agent outcome, and cancelling aborts the run with nothing written.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trUnmappedGeneric is the generic tool of a Claude Code `Grep`, which trUnmapGeneric makes
// the target unable to map.
const trUnmappedGeneric = "content_search"

// trUnmapGeneric makes the harness declare no mapping for the generic tool.
func trUnmapGeneric(harnessID, generic string) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		base := reg.modules[harnessID]
		desc := *base.Descriptor()
		var kept []domain.ToolMapping
		for _, m := range desc.Tools.Mappings {
			if m.Generic != generic {
				kept = append(kept, m)
			}
		}
		desc.Tools.Mappings = kept
		reg.modules[harnessID] = trModelCatalogModule{HarnessModule: base, desc: desc}
	}
}

// trCustomToolRun is a run of the subagent fixture whose target cannot map trUnmappedGeneric.
func trCustomToolRun(t *testing.T, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trNew(t, script, hClaude, hOpen, trUnmapGeneric(hOpen, trUnmappedGeneric))
	r.seed("subagent")
	return r
}

func TestTransformCustomTool_UnmappedGeneric_IsAskedOnceForTheWrittenAgent(t *testing.T) {
	r := trCustomToolRun(t, nil)

	r.mustRun(r.request())

	if got, want := r.spy.customToolSubjects(), []string{trUnmappedGeneric}; !slices.Equal(got, want) {
		t.Errorf("custom tool questions = %q, want %q", got, want)
	}
}

func TestTransformCustomTool_NotAskedForAnAgentThatIsNotWritten(t *testing.T) {
	r := trCustomToolRun(t, nil)
	r.put(hOpen, "subagent", "existing destination") // skipped: destination exists

	r.mustRun(r.request())

	if got := r.spy.customToolSubjects(); len(got) != 0 {
		t.Errorf("custom tool questions asked for an agent that is not written: %q", got)
	}
}

func TestTransformCustomTool_AnsweredServer_IsWrittenAndTheToolIsNotUnmapped(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerText(domain.QCustomTool, trUnmappedGeneric, "my-grep-server").Build()
	r := trCustomToolRun(t, script)

	res := r.mustRun(r.request())

	if got := r.read(r.dest("subagent")); !strings.Contains(got, "my-grep-server") {
		t.Errorf("the written agent does not carry the answered server:\n%s", got)
	}
	if got := trFile(t, res, "subagent").UnmappedTools; len(got) != 0 {
		t.Errorf("UnmappedTools = %q, want none for an answered tool", got)
	}
}

func TestTransformCustomTool_SkippedTool_IsReportedUnmappedAndTheAgentStillWritten(t *testing.T) {
	r := trCustomToolRun(t, nil) // the stub skips an unscripted question

	res := r.mustRun(r.request())

	f := trFile(t, res, "subagent")
	if f.Status != app.StatusTransformed {
		t.Errorf("status = %q, want transformed: a skipped tool does not fail the file", f.Status)
	}
	if !slices.Contains(f.UnmappedTools, trUnmappedGeneric) {
		t.Errorf("UnmappedTools = %q, want it to contain %q", f.UnmappedTools, trUnmappedGeneric)
	}
	if !r.exists(r.dest("subagent")) {
		t.Error("the agent was not written after the tool was skipped")
	}
}

func TestTransformCustomTool_SkipAllRequest_AsksNothingAndReportsTheToolUnmapped(t *testing.T) {
	r := trCustomToolRun(t, nil)
	req := r.request()
	req.SkipAll = map[domain.QuestionID]bool{domain.QCustomTool: true}

	res := r.mustRun(req)

	if got := r.spy.customToolSubjects(); len(got) != 0 {
		t.Errorf("custom tool questions asked despite SkipAll: %q", got)
	}
	if got := trFile(t, res, "subagent").UnmappedTools; !slices.Contains(got, trUnmappedGeneric) {
		t.Errorf("UnmappedTools = %q, want it to contain %q", got, trUnmappedGeneric)
	}
}

func TestTransformCustomTool_CancelledQuestion_AbortsWithNothingWritten(t *testing.T) {
	r := trCustomToolRun(t, nil)
	r.spy.cancelTextOn = domain.QCustomTool
	before := r.snapshot()

	res, err := r.run(r.request())

	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("error = %v, want ErrSelectionCancelled", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("cancelled run left a trace (no file may exist after a cancel): %s", diff)
	}
	if len(r.spy.reviews) != 0 {
		t.Error("the plan review was asked after a cancelled custom tool question")
	}
}
