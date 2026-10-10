---
version: "4.0"
name: "Brownfield TDD Workflow"
description: "New features or significant changes to an existing codebase requiring test-first development with full research and design."
hint: "The main workhorse — best on shell-buildable codebases. Works best at a certain feature size, discovered through use rather than designed in. The two size-mismatch directions fail differently: too small wastes cost on overspecified Requirements/Plan and triggers unnecessary correction round-trips; too big genuinely starves execution agents of context, making them more error-prone and pushing you toward needing stronger models."
author: MOSAIC
id: brownfield-tdd
referenced_agents:
  - codebase-research
  - requirements-refinement
  - requirements-review
  - planner-tdd-soft
  - plan-review
  - contracts-designer
  - contracts-review
  - test-writer-tdd
  - tests-review-tdd
  - implementation-tdd
  - implementation-review
  - test-runner
artifacts:
  - Requirements.md
  - Research.md
  - requirements-review.md
  - Plan.md
  - Stage-*/Plan.md
  - Stage-*/PlanProgress.md
  - plan-review.md
  - ContractsDesign.md
  - contracts-review.md
  - Stage-{Stage}/Plan.md
  - Stage-{Stage}/PlanProgress.md
  - Stage-{Stage}/tests-review-tdd.md
  - Stage-{Stage}/implementation-review.md
  - TestResults.md
---

<Workflow type="core" name="brownfield-tdd" version="4.0">
## Brownfield TDD Workflow

**Use when:** New features or significant changes to an **existing codebase** requiring test-first development with full research and design.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md, Research.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | tests-review-tdd | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | planner-tdd-soft | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
| Stage | Implementation-First | Implementation, Test |
| Stage | Implementation-Only | Implementation |
| Stage | Tests-Only | Test |
</Workflow>

---

## Design Rationale

The king of workflows — the main workhorse. Works very well for software development where the environment is open to shell commands (`dotnet build`, `dotnet test`, etc. — build/test feedback loops the review and test-runner steps can act on). All of MOSAIC's own tooling was built using this workflow.

Quality tracks feature sizing more than it tracks raw context budget. This was never designed in — it's a pattern noticed through use: the workflow simply works well at a certain feature granularity, and the two directions of mismatch fail for different underlying reasons, not just "quality suffers" symmetrically.

**Too small:** Requirements and Plan tend to over-engineer — producing more specificity than the change actually needs. The concrete cost is not a quality defect in the output so much as waste and friction: unnecessary correction round-trips through requirements-review/plan-review, spent on a feature that didn't need that level of scrutiny in the first place.

**Too big:** Plan and Design tend to stay too vague to pin down the real scope, because the non-execution phases genuinely cannot hold enough context to spec a large feature precisely. This one is a real quality risk, not just wasted cost — the execution agents inherit under-specified direction, downstream errors become more likely, and the practical mitigation is reaching for stronger models to compensate, not just accepting slower convergence. Same underlying sensitivity as `greenfield-tdd`'s context-overload behavior, showing up here as an oversized-feature failure mode specifically rather than a general one.

**Fix targets.** Each reviewer's suggested fix target is its own creator. `test-runner` runs after the Stage block has exited, so its fix target is `planner-tdd-soft`: completed stages are frozen, and the planner adds a fix stage, which runs when the block is entered again. When an implementation review traces a defect to the tests, the contracts or the plan rather than to the code, sending the findings there is the orchestrator's decision under its routing policy, not a declared route.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 3.6 | 2026-08-05 | MOSAIC | Initial version |
| 3.7 | 2026-08-06 | MOSAIC | Make test-runner findings route back to planner |
| 3.8 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 3.9 | 2026-09-28 | MOSAIC | Add Research.md to requirements-review inputs; its codebase alignment and feasibility checks run on research findings. |
| 3.10 | 2026-10-05 | MOSAIC | Remove note making design phase optional, it decreased realibility for very little savings |
| 4.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: `[StageNumber]` → declared block `[Stage]` (`**Blocks:**` table), Phase cells in `PHASE.[Block].Group` order, `**Groups:**` table with `Block` column. Routing made scope-correct: `contracts-review` enters `[Stage]`; group-ending reviewers say `COMPLETE` (was a jump into the other group / out of the block). Prose removed from `implementation-review`'s `On Findings` (moved to Design Rationale). Notes removed — redundant with **Use when** and the tables. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **Move HITL off the creator rows onto a convergence-gated `approval-presenter`.** `requirements-refinement`, `planner-tdd-soft`, and `contracts-designer` are all gated `TRUE` directly, so a human reviews every draft they produce — including rounds their paired reviewer (`requirements-review`, `plan-review`, `contracts-review`) would flag anyway — instead of only the version that already converged. `requirements-to-test-cases` proved the fix (a dedicated presenter row reachable only via the reviewer's `On Success`, so it fires once per convergence, not once per round). Revisit this workflow once that pattern has enough real use to trust.

**Dead ends (tried and rejected):**
- (none yet)
