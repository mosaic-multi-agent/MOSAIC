<Workflow type="core" name="consult-row-stage" version="1.0">
## Grouped Staged Workflow for Consultation Row and Stage Recording Tests

A workflow with non-staged planning rows followed by grouped staged EXECUTION
rows. Requires a Plan.md with an Approach column in RunFolder. The routing
table is not expanded per iteration: each EXECUTION row is one table row that
runs once per plan stage.

Row index mapping (zero-based; the recorded WorkflowRow is the index plus one):
  0 = PLANNING / planner
  1 = PLANNING / plan-review
  2 = EXECUTION.[Stage].Test / test-writer
  3 = EXECUTION.[Stage].Implementation / implementer
  4 = EXECUTION.[Stage].Implementation / build-review
  5 = EXECUTION.[Stage].Implementation / impl-review
  6 = REVIEW / final-review

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | plan-review | - | - | Plan.md, Stage-*/Plan.md |
| PLANNING | plan-review | FALSE | [Stage] | planner | - | - |
| EXECUTION.[Stage].Test | test-writer | FALSE | COMPLETE | - | - | - |
| EXECUTION.[Stage].Implementation | implementer | FALSE | NEXT | - | - | - |
| EXECUTION.[Stage].Implementation | build-review | FALSE | NEXT | implementer | - | - |
| EXECUTION.[Stage].Implementation | impl-review | FALSE | COMPLETE | implementer | - | - |
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
