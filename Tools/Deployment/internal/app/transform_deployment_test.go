package app_test

// transform_deployment_test.go specifies the whole-deployment transform from a workspace root:
// every planned agent is written to the target agents folder, the source deployment is never
// touched, the full plan exists before any question is asked or file written, the plan is
// shown for review, and a dry run reports the same plan while writing nothing.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trSeedAll places every agent fixture and every non-agent fixture in the source folder.
func trSeedAll(r *trRun) {
	r.seed(trAgentFixtures...)
	r.seed(trNotAgentFixtures...)
}

func TestTransformDeployment_WritesEveryAgentToTargetAgentsFolder(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)

	res := r.mustRun(r.request())

	for _, key := range trAgentFixtures {
		if !r.exists(r.dest(key)) {
			t.Errorf("agent %q was not written to %s", key, r.dest(key))
		}
		f := trFile(t, res, key)
		if f.Status != app.StatusTransformed {
			t.Errorf("agent %q status = %q, want transformed", key, f.Status)
		}
		if got := filepath.ToSlash(f.DestinationPath); got != r.dest(key) {
			t.Errorf("agent %q destination = %q, want %q", key, got, r.dest(key))
		}
	}
	if res.Transformed != len(trAgentFixtures) {
		t.Errorf("Transformed = %d, want %d", res.Transformed, len(trAgentFixtures))
	}
	if res.SourceHarnessID != hClaude || res.TargetHarnessID != hOpen {
		t.Errorf("harness ids = %q -> %q, want %q -> %q", res.SourceHarnessID, res.TargetHarnessID, hClaude, hOpen)
	}
	if res.Workspace != r.ws {
		t.Errorf("Workspace = %q, want %q", res.Workspace, r.ws)
	}
}

func TestTransformDeployment_FilesThatAreNotAgents_AreSkippedAndNotWritten(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)

	res := r.mustRun(r.request())

	for _, name := range trNotAgentFixtures {
		f := trFileBySource(t, res, name+".md")
		if f.Status != app.StatusSkipped || f.Skip != app.SkipNotAgent {
			t.Errorf("%s: status %q skip %q, want skipped/not-agent", name, f.Status, f.Skip)
		}
		if r.exists(r.dest(name)) {
			t.Errorf("%s is not an agent but was written to the target", name)
		}
	}
}

func TestTransformDeployment_SourceDeploymentIsNeverChanged(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)
	before := r.snapshot()

	r.mustRun(r.request())

	after := r.snapshot()
	for path, content := range before {
		if got, ok := after[path]; !ok || got != content {
			t.Errorf("source file %s was changed or deleted by the transform", path)
		}
	}
}

// The plan-before-question order is discriminated mainly by
// TestTransformModels_OnlyGroupsOfWrittenFilesAreAsked and by the all-skipped test in
// transform_failure_test.go; this test pins the no-write half of it.
func TestTransformDeployment_NothingIsWrittenBeforeTheFirstQuestion(t *testing.T) {
	script := interactiontest.NewBuilder().Build()
	r := trNew(t, script, hClaude, hOpen)
	trSeedAll(r)
	before := r.snapshot()
	var atQuestion map[string]string
	r.spy.atFirstQuestion = func() { atQuestion = r.snapshot() }

	r.mustRun(r.request())

	if atQuestion == nil {
		t.Fatal("no model question was asked, so the order cannot be observed")
	}
	if diff := trSameTree(before, atQuestion); diff != "" {
		t.Errorf("the workspace changed before the first question: %s", diff)
	}
}

func TestTransformDeployment_AllModelQuestionsPrecedeThePlanReview(t *testing.T) {
	script := interactiontest.NewBuilder().Build()
	r := trNew(t, script, hClaude, hOpen)
	trSeedAll(r)
	req := r.request()
	req.AutoConfirmPlan = false

	r.mustRun(req)

	reviewAt, lastModel := -1, -1
	for i, id := range script.QuestionOrder() {
		switch id {
		case domain.QPlanConfirm:
			reviewAt = i
		case domain.QTransformTargetModel:
			lastModel = i
		}
	}
	if reviewAt < 0 || lastModel < 0 {
		t.Fatalf("expected model questions and a review, got order %v", script.QuestionOrder())
	}
	if lastModel > reviewAt {
		t.Errorf("a model question came after the plan review: %v", script.QuestionOrder())
	}
}

