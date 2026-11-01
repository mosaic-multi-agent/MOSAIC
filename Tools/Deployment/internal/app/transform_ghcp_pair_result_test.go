package app_test

// transform_ghcp_pair_result_test.go specifies the run result and dry-run result of a
// transform between the two GHCP targets: every planned agent replacement and removal (and the
// hook ones) is listed, the existing fields keep their meaning, the JSON document only gains
// fields, and other pairs report neither list.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	// trPairInPlaceAgent is the agent file a recorded X.agent.md deployment leaves.
	trPairInPlaceAgent = ".github/agents/worker.agent.md"
	// trPairLegacyAgent is the agent file a recorded legacy deployment leaves.
	trPairLegacyAgent = ".github/agents/worker.md"
)

// trPairResultRunAt is a pair run whose workspace holds one recorded source agent at the given
// workspace-relative path and the source hook bundle.
func trPairResultRunAt(t *testing.T, src, tgt, agentRel string, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trCarryRun(t, src, tgt, script)
	trWrite(t, r.ws, agentRel, trFixture(t, "subagent"))
	trSaveManifest(t, r.ws, src, agentRel)
	r.seedHookBundle(trHookKey)
	return r
}

// trPairResultRun is a pair run over a recorded X.agent.md agent, which is replaced in place.
func trPairResultRun(t *testing.T, src, tgt string, script *interactiontest.Stub) (*trRun, string) {
	t.Helper()
	return trPairResultRunAt(t, src, tgt, trPairInPlaceAgent, script), trPairInPlaceAgent
}

// trPairLegacyResultRun is a pair run over a recorded legacy X.md agent, which is migrated.
func trPairLegacyResultRun(t *testing.T, src, tgt string, script *interactiontest.Stub) (*trRun, string) {
	t.Helper()
	return trPairResultRunAt(t, src, tgt, trPairLegacyAgent, script), trPairLegacyAgent
}

// trChangeAt returns the change with the path, failing the test when absent.
func trChangeAt(t *testing.T, list []app.TransformFileChange, path string) app.TransformFileChange {
	t.Helper()
	for _, c := range list {
		if c.Path == path {
			return c
		}
	}
	t.Fatalf("no change for %q in %+v", path, list)
	return app.TransformFileChange{}
}

func TestTransformGhcpPair_DryRunListsInPlaceAgentAndHookReplacementsAndWritesNothing(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, agentRel := trPairResultRun(t, pair[0], pair[1], nil)
			before := r.snapshot()
			req := r.request()
			req.DryRun = true

			res := r.mustRun(req)

			if diff := trSameTree(before, r.snapshot()); diff != "" {
				t.Errorf("dry run changed the workspace: %s", diff)
			}
			if len(res.Removals) != 0 {
				t.Errorf("Removals = %+v, want none: the recorded X.agent.md is replaced in place", res.Removals)
			}
			if len(res.Replacements) != 2 {
				t.Fatalf("Replacements = %+v, want the agent and the existing hook file", res.Replacements)
			}
			ag := trChangeAt(t, res.Replacements, agentRel)
			if ag.Kind != domain.ArtifactAgent || ag.Key != "worker" || ag.BackupPath != "" {
				t.Errorf("agent replacement = %+v, want agent worker with no backup yet", ag)
			}
			hk := trChangeAt(t, res.Replacements, r.hooksDir(pair[1])+"/logger.sh")
			if hk.Kind != domain.ArtifactHook || hk.Key != trHookKey || hk.BackupPath != "" {
				t.Errorf("hook replacement = %+v, want hook %s with no backup yet", hk, trHookKey)
			}
			if ag.Reason == "" || hk.Reason == "" || strings.ContainsAny(ag.Reason+hk.Reason, "\r\n") {
				t.Errorf("reasons %q / %q, want each non-empty on a single line", ag.Reason, hk.Reason)
			}
		})
	}
}

func TestTransformGhcpPair_DryRunListsLegacyAgentRemovalAndMigrationWritesNothing(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], nil)
			before := r.snapshot()
			req := r.request()
			req.DryRun = true

			res := r.mustRun(req)

			if diff := trSameTree(before, r.snapshot()); diff != "" {
				t.Errorf("dry run changed the workspace: %s", diff)
			}
			if len(res.Removals) != 1 {
				t.Fatalf("Removals = %+v, want exactly the recorded legacy agent", res.Removals)
			}
			rm := trChangeAt(t, res.Removals, legacyRel)
			if rm.Kind != domain.ArtifactAgent || rm.Key != "worker" || rm.BackupPath != "" || rm.Reason == "" {
				t.Errorf("removal = %+v, want agent worker, a reason and no backup yet", rm)
			}
			if len(res.Replacements) != 1 || res.Replacements[0].Kind != domain.ArtifactHook {
				t.Errorf("Replacements = %+v, want only the hook file: no X.agent.md exists to replace", res.Replacements)
			}
			f := trFile(t, res, "worker")
			if f.Status != app.StatusTransformed || f.Action != app.PlanWriteCreate || f.DestinationPath != trPairInPlaceAgent {
				t.Errorf("agent outcome %q/%q at %q, want transformed/create at %s", f.Status, f.Action, f.DestinationPath, trPairInPlaceAgent)
			}
		})
	}
}

