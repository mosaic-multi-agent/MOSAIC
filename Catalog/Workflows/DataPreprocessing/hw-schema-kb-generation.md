---
version: "1.0"
name: HW Schema Knowledge Base Generation Workflow
description: Generate knowledge base documentation for a hardware schematic design. Researches each sheet individually, then synthesizes domain-oriented KB documentation with tiered abstraction — from project overview down to complex circuit subsystems.
hint: "Tested and polished a few times, output verified — not production-proven but solid. Hard prerequisite before deploying: the research step requires a working hw_schema_read tool wired up for your specific schematic format/tooling — this is not a standard tool available out of the box, and the workflow cannot function without it. Verify that tool is provided before selecting this workflow."
author: MOSAIC
id: hw-schema-kb-generation
referenced_agents:
  - hw-schema-planner
  - hw-schema-research
  - hw-schema-kb-generator
  - knowledge-base-flag-sorter
  - knowledge-base-index-assembler
artifacts:
  - Requirements.md
  - HWResearchProgress.md
  - KBProgress.md
  - KBFlags.md
  - KBFlagReport.md
---

<Workflow type="core" name="hw-schema-kb-generation" version="1.0">
## HW Schema Knowledge Base Generation Workflow

**Use when:** Generate knowledge base documentation for a **hardware schematic design**. Researches each sheet individually, then synthesizes domain-oriented KB documentation with tiered abstraction — from project overview down to complex circuit subsystems.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | hw-schema-planner | TRUE | [Sheet] | - | Requirements.md | HWResearchProgress.md |
| RESEARCH.[Sheet] | hw-schema-research | FALSE | COMPLETE | - | Requirements.md, HWResearchProgress.md | HWResearchProgress.md |
| EXECUTION | hw-schema-kb-generator(overview) | TRUE | [Generation] | - | Requirements.md, HWResearchProgress.md | KBProgress.md |
| EXECUTION.[Generation] | hw-schema-kb-generator(generate) | FALSE | COMPLETE | - | Requirements.md, HWResearchProgress.md, KBProgress.md | KBProgress.md, KBFlags.md |
| REVIEW | knowledge-base-flag-sorter | FALSE | [Correction] | - | KBProgress.md, KBFlags.md | KBFlagReport.md, KBProgress.md |
| REVIEW.[Correction] | hw-schema-kb-generator(correct) | FALSE | COMPLETE | - | KBProgress.md, KBFlagReport.md | KBProgress.md |
| COMPLETION | knowledge-base-index-assembler | FALSE | COMPLETE | - | KBProgress.md | KBProgress.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Sheet | HWResearchProgress.md | Stages |
| Generation | KBProgress.md | Generation |
| Correction | KBProgress.md | Corrections |
</Workflow>

---

## Design Rationale

Tested a handful of times with output manually verified and the workflow polished as a result — not yet production-proven, but past the purely theoretical stage.

**This workflow has an unusually explicit external dependency, worth calling out at the workflow level rather than leaving it buried in an agent's tool list.** The research stage requires the ability to actually read hardware schematic sheets, which happens through a dedicated tool the research agent depends on. Unlike the file/terminal tools every other workflow in the catalog assumes are simply available, this one is not generic — it has to be provided by whoever deploys the workflow, wired to their specific schematic format and tooling. Every other workflow in the catalog gets by on generic file/terminal access; this is the one case where the workflow cannot run at all without a project-specific tool being present first. Users select workflows, not agents, so this dependency needs to be visible here, not only inside the research agent's own instructions.

**Seed.** `Requirements.md` is user-written and must name the schematic project path and the KB output path; focus areas are optional.

**Three blocks, three schedules.**
- **Sheet** — the planner writes one row per sheet into `HWResearchProgress.md`. Sheets have no dependencies, so every sheet's research may run in parallel. Each sheet's findings go to a project file (e.g. `SheetsResearch/Sheet-03.md`), not an orchestration artifact; its path is recorded in `HWResearchProgress.md`, which is how the KB generator finds it. These files are temporary and can be removed after the run.
- **Generation** — the Tier 1 overview runs once, outside any block (`hw-schema-kb-generator(overview)`), because it is what discovers the functional domains: it reads every sheet's research and creates `KBProgress.md` with one Generation row per domain. Generation iterations may append deeper-tier rows as they discover topics that need them (WorkflowFormat §9.3), so the block keeps running until none is left. The overview row is human-gated; the Generation row is not, and per-domain review is switched on through the schedule's `HITL` column.
- **Correction** — the flag sorter turns accumulated flags into one correction row per target document, ordered bottom-up by tier and chained through `Depends On`, so deeper documents are corrected before their parents.

Two progress artifacts are kept apart on purpose: `HWResearchProgress.md` tracks per-sheet research, `KBProgress.md` tracks KB generation and correction — different concerns with different lifecycles.

**No fix targets.** None of the rows is a reviewer; inaccuracies found during generation travel as correction flags and are handled by the Correction block, not by routing.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 0.5 | 2026-08-17 | MOSAIC | Changelog tracking begins here; earlier revisions predate this record. |
| 0.6 | 2026-08-26 | MOSAIC | Replace Unicode emoji with ASCII tokens in HITL column (TRUE/FALSE). |
| 1.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16. `[StageNumber]` replaced by three declared blocks — `Sheet` (`HWResearchProgress.md` § Stages), `Generation` (`KBProgress.md` § Generation) and `Correction` (`KBProgress.md` § Corrections). `Waits For` barriers (`hw-schema-research*`, legacy suffix) dropped: block exit is the barrier, so the column is gone. The first KB generator run is now its own row, `hw-schema-kb-generator(overview)`, outside the Generation block, since a block's schedule must exist before the block is entered; it carries the human gate, and the Generation row is `FALSE` with per-domain HITL from the schedule. Routing scope-correct: rows enter blocks with `[Name]`, block rows end their iteration with `COMPLETE`. Notes removed — redundant with the tables or moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- (none yet)

**Dead ends (tried and rejected):**
- (none yet)