func TestTransformDeployment_ReviewShowsTheTransformPlan(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)
	req := r.request()
	req.AutoConfirmPlan = false

	r.mustRun(req)

	if len(r.spy.reviews) != 1 {
		t.Fatalf("review asked %d times, want once", len(r.spy.reviews))
	}
	plan := r.spy.reviews[0]
	if plan.Mode != domain.ModeTransformHarness {
		t.Errorf("review mode = %q, want %q", plan.Mode, domain.ModeTransformHarness)
	}
	if plan.Harness.ID != hOpen {
		t.Errorf("review harness = %q, want the target %q", plan.Harness.ID, hOpen)
	}
	for _, key := range trAgentFixtures {
		if !trPlanHasTarget(plan, r.dest(key)) {
			t.Errorf("review plan has no item for %s", r.dest(key))
		}
	}
	skippedFiles := 0
	for _, g := range plan.Gaps {
		if g.Kind == domain.GapSkippedFile {
			skippedFiles++
		}
	}
	if skippedFiles != len(trNotAgentFixtures) {
		t.Errorf("review plan has %d skipped-file gaps, want %d", skippedFiles, len(trNotAgentFixtures))
	}
}

func trPlanHasTarget(p domain.Plan, rel string) bool {
	for _, it := range p.Items {
		if strings.HasSuffix(filepath.ToSlash(it.TargetPath), rel) {
			return true
		}
	}
	return false
}

func TestTransformDeployment_DeclinedReview_WritesNothingAndReportsCancelled(t *testing.T) {
	script := interactiontest.NewBuilder().AnswerReview(false).Build()
	r := trNew(t, script, hClaude, hOpen)
	trSeedAll(r)
	before := r.snapshot()
	req := r.request()
	req.AutoConfirmPlan = false

	res, err := r.run(req)

	if !errors.Is(err, app.ErrPlanNotConfirmed) {
		t.Fatalf("error = %v, want ErrPlanNotConfirmed", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("declined review left a trace: %s", diff)
	}
}

func TestTransformDeployment_DeclinedReview_ResultCarriesTheDryRunPlan(t *testing.T) {
	dry := trNew(t, nil, hClaude, hOpen)
	trSeedAll(dry)
	dreq := dry.request()
	dreq.DryRun = true
	dryRes := dry.mustRun(dreq)

	declined := trNew(t, interactiontest.NewBuilder().AnswerReview(false).Build(), hClaude, hOpen)
	trSeedAll(declined)
	req := declined.request()
	req.AutoConfirmPlan = false
	res, _ := declined.run(req)

	if res.DryRun || res.TodoPath != "" {
		t.Errorf("declined result DryRun=%v TodoPath=%q, want false and empty", res.DryRun, res.TodoPath)
	}
	if got, want := trPlanLines(res), trPlanLines(dryRes); got != want {
		t.Errorf("declined plan differs from the dry-run plan:\n got %s\nwant %s", got, want)
	}
	if res.Transformed != dryRes.Transformed || res.Skipped != dryRes.Skipped || res.Failed != dryRes.Failed {
		t.Errorf("counts %d/%d/%d, dry run %d/%d/%d", res.Transformed, res.Skipped, res.Failed,
			dryRes.Transformed, dryRes.Skipped, dryRes.Failed)
	}
}

// trPlanLines renders the planned outcomes of a result for comparison; workspace paths differ
// between runs, so only the relative parts are used.
func trPlanLines(res app.TransformHarnessResult) string {
	var b strings.Builder
	for _, f := range res.Files {
		src := filepath.Base(filepath.ToSlash(f.SourcePath))
		b.WriteString(string(f.Kind) + "|" + src + "|" + filepath.ToSlash(f.DestinationPath) + "|" +
			string(f.Action) + "|" + string(f.Status) + "|" + string(f.Skip) + "\n")
	}
	return b.String()
}

func TestTransformDeployment_DryRun_WritesNothingAndAsksNoReview(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)
	before := r.snapshot()
	req := r.request()
	req.AutoConfirmPlan = false
	req.DryRun = true

	res := r.mustRun(req)

	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run changed the workspace: %s", diff)
	}
	if !res.DryRun || res.TodoPath != "" {
		t.Errorf("DryRun=%v TodoPath=%q, want true and empty", res.DryRun, res.TodoPath)
	}
	if len(r.spy.reviews) != 0 {
		t.Errorf("dry run asked for a plan review %d times", len(r.spy.reviews))
	}
}

