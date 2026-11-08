---
type: orchestration-artifact
run_id: "example-run"
workflow: greenfield-tdd
workflow_version: "1.0"
task: "Reply-recovery cutoff fixture task"
started: 2026-01-01T00:00:00Z
last_updated: 2026-01-01T00:10:00Z
global_sequence: 2
checkpoints: disabled
commits: disabled
current_state:
  phase: EXECUTION
  stage: "-"
  last_status: SUCCESS
  last_agent: "helper#1"
  error_code: null
---

<ExecutionLog type="core">
| Seq | Agent | Phase | Stage | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |
|-----|-------|-------|-------|-------------|--------|-----------|---------|--------|------------|
| 1 | greeter#1 | EXECUTION | - | - | SUCCESS | 2026-01-01T00:05:00Z | Greeted. | - | - |
| 2 | helper#1 | EXECUTION | - | - | SUCCESS | 2026-01-01T00:10:00Z | Helped. | - | - |
</ExecutionLog>
