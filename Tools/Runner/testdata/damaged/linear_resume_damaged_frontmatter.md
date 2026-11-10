---
type: orchestration-artifact
run_id: 20260727T170000Z-a3f9
workflow: [linear
workflow_version: "1.0"
task: "test task"
started: 2026-01-01T00:00:00Z
last_updated: 2026-01-01T00:00:00Z
global_sequence: 2
runner_mode: auto
runner_pre_consultation: disabled
runner_manual_resolution: disabled
checkpoints: disabled
current_state:
  phase: PLANNING
  iteration: null
  last_status: SUCCESS
  last_agent: "agent-b#2"
  error_code: null
---

<ExecutionLog type="core">
| Seq | Agent     | Phase    | Iteration | Status  | Timestamp            | Summary | Checkpoint |
| --- | --------- | -------- | --------- | ------- | -------------------- | ------- | ---------- |
| 1   | agent-a#1 | PLANNING | -         | SUCCESS | 2026-01-01T00:00:00Z | planning done | -    |
| 2   | agent-b#2 | PLANNING | -         | SUCCESS | 2026-01-01T00:00:00Z | result written by the second agent | -    |
</ExecutionLog>

<Artifacts type="core">
| Artifact | Created In | Created By |
| -------- | ---------- | ---------- |
| plan.md  | PLANNING   | agent-a#1  |
</Artifacts>

<WorkflowNotes type="core">
| Seq | Note |
| --- | ---- |
</WorkflowNotes>
