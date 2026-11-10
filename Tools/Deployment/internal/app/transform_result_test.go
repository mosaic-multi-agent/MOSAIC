package app_test

// transform_result_test.go specifies the report of the whole-deployment transform: transformed,
// skipped and failed counts, a reason for every skipped file, each tool listed once, the
// both-forms warning, the note that harness injections may be newer than the source, and the
// location transform looked at when it finds no agents.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTransformResult_CountsMatchTheFileOutcomes(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed(trAgentFixtures...)
	r.seed(trNotAgentFixtures...)

	res := r.mustRun(r.request())

	if res.Transformed != len(trAgentFixtures) || res.Skipped != len(trNotAgentFixtures) || res.Failed != 0 {
		t.Errorf("counts transformed/skipped/failed = %d/%d/%d, want %d/%d/0",
			res.Transformed, res.Skipped, res.Failed, len(trAgentFixtures), len(trNotAgentFixtures))
	}
	var transformed, skipped, failed int
	for _, f := range res.Files {
		switch f.Status {
		case app.StatusTransformed:
			transformed++
		case app.StatusSkipped:
			skipped++
		case app.StatusFailed:
			failed++
		}
	}
	if transformed != res.Transformed || skipped != res.Skipped || failed != res.Failed {
		t.Errorf("counts %d/%d/%d do not equal the outcomes %d/%d/%d",
			res.Transformed, res.Skipped, res.Failed, transformed, skipped, failed)
	}
}

func TestTransformResult_EverySkippedFileHasAKindAndAReason(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed(trNotAgentFixtures...)

	res := r.mustRun(r.request())

	for _, f := range res.Files {
		if f.Status == app.StatusSkipped && (f.Reason == "" || f.Skip == "") {
			t.Errorf("skipped %s has skip %q and reason %q, want both set", f.SourcePath, f.Skip, f.Reason)
		}
	}
	if res.Skipped == 0 {
		t.Fatal("no file was reported as skipped")
	}
}

func TestTransformResult_DestinationAlreadyExists_IsSkippedWithAReasonAndKept(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent", "utility")
	r.put(hOpen, "utility", "keep me")

	res := r.mustRun(r.request())

	f := trFile(t, res, "utility")
	if f.Status != app.StatusSkipped || f.Skip != app.SkipDestinationExists || f.Action != app.PlanSkip || f.Reason == "" {
		t.Errorf("utility outcome = %+v, want skipped/destination-exists with a reason", f)
	}
	if got := r.read(r.dest("utility")); got != "keep me" {
		t.Errorf("existing destination was changed to %q", got)
	}
	if trFile(t, res, "subagent").Status != app.StatusTransformed {
		t.Error("a skipped neighbour must not stop the other files")
	}
}

func TestTransformResult_DuplicateKey_IsSkippedWithAReason(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trWrite(t, r.ws, r.agentsDir(hClaude)+"/dup.md", trFixture(t, "subagent"))
	trWrite(t, r.ws, r.agentsDir(hClaude)+"/dup.agent.md", trFixture(t, "subagent"))

	res := r.mustRun(r.request())

	f := trFileBySource(t, res, "dup.agent.md")
	if f.Status != app.StatusSkipped || f.Skip != app.SkipDuplicateKey || f.Reason == "" {
		t.Errorf("dup.agent.md outcome = %+v, want skipped/duplicate-key with a reason", f)
	}
	if trFileBySource(t, res, "dup.md").Status != app.StatusTransformed {
		t.Error("the file with the source harness's own extension must be the one transformed")
	}
}

func TestTransformResult_OutcomeCarriesKindKeyActionAndRole(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent", "orchestrator")

	res := r.mustRun(r.request())

	sub, orch := trFile(t, res, "subagent"), trFile(t, res, "orchestrator")
	if sub.Action != app.PlanWriteCreate || sub.Role != domain.RoleSubagent {
		t.Errorf("subagent action %q role %q, want create/subagent", sub.Action, sub.Role)
	}
	if orch.Role != domain.RoleOrchestrator {
		t.Errorf("orchestrator role = %q, want %q", orch.Role, domain.RoleOrchestrator)
	}
}

