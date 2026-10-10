---
version: "3.0"
name: "Brownfield TDD Build-Verified Workflow"
description: "New features or significant changes to an existing codebase requiring test-first development where compilation/build cannot be verified via standard terminal tools (e.g., PLC/SCL with proprietary toolchains, embedded systems, cross-compilation environments)."
hint: "Field-verified (used for real PLC programming), not just theoretical. Use over standard brownfield-tdd whenever building and deployment require a non-trivial toolchain — offloading those mechanics onto a dedicated build-review agent keeps them out of the writer agents' context, while downstream reviewers execute tests."
author: MOSAIC
id: brownfield-tdd-build-verified
referenced_agents:
  - codebase-research
  - requirements-refinement
  - requirements-review
  - planner-tdd-soft
  - plan-review
  - contracts-designer
  - contracts-review
  - test-writer-tdd
  - build-review
  - tests-review-tdd
  - implementation-tdd
  - implementation-review
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
  - Stage-{Stage}/build-review-tests.md
  - Stage-{Stage}/tests-review-tdd.md
  - Stage-{Stage}/build-review-impl.md
  - Stage-{Stage}/implementation-review.md
---

<Workflow type="core" name="brownfield-tdd-build-verified" version="3.0">
## Brownfield TDD Build-Verified Workflow

**Use when:** New features or significant changes to an **existing codebase** requiring test-first development where **compilation/build cannot be verified via standard terminal tools** (e.g., PLC/SCL with proprietary toolchains, embedded systems, cross-compilation environments).

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md, Research.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | [Stage] | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | build-review(tests) | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | build-review(tests) | FALSE | tests-review-tdd | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-tests.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-tests.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | build-review(impl) | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Implementation | build-review(impl) | FALSE | implementation-review | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/build-review-impl.md |
| EXECUTION.[Stage].Implementation | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md, Stage-{Stage}/build-review-impl.md | Stage-{Stage}/implementation-review.md |

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

Identical to `brownfield-tdd` except for how build/test execution is handled. In standard `brownfield-tdd`, the execution agents (test-writer-tdd, implementation-tdd, and their reviewers) are expected to build and run tests themselves via simple terminal commands. This workflow exists for cases where that path doesn't work — the build/test toolchain is complex enough (proprietary IDEs, COM automation, MCP servers, cross-compilation, target deployment) that making every execution agent competent at it would overload their context with mechanics unrelated to their actual job.

The fix is a dedicated `build-review` agent that owns source import, build, and deployment exclusively, inserted between each writer and its reviewer. It does not execute tests. This keeps build-system and deployment mechanics in one place; downstream review agents use the deployment metadata in its report to execute tests on the target platform — `tests-review-tdd` verifies TDD RED (tests fail because the implementation is missing), `implementation-review` verifies TDD GREEN.

**Two build-review rows.** `build-review` runs once in each group, told apart by the mode qualifiers `build-review(tests)` and `build-review(impl)`. Each has its own fix target (the writer of its group) and its own output artifact, so the test build and the implementation build stay in separate contexts and each reviewer reads only the build report for the code it reviews.

**Fix targets.** Each reviewer and each build-review row sends findings to the writer in its own group. There is no post-block test run: target-platform test execution already happens inside every stage's reviewers. Deeper routing — a review tracing a defect to the plan or to the contracts — is the orchestrator's decision under its routing policy, not a declared route.

Field-verified on real PLC/SCL programming work, not just a theoretical variant.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 2.1 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 2.2 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 2.3 | 2026-09-28 | MOSAIC | Add Research.md to requirements-review inputs; its codebase alignment and feasibility checks run on research findings. |
| 2.4 | 2026-09-29 | MOSAIC | Clarify toolchain responsibility: build-review imports, builds, and deploys but does not execute tests; downstream review agents execute tests and verify TDD RED/GREEN outcomes. |
| 2.5 | 2026-10-05 | MOSAIC | Remove note making design phase optional, it decreased realibility for very little savings |
| 3.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: `[StageNumber]` → declared block `[Stage]` (`**Blocks:**` table), Phase cells in `PHASE.[Block].Group` order, `**Groups:**` table with `Block` column. The two `build-review` rows are now `build-review(tests)` and `build-review(impl)` (unique Subagent cells). Routing made scope-correct: `contracts-review` enters `[Stage]`; `tests-review-tdd` ends its group with `COMPLETE`. Prose removed from `On Findings`. **Use when:** trimmed to its first sentence; Notes removed — redundant with the tables, or moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **Move HITL off the creator rows onto a convergence-gated `approval-presenter`.** Same shape as `brownfield-tdd`: `requirements-refinement`, `planner-tdd-soft`, and `contracts-designer` are gated `TRUE` directly, so a human reviews every draft they produce — including rounds their paired reviewer would flag anyway — instead of only the version that already converged. `requirements-to-test-cases` proved the fix. Revisit this workflow once that pattern has enough real use to trust.

**Dead ends (tried and rejected):**
- (none yet)
