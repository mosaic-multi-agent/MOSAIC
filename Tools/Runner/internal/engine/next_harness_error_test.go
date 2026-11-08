package engine_test

// Routing after a HARNESS_ERROR attempt. The attempt is recorded in the
// Execution Log but never accepted: current_state still names the last accepted
// step (or nothing, when the run's very first dispatch failed). In the auto and
// auto-review modes the engine re-dispatches the failed attempt's own row and
// iteration while the HARNESS_ERROR budget remains and deviates once it is
// spent; the orchestrated mode consults.

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const (
	rtRowResearch   = 1 // RESEARCH, non-staged, first row of the workflow
	heFailureReason = "timeout: harness invocation timed out"
)

// harnessError appends an unaccepted HARNESS_ERROR attempt of a workflow row.
func (l *runLog) harnessError(name, stage string, row int) *runLog {
	return l.stepWith(name, stage, domain.StatusHARNESS_ERROR, domain.ErrorNone, row, heFailureReason)
}

// heInput is the NextInput after the log's last entry, a HARNESS_ERROR attempt.
// accepted is the index of the log entry current_state names, -1 when no step
// has been accepted yet (current_state empty).
func (f retryFixture) heInput(mode domain.ExecutionMode, log *runLog, accepted int) engine.NextInput {
	last := log.last()
	state := log.state()
	state.CurrentState = domain.CurrentState{}
	if accepted >= 0 {
		a := log.entries[accepted]
		state.CurrentState = domain.CurrentState{
			Phase: a.Phase, Stage: a.Stage, LastStatus: a.Status, LastAgent: a.Agent, ErrorCode: a.ErrorCode,
		}
	}
	return engine.NextInput{
		Workflow:  f.aw,
		Schedules: f.schedules,
		State:     state,
		LastResponse: &domain.ProtocolResponse{
			AgentInstanceID: last.Agent,
			StatusCode:      domain.StatusHARNESS_ERROR,
			StatusMessage:   last.Summary,
			ErrorReason:     last.Summary,
		},
		Agents: f.agents,
		Seq:    last.Seq,
		Now:    fixedNow,
		Mode:   mode,
	}
}

func (f retryFixture) nextAfterHE(mode domain.ExecutionMode, log *runLog, accepted int) domain.EngineDecision {
	return engine.Next(f.heInput(mode, log, accepted))
}

// heScenario is a log whose last entry is the HARNESS_ERROR attempt, the
// accepted entry current_state names, and the row and iteration that failed.
type heScenario struct {
	name     string
	build    func(failures int) *runLog
	accepted int // index of the accepted entry; -1 for none
	row      int // 1-based failed row
	stage    string
}

func heScenarios() []heScenario {
	return []heScenario{
		{
			name: "next row after an accepted step",
			build: func(n int) *runLog {
				return (&runLog{}).ok(rtPlanner, "", rtRowPlanner).
					repeat(n, func(l *runLog) { l.harnessError(rtPlanReview, "", rtRowPlanReview) }).inPhase(rtPlanPhase)
			},
			accepted: 0, row: rtRowPlanReview,
		},
		{
			name: "first dispatch of the run",
			build: func(n int) *runLog {
				return (&runLog{}).
					repeat(n, func(l *runLog) { l.harnessError("codebase-research", "", rtRowResearch) }).inPhase("RESEARCH")
			},
			accepted: -1, row: rtRowResearch,
		},
		{
			name: "route-back target inside a block iteration",
			build: func(n int) *runLog {
				return (&runLog{}).ok(rtTestWriter, "1.Test", rtRowTestWriter).
					stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings").
					repeat(n, func(l *runLog) { l.harnessError(rtTestWriter, "1.Test", rtRowTestWriter) })
			},
			accepted: 1, row: rtRowTestWriter, stage: "1.Test",
		},
		{
			name: "later iteration of a block",
			build: func(n int) *runLog {
				return (&runLog{}).ok(rtTestWriter, "1.Test", rtRowTestWriter).ok(rtTestsReview, "1.Test", rtRowTestsReview).
					ok(rtTestWriter, "2.Test", rtRowTestWriter).
					repeat(n, func(l *runLog) { l.harnessError(rtTestsReview, "2.Test", rtRowTestsReview) })
			},
			accepted: 2, row: rtRowTestsReview, stage: "2.Test",
		},
	}
}

