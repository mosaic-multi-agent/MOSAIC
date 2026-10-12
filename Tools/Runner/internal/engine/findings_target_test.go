package engine_test

// On Findings route-back and run-scope On Success targets are resolved by the
// whole Subagent cell a target names, so an agent that fills several routing
// rows is addressed through its mode qualifiers:
//
//   - On Findings goes to the row its cell names, a row of the same scope, in
//     whichever group, stage or direction it lies.
//   - A run-scope On Success naming a qualified agent goes to the one row whose
//     cell it names.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== On Findings: the named row =====

// twoGroupFindingsContent has the same fixer in the Test group (row 1) and in
// the Implementation group (row 3), told apart by mode qualifiers. Each group
// ends with its own gate, whose On Findings names the fixer of its group.
const twoGroupFindingsContent = v216Columns +
	`| EXECUTION.[Stage].Test | fixer(tests) | FALSE | NEXT | - | - | Stage-{Stage}/FixTests.md |
| EXECUTION.[Stage].Test | gate-tests | FALSE | COMPLETE | fixer(tests) | - | Stage-{Stage}/GateTests.md |
| EXECUTION.[Stage].Implementation | fixer(impl) | FALSE | NEXT | - | - | Stage-{Stage}/FixImpl.md |
| EXECUTION.[Stage].Implementation | gate-impl | FALSE | COMPLETE | fixer(impl) | - | Stage-{Stage}/GateImpl.md |
` + v216StageBlock + `
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
`

const (
	tgRowFixTests = 1
	tgRowGateTest = 2
	tgRowFixImpl  = 3
	tgRowGateImpl = 4
)

func twoGroupAgents() map[string]domain.AgentReference {
	return newTestAgents("fixer", "gate-tests", "gate-impl")
}

// twoGroupLogAtImplGate is the run up to the Implementation-group gate
// returning findings.
func twoGroupLogAtImplGate() *runLog {
	log := &runLog{}
	return log.
		workflowStep("fixer", "1.Test", domain.StatusSUCCESS, tgRowFixTests).
		workflowStep("gate-tests", "1.Test", domain.StatusSUCCESS, tgRowGateTest).
		workflowStep("fixer", "1.Implementation", domain.StatusSUCCESS, tgRowFixImpl).
		workflowStep("gate-impl", "1.Implementation", domain.StatusCOMPLETED_NEEDS_ACTION, tgRowGateImpl)
}

func TestNext_OnFindings_SameAgentInTwoGroups_RoutesToFixerNamedByTheGate(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoGroupFindingsContent, "two-group-findings", "1.0")

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), twoGroupAgents(), twoGroupLogAtImplGate())

	if got := agentName(step.Request.AgentInstanceID); got != "fixer" {
		t.Errorf("findings must route to fixer, got %s", got)
	}
	requireStepAt(t, step, tgRowFixImpl-1, "1.Implementation")
}

func TestNext_OnFindings_SameAgentInTwoGroups_TestGateStillRoutesToTestGroupRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoGroupFindingsContent, "two-group-findings", "1.0")
	log := &runLog{}
	log.workflowStep("fixer", "1.Test", domain.StatusSUCCESS, tgRowFixTests).
		workflowStep("gate-tests", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, tgRowGateTest)

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), twoGroupAgents(), log)

	requireStepAt(t, step, tgRowFixTests-1, "1.Test")
}

func TestNext_OnFindings_NamedRowRoute_InjectsReviewOutputs(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoGroupFindingsContent, "two-group-findings", "1.0")
	log := twoGroupLogAtImplGate()
	reviewOutput := "Stage-1/GateImpl.md"

	dec := engine.Next(engine.NextInput{
		Workflow:            aw,
		Schedules:           singleSchedules("TDD"),
		State:               log.state(),
		LastResponse:        cnaResponse(log.last().Agent),
		LastOutputArtifacts: []string{reviewOutput},
		Agents:              twoGroupAgents(),
		Seq:                 log.last().Seq,
		Now:                 fixedNow,
		Mode:                domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	requireStepAt(t, step, tgRowFixImpl-1, "1.Implementation")
	found := false
	for _, a := range step.Request.InputArtifacts {
		if a == reviewOutput {
			found = true
		}
	}
	if !found {
		t.Errorf("want review output %q among inputs, got %v", reviewOutput, step.Request.InputArtifacts)
	}
}

func TestNext_OnFindings_NamedRowRoute_ReviewLoopLimitStillApplies(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoGroupFindingsContent, "two-group-findings", "1.0")
	log := twoGroupLogAtImplGate()
	state := log.state()
	state.ReviewLoopLimit = 1

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, cnaResponse(log.last().Agent))

	dev := requireDeviation(t, dec)
	if dev.Info.Kind != domain.DeviationReviewLoopLimit {
		t.Errorf("want review loop limit deviation, got %q", dev.Info.Kind)
	}
}

