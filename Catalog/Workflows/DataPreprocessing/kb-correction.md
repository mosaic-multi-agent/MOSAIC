---
version: "1.0"
name: Knowledge Base Correction Workflow
description: Apply known corrections to an existing knowledge base. Input is user-provided correction instructions in Requirements.md — could be pasted verification findings, direct feedback, or change descriptions.
hint: "Theoretical — no confirmed real use. Its niche may be narrower than it looks: kb-generation already refreshes an existing KB and flags drift on re-run, and in practice a planner can often fold small doc corrections into a normal feature plan without a dedicated pass. Consider whether generation's own refresh path already covers your case before reaching for this."
author: MOSAIC
id: kb-correction
referenced_agents:
  - knowledge-base-generator
  - knowledge-base-index-assembler
artifacts:
  - Requirements.md
  - KBProgress.md
---

<Workflow type="core" name="kb-correction" version="1.0">
## Knowledge Base Correction Workflow

**Use when:** Apply known corrections to an existing knowledge base. Input is user-provided correction instructions in Requirements.md — could be pasted verification findings, direct feedback, or change descriptions.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | knowledge-base-generator(plan-corrections) | TRUE | [Correction] | - | Requirements.md | KBProgress.md |
| EXECUTION.[Correction] | knowledge-base-generator(correct) | FALSE | COMPLETE | - | Requirements.md, KBProgress.md | KBProgress.md |
| COMPLETION | knowledge-base-index-assembler | FALSE | COMPLETE | - | KBProgress.md | KBProgress.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Correction | KBProgress.md | Corrections |
</Workflow>

---

## Design Rationale

Intended as the targeted-correction counterpart to `kb-generation`: apply known, user-specified corrections (verification findings, direct feedback, change descriptions) rather than doing a full generation/refresh pass. No confirmed real use so far — theoretical.

Its actual necessity is unclear. `kb-generation` already handles refreshing an existing KB and flagging drift when re-run on a changed codebase, and separately, planners running normal feature work have shown they can often fold small KB corrections into their own plan without a dedicated correction pass. Between those two, the scope this workflow is meant to fill — corrections that are neither a full drift-driven refresh nor small enough to ride along in a feature plan — may be narrow enough that this workflow is overkill. Worth revisiting once (or if) a real case shows up that neither of the other two paths covers.

**Shape — shared with `kb-generation`.** The generator appears twice, with the same modes and the same `## Corrections` schedule as `kb-generation`'s correction pass, so the agent has one table layout and one mode set across the KB family. The `plan-corrections` row reads Requirements.md against the existing KB (navigated via `CodeKnowledgeBase/Index.md`, which must exist — the generator returns BLOCKED otherwise) and writes the `## Corrections` schedule in KBProgress.md: one correction per KB document, deepest tier first, each depending on the one before. The `correct` row then applies the corrections to one KB document per Correction iteration, in that bottom-up order. A single row that both created the schedule and worked it would put the block's schedule source inside its own block, which the format forbids (A7).

**HITL.** One human gate, at the start: the user reviews the correction schedule (`plan-corrections`, `TRUE`) before anything is changed. The `correct` rows are `FALSE`; per-correction review is available through the schedule's `HITL` column, set during that review.

**Growing the schedule.** If a correction reveals further documents that need correcting, the `correct` row appends new Correction iterations to KBProgress.md; they run in the same block pass (rows of a block may extend its schedule).

**No re-verification.** After corrections the workflow ends; run a verification workflow separately to confirm the fixes.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 0.1 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 0.2 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 0.3 | 2026-10-04 | MOSAIC | Align the existing-KB index path with the generator and catalog convention: CodeKnowledgeBase/Index.md. |
| 1.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16 and aligned with `kb-generation`: the self-bootstrapping block row is split into `knowledge-base-generator(plan-corrections)` (outside the block, writes the `## Corrections` schedule in KBProgress.md, as A7 requires) and `knowledge-base-generator(correct)` in the declared `[Correction]` block — the same mode and schedule `kb-generation` uses for its correction pass. Bottom-up correction order via `Depends On`. One human gate at the schedule review; correction rows `FALSE` (was `TRUE`). Block row ends its iteration with `COMPLETE`; the block exits to the index assembler. Version quoted. Notes removed — seeds and bootstrap are now in the table; schedule growth and no-re-verification moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **Possible redundancy with kb-generation's refresh path and planner-folded corrections.** Never confirmed in real use. Before investing further in this workflow, check whether a real correction case actually falls outside what kb-generation's refresh/drift-flagging and normal feature-plan doc updates already handle.

- **Index assembly sees only the corrected documents.** KBProgress.md in this workflow holds only the `## Corrections` table, so `knowledge-base-index-assembler` reads the corrected documents (plus the KB folder's top-level areas) rather than the whole KB when looking for cross-area patterns. Fine for small correction passes; revisit if the workflow gets real use.

**Dead ends (tried and rejected):**
- (none yet)