func TestTransformDeployment_DryRun_ReportsTheSamePlanAsARealRun(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)
	dreq := r.request()
	dreq.DryRun = true
	dry := r.mustRun(dreq)

	real := r.mustRun(r.request())

	if got, want := trPlanLines(dry), trPlanLines(real); got != want {
		t.Errorf("dry-run plan differs from the real run:\n dry %s\nreal %s", got, want)
	}
	if dry.Transformed != real.Transformed || dry.Skipped != real.Skipped {
		t.Errorf("dry counts %d/%d, real %d/%d", dry.Transformed, dry.Skipped, real.Transformed, real.Skipped)
	}
}

func TestTransformDeployment_AutoConfirm_AsksNoReview(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)

	r.mustRun(r.request())

	if len(r.spy.reviews) != 0 {
		t.Errorf("AutoConfirmPlan still asked for review %d times", len(r.spy.reviews))
	}
}

func TestTransformDeployment_EmptyTargetHarness_IsAskedAndTheAnswerUsed(t *testing.T) {
	script := interactiontest.NewBuilder().AnswerSelectOne(domain.QTransformTargetHarness, "", hOpen).Build()
	r := trNew(t, script, hClaude, "")
	trSeedAll(r)

	res := r.mustRun(r.request())

	if res.TargetHarnessID != hOpen {
		t.Errorf("TargetHarnessID = %q, want the answered %q", res.TargetHarnessID, hOpen)
	}
	if !r.exists(r.agentsDir(hOpen) + "/subagent.md") {
		t.Error("the agent was not written for the answered target harness")
	}
}

func TestTransformDeployment_AnsweredTargetEqualToSource_FailsBeforeFurtherQuestions(t *testing.T) {
	script := interactiontest.NewBuilder().AnswerSelectOne(domain.QTransformTargetHarness, "", hClaude).Build()
	r := trNew(t, script, hClaude, "")
	trSeedAll(r)
	req := r.request()
	req.Workspace = ""

	_, err := r.run(req)

	if !errors.Is(err, app.ErrTransformSameHarness) {
		t.Fatalf("error = %v, want ErrTransformSameHarness", err)
	}
	for _, id := range script.QuestionOrder() {
		if id != domain.QTransformTargetHarness {
			t.Errorf("question %q was asked after the same-harness target was known", id)
		}
	}
}

func TestTransformDeployment_EmptyWorkspace_IsAskedAndTheAnswerUsed(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	trSeedAll(r)
	script := interactiontest.NewBuilder().AnswerText(domain.QWorkspace, "", r.ws).Build()
	r.spy.Stub = script
	req := r.request()
	req.Workspace = ""

	res := r.mustRun(req)

	if !script.WasAsked(domain.QWorkspace, "") {
		t.Error("QWorkspace was not asked for an empty Workspace")
	}
	if res.Workspace != r.ws || !r.exists(r.dest("subagent")) {
		t.Errorf("the answered workspace %q was not transformed (result workspace %q)", r.ws, res.Workspace)
	}
}

func TestTransformDeployment_CancelledTargetHarnessQuestion_ReportsCancelledAndWritesNothing(t *testing.T) {
	script := interactiontest.NewBuilder().AnswerCancelledChoice(domain.QTransformTargetHarness, "").Build()
	r := trNew(t, script, hClaude, "")
	trSeedAll(r)
	before := r.snapshot()

	res, err := r.run(r.request())

	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("error = %v, want ErrSelectionCancelled", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("cancelled run left a trace: %s", diff)
	}
}

func TestTransformDeployment_CatalogEntryByMosaicID_DecidesTheManifestRefKey(t *testing.T) {
	cat := newMinimalCatalog()
	cat.agents = []domain.Agent{{Key: "catalog-reviewer", NumericID: "55", Role: domain.RoleSubagent}}
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat })
	r.seed("subagent") // carries mosaic_id 55

	r.mustRun(r.request())

	e, ok := trEntryAt(trLoadManifest(t, r.ws), r.dest("subagent"))
	if !ok {
		t.Fatalf("no manifest entry at %s", r.dest("subagent"))
	}
	if e.Ref.Key != "catalog-reviewer" {
		t.Errorf("manifest ref key = %q, want the catalog entry key %q", e.Ref.Key, "catalog-reviewer")
	}
}

func TestTransformDeployment_AgentNotInTheCatalog_IsTransformedWithItsFileKey(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent")

	res := r.mustRun(r.request())

	if trFile(t, res, "subagent").Status != app.StatusTransformed {
		t.Fatal("an agent absent from the catalog must still be transformed")
	}
	e, ok := trEntryAt(trLoadManifest(t, r.ws), r.dest("subagent"))
	if !ok || e.Ref.Key != "subagent" {
		t.Errorf("manifest entry = %+v (found %v), want ref key %q", e.Ref, ok, "subagent")
	}
}
