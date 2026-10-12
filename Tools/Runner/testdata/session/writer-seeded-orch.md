<Workflow type="core" name="writer-seeded" version="1.0">
## Seeded Schedule With An In-Block Writer

The first row begins the block, so Plan.md is a seed. The worker also outputs
Plan.md, so it may extend the schedule while the block runs.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | worker | FALSE | COMPLETE | - | Plan.md | out-{Stage}.md, Plan.md |
| REVIEW | closer | FALSE | COMPLETE | - | - | closed.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>
