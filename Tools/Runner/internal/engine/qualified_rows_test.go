package engine_test

// Mode-qualified rows: a workflow repeats one agent through a mode qualifier
// (build-review(tests), build-review(impl)). The engine dispatches the bare
// agent id, composes the qualifier focus into the task description, and keeps
// resume and findings route-back on the exact row.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const (
	rowTestsReviewer = 2 // zero-based: build-review(tests)
	rowImplReviewer  = 4 // zero-based: build-review(impl)
)

// qualifiedRows: planner, then the grouped block Stage whose Test and
// Implementation groups each end with a build-review row under a different
// qualifier, then the test-runner.
func qualifiedRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Test", agent: "build-review(tests)", onFindings: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "build-review(impl)", onFindings: "implementer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

func qualifiedSchedules() domain.BlockSchedules {
	return scheduleOf("Stage", idsOf("TDD", "1", "2")...)
}

func focusTask(qualifier string) string {
	return domain.GenericTaskDescription + "\n\n" + engine.QualifierFocusPrefix + " " + qualifier
}

func TestQualifiedRows_AutoRoutedDispatch_UsesBareAgentAndComposesQualifierFocus(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, qualifiedRows(), groupedStageTables(), mode)

		trace := scopeRun{aw: aw, schedules: qualifiedSchedules(), mode: mode}.play(t)

		want := map[int]string{rowTestsReviewer: "tests", rowImplReviewer: "impl"}
		seen := map[int]bool{}
		for _, step := range trace.steps {
			qualifier, qualified := want[step.RowIndex]
			if !qualified {
				continue
			}
			seen[step.RowIndex] = true
			if got := agentName(step.Request.AgentInstanceID); got != "build-review" {
				t.Errorf("row %d instance id: want bare agent build-review, got %s", step.RowIndex, step.Request.AgentInstanceID)
			}
			if step.Agent.Identifier != "build-review" {
				t.Errorf("row %d dispatched agent: want build-review, got %q", step.RowIndex, step.Agent.Identifier)
			}
			if step.Request.TaskDescription != focusTask(qualifier) {
				t.Errorf("row %d task description: want %q, got %q", step.RowIndex, focusTask(qualifier), step.Request.TaskDescription)
			}
		}
		if len(seen) != 2 {
			t.Errorf("both qualified rows must be dispatched, got rows %v", trace.rows())
		}
	})
}

func TestQualifiedRows_UnqualifiedRow_KeepsTaskDescriptionWithoutFocus(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, qualifiedRows(), groupedStageTables(), mode)

		trace := scopeRun{aw: aw, schedules: qualifiedSchedules(), mode: mode}.play(t)

		for _, step := range trace.steps {
			if step.RowIndex == rowTestsReviewer || step.RowIndex == rowImplReviewer {
				continue
			}
			if hasText(step.Request.TaskDescription, engine.QualifierFocusPrefix) {
				t.Errorf("row %d has no qualifier but its task description carries a focus: %q", step.RowIndex, step.Request.TaskDescription)
			}
		}
	})
}

func TestQualifiedRows_PartiallyDoneRedispatch_CarriesFocusThenPreviousStatus(t *testing.T) {
	aw := mustAdmitScope(t, qualifiedRows(), groupedStageTables(), domain.ExecutionModeAuto)
	log := (&runLog{}).workflowStep("build-review", "1.Test", domain.StatusPARTIALLY_DONE, rowTestsReviewer+1)
	resp := &domain.ProtocolResponse{
		AgentInstanceID: log.last().Agent,
		StatusCode:      domain.StatusPARTIALLY_DONE,
		StatusMessage:   "reviewed half of the tests",
	}

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    qualifiedSchedules(),
		State:        log.state(),
		LastResponse: resp,
		Agents:       scopeAgents(aw),
		Seq:          log.last().Seq,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAuto,
	})

	step := requireDispatch(t, dec)
	want := focusTask("tests") + "\n\n" + engine.PartiallyDoneContextPrefix + "\n" + "reviewed half of the tests"
	if step.RowIndex != rowTestsReviewer || step.Request.TaskDescription != want {
		t.Errorf("want row %d re-dispatched with %q, got row %d with %q",
			rowTestsReviewer, want, step.RowIndex, step.Request.TaskDescription)
	}
}

// ---- Engine-built dispatch paths other than the routed success ----

func TestQualifiedRows_InitialDispatchOfQualifiedFirstRow_UsesBareAgentAndFocus(t *testing.T) {
	rows := []scopeRow{
		{phase: "REVIEW", agent: "build-review(tests)"},
		{phase: "REPORT", agent: "closer"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		step := requireDispatch(t, firstDecision(aw, nil, mode))

		if step.Request.AgentInstanceID != "build-review#1" {
			t.Errorf("instance id: want build-review#1, got %s", step.Request.AgentInstanceID)
		}
		if step.Agent.Identifier != "build-review" {
			t.Errorf("dispatched agent: want build-review, got %q", step.Agent.Identifier)
		}
		if step.Request.TaskDescription != focusTask("tests") {
			t.Errorf("task description: want %q, got %q", focusTask("tests"), step.Request.TaskDescription)
		}
	})
}

func TestQualifiedRows_NextAfterRecordedRow_DispatchesQualifiedRowWithFocus(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, repeatedAgentRows("build-review(impl)"), "", mode)
		log := (&runLog{}).workflowStep("build-review", "", domain.StatusSUCCESS, 1).lastInPhase("RESEARCH")

		step := requireDispatch(t, nextAfterLogMode(aw, nil, log, mode, domain.StageSource{}))

		if step.RowIndex != 1 {
			t.Fatalf("want row 1 (build-review(impl)), got row %d", step.RowIndex)
		}
		if got := agentName(step.Request.AgentInstanceID); got != "build-review" {
			t.Errorf("instance id: want bare agent build-review, got %s", step.Request.AgentInstanceID)
		}
		if step.Request.TaskDescription != focusTask("impl") {
			t.Errorf("task description: want %q, got %q", focusTask("impl"), step.Request.TaskDescription)
		}
	})
}

