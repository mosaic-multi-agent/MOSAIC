<Workflow type="core" name="writer-produced" version="1.0">
## Produced Schedule With An In-Block Writer

The planner produces Plan.md before the block is entered. The worker, a row
of the block, also outputs Plan.md, so it may extend the schedule while the
block runs.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Stage] | - | - | Plan.md |
| EXECUTION.[Stage] | worker | FALSE | COMPLETE | - | Plan.md | out-{Stage}.md, Plan.md |
| REVIEW | closer | FALSE | COMPLETE | - | - | closed.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
