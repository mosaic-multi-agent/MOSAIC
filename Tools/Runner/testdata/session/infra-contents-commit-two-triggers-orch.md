<Workflow type="core" name="product-block" version="1.0">
## Product Block With A Commit Agent On ITERATION_END And INVOCATION_INTERVAL

One Product block of builder and checker rows; the planner writes
ProductPlan.md, the schedule of the block. With one iteration the planner,
builder and checker are the first three workflow steps.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Product] | - | - | ProductPlan.md |
| EXECUTION.[Product] | builder | FALSE | NEXT | - | ProductPlan.md | built-{Product}.md |
| EXECUTION.[Product] | checker | FALSE | COMPLETE | builder | built-{Product}.md | checked-{Product}.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Product | ProductPlan.md | Products |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | halt | Git commit manager |
| commit | INVOCATION_INTERVAL | 3 | halt | Git commit manager |
</InfrastructureAgent>
</InfrastructureAgents>
