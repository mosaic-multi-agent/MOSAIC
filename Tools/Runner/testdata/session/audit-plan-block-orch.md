<Workflow type="core" name="audit-plan-block" version="1.0">
## Block Schedule From AuditPlan.md

The planner outputs AuditPlan.md, which is the declared Source of the Stage
block. The schedule table has no Depends On column. The merger follows the
block's exit at run scope.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Stage] | - | - | AuditPlan.md |
| EXECUTION.[Stage] | auditor | FALSE | COMPLETE | - | AuditPlan.md | out-{Stage}.md |
| REVIEW | merger | FALSE | COMPLETE | - | - | merged.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | AuditPlan.md | Stages |
</Workflow>
