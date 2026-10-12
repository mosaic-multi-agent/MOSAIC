<Workflow type="core" name="manual-routing" version="1.0">
## Grouped Staged Workflow with Thirteen Rows for Manual Routing Tests

A workflow whose build-review agent is the twelfth routing table row, a staged
Implementation row. Requires a Plan.md with an Approach column in RunFolder.

Row numbers (1-based; the recorded WorkflowRow equals the row number):
   1 = PLANNING / planner
   2 = PLANNING / plan-review
   3 = PLANNING / spec-writer
   4 = PLANNING / spec-review
   5 = EXECUTION.[Stage].Test / test-writer
   6 = EXECUTION.[Stage].Test / test-review
   7 = EXECUTION.[Stage].Implementation / implementer
   8 = EXECUTION.[Stage].Implementation / refactorer
   9 = EXECUTION.[Stage].Implementation / lint-check
  10 = EXECUTION.[Stage].Implementation / doc-writer
  11 = EXECUTION.[Stage].Implementation / impl-review
  12 = EXECUTION.[Stage].Implementation / build-review
  13 = REVIEW / final-review

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | plan-review | - | - | Plan.md, Stage-*/Plan.md |
| PLANNING | plan-review | FALSE | spec-writer | planner | - | - |
| PLANNING | spec-writer | FALSE | spec-review | - | - | - |
| PLANNING | spec-review | FALSE | [Stage] | spec-writer | - | - |
| EXECUTION.[Stage].Test | test-writer | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Test | test-review | FALSE | COMPLETE | test-writer | - | - |
| EXECUTION.[Stage].Implementation | implementer | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Implementation | refactorer | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Implementation | lint-check | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Implementation | doc-writer | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Implementation | impl-review | FALSE | NEXT | implementer | - | - |
| EXECUTION.[Stage].Implementation | build-review | FALSE | COMPLETE | implementer | Stage-{Stage}/Plan.md | Stage-{Stage}/Build.md |
| REVIEW | final-review | FALSE | COMPLETE | - | - | - |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-Only | Implementation |
</Workflow>
