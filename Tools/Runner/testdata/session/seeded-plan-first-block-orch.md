<Workflow type="core" name="seeded-plan-first-block" version="1.0">
## Seeded Plan.md, First Row Begins The Block

The first row is a block row, so Plan.md is a seed. No row outputs it.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementer | FALSE | reviewer | - | Plan.md, Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | reviewer | FALSE | COMPLETE | implementer | Plan.md, Stage-{Stage}/Plan.md | Stage-{Stage}/review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
