---
version: "4.0"
name: "Brownfield Design Workflow"
description: "Architecture review, design proposals, or planning large features for an existing codebase without implementation."
hint: "The RESEARCH/PLANNING/DESIGN head of brownfield-tdd, ending before EXECUTION. Use it when you want a design without implementing it — a real, distinct request, not a way to split a run (native run continuation covers resuming)."
author: MOSAIC
id: brownfield-design
referenced_agents:
  - codebase-research
  - requirements-refinement
  - requirements-review
  - planner-tdd-soft
  - plan-review
  - contracts-designer
  - contracts-review
artifacts:
  - Requirements.md
  - Research.md
  - requirements-review.md
  - Plan.md
  - "Stage-*/Plan.md"
  - "Stage-*/PlanProgress.md"
  - plan-review.md
  - ContractsDesign.md
  - contracts-review.md
---

<Workflow type="core" name="brownfield-design" version="4.0">
## Brownfield Design Workflow

**Use when:** Architecture review, design proposals, or planning large features for an **existing codebase** without implementation.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | codebase-research | FALSE | requirements-refinement | - | Requirements.md | Research.md |
| RESEARCH | requirements-refinement | TRUE | requirements-review | - | Research.md, Requirements.md | Requirements.md |
| RESEARCH | requirements-review | FALSE | planner-tdd-soft | requirements-refinement | Requirements.md, Research.md | requirements-review.md |
| PLANNING | planner-tdd-soft | TRUE | plan-review | - | Research.md, Requirements.md | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | contracts-designer | planner-tdd-soft | Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md | plan-review.md |
| DESIGN | contracts-designer | TRUE | contracts-review | - | Research.md, Requirements.md, Plan.md, Stage-*/Plan.md | ContractsDesign.md |
| DESIGN | contracts-review | FALSE | COMPLETE | contracts-designer | Plan.md, Stage-*/Plan.md, ContractsDesign.md | contracts-review.md |
</Workflow>

---

## Design Rationale

Structurally the RESEARCH → PLANNING → DESIGN head of `brownfield-tdd`, stopping before EXECUTION. It exists because "produce an architecture review, a design proposal, or a plan for a large feature, but stop before writing any code" is a legitimate, distinct request in its own right — a user asking for a design review does not want implementation to happen next. It is not a way to split a long run; native run continuation covers resuming.

**No Stage block.** `planner-tdd-soft` still writes `Plan.md` with its `## Stages` schedule and the per-stage files, but no row iterates over them: the plan is a deliverable here, not something the run executes.
**Fix targets.** Each reviewer's suggested fix target is its own creator. A `contracts-review` finding that traces back to the plan, or a `plan-review` finding that traces back to requirements, is routed upstream by the orchestrator's routing policy, not by a declared route.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 3.2 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 3.3 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 3.4 | 2026-09-28 | MOSAIC | Add Research.md to requirements-review inputs; its codebase alignment and feasibility checks run on research findings. |
| 3.5 | 2026-10-05 | MOSAIC | Remove note making design phase optional, it decreased realibility for very little savings |
| 4.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16. The routing table was already compliant (no blocks, all targets in the run's scope); removed the `**Notes:**` "Brownfield" definition, redundant with **Use when:**. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **Move HITL off the creator rows onto a convergence-gated `approval-presenter`.** `requirements-refinement`, `planner-tdd-soft`, and `contracts-designer` are all gated `TRUE` directly, so a human reviews every draft they produce — including rounds their paired reviewer would flag anyway — instead of only the version that already converged. `requirements-to-test-cases` proved the fix (a dedicated presenter row reachable only via the reviewer's `On Success`). Revisit this workflow once that pattern has enough real use to trust.

**Dead ends (tried and rejected):**
- (none yet)
