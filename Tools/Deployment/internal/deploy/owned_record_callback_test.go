package deploy_test

// owned_record_callback_test.go specifies the agent-record channel of the executor: when the
// request supplies AgentOwnedContent, every agent manifest entry the executor builds (create,
// update, conflict overwrite, backup-then-overwrite, dry run) carries the value the callback
// returns for the written bytes, stored verbatim. Skill and hook records never go through the
// callback, and ContentHash stays over the exact written bytes.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// callbackRecord is the record the fake callback returns; it cannot equal a record computed
// by the executor from any bytes.
var callbackRecord = domain.OwnedContent{
	Hash:    "sha256:from-callback",
	Regions: []domain.RegionHash{{Name: "Identity", Hash: "sha256:region-from-callback"}},
}

// callbackCall is one invocation of the fake callback.
type callbackCall struct {
	item    domain.PlanItem
	written []byte
}

// recordingCallback returns an AgentOwnedContent callback that records its calls and returns
// callbackRecord (or nil when returnNil is set).
func recordingCallback(calls *[]callbackCall, returnNil bool) func(domain.PlanItem, []byte) *domain.OwnedContent {
	return func(item domain.PlanItem, written []byte) *domain.OwnedContent {
		*calls = append(*calls, callbackCall{item: item, written: append([]byte(nil), written...)})
		if returnNil {
			return nil
		}
		rec := callbackRecord
		return &rec
	}
}

// executeWithCallback runs the executor over one item with the callback set.
func executeWithCallback(t *testing.T, workspace string, item domain.PlanItem, content string,
	cb func(domain.PlanItem, []byte) *domain.OwnedContent, tweak func(*deploy.ExecRequest)) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:              newPlan(workspace, item),
		MosaicRoot:        t.TempDir(),
		Content:           fixedContent([]byte(content)),
		AgentOwnedContent: cb,
	}
	if tweak != nil {
		tweak(&req)
	}
	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

// requireCallbackRecord asserts the entry of ref holds exactly the callback's record and that
// the callback saw the written bytes once.
func requireCallbackRecord(t *testing.T, res deploy.ExecResult, item domain.PlanItem, calls []callbackCall, written string) {
	t.Helper()
	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok {
		t.Fatalf("no manifest entry for %v", item.Ref)
	}
	if entry.Owned == nil || entry.Owned.Hash != callbackRecord.Hash ||
		len(entry.Owned.Regions) != 1 || entry.Owned.Regions[0] != callbackRecord.Regions[0] {
		t.Errorf("entry.Owned = %+v; want the callback's record %+v", entry.Owned, callbackRecord)
	}
	if len(calls) != 1 || string(calls[0].written) != written || calls[0].item.Ref != item.Ref {
		t.Errorf("callback calls = %d (%+v); want exactly one call with the written bytes for the item", len(calls), calls)
	}
	if entry.ContentHash != manifest.Hash([]byte(written)) {
		t.Errorf("ContentHash = %q; want the hash of the exact written bytes", entry.ContentHash)
	}
}

func TestExecute_AgentCreate_UsesCallbackRecord(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, ownedAgentBody, recordingCallback(&calls, false), nil)

	requireCallbackRecord(t, res, item, calls, ownedAgentBody)
}

func TestExecute_AgentUpdate_UsesCallbackRecord(t *testing.T) {
	workspace := t.TempDir()
	item := newAgentItem("runner", "agents/runner.md", domain.ActionUpdate)
	writeExisting(t, workspace, item.TargetPath, []byte("older content"))
	var calls []callbackCall

	res := executeWithCallback(t, workspace, item, ownedAgentBody, recordingCallback(&calls, false), nil)

	requireCallbackRecord(t, res, item, calls, ownedAgentBody)
}

func TestExecute_AgentConflictOverwrite_UsesCallbackRecord(t *testing.T) {
	workspace := t.TempDir()
	item := newConflictItem("runner", "agents/runner.md")
	writeExisting(t, workspace, item.TargetPath, []byte("locally modified"))
	var calls []callbackCall

	res := executeWithCallback(t, workspace, item, ownedAgentBody, recordingCallback(&calls, false), func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{item.TargetPath: domain.DecisionOverwrite}
	})

	requireCallbackRecord(t, res, item, calls, ownedAgentBody)
}