func TestTransformResult_EachToolIsListedOnce(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	doc := strings.Replace(trFixture(t, "subagent"), "tools: Read, Grep", "tools: Read, Grep, Read, Grep", 1)
	trWrite(t, r.ws, r.agentsDir(hClaude)+"/repeated-tools.md", doc)

	res := r.mustRun(r.request())

	f := trFile(t, res, "repeated-tools")
	if len(f.Tools) == 0 {
		t.Fatal("the outcome lists no tools")
	}
	if dup := trDuplicates(f.Tools); len(dup) != 0 {
		t.Errorf("tools listed more than once: %q (all: %q)", dup, f.Tools)
	}
	var dropped []string
	for _, d := range f.DroppedTools {
		dropped = append(dropped, d.Name)
	}
	if dup := trDuplicates(dropped); len(dup) != 0 {
		t.Errorf("dropped tools listed more than once: %q", dup)
	}
}

func TestTransformResult_BothFormsInTheTargetFolder_WarnsWithBothPaths(t *testing.T) {
	r := trNew(t, nil, hClaude, hCLI)
	r.seed("subagent")
	trWrite(t, r.ws, r.agentsDir(hCLI)+"/subagent.md", "# plain copy of the same agent")

	res := r.mustRun(r.request())

	notes := trNotes(res, app.NoteBothForms)
	if len(notes) != 1 {
		t.Fatalf("both-forms notes = %d, want 1: %+v", len(notes), res.Notes)
	}
	joined := filepath.ToSlash(strings.Join(notes[0].Paths, " "))
	for _, want := range []string{"subagent.md", "subagent.agent.md"} {
		if !strings.Contains(joined, want) {
			t.Errorf("both-forms note paths %q do not name %s", joined, want)
		}
	}
	if trFile(t, res, "subagent").Status != app.StatusTransformed {
		t.Error("the both-forms warning must not stop the write")
	}
}

func TestTransformResult_NoBothForms_NoWarning(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent")

	res := r.mustRun(r.request())

	if notes := trNotes(res, app.NoteBothForms); len(notes) != 0 {
		t.Errorf("unexpected both-forms notes: %+v", notes)
	}
}

func TestTransformResult_NewerInjectionsNote_PresentWhenSomethingIsTransformed(t *testing.T) {
	for _, dry := range []bool{false, true} {
		r := trNew(t, nil, hClaude, hOpen)
		r.seed("subagent")
		req := r.request()
		req.DryRun = dry

		res := r.mustRun(req)

		if len(trNotes(res, app.NoteNewerTargetContent)) == 0 {
			t.Errorf("dryRun=%v: no newer-target-content note although an agent is transformed", dry)
		}
	}
}

func TestTransformResult_NewerInjectionsNote_AbsentWhenNothingIsTransformed(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent")
	r.mustRun(r.request())

	res := r.mustRun(r.request()) // repeat: everything is skipped

	if notes := trNotes(res, app.NoteNewerTargetContent); len(notes) != 0 {
		t.Errorf("newer-target-content note present although nothing was transformed: %+v", notes)
	}
}

func TestTransformResult_NoAgents_SaysWhereTransformLooked(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	before := r.snapshot()
	wantDir := filepath.Join(r.ws, filepath.FromSlash(r.agentsDir(hClaude)))

	res := r.mustRun(r.request())

	if !strings.EqualFold(filepath.Clean(res.SourceAgentsDir), filepath.Clean(wantDir)) {
		t.Errorf("SourceAgentsDir = %q, want %q", res.SourceAgentsDir, wantDir)
	}
	notes := trNotes(res, app.NoteNoAgentsFound)
	if len(notes) != 1 || len(notes[0].Paths) == 0 || !strings.EqualFold(filepath.Clean(notes[0].Paths[0]), filepath.Clean(wantDir)) {
		t.Errorf("no-agents note = %+v, want one note whose first path is %q", notes, wantDir)
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("a transform without agents wrote something: %s", diff)
	}
	if res.Transformed != 0 || res.Failed != 0 {
		t.Errorf("counts transformed/failed = %d/%d, want 0/0", res.Transformed, res.Failed)
	}
}

func TestTransformResult_OnlyNonAgentFiles_SaysWhereTransformLooked(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed(trNotAgentFixtures...)

	res := r.mustRun(r.request())

	if len(trNotes(res, app.NoteNoAgentsFound)) != 1 {
		t.Errorf("notes = %+v, want one no-agents-found note", res.Notes)
	}
	if res.SourceAgentsDir == "" {
		t.Error("SourceAgentsDir is empty")
	}
}

func TestTransformResult_WithAgents_HasNoNoAgentsNote(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent")

	res := r.mustRun(r.request())

	if notes := trNotes(res, app.NoteNoAgentsFound); len(notes) != 0 {
		t.Errorf("unexpected no-agents note: %+v", notes)
	}
	if res.SourceAgentsDir == "" {
		t.Error("SourceAgentsDir is empty although transform read it")
	}
}