func TestNext_HarnessError_BudgetLeft_RedispatchesFailedRowAndIteration(t *testing.T) {
	f := newRetryFixture(t)
	for _, sc := range heScenarios() {
		for _, mode := range autoModes {
			for n := 1; n < engine.HarnessErrorAttemptLimit; n++ {
				t.Run(sc.name+"/"+modeName(mode)+"/failures="+strconv.Itoa(n), func(t *testing.T) {
					log := sc.build(n)

					step := requireRetryStep(t, f.nextAfterHE(mode, log, sc.accepted), sc.row, sc.stage, domain.RetryHarnessError)

					want := sc.agentOfRow(f) + "#" + strconv.Itoa(len(log.entries)+1)
					if step.Request.AgentInstanceID != want {
						t.Errorf("agent instance: want %s, got %s", want, step.Request.AgentInstanceID)
					}
				})
			}
		}
	}
}

func (sc heScenario) agentOfRow(f retryFixture) string {
	return f.aw.Table.Rows[sc.row-1].BareAgentID()
}

// The re-dispatch is built exactly as the BLOCKED/E501 mechanical re-dispatch
// of the same row: same agent, request, phase, stage and HITL.
func TestNext_HarnessError_Redispatch_IsBuiltLikeTheE501Redispatch(t *testing.T) {
	f := newRetryFixture(t)
	heLog := (&runLog{}).ok(rtPlanner, "", rtRowPlanner).harnessError(rtPlanReview, "", rtRowPlanReview).inPhase(rtPlanPhase)
	e501Log := (&runLog{}).ok(rtPlanner, "", rtRowPlanner).blocked(rtPlanReview, "", e501, rtRowPlanReview).inPhase(rtPlanPhase)
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) {
			got := requireRetryStep(t, f.nextAfterHE(mode, heLog, 0), rtRowPlanReview, "", domain.RetryHarnessError)
			want := requireRetryStep(t, f.next(mode, e501Log, false), rtRowPlanReview, "", domain.RetryToolUnavailable)

			got.Retry, want.Retry = "", ""
			if !reflect.DeepEqual(got, want) {
				t.Errorf("re-dispatch step differs from the E501 re-dispatch:\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

func TestNext_HarnessError_BudgetUsedUp_DeviatesWithFailedRowPosition(t *testing.T) {
	f := newRetryFixture(t)
	for _, sc := range heScenarios() {
		for _, mode := range autoModes {
			t.Run(sc.name+"/"+modeName(mode), func(t *testing.T) {
				in := f.heInput(mode, sc.build(engine.HarnessErrorAttemptLimit), sc.accepted)

				dev := requireNonSuccessDeviation(t, engine.Next(in))

				info := dev.Info
				if info.CurrentRow != sc.row-1 {
					t.Errorf("deviation row index: want %d, got %d", sc.row-1, info.CurrentRow)
				}
				if info.CurrentStage != sc.stage {
					t.Errorf("deviation stage: want %q, got %q", sc.stage, info.CurrentStage)
				}
				if want := f.aw.Table.Rows[sc.row-1].Phase; info.CurrentPhase != want {
					t.Errorf("deviation phase: want %q, got %q", want, info.CurrentPhase)
				}
				if info.Response.StatusCode != domain.StatusHARNESS_ERROR || info.Response.ErrorReason != heFailureReason {
					t.Errorf("deviation must carry the HARNESS_ERROR response, got %+v", info.Response)
				}
			})
		}
	}
}

// A success at the failed row and iteration gives a fresh budget.
func TestNext_HarnessError_FailuresAfterSuccessAtSameRow_UseFreshBudget(t *testing.T) {
	f := newRetryFixture(t)
	log := (&runLog{}).
		repeat(engine.HarnessErrorAttemptLimit, func(l *runLog) { l.harnessError(rtPlanReview, "", rtRowPlanReview) }).
		ok(rtPlanReview, "", rtRowPlanReview).
		harnessError(rtPlanReview, "", rtRowPlanReview).inPhase(rtPlanPhase)

	for _, mode := range autoModes {
		requireRetryStep(t, f.nextAfterHE(mode, log, 3), rtRowPlanReview, "", domain.RetryHarnessError)
	}
}

// The two budgets are independent: spent E501 rows do not shorten the
// HARNESS_ERROR budget and spent HARNESS_ERROR rows do not shorten the E501
// budget.
func TestNext_HarnessError_AndE501Budgets_AreIndependent(t *testing.T) {
	f := newRetryFixture(t)
	for _, mode := range autoModes {
		t.Run(modeName(mode)+"/spent E501 budget, one harness error", func(t *testing.T) {
			log := (&runLog{}).
				repeat(engine.E501AttemptLimit, func(l *runLog) { l.blocked(rtPlanner, "", e501, rtRowPlanner) }).
				harnessError(rtPlanner, "", rtRowPlanner).inPhase(rtPlanPhase)

			requireRetryStep(t, f.nextAfterHE(mode, log, engine.E501AttemptLimit-1), rtRowPlanner, "", domain.RetryHarnessError)
		})
		t.Run(modeName(mode)+"/spent harness error budget, one returned E501", func(t *testing.T) {
			log := (&runLog{}).
				repeat(engine.HarnessErrorAttemptLimit, func(l *runLog) { l.harnessError(rtPlanner, "", rtRowPlanner) }).
				blocked(rtPlanner, "", e501, rtRowPlanner).inPhase(rtPlanPhase)

			requireRetryStep(t, f.next(mode, log, false), rtRowPlanner, "", domain.RetryToolUnavailable)
		})
	}
}

// A subagent-returned BLOCKED/E501 keeps using the E501 path only, and a
// HARNESS_ERROR row in the log does not make it a harness retry.
func TestNext_ReturnedE501_AfterUnacceptedHarnessErrorRows_StillUsesE501Budget(t *testing.T) {
	f := newRetryFixture(t)
	log := (&runLog{}).
		repeat(engine.HarnessErrorAttemptLimit-1, func(l *runLog) { l.harnessError(rtPlanner, "", rtRowPlanner) }).
		repeat(engine.E501AttemptLimit, func(l *runLog) { l.blocked(rtPlanner, "", e501, rtRowPlanner) }).
		inPhase(rtPlanPhase)

	for _, mode := range autoModes {
		dev := requireNonSuccessDeviation(t, f.next(mode, log, false))

		if dev.Info.Response.StatusCode != domain.StatusBLOCKED || dev.Info.Response.ErrorCode != e501 {
			t.Errorf("deviation must carry the returned BLOCKED/E501 response, got %+v", dev.Info.Response)
		}
	}
}

// An accepted step after earlier HARNESS_ERROR attempts routes as any accepted
// step: those rows are history, not a retry state.
func TestNext_AcceptedStepAfterHarnessErrorRows_RoutesNormally(t *testing.T) {
	f := newRetryFixture(t)
	log := (&runLog{}).
		repeat(engine.HarnessErrorAttemptLimit-1, func(l *runLog) { l.harnessError(rtPlanner, "", rtRowPlanner) }).
		ok(rtPlanner, "", rtRowPlanner).inPhase(rtPlanPhase)

	for _, mode := range autoModes {
		step := requireDispatch(t, f.next(mode, log, false))

		requireStepAt(t, step, rtRowPlanReview-1, "")
		if step.Retry != domain.RetryNone {
			t.Errorf("dispatch Retry: want none, got %q", step.Retry)
		}
	}
}

func TestNext_HarnessError_Orchestrated_ConsultsWithFailedRowPosition(t *testing.T) {
	f := newRetryFixture(t)
	for _, sc := range heScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			consult := requireConsult(t, f.nextAfterHE(domain.ExecutionModeOrchestrated, sc.build(1), sc.accepted))

			if consult.Trigger != domain.ConsultTriggerOrchestratedMode {
				t.Errorf("trigger: want orchestrated mode, got %q", consult.Trigger)
			}
			if consult.CurrentRow != sc.row-1 {
				t.Errorf("consult row index: want %d, got %d", sc.row-1, consult.CurrentRow)
			}
			if consult.CurrentStage != sc.stage {
				t.Errorf("consult stage: want %q, got %q", sc.stage, consult.CurrentStage)
			}
		})
	}
}

