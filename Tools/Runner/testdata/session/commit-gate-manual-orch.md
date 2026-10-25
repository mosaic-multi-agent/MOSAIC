<Workflow type="core" name="product-block" version="1.0">
## Product Block With A Commit Agent That Declares Only A MANUAL Trigger

The planner writes ProductPlan.md, the schedule of the Product block. Each
iteration has a builder row and a checker row. The commit agent declares no
automatic trigger: it fires after an iteration only when an
infrastructure_overrides entry gives it ITERATION_END.

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
| commit | MANUAL | - | halt | Git commit manager (declares no automatic trigger) |
</InfrastructureAgent>
</InfrastructureAgents>
