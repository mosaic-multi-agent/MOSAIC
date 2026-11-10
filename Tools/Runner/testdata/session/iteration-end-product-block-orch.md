<Workflow type="core" name="product-block" version="1.0">
## Block Not Named Stage, With A Commit Agent On ITERATION_END

The planner writes ProductPlan.md, the schedule of the Product block. Each
iteration of the block has two rows: a builder, then a checker. The commit
agent fires after the last row of every iteration, whatever the block is named.

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
| commit | ITERATION_END | - | halt | Git commit manager (fires at iteration end) |
</InfrastructureAgent>
</InfrastructureAgents>