// The engine refuses to guess: a HARNESS_ERROR response whose attempt is not in
// the log, or whose recorded row is not the agent's, stops naming the position.
func TestNext_HarnessError_AttemptNotResolvable_StopsWithPositionError(t *testing.T) {
	f := newRetryFixture(t)
	cases := map[string]func(*runLog) engine.NextInput{
		"no log entry for the responding instance": func(log *runLog) engine.NextInput {
			in := f.heInput(domain.ExecutionModeAuto, log, 0)
			in.LastResponse.AgentInstanceID = rtPlanReview + "#99"
			return in
		},
		"recorded row holds another agent": func(log *runLog) engine.NextInput {
			log.entries[len(log.entries)-1].WorkflowRow = domain.WorkflowRow(rtRowTestWriter)
			return f.heInput(domain.ExecutionModeAuto, log, 0)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			log := (&runLog{}).ok(rtPlanner, "", rtRowPlanner).harnessError(rtPlanReview, "", rtRowPlanReview).inPhase(rtPlanPhase)

			stop := requireStop(t, engine.Next(build(log)))

			var perr *domain.PositionUnresolvedError
			if !errors.As(stop.Err, &perr) {
				t.Errorf("stop must carry a *PositionUnresolvedError, got Err=%v (reason %q)", stop.Err, stop.Reason)
			}
		})
	}
}
