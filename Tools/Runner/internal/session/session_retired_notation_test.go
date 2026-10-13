package session_test

// Tests that WorkflowFormat 1.x notation is refused before the first dispatch,
// both when a run starts and when an existing run is resumed, in every
// execution mode. Each case is a four-row workflow of valid 2.16 rows in which
// one row (or one trailing table) carries a single 1.x form, so the form is the
// only reason for the refusal.
//
// The refusal must be a *domain.RefusalError carrying the workflow id, the
// 1-based routing row or the offending token, and the WorkflowFormat sec. 10
// rule ID where one exists.
// These inputs deliberately contain 1.x tokens; the notation scan excludes
// this file by name.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// retiredWorkflowID is the id of the workflow every case defines.
const retiredWorkflowID = "retirednotation"

// retiredHeader is the 7-column header of a 2.16 routing table.
const retiredHeader = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

// retiredMarkerFile is a file placed in the run folder before the run, to
// observe whether the folder is kept or removed by a refusal.
const retiredMarkerFile = "marker.md"

// Column indexes of a routing row.
const (
	cellPhase      = 0
	cellOnSuccess  = 3
	cellOnFindings = 4
	cellInput      = 5
	cellOutput     = 6
)

// retiredAgents are the agents the workflows dispatch.
var retiredAgents = []string{"planner", "agent-a", "agent-b", "agent-c"}

// retiredForm is one 1.x form to refuse.
type retiredForm struct {
	name string
	// edit alters the valid rows (and may return different ones); nil keeps them.
	edit func(rows [][]string) [][]string
	// tail is appended after the routing table.
	tail string
	// token is text the refusal's Token must relate to; "" asserts none.
	token string
	// rules lists the rule IDs of which the refusal's Rule must be one; nil
	// asserts no rule (the form has none, or the implementation may map one).
	rules []string
	// row is the 1-based row the refusal must name; 0 means the refusal is not
	// about a row and must carry domain.NoWorkflowRow.
	row int
}

// retiredValidRows returns the four rows of the valid workflow.
func retiredValidRows() [][]string {
	return [][]string{
		{"PLANNING", "planner", "FALSE", "agent-a", "-", "-", "Plan.md"},
		{"REVIEW", "agent-a", "FALSE", "agent-b", "-", "Plan.md", "a.md"},
		{"REVIEW", "agent-b", "FALSE", "agent-c", "-", "a.md", "b.md"},
		{"REVIEW", "agent-c", "FALSE", "COMPLETE", "-", "a.md", "c.md"},
	}
}

// setCell returns an edit that replaces one cell of the 1-based row.
func setCell(row, col int, cell string) func([][]string) [][]string {
	return func(rows [][]string) [][]string {
		rows[row-1][col] = cell
		return rows
	}
}

// retiredBlockRows are the rows of a valid 2.16 block workflow: a planning row
// that schedules the block Stage and one row that runs it.
func retiredBlockRows([][]string) [][]string {
	return [][]string{
		{"PLANNING", "planner", "FALSE", "[Stage]", "-", "-", "Plan.md"},
		{"EXECUTION.[Stage]", "agent-a", "FALSE", "COMPLETE", "-", "Plan.md", "a.md"},
	}
}

