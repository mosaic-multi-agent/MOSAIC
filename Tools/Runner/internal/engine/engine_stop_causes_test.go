package engine_test

// The two stops Next makes on its own carry typed causes: a block with no
// schedule entry (MissingScheduleError) and a completed row whose routing entry
// is not routable (UnroutableEntryError). Tests assert the typed fields and
// that the stop reason is the error text, never the wording.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// nextAfterLogMode calls engine.Next in mode for the step that ended the log.
func nextAfterLogMode(
	aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, log *runLog,
	mode domain.ExecutionMode, src domain.StageSource,
) domain.EngineDecision {
	last := log.last()
	return engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        log.state(),
		LastResponse: responseFor(last),
		Agents:       scopeAgents(aw),
		Seq:          last.Seq,
		Now:          fixedNow,
		StageSource:  src,
		Mode:         mode,
	})
}

// requireMissingSchedule requires a stop whose typed cause is a
// MissingScheduleError and returns it.
func requireMissingSchedule(t *testing.T, dec domain.EngineDecision) *domain.MissingScheduleError {
	t.Helper()
	if dec.Stop == nil {
		t.Fatalf("want a StopDecision for the missing schedule, got %+v", dec)
	}
	var missing *domain.MissingScheduleError
	if !errors.As(dec.Stop.Err, &missing) {
		t.Fatalf("StopDecision.Err: want *MissingScheduleError, got %T (%v)", dec.Stop.Err, dec.Stop.Err)
	}
	if dec.Stop.Reason != dec.Stop.Err.Error() {
		t.Errorf("StopDecision.Reason must equal Err.Error(): %q vs %q", dec.Stop.Reason, dec.Stop.Err.Error())
	}
	return missing
}

// requireUnroutable requires a stop, and nothing else, whose typed cause is an
// UnroutableEntryError with the given fields.
func requireUnroutable(t *testing.T, dec domain.EngineDecision, row domain.WorkflowRow, column, kind string) {
	t.Helper()
	if dec.Dispatch != nil || dec.Deviation != nil || dec.Complete != nil || dec.Consult != nil {
		t.Fatalf("an unroutable entry must produce a stop only, got %+v", dec)
	}
	if dec.Stop == nil {
		t.Fatal("want a StopDecision for the unroutable entry, got none")
	}
	var unroutable *domain.UnroutableEntryError
	if !errors.As(dec.Stop.Err, &unroutable) {
		t.Fatalf("StopDecision.Err: want *UnroutableEntryError, got %T (%v)", dec.Stop.Err, dec.Stop.Err)
	}
	if unroutable.Row != row || unroutable.Column != column || unroutable.Kind != kind {
		t.Errorf("UnroutableEntryError: want row %d column %q kind %q, got row %d column %q kind %q",
			row, column, kind, unroutable.Row, unroutable.Column, unroutable.Kind)
	}
	if dec.Stop.Reason != dec.Stop.Err.Error() {
		t.Errorf("StopDecision.Reason must equal Err.Error(): %q vs %q", dec.Stop.Reason, dec.Stop.Err.Error())
	}
}

// withRowEdit returns aw with row i edited, leaving the original rows intact:
// the hand-built model value of a row the parser would never produce.
func withRowEdit(aw domain.AdmittedWorkflow, i int, edit func(*domain.RoutingRow)) domain.AdmittedWorkflow {
	rows := append([]domain.RoutingRow(nil), aw.Table.Rows...)
	edit(&rows[i])
	aw.Table.Rows = rows
	return aw
}

// A hand-built zero-kind entry under an unset mode keeps the ambiguous-route
// deviation: the unroutable-entry stop belongs to Modes 2 and 3 only.
func TestUnroutable_ZeroSuccessKindUnderUnsetMode_KeepsAmbiguousRouteDeviation(t *testing.T) {
	aw := withRowEdit(mustAdmitScope(t, plainRows(), "", domain.ExecutionModeAuto), 0, func(r *domain.RoutingRow) {
		r.Success = domain.SuccessEntry{}
	})
	log := (&runLog{}).workflowStep("scout", "", domain.StatusSUCCESS, 1).lastInPhase("RESEARCH")

	dec := nextAfterLogMode(aw, nil, log, domain.ExecutionModeUnset, domain.StageSource{})

	dev := requireDeviation(t, dec)
	if dev.Info.Kind != domain.DeviationAmbiguousRoute {
		t.Errorf("unset mode: want kind %q, got %q", domain.DeviationAmbiguousRoute, dev.Info.Kind)
	}
}

// ---- Missing schedule ----

var missingScheduleSources = []struct {
	name string
	src  domain.StageSource
}{
	{"seeded descriptor", domain.StageSource{Path: "/runs/example/Plan.md", Seeded: true}},
	{"unseeded descriptor", domain.StageSource{Path: "/runs/example/Plan.md", Seeded: false}},
	{"no descriptor", domain.StageSource{}},
}

