package app

// transform_plan_models_test.go specifies the model-question input of a transform plan: only
// agents whose action writes contribute a source model, distinct and in first-seen order.

import (
	"reflect"
	"testing"
)

func TestTransformPlanSourceModels_OnlyWrittenAgentsContributeDistinctInFirstSeenOrder(t *testing.T) {
	p := TransformPlan{Agents: []PlannedAgent{
		{FileName: "a.md", Action: PlanWriteCreate, SourceModel: "m-one"},
		{FileName: "b.md", Action: PlanSkip, Skip: SkipDestinationExists, SourceModel: "m-skipped"},
		{FileName: "c.md", Action: PlanWriteOverwrite, SourceModel: "m-two"},
		{FileName: "d.md", Action: PlanWriteCreate, SourceModel: "m-one"},
		{FileName: "e.md", Action: PlanSkip, Skip: SkipNotAgent, SourceModel: "m-notagent"},
		{FileName: "f.md", Action: PlanWriteCreate, SourceModel: ""},
		{FileName: "g.md", Action: PlanWriteCreate, SourceModel: "m-three"},
	}}

	got := p.SourceModels()

	want := []string{"m-one", "m-two", "", "m-three"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SourceModels() = %q, want %q", got, want)
	}
}

func TestPlanTransform_SourceModelsListEveryWrittenAgentModelOnce(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAllAgents(t, ws, src)

	p := tpPlan(t, ws, src, tgt, false)

	// File-name order: harness-only, orchestrator-script, orchestrator, standalone (no model),
	// subagent, utility.
	want := []string{"claude-sonnet-4-5", "claude-opus-4-5", "", "claude-haiku-4-5"}
	if got := p.SourceModels(); !reflect.DeepEqual(got, want) {
		t.Errorf("SourceModels() = %q, want %q", got, want)
	}
	if a := tpAgent(t, p, "utility.md"); a.SourceModel != "claude-haiku-4-5" {
		t.Errorf("utility.md SourceModel = %q, want claude-haiku-4-5", a.SourceModel)
	}
	if a := tpAgent(t, p, "standalone.md"); a.SourceModel != "" {
		t.Errorf("standalone.md SourceModel = %q, want empty", a.SourceModel)
	}
}

func TestPlanTransform_AgentsWithExistingDestinationAskNoModelQuestion(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAllAgents(t, ws, src)
	tpWriteFile(t, ws, ".opencode/agents/orchestrator.md", "kept\n")
	tpWriteFile(t, ws, ".opencode/agents/orchestrator-script.md", "kept\n")

	withoutOverwrite := tpPlan(t, ws, src, tgt, false).SourceModels()
	withOverwrite := tpPlan(t, ws, src, tgt, true).SourceModels()

	if want := []string{"claude-sonnet-4-5", "", "claude-haiku-4-5"}; !reflect.DeepEqual(withoutOverwrite, want) {
		t.Errorf("without overwrite SourceModels() = %q, want %q (opus agents are skipped)", withoutOverwrite, want)
	}
	if want := []string{"claude-sonnet-4-5", "claude-opus-4-5", "", "claude-haiku-4-5"}; !reflect.DeepEqual(withOverwrite, want) {
		t.Errorf("with overwrite SourceModels() = %q, want %q", withOverwrite, want)
	}
}

func TestPlanTransform_NonAgentFilesNeverContributeAModel(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	dir := src.desc.Paths.Agents.Project
	tpPlaceAgent(t, ws, dir, "plain-notes.md", "plain-notes.md") // carries a model key, not an agent
	tpPlaceAgent(t, ws, dir, "utility.md", "utility.md")

	got := tpPlan(t, ws, src, tgt, false).SourceModels()

	if want := []string{"claude-haiku-4-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SourceModels() = %q, want %q", got, want)
	}
}
