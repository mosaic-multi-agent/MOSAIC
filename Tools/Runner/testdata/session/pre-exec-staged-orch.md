<Workflow type="core" name="pre-exec-staged" version="1.0">
## Planning Then Staged Execution Workflow

Workflow with pre-EXECUTION (PLANNING) rows ahead of a staged EXECUTION phase.
Used to verify that a run of a staged workflow is not refused when no plan
file exists yet, and dispatches its next pre-EXECUTION row normally.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | reviewer | - | - | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | reviewer | FALSE | [Stage] | planner | Plan.md | review.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md | Stage-{Stage}/implementation-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
