<Workflow type="core" name="audit-plan-block-star" version="1.0">
## Block Schedule From AuditPlan.md, Producer Output Uses A Wildcard

The planner outputs Audit*.md, where * stands for text within one path
segment, so its output matches the declared Source AuditPlan.md. The merger
follows the block's exit at run scope.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Stage] | - | - | Audit*.md |
| EXECUTION.[Stage] | auditor | FALSE | COMPLETE | - | AuditPlan.md | out-{Stage}.md |
| REVIEW | merger | FALSE | COMPLETE | - | - | merged.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | AuditPlan.md | Stages |
</Workflow>