// ---- Findings route-back lands on the row On Findings names ----

// repeatedAgentRows: the same bare agent appears at rows 1 and 2 under
// different qualifiers; the reviewer's On Findings names the given one.
func repeatedAgentRows(findings string) []scopeRow {
	return []scopeRow{
		{phase: "RESEARCH", agent: "build-review(tests)"},
		{phase: "PLANNING", agent: "build-review(impl)"},
		{phase: "REVIEW", agent: "reviewer", onFindings: findings},
		{phase: "REPORT", agent: "closer"},
	}
}

// firstVisitFindings scripts COMPLETED_NEEDS_ACTION for the first visit of
// the reviewer row (row index 2) and SUCCESS for every other step.
func firstVisitFindings() func(int, domain.DispatchStep) domain.StatusCode {
	given := false
	return func(_ int, step domain.DispatchStep) domain.StatusCode {
		if step.RowIndex == 2 && !given {
			given = true
			return domain.StatusCOMPLETED_NEEDS_ACTION
		}
		return domain.StatusSUCCESS
	}
}

func TestQualifiedRows_FindingsRouteBack_LandsOnRowNamedNotOtherRowOfSameAgent(t *testing.T) {
	cases := []struct {
		name      string
		findings  string
		wantRows  []int
		wantFocus string
		// wantFinished are the rows the engine names in a finished-work
		// deviation: the row after the route-back target already ran.
		wantFinished []int
	}{
		{"second qualified row", "build-review(impl)", []int{0, 1, 2, 1, 2, 3}, "impl", nil},
		{"first qualified row", "build-review(tests)", []int{0, 1, 2, 0, 1, 2, 3}, "tests", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmitScope(t, repeatedAgentRows(tc.findings), "", domain.ExecutionModeAutoReview)
			run := scopeRun{
				aw: aw, mode: domain.ExecutionModeAutoReview, statusOf: firstVisitFindings(),
				consultantContinues: true,
			}

			trace := run.play(t)

			requireRowsThenComplete(t, trace, tc.wantRows)
			requireFinishedWorkAt(t, trace, tc.wantFinished, "")
			if len(trace.steps) < 4 {
				t.Fatalf("want a route-back dispatch after the findings, got rows %v", trace.rows())
			}
			routed := trace.steps[3]
			if got := agentName(routed.Request.AgentInstanceID); got != "build-review" {
				t.Errorf("route-back dispatches bare agent build-review, got %s", routed.Request.AgentInstanceID)
			}
			if routed.Request.TaskDescription != focusTask(tc.wantFocus) {
				t.Errorf("route-back task description: want %q, got %q", focusTask(tc.wantFocus), routed.Request.TaskDescription)
			}
		})
	}
}

// ---- Resume continues from the exact recorded row ----

func qualifiedResume(t *testing.T, log *runLog, interrupted bool) (domain.ResumeInfo, error) {
	t.Helper()
	aw := mustAdmitScope(t, qualifiedRows(), groupedStageTables(), domain.ExecutionModeAutoReview)
	return resumeLogIDs(aw, qualifiedSchedules(), log, interrupted)
}

func TestQualifiedRows_ResumeInterrupted_ReturnsExactRecordedRow(t *testing.T) {
	cases := []struct {
		name     string
		row      int // 1-based recorded row
		stage    string
		wantRow  int
		wantBlk  domain.BlockName
		wantIter domain.IterationID
		wantGrp  int
	}{
		{"tests reviewer", rowTestsReviewer + 1, "1.Test", rowTestsReviewer, "Stage", "1", 0},
		{"impl reviewer", rowImplReviewer + 1, "2.Implementation", rowImplReviewer, "Stage", "2", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := (&runLog{}).
				workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
				workflowStep("build-review", tc.stage, domain.StatusSUCCESS, tc.row)

			info, err := qualifiedResume(t, log, true)

			requireResumeBlock(t, info, err, tc.wantRow, true, tc.wantBlk, tc.wantIter)
			requireResumeGroup(t, info, tc.wantGrp)
		})
	}
}

func TestQualifiedRows_ResumeAdvanced_ContinuesAfterExactRecordedRow(t *testing.T) {
	cases := []struct {
		name     string
		row      int // 1-based recorded row
		stage    string
		wantRow  int
		wantIter domain.IterationID
		wantGrp  int
	}{
		{"after tests reviewer: implementation group", rowTestsReviewer + 1, "1.Test", 3, "1", 1},
		{"after impl reviewer: next iteration", rowImplReviewer + 1, "1.Implementation", 1, "2", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := (&runLog{}).
				workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
				workflowStep("build-review", tc.stage, domain.StatusSUCCESS, tc.row)

			info, err := qualifiedResume(t, log, false)

			requireResumeBlock(t, info, err, tc.wantRow, false, "Stage", tc.wantIter)
			requireResumeGroup(t, info, tc.wantGrp)
		})
	}
}

// An entry without a recorded row is matched by the bare agent id and the
// group of its recorded stage value; the whole Subagent cell is never what a
// log entry carries.
func TestQualifiedRows_ResumeEntryWithoutRecordedRow_MatchesBareAgentIdAndGroup(t *testing.T) {
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("build-review", "1.Implementation", domain.StatusSUCCESS, 0)

	info, err := qualifiedResume(t, log, true)

	requireResumeBlock(t, info, err, rowImplReviewer, true, "Stage", "1")
}