// findingsLoopContent has one agent filling three Test-group rows and one
// Implementation-group row; the gate row names the writer row of that same
// agent as its On Findings target.
const findingsLoopContent = v216Columns +
	`| EXECUTION.[Stage].Test | loop-agent(writer) | FALSE | NEXT | - | - | Stage-{Stage}/Written.md |
| EXECUTION.[Stage].Test | loop-agent(gate) | FALSE | NEXT | loop-agent(writer) | - | Stage-{Stage}/Gate.md |
| EXECUTION.[Stage].Test | loop-agent(review) | FALSE | COMPLETE | - | - | Stage-{Stage}/Review.md |
| EXECUTION.[Stage].Implementation | loop-agent(impl) | FALSE | COMPLETE | - | - | Stage-{Stage}/Impl.md |
` + v216StageBlock + `
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
`

const (
	findingsLoopRowWriter = 1
	findingsLoopRowGate   = 2
	findingsLoopRowReview = 3
	findingsLoopRowImpl   = 4
)

func TestNext_OnFindings_SameAgentWriterGateReview_RoutesToWriterRowNamedByGate(t *testing.T) {
	aw := mustParseAndAdmitV216(t, findingsLoopContent, "findings-loop", "1.0")
	log := &runLog{}
	log.workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, findingsLoopRowWriter).
		workflowStep("loop-agent", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, findingsLoopRowGate)

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), newTestAgents("loop-agent"), log)

	requireStepAt(t, step, findingsLoopRowWriter-1, "1.Test")
}

// targetBelowContent has the On Findings target only below the reviewer.
const targetBelowContent = v216Columns +
	`| PLANNING | checker | FALSE | NEXT | fixer | - | check.md |
| DESIGN | fixer | FALSE | COMPLETE | - | - | Plan.md |
`

func TestNext_OnFindings_TargetBelow_RoutesToNamedRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, targetBelowContent, "target-below", "1.0")
	state := stateAfterRow("PLANNING", "", "checker#1", domain.StatusCOMPLETED_NEEDS_ACTION, 1, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    singleSchedules("TDD"),
		State:        state,
		LastResponse: cnaResponse("checker#1"),
		Agents:       newTestAgents("checker", "fixer"),
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if step.RowIndex != 1 {
		t.Errorf("want the fixer row below the checker (index 1), got index %d", step.RowIndex)
	}
}

func TestNext_OnFindings_SameAgentAboveAndBelow_RoutesToTheRowNamed(t *testing.T) {
	const content = v216Columns +
		`| RESEARCH | fixer(draft) | FALSE | checker | - | - | Plan.md |
| PLANNING | checker | FALSE | NEXT | fixer(draft) | - | check.md |
| DESIGN | fixer(final) | FALSE | COMPLETE | - | - | Final.md |
`
	aw := mustParseAndAdmitV216(t, content, "above-and-below", "1.0")
	state := stateAfterRow("PLANNING", "", "checker#2", domain.StatusCOMPLETED_NEEDS_ACTION, 2, 2)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    singleSchedules("TDD"),
		State:        state,
		LastResponse: cnaResponse("checker#2"),
		Agents:       newTestAgents("checker", "fixer"),
		Seq:          2,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if step.RowIndex != 0 {
		t.Errorf("want the fixer(draft) row named by the checker (index 0), got index %d", step.RowIndex)
	}
}

// ===== Run-scope On Success naming a qualified agent =====

// nextAfterSuccess calls engine.Next for a SUCCESS of agentID, recorded as
// running the given 1-based row of the admitted content.
func nextAfterSuccess(t *testing.T, content, phase, agentID string, seq, row int) domain.DispatchStep {
	t.Helper()
	aw := mustParseAndAdmitV216(t, content, "on-success-lookup", "1.0")
	state := stateAfterRow(phase, "", agentID, domain.StatusSUCCESS, seq, row)
	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    singleSchedules("TDD"),
		State:        state,
		LastResponse: successResponse(agentID),
		Agents:       scopeAgents(aw),
		Seq:          seq,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})
	return requireDispatch(t, dec)
}

func TestNext_RunScopeOnSuccess_QualifiedAgent_DispatchesTheRowNamed(t *testing.T) {
	const content = v216Columns +
		`| RESEARCH | checker(draft) | FALSE | NEXT | - | - | a.md |
| PLANNING | drafter | FALSE | checker(final) | - | - | b.md |
| REVIEW | checker(final) | FALSE | COMPLETE | - | - | c.md |
`

	step := nextAfterSuccess(t, content, "PLANNING", "drafter#1", 1, 2)

	if got := agentName(step.Request.AgentInstanceID); got != "checker" || step.RowIndex != 2 {
		t.Errorf("want checker (final) at index 2, got %s at index %d", got, step.RowIndex)
	}
}

