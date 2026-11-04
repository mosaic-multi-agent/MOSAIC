<Workflow type="core" name="product-block" version="1.0">
## Product Block With A Commit Agent And A Review Agent On ITERATION_END

The planner writes ProductPlan.md, the schedule of the Product block. Each
iteration has a builder row and a checker row. After every iteration both the
commit agent and the review agent fire.

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
<InfrastructureAgent type="core" name="orchestration-review" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| review | ITERATION_END | - | continue | Orchestration review agent |
</InfrastructureAgent>
</InfrastructureAgents>
