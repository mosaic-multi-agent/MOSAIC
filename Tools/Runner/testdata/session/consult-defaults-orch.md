<Workflow type="core" name="consult-row-stage" version="1.0">
## Grouped Staged Workflow with Artifact Defaults for Consultation Defaults Tests

Rows declare input and output artifacts, including Stage-* and {Stage}
patterns, so that the defaults a dispatch receives can be compared between the
engine-routed and the consultation-routed paths. Requires a Plan.md in RunFolder.

Row index mapping (zero-based; the recorded WorkflowRow is the index plus one):
  0 = PLANNING / planner
  1 = EXECUTION.[Stage].Test / test-writer
  2 = EXECUTION.[Stage].Implementation / implementer
  3 = EXECUTION.[Stage].Implementation / build-review
  4 = REVIEW / final-review (Stage-* input on a row outside the block)
  5 = REVIEW / impl-review

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | [Stage] | - | - | Plan.md, Stage-*/Plan.md |
| EXECUTION.[Stage].Test | test-writer | FALSE | COMPLETE | - | Stage-*/Plan.md, Stage-{Stage}/notes.md | Stage-{Stage}/tests.md |
| EXECUTION.[Stage].Implementation | implementer | FALSE | NEXT | - | Stage-{Stage}/tests.md | Stage-{Stage}/impl.md |
| EXECUTION.[Stage].Implementation | build-review | FALSE | COMPLETE | implementer | Stage-{Stage}/impl.md | Stage-{Stage}/build-review.md |
| REVIEW | final-review | FALSE | impl-review | - | Stage-*/Plan.md | final-notes.md |
| REVIEW | impl-review | FALSE | COMPLETE | - | - | - |

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
