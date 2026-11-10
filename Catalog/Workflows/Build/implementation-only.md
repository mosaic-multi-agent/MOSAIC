---
version: "4.0"
name: "Implementation Only Workflow"
description: "Research, planning, and design already complete. Direct implementation from existing artifacts."
hint: "Likely obsolete — this is the EXECUTION/REVIEW tail of brownfield-tdd/greenfield-tdd, split out to manually resume a long workflow past RESEARCH/PLANNING/DESIGN. Native mid-execution run persistence and continuation now covers that need directly; prefer continuing the original run over starting this one."
author: MOSAIC
id: implementation-only
referenced_agents:
  - implementation-tdd
  - implementation-review
  - test-runner
artifacts:
  - Plan.md
  - Stage-{Stage}/Plan.md
  - ContractsDesign.md
  - Stage-{Stage}/PlanProgress.md
  - Stage-{Stage}/implementation-review.md
  - TestResults.md
---

<Workflow type="core" name="implementation-only" version="4.0">
## Implementation Only Workflow

**Use when:** Research, planning, and design already complete. Direct implementation from existing artifacts.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | implementation-review | - | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, ContractsDesign.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>

---

## Design Rationale

Created by splitting the tail (EXECUTION + REVIEW) off `brownfield-tdd`/`greenfield-tdd`, so a run with Plan.md and ContractsDesign.md already produced elsewhere could jump straight to implementation without repeating RESEARCH/PLANNING/DESIGN. That was the only way to "continue" a long workflow at the time: hand-carry the prerequisite artifacts into a fresh run of a shorter workflow.

Mid-execution run persistence and continuation now solves the same problem natively — resuming the actual run that produced those artifacts, rather than starting a new, differently-scoped workflow against artifacts it never produced itself. That makes this workflow's original justification largely obsolete; it survives mainly as a fallback for cases where the artifacts genuinely originate outside any MOSAIC run (e.g. hand-written Plan/ContractsDesign) rather than as a continuation mechanism.

**Seeds.** The first row begins the Stage block, so the seeds are its Inputs — `ContractsDesign.md` and, per stage, `Stage-{Stage}/Plan.md` and `Stage-{Stage}/PlanProgress.md` — plus the block schedule source `Plan.md` (its `## Stages` table). All must be supplied by the user.

**Ungrouped block.** The sequence per stage is fixed (implementation-tdd → implementation-review), so the Stage block has no groups and ignores the `Approach` column of the plan's stage table. A plan whose stages need tests written first belongs in a TDD workflow, not here.

**Fix targets.** `implementation-review` sends findings to `implementation-tdd` within the same stage. `test-runner` runs after the Stage block has exited and has no declared fix target: there is no planner in this workflow to add a fix stage, and a block row cannot be named from outside the block. Where failing tests go — re-dispatching implementation, or escalating to the user to amend the plan — is the orchestrator's decision under its routing policy.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 3.1 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 3.2 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 4.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: `[StageNumber]` → declared block `[Stage]` (`**Blocks:**` table, source `Plan.md` § `Stages`). `implementation-review` ends the stage with `COMPLETE` (was a jump out of the block to `test-runner`; the block's exit now continues there). `test-runner` fix target removed (was a block row, out of scope). Notes removed: the per-stage loop is expressed by the block; prerequisites are the seeds (moved to Design Rationale). |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- (none yet)

**Dead ends (tried and rejected):**
- **Splitting workflows to enable manual continuation:** the underlying pattern this workflow represents. Superseded by native mid-execution run persistence/continuation. Worth remembering if the temptation to split another long workflow into a "resume from phase X" variant comes up again — the run-continuation mechanism is very likely the better answer now.