func TestTransformGhcpPair_DryRunListsTheReplacedAgentMdBesideALegacyFileAndRemovesTheLegacyOne(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], nil)
			trWrite(t, r.ws, trPairInPlaceAgent, "an X.agent.md the manifest does not record\n")
			req := r.request()
			req.DryRun = true

			res := r.mustRun(req)

			trChangeAt(t, res.Removals, legacyRel)
			trChangeAt(t, res.Replacements, trPairInPlaceAgent)
			if f := trFile(t, res, "worker"); f.Action != app.PlanWriteOverwrite || f.DestinationPath != trPairInPlaceAgent {
				t.Errorf("agent outcome action %q at %q, want overwrite at %s", f.Action, f.DestinationPath, trPairInPlaceAgent)
			}
			agents := 0
			for _, f := range res.Files {
				if f.Kind == domain.ArtifactAgent {
					agents++
				}
			}
			if agents != 1 {
				t.Errorf("agent outcomes = %d, want one for the migrated agent", agents)
			}
		})
	}
}

func TestTransformGhcpPair_UnrecordedAgentsAppearInNeitherListAndAreSkipped(t *testing.T) {
	r := trCarryRun(t, hCLI, hVSCode, nil)
	trWrite(t, r.ws, ".github/agents/mine.agent.md", trFixture(t, "subagent"))
	trWrite(t, r.ws, ".github/agents/old.md", trFixture(t, "subagent"))
	trSaveManifest(t, r.ws, hCLI)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	for _, key := range []string{"mine", "old"} {
		if f := trFile(t, res, key); f.Status != app.StatusSkipped || f.Skip != app.SkipNotRecorded {
			t.Errorf("%s outcome %q skip %q, want skipped as %q", key, f.Status, f.Skip, app.SkipNotRecorded)
		}
	}
	if len(res.Removals) != 0 || len(res.Replacements) != 0 {
		t.Errorf("Removals %+v / Replacements %+v, want none for unrecorded agents", res.Removals, res.Replacements)
	}
}

func TestTransformGhcpPair_ExistingFieldsKeepTheirMeaningAndListsAreNotCounted(t *testing.T) {
	r, agentRel := trPairResultRun(t, hCLI, hVSCode, nil)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	if len(res.Replacements) == 0 {
		t.Fatal("setup: the result lists no replacement")
	}
	if res.SourceHarnessID != hCLI || res.TargetHarnessID != hVSCode || res.Workspace != r.ws || !res.DryRun || res.Cancelled {
		t.Errorf("identity fields = %+v, want the request echoed, dry run, not cancelled", res)
	}
	f := trFile(t, res, "worker")
	if f.Status != app.StatusTransformed || f.Action != app.PlanWriteOverwrite || f.DestinationPath != agentRel {
		t.Errorf("agent outcome %q/%q at %q, want transformed/overwrite at %s", f.Status, f.Action, f.DestinationPath, agentRel)
	}
	if f := trOutcome(t, res, domain.ArtifactHook, trHookKey); f.Skip != "" || f.Status != app.StatusTransformed {
		t.Errorf("hook outcome %q skip %q, want transformed with no skip", f.Status, f.Skip)
	}
	if res.Transformed+res.Skipped+res.Failed != len(res.Files) {
		t.Errorf("counts %d+%d+%d do not add up to the %d files: removals and replacements must not be counted",
			res.Transformed, res.Skipped, res.Failed, len(res.Files))
	}
}

func TestTransformGhcpPair_RemovedLegacyFileNeverAppearsAsAWrittenDestination(t *testing.T) {
	r, legacyRel := trPairLegacyResultRun(t, hVSCode, hCLI, nil)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)

	trChangeAt(t, res.Removals, legacyRel)
	for _, f := range res.Files {
		if f.DestinationPath == legacyRel {
			t.Errorf("the removed legacy file appears as a written destination: %+v", f)
		}
	}
}

