---
version: "4.0"
name: "Greenfield TDD Workflow"
description: "Building a new project from scratch requiring system architecture, test-first development, and full design."
hint: "Proven and powerful, but early phases can be overwhelmed on large scopes — quality degrades gradually, not a hard failure. Size the build accordingly."
author: MOSAIC
id: greenfield-tdd
referenced_agents:
  - requirements-refinement
  - requirements-review
  - system-designer
  - system-design-review
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
  - requirements-review.md
  - SystemDesign.md
  - system-design-review.md
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

<Workflow type="core" name="greenfield-tdd" version="4.0">
## Greenfield TDD Workflow

**Use when:** Building a **new project from scratch** requiring system architecture, test-first development, and full design.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | system-designer | requirements-refinement | Requirements.md | requirements-review.md |
| ARCHITECTURE | system-designer | TRUE | system-design-review | - | Requirements.md | SystemDesign.md |
| ARCHITECTURE | system-design-review | FALSE | planner-tdd-soft | system-designer | Requirements.md, SystemDesign.md | system-design-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Requirements.md, SystemDesign.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Requirements.md, Plan.md, Stage-*/Plan.md, SystemDesign.md | ContractsDesign.md |
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

Close sibling of `brownfield-tdd`, differing mainly in the added ARCHITECTURE phase (system-designer / system-design-review) needed because there is no existing codebase to anchor design decisions in.

Proven and heavily used in practice, with consistently good results. Quality is sensitive to the size of the thing being built: the early phases (requirements, architecture, planning) must hold a large amount of context, and on larger greenfield scopes they can become overwhelmed. This does not cause a hard failure — no collapse — but produces a gradual quality dropoff, the same pattern seen under context overload generally.

**Fix targets.** Each reviewer's suggested fix target is its own creator. `test-runner` runs after the Stage block has exited, so its fix target is `planner-tdd-soft`: completed stages are frozen, and the planner adds a fix stage, which runs when the block is entered again. Deeper routing — `system-design-review` uncovering a requirements gap that `system-designer` cannot resolve and that has to go back to `requirements-refinement`, or an implementation review tracing a defect to the plan — is the orchestrator's decision under its routing policy, not a declared route.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 3.4 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 3.5 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 4.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: `[StageNumber]` → declared block `[Stage]` (`**Blocks:**` table), Phase cells in `PHASE.[Block].Group` order, `**Groups:**` table with `Block` column. Routing made scope-correct: `contracts-review` enters `[Stage]`; group-ending reviewers say `COMPLETE`; `test-runner` fix target is `planner-tdd-soft` (was a block row). Prose removed from `On Findings`. Notes removed — redundant with the tables or executor policy (moved to Design Rationale). |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **Third dimension: `{Domain}` alongside `{Phase}`.** For large greenfield scopes, partition planning/execution artifacts by domain as well as phase — e.g. `{Domain}/Plan.md`, `{Domain}/Stage-{Stage}/Plan.md` — instead of one flat phase sequence trying to hold the whole system. This workflow is the first candidate to receive it. WorkflowFormat 2.1+ now expresses this as a nested `[Domain].[Stage]` block; the remaining gate is solid test coverage of the orchestrator, since this is a structural change to how artifacts are addressed.
- **Move HITL off the creator rows onto a convergence-gated `approval-presenter`.** `requirements-refinement`, `system-designer`, `planner-tdd-soft`, and `contracts-designer` are all gated `TRUE` directly, so a human reviews every draft they produce — including rounds their paired reviewer would flag anyway — instead of only the version that already converged. `requirements-to-test-cases` proved the fix (a dedicated presenter row reachable only via the reviewer's `On Success`). Revisit this workflow once that pattern has enough real use to trust.

**Dead ends (tried and rejected):**
- (none yet)
