package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== Incompatible workflows refused per FR-18a (file store) =====

// TestIntegration_IncompatibleWorkflow_RefusedFR18a verifies that each
// incompatible workflow shape causes the session to return RunRefused before
// any harness invocation, with the full stack including the real file-based
// artifact store.
//
// The compat and workflow unit tests cover the individual rules. This
// integration test verifies that a refusal correctly propagates through the
// full session -> workflow/compat -> RunRefused path, and that the message
// names the WorkflowFormat validity rule that was broken, for three
// representative shapes: a fork on a block row, a block schedule source no row
// produces before the block is entered, and a repeated Subagent cell.
func TestIntegration_IncompatibleWorkflow_RefusedFR18a(t *testing.T) {
	cases := []struct {
		name       string
		workflowID string
		content    string
		wantRule   string // WorkflowFormat validity rule ID the refusal must name
	}{
		{
			name:       "parallel-dispatch",
			workflowID: "parallel-dispatch",
			// A comma list in On Success is a fork; a block row may not fork (B1).
			content: `<Workflow type="core" name="parallel-dispatch" version="1.0">
## Parallel Dispatch Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | agent-a, agent-b | - | Stage-{Stage}/Plan.md | Stage-{Stage}/out.md |
| EXECUTION.[Stage] | agent-a | FALSE | COMPLETE | - | Stage-{Stage}/out.md | Stage-{Stage}/a.md |
| EXECUTION.[Stage] | agent-b | FALSE | COMPLETE | - | Stage-{Stage}/out.md | Stage-{Stage}/b.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
`,
			wantRule: "B1",
		},
		{
			name:       "dynamic-stage-set",
			workflowID: "dynamic-stages",
			// The block schedule source Plan.md is neither a seed (the first row
			// does not list it and does not begin the block) nor output by a row
			// outside the block that runs before it is entered: only a row of the
			// block writes it, and a block row alone never produces its own
			// schedule (A7).
			content: `<Workflow type="core" name="dynamic-stages" version="1.0">
## Dynamic Stage Set Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner-tdd-soft | FALSE | NEXT | - | Requirements.md | Notes.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Plan.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
`,
			wantRule: "A7",
		},
		{
			name:       "repeated-subagent-cell",
			workflowID: "mode-notation",
			// Mode qualifiers are valid, but a Subagent cell, qualifier included,
			// names exactly one row (T8).
			content: `<Workflow type="core" name="mode-notation" version="1.0">
## Mode Notation Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a(review) | FALSE | NEXT | - | - | out.md |
| PLANNING | agent-a(review) | FALSE | COMPLETE | - | out.md | result.md |
</Workflow>
`,
			wantRule: "T8",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			orchPath := writeOrchFile(t, dir, "orchestrator.md", tc.content)

			// No artifact file written: these runs are refused before the artifact
			// is created (compat check at step 4 runs before artifact creation at
			// step 8 in the run-start sequence).
			artifactPath := filepath.Join(dir, "Orchestration.md")
			f := harness.NewMockAdapter()
			sess := newSession(f, artifactPath)

			cfg := domain.RunConfig{
				RunID: integrationRunID,
				OrchestratorFilePath: orchPath,
				WorkflowID:           domain.WorkflowID(tc.workflowID),
				Task:                 "task",
				IsNewRun:             true,
				RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
			}

			got, err := sess.Start(context.Background(), cfg)
			msg := requireRefused(t, got, err)

			if !strings.Contains(msg, "rule "+tc.wantRule) {
				t.Errorf("want refusal message to name rule %s, got %q",
					tc.wantRule, msg)
			}

			// No harness invocations should have been made before the refusal.
			if len(f.Invocations()) > 0 {
				t.Error("want no harness invocations for incompatible workflow refusal, but got some")
			}
		})
	}
}

// ===== Non-canonical existing file refused per FR-7a (file store) =====

