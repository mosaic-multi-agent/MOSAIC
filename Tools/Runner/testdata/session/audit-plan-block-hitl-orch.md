<Workflow type="core" name="audit-plan-block-hitl" version="1.0">
## Block Schedule From AuditPlan.md, Planner Needs Approval

The planner outputs AuditPlan.md, the declared Source of the Stage block, and
needs human approval before the run goes on, so the schedule is read again
before the approval is resolved.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | TRUE | [Stage] | - | - | AuditPlan.md |
| EXECUTION.[Stage] | auditor | FALSE | COMPLETE | - | AuditPlan.md | out-{Stage}.md |
| REVIEW | merger | FALSE | COMPLETE | - | - | merged.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | AuditPlan.md | Stages |
</Workflow>
