package session_test

// Tests that a consultation-routed step that uses a row's defaults gets
// exactly the inputs, outputs and HITL value an engine-routed dispatch of the
// same row and stage gets, and that explicit overrides reach the agent.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

// defaultsSeed is a run whose last step is the build-review of stage 1, so the
// engine's next dispatch is the test-writer of stage 2 (plan stage HITL true).
func defaultsSeed() domain.ArtifactState {
	return rowStageSeed("build-review", 4, "EXECUTION", "1.Implementation")
}

// engineRoutedTestWriter returns the request the engine-routed dispatch of the
// test-writer at stage 2 sends to the harness.
func engineRoutedTestWriter(t *testing.T) domain.ProtocolRequest {
	t.Helper()
	rig := newDefaultsRig(t, stoppingConsultant(3), domain.ExecutionModeAuto, defaultsSeed())
	queueSuccess(rig.f, "test-writer", "test-writer#3")

	_, err := rig.ses.Start(context.Background(), rig.cfg)

	if err != nil {
		t.Fatalf("engine-routed run: %v", err)
	}
	return firstRequestTo(t, rig.f, "test-writer")
}

// consultRoutedTestWriter returns the request the consultation-routed dispatch
// of the test-writer at stage 2 sends to the harness; shape queues the
// instruction (with any overrides) before the run.
func consultRoutedTestWriter(t *testing.T, shape func(c *scriptedRoutingConsultant)) domain.ProtocolRequest {
	t.Helper()
	consultant := &scriptedRoutingConsultant{}
	shape(consultant)
	consultant.queueStop("done")
	rig := newDefaultsRig(t, consultant, domain.ExecutionModeOrchestrated, defaultsSeed())
	queueSuccess(rig.f, "test-writer", "test-writer#3")

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	return firstRequestTo(t, rig.f, "test-writer")
}

func TestSession_ConsultRoute_NoOverrides_GetsSameInputsOutputsAndHITLAsEngineRouting(t *testing.T) {
	engineReq := engineRoutedTestWriter(t)

	consultReq := consultRoutedTestWriter(t, func(c *scriptedRoutingConsultant) {
		c.queueStagedDispatch("test-writer", "write the tests for stage 2", cdTestWriter, "2")
	})

	if !engineReq.HumanInTheLoop {
		t.Fatalf("fixture check: the engine-routed stage 2 dispatch should be HITL (plan stage HITL), got false")
	}
	if !reflect.DeepEqual(consultReq.InputArtifacts, engineReq.InputArtifacts) {
		t.Errorf("inputs differ:\n consult %v\n engine  %v", consultReq.InputArtifacts, engineReq.InputArtifacts)
	}
	if !reflect.DeepEqual(consultReq.OutputArtifacts, engineReq.OutputArtifacts) {
		t.Errorf("outputs differ:\n consult %v\n engine  %v", consultReq.OutputArtifacts, engineReq.OutputArtifacts)
	}
	if consultReq.HumanInTheLoop != engineReq.HumanInTheLoop {
		t.Errorf("HITL differs: consult %v, engine %v", consultReq.HumanInTheLoop, engineReq.HumanInTheLoop)
	}
}

// writeStagePlanFiles writes one Plan.md per plan iteration into the run
// folder, so that a Stage-*/Plan.md Input wildcard has artifacts to list.
func writeStagePlanFiles(t *testing.T, runFolder string) {
	t.Helper()
	for _, rel := range []string{"Stage-1/Plan.md", "Stage-2/Plan.md"} {
		writeRunFile(t, runFolder, rel, "# Plan\n")
	}
}

