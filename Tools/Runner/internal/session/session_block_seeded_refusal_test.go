package session_test

// Tests that a defective seeded block schedule is refused before the first
// dispatch, in every mode, naming the rule, the schedule row or token, the
// block, the source and the workflow; that a refused new run's folder is
// removed; and that the same refusal applies at resume.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

const seededWorkflow = "seeded-plan-first-block"

// seededDefect is a seeded schedule that breaks one schedule rule, with the items the
// refusal must carry.
type seededDefect struct {
	name   string
	block  string // declared block of the schedule
	source string // declared Source of the schedule
	plan   string
	rule   string
	row    int    // 1-based schedule row; 0 when the defect is not about a row
	token  string // offending ID or heading
}

var seededDefects = []seededDefect{
	{
		name: "duplicate IDs", block: "Stage", source: "Plan.md",
		plan: `# Plan

## Stages

| Stage | Name | Goal | HITL |
|-------|------|------|:----:|
| DUP-1 | One | First | FALSE |
| B | Two | Second | FALSE |
| dup-1 | Three | Repeats the first ID in other case | FALSE |
`,
		rule: "V2", row: 3, token: "dup-1",
	},
	{
		name: "missing section", block: "Stage", source: "Plan.md",
		plan: `# Plan

## Tasks

| Stage | Name | Goal | HITL |
|-------|------|------|:----:|
| A | One | First | FALSE |
`,
		rule: "V1", row: 0, token: "## Stages",
	},
}

// requireSeededRefusal asserts the outcome is a refusal of the defective
// seeded schedule: structured items, text that names them, and no dispatch.
func requireSeededRefusal(t *testing.T, rig *blockRig, got domain.RunOutcome, err error, resource string, d seededDefect) {
	t.Helper()
	requireRefused(t, got, err)

	var refusal *domain.RefusalError
	if !errors.As(got.Cause, &refusal) {
		t.Fatalf("want outcome Cause to carry a *domain.RefusalError, got %v (%T)", got.Cause, got.Cause)
	}
	if refusal.Resource != resource {
		t.Errorf("RefusalError.Resource: want %q, got %q", resource, refusal.Resource)
	}
	if refusal.Rule != d.rule {
		t.Errorf("RefusalError.Rule: want %q, got %q", d.rule, refusal.Rule)
	}
	if refusal.Token != d.token {
		t.Errorf("RefusalError.Token: want %q, got %q", d.token, refusal.Token)
	}

	var defect *domain.ScheduleDefectError
	if !errors.As(got.Cause, &defect) {
		t.Fatalf("want the refusal to wrap a *domain.ScheduleDefectError, got %v", got.Cause)
	}
	if defect.Rule != d.rule || defect.Row != d.row || defect.Token != d.token {
		t.Errorf("defect: want rule %q row %d token %q, got rule %q row %d token %q",
			d.rule, d.row, d.token, defect.Rule, defect.Row, defect.Token)
	}
	if string(defect.Block) != d.block || defect.Source != d.source {
		t.Errorf("defect: want block %q and source %q, got block %q source %q", d.block, d.source, defect.Block, defect.Source)
	}
	for _, want := range []string{d.block, d.source} {
		if !strings.Contains(refusal.Reason, want) {
			t.Errorf("want RefusalError.Reason to name %q, got %q", want, refusal.Reason)
		}
	}

	for _, want := range []string{d.rule, d.token, d.block, d.source, resource} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("want the refusal message to name %q, got %q", want, got.Message)
		}
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	if n := rig.consult.CallCount; n != 0 {
		t.Errorf("want no consultation before the refusal, got %d", n)
	}
}

// TestSession_Start_DefectiveSeededSchedule_RefusedBeforeAnyDispatch covers
// the refusal for both defects in each execution mode.
func TestSession_Start_DefectiveSeededSchedule_RefusedBeforeAnyDispatch(t *testing.T) {
	modes := []domain.ExecutionMode{
		domain.ExecutionModeAuto,
		domain.ExecutionModeAutoReview,
		domain.ExecutionModeOrchestrated,
	}
	for _, d := range seededDefects {
		for _, mode := range modes {
			t.Run(d.name+"/"+string(mode), func(t *testing.T) {
				rig := newBlockRig(t, seededWorkflow+"-orch.md", nil, "implementer", "reviewer", "test-runner")
				rig.writeRunFile(t, "Plan.md", d.plan)

				got, err := rig.ses.Start(context.Background(), rig.config(seededWorkflow, mode))

				requireSeededRefusal(t, rig, got, err, seededWorkflow, d)
			})
		}
	}
}

