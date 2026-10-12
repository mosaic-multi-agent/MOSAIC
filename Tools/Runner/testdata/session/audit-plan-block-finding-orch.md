<Workflow type="core" name="audit-plan-block-finding" version="1.0">
## Block Named Finding, Schedule From AuditPlan.md

The planner outputs AuditPlan.md, the declared Source of the Finding block.
Used to check that a stop for a Source that is never written names the block
and the Source.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Finding] | - | - | AuditPlan.md |
| EXECUTION.[Finding] | auditor | FALSE | COMPLETE | - | AuditPlan.md | out-{Finding}.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Finding | AuditPlan.md | Findings |
</Workflow>
