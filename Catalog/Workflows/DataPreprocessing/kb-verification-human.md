---
version: "1.0"
name: Knowledge Verification (Human) Workflow
description: Verify knowledge quality using architect-provided challenge questions. Tests whether an agent can answer expert questions using available knowledge sources + codebase. Produces a diagnostic report — remediation is a separate concern.
hint: "Theoretical — never used in practice. Beyond its stated diagnostic purpose, this is also a natural harness for an A/B comparison: run the same question set with and without an existing KB in scope to measure the actual delta in speed, cost, and answer precision the KB provides, rather than assuming it."
author: MOSAIC
id: kb-verification-human
referenced_agents:
  - verification-questions-preparer
  - codebase-research
  - verification-answer-validator
artifacts:
  - VerificationQuestions.md
  - VerificationAnswers.md
  - VerificationAttemptedAnswers.md
  - VerificationReport.md
---

<Workflow type="core" name="kb-verification-human" version="1.0">
## Knowledge Verification (Human) Workflow

**Use when:** Verify knowledge quality using **architect-provided challenge questions**. Tests whether an agent can answer expert questions using available knowledge sources + codebase. Produces a diagnostic report — remediation is a separate concern.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | verification-questions-preparer | TRUE | [Batch] | - | - | VerificationQuestions.md, VerificationAnswers.md, VerificationAttemptedAnswers.md |
| EXECUTION.[Batch] | codebase-research | FALSE | COMPLETE | - | VerificationQuestions.md, VerificationAttemptedAnswers.md | VerificationAttemptedAnswers.md |
| REVIEW | verification-answer-validator | TRUE | COMPLETE | - | VerificationQuestions.md, VerificationAnswers.md, VerificationAttemptedAnswers.md | VerificationReport.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Batch | VerificationAttemptedAnswers.md | Batches |
</Workflow>

---

## Design Rationale

Theoretical at this point — designed but never actually run. Its stated purpose is diagnostic: does the available knowledge (KB, docs, code comments, whatever the research agent can reach) let an agent correctly answer expert-authored challenge questions. The workflow ends at VerificationReport.md; acting on the findings is a separate remediation workflow (e.g. `kb-correction`).

There's a second use this workflow's structure naturally supports and hasn't been exploited yet: because it's knowledge-source agnostic by design, the same question set can be run twice — once with an existing KB in scope, once without — to directly measure what the KB actually buys in speed, cost, and answer precision, rather than assuming `kb-generation` is worth its cost on faith.

**Batches as a block.** Each question batch is one iteration of the `Batch` block; the preparer's `## Batches` schedule in VerificationAttemptedAnswers.md lists them. Without a `Depends On` column every batch may run concurrently — the planned-parallelism replacement for the former `codebase-research*` fan-out and its `Waits For` join, which the block's exit now provides. Each `codebase-research` instance answers only its own batch's `## Batch {N}` section of the shared VerificationAttemptedAnswers.md; the task description carries the batch ID.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 0.4 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 0.5 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 1.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: `[StageNumber]` → declared block `[Batch]` (source VerificationAttemptedAnswers.md, section `Batches`); `codebase-research*` fan-out and `Waits For` join replaced by block entry (`[Batch]`) and block exit; `Waits For` column dropped; version quoted. Notes removed — redundant with the tables or moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **With/without-KB comparison run.** Run the same VerificationQuestions.md twice — once against a codebase with an existing KB, once with the KB excluded from scope — and diff the resulting VerificationReport.md for speed, cost, and precision. Would turn this from a pass/fail diagnostic into actual evidence for whether kb-generation is worth running on a given codebase.
- **Per-batch attempted-answer files.** Concurrent batches all write the one VerificationAttemptedAnswers.md. Splitting it into `Batch-{Batch}/AttemptedAnswers.md` would give each iteration its own per-iteration artifact and remove the shared-file write contention; needs preparer and validator changes.

**Dead ends (tried and rejected):**
- (none yet)