func TestExecute_AgentConflictBackupThenOverwrite_UsesCallbackRecord(t *testing.T) {
	workspace := t.TempDir()
	item := newConflictItem("runner", "agents/runner.md")
	writeExisting(t, workspace, item.TargetPath, []byte("locally modified"))
	var calls []callbackCall

	res := executeWithCallback(t, workspace, item, ownedAgentBody, recordingCallback(&calls, false), func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{item.TargetPath: domain.DecisionBackupThenOverwrite}
	})

	requireCallbackRecord(t, res, item, calls, ownedAgentBody)
}

func TestExecute_AgentDryRun_SimulatedEntryUsesCallbackRecord(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, ownedAgentBody, recordingCallback(&calls, false), func(r *deploy.ExecRequest) {
		r.DryRun = true
	})

	requireCallbackRecord(t, res, item, calls, ownedAgentBody)
}

func TestExecute_AgentCallbackReturnsNil_LeavesNoRecord(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, ownedAgentBody, recordingCallback(&calls, true), nil)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok {
		t.Fatal("no manifest entry for the written agent")
	}
	if len(calls) != 1 {
		t.Errorf("callback calls = %d; want the callback consulted once", len(calls))
	}
	if entry.Owned != nil {
		t.Errorf("entry.Owned = %+v; a nil callback result must leave no record (no fallback to whole-file hashing)", entry.Owned)
	}
}

func TestExecute_SkillEntry_NeverUsesAgentCallback(t *testing.T) {
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "lean-tdd"},
		SourcePath: "testdata/lean-tdd.md",
		TargetPath: "skills/lean-tdd/SKILL.md",
		Action:     domain.ActionCreate,
	}
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, ownedSkillBody, recordingCallback(&calls, false), nil)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned == nil {
		t.Fatalf("entry = %+v, found = %v; want a skill entry with a whole-file record", entry, ok)
	}
	if len(calls) != 0 || entry.Owned.Hash == callbackRecord.Hash {
		t.Errorf("callback calls = %d, Owned = %+v; skills must be recorded by the executor from the whole file", len(calls), entry.Owned)
	}
}

func TestExecute_HookEntry_NeverUsesAgentCallback(t *testing.T) {
	const hookBody = "#!/bin/sh\necho hook\n"
	source := filepath.Join(t.TempDir(), "pre-tool.sh")
	if err := os.WriteFile(source, []byte(hookBody), 0o644); err != nil {
		t.Fatalf("write hook source: %v", err)
	}
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "pre-tool"},
		SourcePath: source,
		TargetPath: "hooks/pre-tool.sh",
		Action:     domain.ActionCreate,
	}
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, "", recordingCallback(&calls, false), nil)

	entry, ok := res.Manifest.LookupAt(item.Ref, item.TargetPath)
	if !ok || entry.Owned == nil {
		t.Fatalf("entry = %+v, found = %v; want a hook entry with a whole-file record", entry, ok)
	}
	if len(calls) != 0 || entry.Owned.Hash == callbackRecord.Hash {
		t.Errorf("callback calls = %d, Owned = %+v; hooks must be recorded by the executor from the whole file", len(calls), entry.Owned)
	}
	if entry.ContentHash != manifest.Hash([]byte(hookBody)) {
		t.Errorf("ContentHash = %q; want the hash of the exact written bytes", entry.ContentHash)
	}
}

func TestExecute_AgentEmptyContent_DoesNotCallCallback(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)
	var calls []callbackCall

	res := executeWithCallback(t, t.TempDir(), item, "", recordingCallback(&calls, false), nil)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok {
		t.Fatal("no manifest entry for the written agent")
	}
	if len(calls) != 0 {
		t.Errorf("callback calls = %d; the callback must not be called for empty content", len(calls))
	}
	if entry.Owned != nil {
		t.Errorf("entry.Owned = %+v; empty content must leave no record", entry.Owned)
	}
}