// TestIntegration_NonCanonicalArtifact_Refused_FR7a verifies that when a file
// exists at the artifact location but is not in the canonical Orchestration.md
// format (FR-7a), the real file-based artifact store returns a RefusalError and
// the session returns RunRefused without invoking any agents.
//
// This differs from the session unit test (which injects a mock store that
// returns RefusalError directly) by exercising the real FileStore.Read and
// artifact.Parse path against actual non-canonical file bytes.
func TestIntegration_NonCanonicalArtifact_Refused_FR7a(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "linear-orch.md"))
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")

	// Write a file at the artifact path that is NOT in the canonical format:
	// no YAML frontmatter, no <SectionName type="..."> tags. The real FileStore calls
	// artifact.Parse which returns *domain.RefusalError for this content.
	runFolder := scopedRunFolder(t, dir)
	artifactPath := filepath.Join(runFolder, "Orchestration.md")
	const nonCanonicalContent = `# Orchestration

This file exists but is not in the canonical orchestration-artifact format.
It has no YAML frontmatter and no SECTION boundary tags.
`
	if err := os.WriteFile(artifactPath, []byte(nonCanonicalContent), 0600); err != nil {
		t.Fatalf("write non-canonical artifact: %v", err)
	}

	f := harness.NewMockAdapter()
	sess := newSession(f, artifactPath)

	cfg := domain.RunConfig{
		RunID: integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "linear",
		Task:                 "test task",
		IsNewRun:             false, // resume: non-canonical artifact already written
		RunFolder:            runFolder,
	}

	got, err := sess.Start(context.Background(), cfg)
	msg := requireRefused(t, got, err)

	// The refusal message must describe the non-canonical artifact condition.
	// The artifact.Parse function returns "artifact: refused ...: missing
	// frontmatter ..." which contains both "artifact" and "frontmatter".
	if !strings.Contains(msg, "artifact") && !strings.Contains(msg, "frontmatter") {
		t.Errorf("want refusal message to mention non-canonical artifact condition, got %q", msg)
	}

	// The harness must not have been invoked: refusal happens at the artifact
	// read step (step 3) before any agent is dispatched.
	if len(f.Invocations()) > 0 {
		t.Error("want no harness invocations when artifact is non-canonical, but got some")
	}
}

// ===== Run identity =====

// TestIntegration_Resume_InvalidRunIdentity_RefusedBeforeAnyInvocation verifies,
// through the real file store, that a resume whose artifact has an absent,
// empty, malformed or folder-mismatched run_id is refused before any agent is
// invoked, that the refusal names the problem, and that the artifact on disk
// is left exactly as it was (no replacement identity is minted or written).
func TestIntegration_Resume_InvalidRunIdentity_RefusedBeforeAnyInvocation(t *testing.T) {
	const otherRunID = "20260101T000000Z-abcd"
	tests := []struct {
		name      string
		runIDLine string // "" omits the key
		mentions  string
	}{
		{"run_id key absent", "", "run_id"},
		{"run_id empty", `run_id: ""`, "run_id"},
		{"run_id malformed", "run_id: not-a-run-id", "malformed"},
		{"run_id names a different folder", "run_id: " + otherRunID, otherRunID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			orchPath := copyFile(t, dir, "orchestrator.md",
				filepath.Join(sessionTestdataDir, "linear-orch.md"))
			writeAgentFile(t, dir, "agent-a")
			writeAgentFile(t, dir, "agent-b")
			runFolder := scopedRunFolder(t, dir)
			artifactPath := filepath.Join(runFolder, "Orchestration.md")
			line := ""
			if tc.runIDLine != "" {
				line = tc.runIDLine + "\n"
			}
			content := "---\ntype: orchestration-artifact\n" + line + `workflow: linear
workflow_version: "1.0"
task: "test task"
started: 2026-01-01T00:00:00Z
last_updated: 2026-01-01T00:00:00Z
global_sequence: 1
runner_mode: auto
runner_pre_consultation: disabled
runner_manual_resolution: disabled
checkpoints: disabled
current_state:
  phase: PLANNING
  iteration: null
  last_status: SUCCESS
  last_agent: "agent-a#1"
  error_code: null
---

<ExecutionLog type="core">
| Seq | Agent     | Phase    | Iteration | Status  | Timestamp            | Summary       | Checkpoint |
| --- | --------- | -------- | ----- | ------- | -------------------- | ------------- | ---------- |
| 1   | agent-a#1 | PLANNING | -     | SUCCESS | 2026-01-01T00:00:00Z | planning done | -          |
</ExecutionLog>

<Artifacts type="core">
| Artifact | Created In | Created By |
| -------- | ---------- | ---------- |
</Artifacts>
`
			if err := os.WriteFile(artifactPath, []byte(content), 0600); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			f := harness.NewMockAdapter()
			sess := newSession(f, artifactPath)
			cfg := domain.RunConfig{
				RunID:                integrationRunID,
				OrchestratorFilePath: orchPath,
				WorkflowID:           "linear",
				Task:                 "test task",
				IsNewRun:             false,
				RunFolder:            runFolder,
			}

			// Act
			got, err := sess.Start(context.Background(), cfg)

			// Assert
			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, tc.mentions) {
				t.Errorf("refusal must name the problem (want it to mention %q), got %q", tc.mentions, msg)
			}
			if n := len(f.Invocations()); n != 0 {
				t.Errorf("want no harness invocation before the refusal, got %d", n)
			}
			after, readErr := os.ReadFile(artifactPath)
			if readErr != nil {
				t.Fatalf("read artifact back: %v", readErr)
			}
			if string(after) != content {
				t.Error("artifact on disk changed during a refused resume; no identity may be minted or repaired")
			}
		})
	}
}