// TestSession_Start_DefectiveSeededSchedule_NewRunFolderRemoved verifies that
// the folder of a refused new run does not stay behind.
func TestSession_Start_DefectiveSeededSchedule_NewRunFolderRemoved(t *testing.T) {
	rig := newBlockRig(t, seededWorkflow+"-orch.md", nil, "implementer", "reviewer", "test-runner")
	rig.writeRunFile(t, "Plan.md", seededDefects[0].plan)

	got, err := rig.ses.Start(context.Background(), rig.config(seededWorkflow, domain.ExecutionModeAuto))

	requireSeededRefusal(t, rig, got, err, seededWorkflow, seededDefects[0])
	if _, statErr := os.Stat(rig.runFolder); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("want the refused new run's folder removed, stat error: %v", statErr)
	}
}

// TestSession_Start_Resume_DefectiveSeededSchedule_RefusedBeforeAnyDispatch
// verifies that a resume refuses a defective seeded schedule the same way,
// naming the run as the resource.
func TestSession_Start_Resume_DefectiveSeededSchedule_RefusedBeforeAnyDispatch(t *testing.T) {
	rig := newBlockRig(t, seededWorkflow+"-orch.md", nil, "implementer", "reviewer", "test-runner")
	rig.writeRunFile(t, "Plan.md", seededDefects[0].plan)
	rig.resumeFrom(seededWorkflow, domain.ExecutionModeAuto)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(seededWorkflow, domain.ExecutionModeAuto))

	d := seededDefects[0]
	requireRefused(t, got, err)
	var defect *domain.ScheduleDefectError
	if !errors.As(got.Cause, &defect) {
		t.Fatalf("want the refusal to wrap a *domain.ScheduleDefectError, got %v", got.Cause)
	}
	if defect.Rule != d.rule || defect.Row != d.row || defect.Token != d.token {
		t.Errorf("defect: want rule %q row %d token %q, got rule %q row %d token %q",
			d.rule, d.row, d.token, defect.Rule, defect.Row, defect.Token)
	}
	var refusal *domain.RefusalError
	if errors.As(got.Cause, &refusal) && refusal.Resource != testRunID {
		t.Errorf("RefusalError.Resource: want the run %q, got %q", testRunID, refusal.Resource)
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	requireRunFolderSurvives(t, rig, "Plan.md")
}

// TestSession_Start_Resume_MissingSeededSource_RefusedBeforeAnyDispatch
// verifies that a resume whose seeded Source file does not exist is refused
// before any dispatch rather than started without a schedule.
func TestSession_Start_Resume_MissingSeededSource_RefusedBeforeAnyDispatch(t *testing.T) {
	rig := newBlockRig(t, seededWorkflow+"-orch.md", nil, "implementer", "reviewer", "test-runner")
	rig.resumeFrom(seededWorkflow, domain.ExecutionModeAuto)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(seededWorkflow, domain.ExecutionModeAuto))

	msg := requireRefused(t, got, err)
	var defect *domain.ScheduleDefectError
	if !errors.As(got.Cause, &defect) {
		t.Fatalf("want the refusal to wrap a *domain.ScheduleDefectError, got %v", got.Cause)
	}
	if !errors.Is(defect, fs.ErrNotExist) {
		t.Errorf("want the defect to report a missing source (fs.ErrNotExist), got cause %v", defect.Cause)
	}
	if defect.Source != "Plan.md" || defect.Token != "Plan.md" {
		t.Errorf("defect: want source and token Plan.md, got source %q token %q", defect.Source, defect.Token)
	}
	if !strings.Contains(msg, "Plan.md") {
		t.Errorf("want the refusal message to name Plan.md, got %q", msg)
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	requireRunFolderSurvives(t, rig, "")
}

// requireRunFolderSurvives asserts that a refused resume left the existing
// run folder in place, and the file rel in it when rel is not empty: only a
// refused new run removes its folder.
func requireRunFolderSurvives(t *testing.T, rig *blockRig, rel string) {
	t.Helper()
	if _, statErr := os.Stat(rig.runFolder); statErr != nil {
		t.Errorf("want the resumed run's folder kept after the refusal, stat error: %v", statErr)
	}
	if rel == "" {
		return
	}
	if _, statErr := os.Stat(filepath.Join(rig.runFolder, rel)); statErr != nil {
		t.Errorf("want %s kept after the refusal, stat error: %v", rel, statErr)
	}
}
