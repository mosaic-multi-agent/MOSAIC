---
type: orchestration-artifact
run_id: 20260727T170000Z-a3f9
workflow: test-workflow
workflow_version: "1.0"
task: "test task"
started: 2026-01-01T00:00:00Z
last_updated: 2026-01-01T00:00:00Z
global_sequence: 2
runner_mode: auto
runner_pre_consultation: enabled
runner_manual_resolution: disabled
checkpoints: disabled
commits: enabled
current_state:
  phase: EXECUTION
  iteration: null
  last_status: SUCCESS
  last_agent: "agent#2"
  error_code: null
---

<ExecutionLog type="core">
| Seq | Agent   | Phase     | Iteration | Status  | Timestamp            | Summary | Checkpoint |
| --- | ------- | --------- | --------- | ------- | -------------------- | ------- | ---------- |
| 1   | agent#1 | EXECUTION | -         | SUCCESS | 2026-01-01T00:00:00Z | done    | -          |
| 2   | agent#2 | EXECUTION | -         | SUCCESS | 2026-01-01T00:00:00Z | first half of a summary
that a stray line break split | -          |
</ExecutionLog>

<Artifacts type="core">
| Artifact | Created In | Created By |
| -------- | ---------- | ---------- |
</Artifacts>
