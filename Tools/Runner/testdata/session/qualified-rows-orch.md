<Workflow type="core" name="qualified-rows" version="1.0">
## Qualified Rows Workflow

One agent (build-review) fills two rows, told apart by mode qualifiers. A single
build-review.md agent file serves both rows.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | build-review(tests) | - | - | plan.md |
| PLANNING | build-review(tests) | FALSE | implementer | planner | plan.md | tests-review.md |
| EXECUTION | implementer | FALSE | build-review(impl) | - | plan.md | impl.md |
| REVIEW | build-review(impl) | FALSE | COMPLETE | implementer | impl.md | impl-review.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="checkpoint-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| checkpoint | STAGE_END | - | halt | Git checkpoint manager |
</InfrastructureAgent>
</InfrastructureAgents>
