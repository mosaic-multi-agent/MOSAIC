---
version: "2.0"
name: "Knowledge Base Generation Workflow"
description: "Generate N-tier knowledge base documentation for a codebase, producing hierarchical documentation optimized for AI agent navigation — tiered from project overview down to complex subsystem specs."
hint: "Recommended first step before any other workflow on a codebase — research agents downstream actively look for an existing KB and perform noticeably better when one is present. The KB is deliberately abstract (no concrete file/class/method names), which makes it resistant to going stale — in practice, feature planners can often fold doc updates into a normal feature plan without much quality loss, so a dedicated re-run of this workflow isn't required after every change."
author: MOSAIC
id: kb-generation
referenced_agents:
  - knowledge-base-generator
  - knowledge-base-flag-sorter
  - knowledge-base-index-assembler
artifacts:
  - Requirements.md
  - KBProgress.md
  - KBFlags.md
  - KBFlagReport.md
---

<Workflow type="core" name="kb-generation" version="2.0">
## Knowledge Base Generation Workflow

**Use when:** Generate N-tier knowledge base documentation for a codebase. Produces hierarchical documentation optimized for AI agent navigation — tiered from project overview down to complex subsystem specs.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | knowledge-base-generator(bootstrap) | TRUE | [Generation] | - | Requirements.md | KBProgress.md |
| EXECUTION.[Generation] | knowledge-base-generator(generate) | FALSE | COMPLETE | - | Requirements.md, KBProgress.md | KBProgress.md, KBFlags.md |
| REVIEW | knowledge-base-flag-sorter | FALSE | [Correction] | - | KBProgress.md, KBFlags.md | KBFlagReport.md, KBProgress.md |
| REVIEW.[Correction] | knowledge-base-generator(correct) | FALSE | COMPLETE | - | KBProgress.md, KBFlagReport.md | KBProgress.md |
| COMPLETION | knowledge-base-index-assembler | FALSE | COMPLETE | - | KBProgress.md | KBProgress.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Generation | KBProgress.md | Generation |
| Correction | KBProgress.md | Corrections |
</Workflow>

---

## Design Rationale

Excellent, recommended workflow — run this first on a codebase before starting any other workflow against it. The research-phase agents used across the Build/Audit/Design workflows actively look for an existing KB when they run, and their performance improves noticeably when one is present.

The KB's tiered structure is deliberately kept abstract — general subsystem descriptions rather than concrete file, class, or method names — which trades a little specificity for resistance to going stale as the codebase changes underneath it. This pays off in practice: planning agents elsewhere in the catalog can often fold documentation updates for a change directly into a normal feature plan without much quality loss, precisely because the abstraction level means most code-level changes don't invalidate what the KB actually says. That means `kb-correction` is not required after every change — it's there for drift that accumulates past what in-line plan updates catch, not as a mandatory step in every feature's lifecycle.

**Seed.** `Requirements.md` is user-written: the generation scope, optional focus areas, and optionally the KB output path. Re-running the workflow on an existing KB refreshes it after codebase changes — the generator updates existing documents and flags new drift. KB quality is tested separately, by a knowledge-verification workflow run after generation.

**Two blocks over one progress artifact.** `KBProgress.md` holds two block schedules: `## Generation` (one iteration per KB document to write) and `## Corrections` (one iteration per KB document to correct). The generator appears in three rows, told apart by mode: `(bootstrap)` creates `KBProgress.md` with the initial Generation iteration(s) before the block is entered — a block schedule must exist before its block runs — then `(generate)` writes one document per iteration, and `(correct)` applies the sorted flags to one document per iteration.

**Dynamic, tier-sequential generation.** Each generation iteration may append deeper-tier iterations to the Generation schedule as it discovers areas that need them; their `Depends On` makes each tier wait for the tier above, so tiers run one after another while the documents of one tier run in parallel. The block exits only once every iteration — including the ones added during the run — has completed, so the flag sorter sees every flag.

**Bottom-up corrections.** The flag sorter writes the Corrections schedule ordered deepest tier first, each correction depending on the one before, so corrections run one at a time: a higher-tier document is corrected only after the lower-tier documents it summarises are accurate. With no flags, the Corrections schedule is empty and the block exits at once.

**HITL.** One human gate, at the start: the `bootstrap` row is `TRUE`, so the user reviews the initial Generation schedule — scope and Tier 1 plan — before any documentation is written. Every following row is `FALSE`, and the generator writes every schedule row with `HITL` `FALSE`, keeping the rest of the run frictionless. Per-iteration review stays available through the `HITL` column of the Generation schedule — set it to `TRUE` during the bootstrap review for the iterations you want to see.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 1.1 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 1.2 | 2026-09-10 | MOSAIC | Set HITL to FALSE by default for frictionless first-run experience. Per-stage HITL still configurable in KBProgress.md. |
| 2.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16. `[StageNumber]` split into two declared blocks over `KBProgress.md` — `[Generation]` (section `Generation`) and `[Correction]` (section `Corrections`). New `knowledge-base-generator(bootstrap)` row creates the Generation schedule before the block (A7); the first generation iteration no longer bootstraps it. `Waits For` column removed — the flag sorter follows the Generation block's exit, so no join is needed (T13); legacy `*` suffix gone. Block rows end their iteration with `COMPLETE`. Tier-sequential and bottom-up ordering now come from `Depends On` in the schedules. The `bootstrap` row is the one human gate (`TRUE`); all following rows are `FALSE`. Notes removed — redundant with the tables or moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- (none yet)

**Dead ends (tried and rejected):**
- (none yet)