// For a row outside the block the consultation path must list the artifacts a
// Stage-* default input matches, exactly as the engine-routed dispatch does.
// The seed holds the completed last step of both iterations, so the engine
// leaves the block for the final review.
func TestSession_ConsultRoute_RowOutsideBlockWithStageWildcardDefault_GetsSameInputsAsEngineRouting(t *testing.T) {
	seed := func() domain.ArtifactState {
		s := rowStageSeed("build-review", 4, "EXECUTION", "2.Implementation")
		first := domain.ExecutionLogEntry{
			Seq: 1, Agent: "build-review#1", Phase: "EXECUTION", Stage: "1.Implementation",
			WorkflowRow: 4, Status: domain.StatusSUCCESS,
		}
		s.ExecutionLog = append([]domain.ExecutionLogEntry{first}, s.ExecutionLog...)
		return s
	}
	engineRig := newDefaultsRig(t, stoppingConsultant(3), domain.ExecutionModeAuto, seed())
	writeStagePlanFiles(t, engineRig.cfg.RunFolder)
	queueSuccess(engineRig.f, "final-review", "final-review#3")
	if _, err := engineRig.ses.Start(context.Background(), engineRig.cfg); err != nil {
		t.Fatalf("engine-routed run: %v", err)
	}
	engineReq := firstRequestTo(t, engineRig.f, "final-review")

	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("final-review", "final review", cdFinalReview)
	consultant.queueStop("done")
	consultRig := newDefaultsRig(t, consultant, domain.ExecutionModeOrchestrated, seed())
	writeStagePlanFiles(t, consultRig.cfg.RunFolder)
	queueSuccess(consultRig.f, "final-review", "final-review#3")
	got, err := consultRig.ses.Start(context.Background(), consultRig.cfg)
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	consultReq := firstRequestTo(t, consultRig.f, "final-review")

	if len(engineReq.InputArtifacts) < 2 {
		t.Fatalf("fixture check: engine-routed Stage-* default should expand per stage, got %v", engineReq.InputArtifacts)
	}
	if !reflect.DeepEqual(consultReq.InputArtifacts, engineReq.InputArtifacts) {
		t.Errorf("inputs differ:\n consult %v\n engine  %v", consultReq.InputArtifacts, engineReq.InputArtifacts)
	}
	if !reflect.DeepEqual(consultReq.OutputArtifacts, engineReq.OutputArtifacts) {
		t.Errorf("outputs differ:\n consult %v\n engine  %v", consultReq.OutputArtifacts, engineReq.OutputArtifacts)
	}
}

// An explicit Input wildcard is listed like a declared one: the agent receives
// the existing artifacts it matches, in lexical order.
func TestSession_ConsultRoute_ExplicitStageWildcardInput_ListsTheMatchingArtifacts(t *testing.T) {
	override := []string{"Stage-*/Plan.md"}
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatchWithInputs("test-writer", "write the tests for stage 2", cdTestWriter, &override)
	consultant.stageLastDispatch("2")
	consultant.queueStop("done")
	rig := newDefaultsRig(t, consultant, domain.ExecutionModeOrchestrated, defaultsSeed())
	writeStagePlanFiles(t, rig.cfg.RunFolder)
	queueSuccess(rig.f, "test-writer", "test-writer#3")

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	req := firstRequestTo(t, rig.f, "test-writer")
	want := []string{"Stage-1/Plan.md", "Stage-2/Plan.md"}
	if inputs := bare(req, req.InputArtifacts); !reflect.DeepEqual(inputs, want) {
		t.Errorf("explicit wildcard inputs = %v, want the matching artifacts %v", inputs, want)
	}
}

func TestSession_ConsultRoute_ExplicitEmptyInputs_AreNotReplacedByDefaults(t *testing.T) {
	override := []string{}

	req := consultRoutedTestWriter(t, func(c *scriptedRoutingConsultant) {
		c.queueDispatchWithInputs("test-writer", "write the tests for stage 2", cdTestWriter, &override)
		c.stageLastDispatch("2")
	})

	if len(req.InputArtifacts) != 0 {
		t.Errorf("explicit empty inputs: want none, got %v", req.InputArtifacts)
	}
}

func TestSession_ConsultRoute_ExplicitOutputsWithStageToken_ReachAgentUnresolved(t *testing.T) {
	override := []string{"Stage-{Stage}/custom.md", "Stage-*/Plan.md"}

	req := consultRoutedTestWriter(t, func(c *scriptedRoutingConsultant) {
		c.queueDispatchWithOutputs("test-writer", "write the tests for stage 2", cdTestWriter, &override)
		c.stageLastDispatch("2")
	})

	if got := bare(req, req.OutputArtifacts); !reflect.DeepEqual(got, override) {
		t.Errorf("explicit outputs = %v, want %v verbatim", got, override)
	}
}

// Guards against an implementation that ORs an explicit HITL override with the
// plan stage HITL: the stage is HITL, the explicit override says no.
func TestSession_ConsultRoute_ExplicitHITLFalse_WinsOverPlanStageHITL(t *testing.T) {
	off := false

	req := consultRoutedTestWriter(t, func(c *scriptedRoutingConsultant) {
		c.queueDispatchWithHITL("test-writer", "write the tests for stage 2", cdTestWriter, &off)
		c.stageLastDispatch("2")
	})

	if req.HumanInTheLoop {
		t.Errorf("explicit HITL override false should win over the plan stage HITL, got true")
	}
}

// Guards the verbatim rule in the other direction: neither the plan stage nor
// the row is HITL, the explicit override says yes.
func TestSession_ConsultRoute_ExplicitHITLTrue_WinsOverNonHITLStageAndRow(t *testing.T) {
	on := true

	req := consultRoutedTestWriter(t, func(c *scriptedRoutingConsultant) {
		c.queueDispatchWithHITL("test-writer", "write the tests for stage 1", cdTestWriter, &on)
		c.stageLastDispatch("1")
	})

	if !req.HumanInTheLoop {
		t.Errorf("explicit HITL override true should win over a non-HITL stage and row, got false")
	}
}