func TestNext_RunScopeOnSuccess_UniqueAgents_DispatchesNamedRow(t *testing.T) {
	step := nextAfterSuccess(t, quickFixV216Content, "PLANNING", "planner-tdd-soft#1", 1, 1)

	if got := agentName(step.Request.AgentInstanceID); got != "plan-review" || step.RowIndex != 1 {
		t.Errorf("want plan-review at index 1, got %s at index %d", got, step.RowIndex)
	}
}

// ===== Full sequence of the staged findings loop =====

// stagedLoopStep is one step of the loop run and what must be dispatched next.
type stagedLoopStep struct {
	name      string
	stage     string
	status    domain.StatusCode
	row       int
	wantIndex int
	wantStage string
}

// runStagedLoop replays the steps against the staged findings loop workflow,
// checking the next dispatch after each. The last step has no further dispatch
// when wantComplete is set.
func runStagedLoop(t *testing.T, schedules domain.BlockSchedules, steps []stagedLoopStep, wantComplete bool) {
	t.Helper()
	aw := mustParseAndAdmitV216(t, findingsLoopContent, "findings-loop", "1.0")
	agents := newTestAgents("loop-agent")
	log := &runLog{}
	for i, s := range steps {
		log.workflowStep("loop-agent", s.stage, s.status, s.row)
		snapshot := &runLog{entries: append([]domain.ExecutionLogEntry(nil), log.entries...)}
		last := i == len(steps)-1

		t.Run(s.name, func(t *testing.T) {
			dec := nextAfterLogIDs(aw, schedules, agents, snapshot)
			if last && wantComplete {
				requireComplete(t, dec)
				return
			}
			requireStepAt(t, requireDispatch(t, dec), s.wantIndex, s.wantStage)
		})
	}
}

func TestNext_StagedFindingsLoop_SingleStage_RunsAllSixStepsInOrder(t *testing.T) {
	const (
		writer = findingsLoopRowWriter
		gate   = findingsLoopRowGate
		review = findingsLoopRowReview
		impl   = findingsLoopRowImpl
	)
	steps := []stagedLoopStep{
		{"writer", "1.Test", domain.StatusSUCCESS, writer, gate - 1, "1.Test"},
		{"gate needs action", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, gate, writer - 1, "1.Test"},
		{"writer again", "1.Test", domain.StatusSUCCESS, writer, gate - 1, "1.Test"},
		{"gate succeeds", "1.Test", domain.StatusSUCCESS, gate, review - 1, "1.Test"},
		{"review", "1.Test", domain.StatusSUCCESS, review, impl - 1, "1.Implementation"},
		{"impl", "1.Implementation", domain.StatusSUCCESS, impl, 0, ""},
	}

	runStagedLoop(t, singleSchedules("TDD"), steps, true)
}

func TestNext_StagedFindingsLoop_RouteBackInStageOne_StageTwoRunsEveryRow(t *testing.T) {
	const (
		writer = findingsLoopRowWriter
		gate   = findingsLoopRowGate
		review = findingsLoopRowReview
		impl   = findingsLoopRowImpl
	)
	steps := []stagedLoopStep{
		{"stage 1 writer", "1.Test", domain.StatusSUCCESS, writer, gate - 1, "1.Test"},
		{"stage 1 gate needs action", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, gate, writer - 1, "1.Test"},
		{"stage 1 writer again", "1.Test", domain.StatusSUCCESS, writer, gate - 1, "1.Test"},
		{"stage 1 gate succeeds", "1.Test", domain.StatusSUCCESS, gate, review - 1, "1.Test"},
		{"stage 1 review", "1.Test", domain.StatusSUCCESS, review, impl - 1, "1.Implementation"},
		{"stage 1 impl", "1.Implementation", domain.StatusSUCCESS, impl, writer - 1, "2.Test"},
		{"stage 2 writer", "2.Test", domain.StatusSUCCESS, writer, gate - 1, "2.Test"},
		{"stage 2 gate", "2.Test", domain.StatusSUCCESS, gate, review - 1, "2.Test"},
		{"stage 2 review", "2.Test", domain.StatusSUCCESS, review, impl - 1, "2.Implementation"},
		{"stage 2 impl", "2.Implementation", domain.StatusSUCCESS, impl, 0, ""},
	}

	runStagedLoop(t, twoSchedules("TDD"), steps, true)
}
