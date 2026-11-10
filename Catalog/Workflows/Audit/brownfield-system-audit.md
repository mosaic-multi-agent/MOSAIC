---
version: "2.0"
name: "Brownfield System Audit Workflow"
description: "High-level quality assessment of an existing codebase or major subsystem — architecture and contracts audit without file-level analysis."
hint: "Theoretical, at most field-tested once — treat as unproven. Essentially the fixed architecture/contracts tracks trimmed out of brownfield-pr-audit, with the expensive staged per-file audits and PR comment integration removed. Use to scope out problem areas cheaply before committing to a deeper, per-file audit."
author: MOSAIC
id: brownfield-system-audit
referenced_agents:
  - requirements-refinement
  - codebase-research
  - architecture-audit
  - contracts-audit
artifacts:
  - Requirements.md
  - Research.md
  - ResearchArchitecture.md
  - ResearchContracts.md
  - ArchitectureAudit.md
  - ContractsAudit.md
---

<Workflow type="core" name="brownfield-system-audit" version="2.0">
## Brownfield System Audit Workflow

**Use when:** High-level quality assessment of an **existing codebase** or major subsystem — architecture and contracts audit without file-level analysis. Use to identify problem areas before deeper per-component audits.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | requirements-refinement | TRUE | codebase-research | - | Requirements.md | Requirements.md |
| RESEARCH | codebase-research | FALSE | codebase-research(architecture) | - | Requirements.md | Research.md |
| RESEARCH | codebase-research(architecture) | FALSE | codebase-research(contracts), architecture-audit | - | Requirements.md, Research.md | ResearchArchitecture.md |
| RESEARCH | codebase-research(contracts) | FALSE | contracts-audit | - | Requirements.md, Research.md, ResearchArchitecture.md | ResearchContracts.md |
| EXECUTION | architecture-audit | FALSE | COMPLETE | - | Requirements.md, Research.md, ResearchArchitecture.md | ArchitectureAudit.md |
| EXECUTION | contracts-audit | FALSE | COMPLETE | - | Requirements.md, Research.md, ResearchContracts.md | ContractsAudit.md |
</Workflow>

---

## Design Rationale

An essential trim of `brownfield-pr-audit`: keeps its fixed architecture and contracts audit tracks (the high-level, whole-codebase passes) but drops the staged per-file audit tracks (tests-audit, implementation-audit) and the entire PR comment integration layer (audit-to-pull-request, audit-response-merger, pull-request-comment-interface). What's left is a cheap, high-level pass meant to identify problem areas before paying for expensive per-file analysis — its output (ArchitectureAudit.md, ContractsAudit.md) can guide follow-up per-component PR audits.

Theoretical in practice — tested at most once. Treat any claims about its real-world output quality as unverified until it accumulates more actual use.

**Seed.** The user supplies `Requirements.md`, stating the audit scope (subsystem, codebase area, or full system) and focus areas; `requirements-refinement` sharpens it with the user.

**Parallel tracks.** `codebase-research(architecture)` forks into two branches: the architecture audit, which needs only the architecture research, and the contracts track (contracts research, then contracts audit). The fork closes at the end of the run — each branch ends with `COMPLETE`, and the run ends when both audits have finished — so no join and no `Waits For` column are needed.

**No fix targets.** Audit findings are the deliverable, not a failure state, so every `On Findings` is `-`.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 1.1 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 2.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16: removed in-region `> **Version:**` line; dropped the `Waits For` column (no row joins — the fork closes at the end of the run, T13). Notes removed — redundant with the table or the format (parallel execution, mode qualifiers, empty fix targets, completion); seed and purpose guidance moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- (none yet)

**Dead ends (tried and rejected):**
- (none yet)
