<Workflow type="core" name="staged" version="1.0">
## Staged Workflow (Implementation-Only)

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
