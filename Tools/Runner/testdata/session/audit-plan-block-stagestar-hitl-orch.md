<Workflow type="core" name="audit-plan-block-stagestar-hitl" version="1.0">
## Block Schedule From AuditPlan.md, Planner Needs Approval And Outputs Stage-*

The planner outputs AuditPlan.md, the declared Source of the Stage block, and
a per-iteration notes path written with Stage-*. The planner needs human
approval, so the schedule it wrote decides which notes paths the approval is
asked about.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | TRUE | [Stage] | - | - | AuditPlan.md, Stage-*/notes.md |
| EXECUTION.[Stage] | auditor | FALSE | COMPLETE | - | AuditPlan.md | out-{Stage}.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | AuditPlan.md | Stages |
</Workflow>
