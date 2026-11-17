package app_test

// transform_opencode_pair_result_test.go specifies the run result and the JSON document of a
// transform between the two OpenCode targets: a dry run lists the removals and replacements the
// real run performs, writes nothing and backs nothing up; agents are listed before hook files;
// each outcome says what happens to its file; the JSON document carries both lists.

import (
	"encoding/json"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTransformOpenCodePair_DryRunListsTheRemovalAndReplacementsAndWritesNothing(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		before := r.snapshot()
		req := r.request()
		req.DryRun = true

		res := r.mustRun(req)

		if diff := trSameTree(before, r.snapshot()); diff != "" {
			t.Errorf("dry run changed the workspace: %s", diff)
		}
		if !res.DryRun {
			t.Error("result.DryRun = false, want true")
		}
		if len(res.Removals) != 1 {
			t.Fatalf("Removals = %+v, want only the source-only helper", res.Removals)
		}
		rm := trChangeAt(t, res.Removals, ocPluginsDir+"/"+ocOnlyHelper[r.src])
		if rm.Kind != domain.ArtifactHook || rm.Key != ocHookKey || rm.BackupPath != "" || rm.Reason == "" {
			t.Errorf("removal = %+v, want hook %s with a reason and no backup yet", rm, ocHookKey)
		}
		if len(res.Replacements) != 2 {
			t.Fatalf("Replacements = %+v, want the agent and the logger entry file", res.Replacements)
		}
		ag := res.Replacements[0]
		if ag.Kind != domain.ArtifactAgent || ag.Path != ocWorker || ag.Key != "worker" || ag.BackupPath != "" || ag.Reason == "" {
			t.Errorf("first replacement = %+v, want the agent %s with a reason and no backup yet", ag, ocWorker)
		}
		hk := res.Replacements[1]
		if hk.Kind != domain.ArtifactHook || hk.Path != ocPluginsDir+"/mosaic-logger.ts" || hk.Key != ocHookKey || hk.BackupPath != "" {
			t.Errorf("second replacement = %+v, want the hook entry file after the agent", hk)
		}
	})
}

func TestTransformOpenCodePair_OutcomesReportTheInPlaceWrites(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		req := r.request()
		req.DryRun = true

		res := r.mustRun(req)

		f := trFile(t, res, "worker")
		if f.Status != app.StatusTransformed || f.Action != app.PlanWriteOverwrite || f.DestinationPath != ocWorker {
			t.Errorf("agent outcome %q/%q at %q, want transformed/overwrite at %s", f.Status, f.Action, f.DestinationPath, ocWorker)
		}
		sk := trOutcome(t, res, domain.ArtifactSkill, trSkillKey)
		if sk.Status != app.StatusTransformed {
			t.Errorf("skill outcome %q, want transformed (carried in place)", sk.Status)
		}
		hk := trOutcome(t, res, domain.ArtifactHook, ocHookKey)
		if hk.Status != app.StatusTransformed || hk.Action != app.PlanWriteOverwrite {
			t.Errorf("hook outcome %q/%q, want transformed/overwrite", hk.Status, hk.Action)
		}
		if res.Failed != 0 {
			t.Errorf("Failed = %d, want 0", res.Failed)
		}
	})
}

func TestTransformOpenCodePair_UnrecordedAgentIsReportedAsNotRecordedAndKept(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		trWrite(t, r.ws, ".opencode/agents/mine.md", trFixture(t, "utility"))
		before := r.read(".opencode/agents/mine.md")

		res := r.mustRun(r.request())

		f := trFileBySource(t, res, "mine.md")
		if f.Status != app.StatusSkipped || f.Skip != app.SkipNotRecorded {
			t.Errorf("outcome %q/%q, want skipped/%s", f.Status, f.Skip, app.SkipNotRecorded)
		}
		if got := r.read(".opencode/agents/mine.md"); got != before {
			t.Errorf("unrecorded agent changed: %q", got)
		}
	})
}

func TestTransformOpenCodePair_JSONListsRemovalsAndReplacements(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
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

		if !jsonListHas(doc.Removals, "hook", ocPluginsDir+"/"+ocOnlyHelper[r.src]) {
			t.Errorf("JSON removals = %v, want the source-only helper", doc.Removals)
		}
		if !jsonListHas(doc.Replacements, "agent", ocWorker) || !jsonListHas(doc.Replacements, "hook", ocPluginsDir+"/mosaic-logger.ts") {
			t.Errorf("JSON replacements = %v, want the agent and the logger entry file", doc.Replacements)
		}
	})
}