// retiredForms lists every 1.x form the Runner no longer accepts.
func retiredForms() []retiredForm {
	return []retiredForm{
		{
			name:  "block reference [StageNumber]",
			edit:  setCell(3, cellPhase, "EXECUTION.[StageNumber]"),
			token: "[StageNumber]",
			row:   3,
		},
		{
			name:  "group-qualified block reference",
			edit:  setCell(3, cellPhase, "EXECUTION.Test.[StageNumber]"),
			token: "[StageNumber]",
			row:   3,
		},
		{
			name: "block reference whose schedule is inferred from a Stage plan name",
			edit: func(rows [][]string) [][]string {
				rows[1][cellOutput] = "Stage-*/Plan.md"
				rows[2][cellPhase] = "EXECUTION.[StageNumber]"
				return rows
			},
			token: "[StageNumber]",
			row:   3,
		},
		{
			name:  "fill-in {StageNumber} in an Input cell",
			edit:  setCell(3, cellInput, "Stage-{StageNumber}/Plan.md"),
			token: "{StageNumber}",
			row:   3,
		},
		{
			name:  "fill-in {StageNumber} in an Output cell",
			edit:  setCell(3, cellOutput, "Stage-{StageNumber}/b.md"),
			token: "{StageNumber}",
			row:   3,
		},
		{
			name: "fill-in {StageNumber} in a Blocks Source cell",
			edit: retiredBlockRows,
			tail: "\n**Blocks:**\n\n| Block | Source | Section |\n|-------|--------|---------|\n" +
				"| Stage | Stage-{StageNumber}/Plan.md | Stages |\n",
			token: "{StageNumber}",
		},
		{
			name:  "Execution Groups heading",
			tail:  "\n**Execution Groups:**\n\n| Approach | Groups |\n|----------|--------|\n| TDD | Test, Implementation |\n",
			token: "Execution Groups",
		},
		{
			name:  "star suffix on an On Success target",
			edit:  setCell(3, cellOnSuccess, "agent-c*"),
			token: "agent-c*",
			rules: []string{"T6"},
			row:   3,
		},
		{
			name:  "star suffix on an On Findings target",
			edit:  setCell(3, cellOnFindings, "agent-a*"),
			token: "agent-a*",
			rules: []string{"T6"},
			row:   3,
		},
		{
			name:  "lowercase next",
			edit:  setCell(3, cellOnSuccess, "next"),
			token: "next",
			row:   3,
		},
		{
			name:  "dash in On Success",
			edit:  setCell(3, cellOnSuccess, "-"),
			rules: []string{"T5", "T12"},
			row:   3,
		},
	}
}

// retiredWorkflow renders the orchestrator file for the rows the form's edit
// yields from the valid rows, then tail inside the workflow region.
func retiredWorkflow(edit func([][]string) [][]string, tail string) string {
	rows := retiredValidRows()
	if edit != nil {
		rows = edit(rows)
	}
	var b strings.Builder
	b.WriteString("<Workflow type=\"core\" name=\"" + retiredWorkflowID + "\" version=\"1.0\">\n")
	b.WriteString("## Retired Notation Workflow\n\n")
	b.WriteString(retiredHeader)
	for _, r := range rows {
		b.WriteString("| " + strings.Join(r, " | ") + " |\n")
	}
	b.WriteString(tail)
	b.WriteString("</Workflow>\n")
	return b.String()
}

// retiredRig is a session over one generated orchestrator file and a run
// folder that holds a marker file.
type retiredRig struct {
	ses       session.Session
	adapter   *harness.MockAdapter
	store     *memStore
	cfg       domain.RunConfig
	runFolder string
}

