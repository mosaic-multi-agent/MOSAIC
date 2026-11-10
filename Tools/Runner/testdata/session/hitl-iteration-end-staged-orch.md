<Workflow type="core" name="hitl-iteration-end" version="1.0">
## Staged HITL Review Workflow

The stage's last step is a HITL-dispatched reviewer. A commit-class agent fires
at the end of each stage. Used to verify that ITERATION_END follows the routed
status after a HITL re-dispatch.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | TRUE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md | Stage-{Stage}/implementation-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | halt | Git commit manager (fires on stage transition) |
</InfrastructureAgent>
</InfrastructureAgents>