func TestTransformGhcpPair_DeclinedPlanStillListsRemovalsAndReplacements(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			script := interactiontest.NewBuilder().AnswerReview(false).Build()
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], script)
			trWrite(t, r.ws, trPairInPlaceAgent, "an X.agent.md the manifest does not record\n")
			before := r.snapshot()
			req := r.request()
			req.AutoConfirmPlan = false

			res, err := r.run(req)

			if !errors.Is(err, app.ErrPlanNotConfirmed) {
				t.Fatalf("error = %v, want ErrPlanNotConfirmed", err)
			}
			if !res.Cancelled {
				t.Error("Cancelled = false, want true for a declined plan")
			}
			trChangeAt(t, res.Removals, legacyRel)
			trChangeAt(t, res.Replacements, trPairInPlaceAgent)
			trChangeAt(t, res.Replacements, r.hooksDir(pair[1])+"/logger.sh")
			if diff := trSameTree(before, r.snapshot()); diff != "" {
				t.Errorf("declined run changed the workspace: %s", diff)
			}
		})
	}
}

func TestTransformGhcpPair_SuccessfulRunListsTheRemovedLegacyAgentAndTheReplacedOne(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], nil)
			trWrite(t, r.ws, trPairInPlaceAgent, "an X.agent.md the manifest does not record\n")

			res := r.mustRun(r.request())

			trChangeAt(t, res.Removals, legacyRel)
			trChangeAt(t, res.Replacements, trPairInPlaceAgent)
			if res.Cancelled {
				t.Error("Cancelled = true on a confirmed run")
			}
		})
	}
}

func TestTransformGhcpPair_SuccessfulRunListsTheInPlaceAgentReplacement(t *testing.T) {
	r, agentRel := trPairResultRun(t, hVSCode, hCLI, nil)

	res := r.mustRun(r.request())

	trChangeAt(t, res.Replacements, agentRel)
	if len(res.Removals) != 0 {
		t.Errorf("Removals = %+v, want none for an in-place agent", res.Removals)
	}
}

func TestTransformGhcpPair_JSONDocumentOnlyGainsRemovalsAndReplacements(t *testing.T) {
	r, agentRel := trPairResultRun(t, hCLI, hVSCode, nil)
	req := r.request()
	req.DryRun = true

	res := r.mustRun(req)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, k := range []string{"sourceHarnessId", "targetHarnessId", "workspace", "sourceAgentsDir",
		"dryRun", "cancelled", "files", "transformed", "skipped", "failed"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("JSON lacks the existing field %q", k)
		}
	}
	if _, ok := doc["removals"]; ok {
		t.Errorf("JSON carries removals %s for an in-place agent, want the field absent", doc["removals"])
	}
	var replacements []map[string]any
	if err := json.Unmarshal(doc["replacements"], &replacements); err != nil || len(replacements) != 2 {
		t.Fatalf("replacements = %s (%v), want the agent and the hook file", doc["replacements"], err)
	}
	if !jsonListHas(replacements, "agent", agentRel) || !jsonListHas(replacements, "hook", r.hooksDir(hVSCode)+"/logger.sh") {
		t.Errorf("replacements = %v, want an agent entry for %s and a hook entry", replacements, agentRel)
	}
	for _, e := range replacements {
		if _, ok := e["backupPath"]; ok {
			t.Errorf("dry-run replacement %v carries backupPath, want it absent", e)
		}
	}
}

func TestTransformGhcpPair_JSONListsTheLegacyAgentRemoval(t *testing.T) {
	r, legacyRel := trPairLegacyResultRun(t, hVSCode, hCLI, nil)
	req := r.request()
	req.DryRun = true

	raw, err := json.Marshal(r.mustRun(req))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Removals []map[string]any `json:"removals"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(doc.Removals) != 1 || !jsonListHas(doc.Removals, "agent", legacyRel) {
		t.Fatalf("JSON removals = %v, want one agent entry for %s", doc.Removals, legacyRel)
	}
	if doc.Removals[0]["key"] != "worker" {
		t.Errorf("removal entry = %v, want key worker", doc.Removals[0])
	}
}

func TestTransformHarness_OtherPairsReportNeitherRemovalsNorReplacements(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	r.seed("subagent")
	trSaveManifest(t, r.ws, hClaude, r.agentsDir(hClaude)+"/subagent.md")
	r.seedHookBundle(trHookKey)
	req := r.request()
	req.DryRun = true
	req.Overwrite = true

	res := r.mustRun(req)

	if res.Removals != nil || res.Replacements != nil {
		t.Errorf("Removals %+v / Replacements %+v, want none outside the GHCP pair", res.Removals, res.Replacements)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"removals", "replacements"} {
		if _, ok := doc[k]; ok {
			t.Errorf("JSON of another pair carries %q, want it absent", k)
		}
	}
}