// newRetiredRig writes content and the agent definitions into a temporary
// directory and builds a session for a new run in the given mode.
func newRetiredRig(t *testing.T, content string, mode domain.ExecutionMode) *retiredRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := filepath.Join(dir, "orchestrator.md")
	if err := os.WriteFile(orchPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write orchestrator file: %v", err)
	}
	for _, agent := range retiredAgents {
		writeAgentFile(t, dir, agent)
	}
	runFolder := filepath.Join(dir, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("create run folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runFolder, retiredMarkerFile), []byte("kept"), 0o600); err != nil {
		t.Fatalf("write marker file: %v", err)
	}
	adapter := harness.NewMockAdapter()
	store := &memStore{}
	ses := session.New(session.Deps{
		Harness:  adapter,
		Store:    store,
		Routing:  &scriptedRoutingConsultant{},
		Manual:   &scriptedRoutingConsultant{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	return &retiredRig{
		ses:       ses,
		adapter:   adapter,
		store:     store,
		runFolder: runFolder,
		cfg: domain.RunConfig{
			OrchestratorFilePath: orchPath,
			WorkflowID:           retiredWorkflowID,
			Task:                 "task",
			IsNewRun:             true,
			RunFolder:            runFolder,
			RunSettings:          domain.RunSettings{Mode: mode},
		},
	}
}

// asResume turns the rig's request into a resume of a run whose first row
// already completed.
func (r *retiredRig) asResume(mode domain.ExecutionMode) {
	r.store.state = domain.ArtifactState{
		Type:            "orchestration-artifact",
		Workflow:        retiredWorkflowID,
		WorkflowVersion: "1.0",
		Task:            "task",
		GlobalSequence:  1,
		RunSettings:     domain.RunSettings{Mode: mode},
		CurrentState: domain.CurrentState{
			Phase:      "PLANNING",
			LastStatus: domain.StatusSUCCESS,
			LastAgent:  "planner#1",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			{Seq: 1, Agent: "planner#1", Phase: "PLANNING", Status: domain.StatusSUCCESS},
		},
	}
	r.store.exists = true
	markResume(&r.cfg)
}

// requireRetiredRefusal asserts that the outcome refuses the form before any
// dispatch and that the refusal is a *domain.RefusalError naming the workflow
// id, the 1-based row or the token, and the rule where the form has one.
func requireRetiredRefusal(t *testing.T, rig *retiredRig, got domain.RunOutcome, err error, f retiredForm) {
	t.Helper()
	msg := requireRefused(t, got, err)

	var re *domain.RefusalError
	if !errors.As(got.Cause, &re) {
		t.Fatalf("want the outcome's Cause to be a *domain.RefusalError, got %v (message %q)", got.Cause, msg)
	}
	if !strings.Contains(re.Resource, retiredWorkflowID) && !strings.Contains(msg, retiredWorkflowID) {
		t.Errorf("want the refusal to name workflow %q, got resource %q, message %q", retiredWorkflowID, re.Resource, msg)
	}
	wantRow := domain.WorkflowRow(f.row)
	if f.row == 0 {
		wantRow = domain.NoWorkflowRow
	}
	if re.Row != wantRow {
		t.Errorf("Row: want %d, got %d (rule %q, token %q)", wantRow, re.Row, re.Rule, re.Token)
	}
	if f.token != "" && !strings.Contains(re.Token, f.token) && !strings.Contains(f.token, re.Token) {
		t.Errorf("Token: want text related to %q, got %q", f.token, re.Token)
	}
	if f.token != "" && re.Token == "" {
		t.Errorf("Token: want the offending token %q, got empty", f.token)
	}
	if len(f.rules) > 0 && !containsRule(f.rules, re.Rule) {
		t.Errorf("Rule: want one of %v, got %q", f.rules, re.Rule)
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no agent invoked before the refusal, got %d invocation(s)", n)
	}
	if len(rig.store.Applied) != 0 {
		t.Errorf("want nothing recorded before the refusal, got %d applied step(s)", len(rig.store.Applied))
	}
}

// containsRule reports whether rule is one of rules.
func containsRule(rules []string, rule string) bool {
	for _, r := range rules {
		if r == rule {
			return true
		}
	}
	return false
}

// retiredModes are the execution modes every refusal is checked in.
var retiredModes = []domain.ExecutionMode{
	domain.ExecutionModeAuto,
	domain.ExecutionModeAutoReview,
	domain.ExecutionModeOrchestrated,
}

// TestSession_Start_RetiredNotation_RefusedBeforeAnyDispatchInEveryMode
// verifies that a new run whose workflow uses a 1.x form is refused naming the
// workflow, the row or token and the rule, that nothing is dispatched, and that
// the refused new run's folder is removed.
func TestSession_Start_RetiredNotation_RefusedBeforeAnyDispatchInEveryMode(t *testing.T) {
	for _, f := range retiredForms() {
		for _, mode := range retiredModes {
			t.Run(f.name+"/"+string(mode), func(t *testing.T) {
				rig := newRetiredRig(t, retiredWorkflow(f.edit, f.tail), mode)

				got, err := rig.ses.Start(context.Background(), rig.cfg)

				requireRetiredRefusal(t, rig, got, err, f)
				if _, statErr := os.Stat(rig.runFolder); !errors.Is(statErr, fs.ErrNotExist) {
					t.Errorf("want the refused new run's folder removed, stat error: %v", statErr)
				}
			})
		}
	}
}

// TestSession_Start_Resume_RetiredNotation_RefusedNotResumed verifies that an
// existing run whose deployed workflow is in 1.x notation is refused when it is
// resumed, rather than continued from its recorded position, and that the
// existing run's state and folder are left intact.
func TestSession_Start_Resume_RetiredNotation_RefusedNotResumed(t *testing.T) {
	for _, f := range retiredForms() {
		for _, mode := range retiredModes {
			t.Run(f.name+"/"+string(mode), func(t *testing.T) {
				rig := newRetiredRig(t, retiredWorkflow(f.edit, f.tail), mode)
				rig.asResume(mode)
				before := rig.store.state

				got, err := rig.ses.Start(context.Background(), rig.cfg)

				requireRetiredRefusal(t, rig, got, err, f)
				if !reflect.DeepEqual(rig.store.state, before) {
					t.Errorf("want the stored run state unchanged by the refusal, got %+v", rig.store.state)
				}
				marker := filepath.Join(rig.runFolder, retiredMarkerFile)
				if _, statErr := os.Stat(marker); statErr != nil {
					t.Errorf("want the resumed run's folder and files kept after the refusal, stat error: %v", statErr)
				}
			})
		}
	}
}

// TestSession_Start_BlockRowWithoutBlocksTable_RefusedByAdmission is regression
// coverage of the existing admission rule: a block reference whose block is not
// declared in a Blocks table is refused, naming the block and rule A2, and no
// schedule source is inferred from an artifact name. It passes before the 1.x
// path is removed; the 1.x inference itself is covered by the "block reference
// whose schedule is inferred from a Stage plan name" case above.
func TestSession_Start_BlockRowWithoutBlocksTable_RefusedByAdmission(t *testing.T) {
	for _, mode := range retiredModes {
		t.Run(string(mode), func(t *testing.T) {
			edit := func(rows [][]string) [][]string {
				rows[2][cellPhase] = "EXECUTION.[Stage]"
				rows[2][cellOutput] = "b-{Stage}.md"
				return rows
			}
			rig := newRetiredRig(t, retiredWorkflow(edit, ""), mode)

			got, err := rig.ses.Start(context.Background(), rig.cfg)

			msg := requireRefused(t, got, err)
			for _, want := range []string{retiredWorkflowID, "[rule A2]", "[row 3]", `[token "Stage"]`} {
				if !strings.Contains(msg, want) {
					t.Errorf("want the refusal to contain %q, got %q", want, msg)
				}
			}
			if n := len(rig.adapter.Invocations()); n != 0 {
				t.Errorf("want no agent invoked before the refusal, got %d invocation(s)", n)
			}
		})
	}
}

// TestSession_Start_ValidNotationOfTheSameShape_RunsToCompletion is the
// control for the refusals above: the same four-row workflow without any 1.x
// form is accepted and runs, so a refusal is caused by the form alone.
func TestSession_Start_ValidNotationOfTheSameShape_RunsToCompletion(t *testing.T) {
	rig := newRetiredRig(t, retiredWorkflow(nil, ""), domain.ExecutionModeAuto)
	for i, agent := range retiredAgents {
		rig.adapter.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#" + strconv.Itoa(i+1),
			StatusCode:      domain.StatusSUCCESS,
			StatusMessage:   "done",
		}})
	}

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := len(rig.adapter.Invocations()); n != len(retiredAgents) {
		t.Errorf("want %d invocations, got %d", len(retiredAgents), n)
	}
}