func TestMissingSchedule_InitialDispatch_StopsWithTypedCauseCarryingDescriptor(t *testing.T) {
	for _, tc := range missingScheduleSources {
		t.Run(tc.name, func(t *testing.T) {
			forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				aw := mustAdmitScope(t, blockFirstRows(), stageBlockTables(), mode)

				dec := engine.Next(engine.NextInput{
					Workflow:    aw,
					Schedules:   domain.BlockSchedules{},
					State:       emptyState(),
					Agents:      scopeAgents(aw),
					Now:         fixedNow,
					StageSource: tc.src,
					Mode:        mode,
				})

				missing := requireMissingSchedule(t, dec)
				if missing.Block != "Stage" || missing.Path != tc.src.Path || missing.Seeded != tc.src.Seeded {
					t.Errorf("want block Stage path %q seeded %v, got block %q path %q seeded %v",
						tc.src.Path, tc.src.Seeded, missing.Block, missing.Path, missing.Seeded)
				}
			})
		})
	}
}

func TestMissingSchedule_EnteringBlockFromPriorRow_StopsWithTypedCause(t *testing.T) {
	src := domain.StageSource{Path: "/runs/example/Plan.md", Seeded: true}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, buildBlockRows(), stageBlockTables(), mode)
		log := (&runLog{}).workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING")

		dec := nextAfterLogMode(aw, domain.BlockSchedules{}, log, mode, src)

		missing := requireMissingSchedule(t, dec)
		if missing.Block != "Stage" || missing.Path != src.Path || !missing.Seeded {
			t.Errorf("want block Stage path %q seeded, got block %q path %q seeded %v",
				src.Path, missing.Block, missing.Path, missing.Seeded)
		}
	})
}

func TestMissingSchedule_EnteringBlockWithoutDescriptor_HasEmptyDescriptorFields(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, buildBlockRows(), stageBlockTables(), mode)
		log := (&runLog{}).workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING")

		dec := nextAfterLogMode(aw, domain.BlockSchedules{}, log, mode, domain.StageSource{})

		missing := requireMissingSchedule(t, dec)
		if missing.Path != "" || missing.Seeded || missing.Source != "" {
			t.Errorf("no descriptor: want empty descriptor fields, got path %q seeded %v source %q",
				missing.Path, missing.Seeded, missing.Source)
		}
	})
}

// The second sibling block has no entry while the first one does: the stop
// names the block that is missing, reached on block exit.
func TestMissingSchedule_SecondSiblingBlockWithoutEntry_StopsNamingThatBlock(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, siblingRows(), siblingTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1")...)
		log := (&runLog{}).
			workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
			workflowStep("impl-a", "1", domain.StatusSUCCESS, 2).
			workflowStep("impl-b", "1", domain.StatusSUCCESS, 3)

		dec := nextAfterLogMode(aw, schedules, log, mode, domain.StageSource{})

		if missing := requireMissingSchedule(t, dec); missing.Block != "Audit" {
			t.Errorf("want the missing block Audit, got %q", missing.Block)
		}
	})
}

// ---- Unroutable entries (hand-built model values) ----

func plainRows() []scopeRow {
	return []scopeRow{
		{phase: "RESEARCH", agent: "scout"},
		{phase: "PLANNING", agent: "planner", onFindings: "scout"},
		{phase: "REVIEW", agent: "checker"},
	}
}

func TestUnroutable_ZeroSuccessKind_StopsWithRowAndColumn(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := withRowEdit(mustAdmitScope(t, plainRows(), "", mode), 0, func(r *domain.RoutingRow) {
			r.Success = domain.SuccessEntry{}
		})
		log := (&runLog{}).workflowStep("scout", "", domain.StatusSUCCESS, 1).lastInPhase("RESEARCH")

		dec := nextAfterLogMode(aw, nil, log, mode, domain.StageSource{})

		requireUnroutable(t, dec, 1, "On Success", "")
	})
}

func TestUnroutable_ZeroFindingsKindInAutoReview_StopsWithRowAndColumn(t *testing.T) {
	aw := withRowEdit(mustAdmitScope(t, plainRows(), "", domain.ExecutionModeAutoReview), 1, func(r *domain.RoutingRow) {
		r.Findings = domain.FindingsEntry{}
	})
	log := (&runLog{}).
		workflowStep("scout", "", domain.StatusSUCCESS, 1).lastInPhase("RESEARCH").
		workflowStep("planner", "", domain.StatusCOMPLETED_NEEDS_ACTION, 2).lastInPhase("PLANNING")

	dec := nextAfterLogMode(aw, nil, log, domain.ExecutionModeAutoReview, domain.StageSource{})

	requireUnroutable(t, dec, 2, "On Findings", "")
}

func TestUnroutable_ForkOutsideModeOne_StopsWithForkKind(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := withRowEdit(mustAdmitScope(t, plainRows(), "", mode), 0, func(r *domain.RoutingRow) {
			r.Success = domain.SuccessEntry{
				Kind: domain.SuccessFork,
				Fork: []domain.RowRef{{Cell: "planner", Index: 1}, {Cell: "checker", Index: 2}},
				Raw:  "planner, checker",
			}
		})
		log := (&runLog{}).workflowStep("scout", "", domain.StatusSUCCESS, 1).lastInPhase("RESEARCH")

		dec := nextAfterLogMode(aw, nil, log, mode, domain.StageSource{})

		requireUnroutable(t, dec, 1, "On Success", "fork")
	})
}
