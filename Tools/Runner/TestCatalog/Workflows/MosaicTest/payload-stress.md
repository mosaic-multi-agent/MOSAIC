---
version: "1.1"
name: "MosaicTest Payload Stress Workflow"
description: "Harness conformance fixture — three awkward status_message payloads (unicode, fenced backticks, JSON-in-JSON) returned through one run, to find where a harness mangles encoding or escaping."
hint: "Harness test — unicode, backtick and JSON-in-JSON payload fidelity"
author: MOSAIC
id: payload-stress
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/payload-unicode.md
  - MosaicTestScript/payload-fences.md
  - MosaicTestScript/payload-json.md
  - Plan.md
modes:
  - auto
  - auto-review
  - orchestrated
---

<Workflow type="core" name="payload-stress" version="1.1">
## MosaicTest Payload Stress Workflow

**Use when:** Checking whether a harness carries awkward `status_message` content through intact. This is where harnesses genuinely differ, and a failure here is usually an escaping or encoding bug rather than a routing one.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | mosaictest-scripted(unicode) | FALSE | NEXT | - | MosaicTestScript/payload-unicode.md | - |
| EXECUTION.[Stage] | mosaictest-scripted(fences) | FALSE | NEXT | - | MosaicTestScript/payload-fences.md | - |
| EXECUTION.[Stage] | mosaictest-scripted(json) | FALSE | COMPLETE | - | MosaicTestScript/payload-json.md | - |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Notes:**
- **`Plan.md` is a pre-placed fixture.** It and the scripts must be seeded into the run folder at run creation. Seed `Fixtures/payload-stress` — the whole directory, not anything inside it — as the single seed path. See `Fixtures/README.md`.
- The `Stage` block runs once per row of the `## Stages` table in `Plan.md`. The fixture plan declares a single stage, so each row runs exactly once.
- One payload class per row, so a failure names the class that broke rather than "the payload broke." The three rows run the same agent, so their `Subagent` cells are told apart by the mode qualifiers `unicode`, `fences` and `json`; dispatch, instance ids and logs use the bare id `mosaictest-scripted`.
- The block has no groups, so the `Approach` column of the plan is ignored.
- Nothing is written. The payload under test travels only in `status_message`, which is where a harness's own JSON envelope has to carry it.

</Workflow>

---

## Design Rationale

The three rows sit in a `Stage` block over a pre-placed single-stage plan so that the payloads travel through the block-iteration path of the Runner as well as the plain dispatch path. The same agent occupies all three rows, and every row is a distinct `Subagent` cell through a mode qualifier; inside the iteration `NEXT` moves from one row to the next and the last row ends the iteration. A single-stage plan gives three sequential invocations of one stub — the shape this fixture needs — at the cost of a pre-placed plan.

The payload classes are separated because they fail for different reasons and are fixed in different places. Unicode failures are encoding — code page, byte-order, or astral-plane handling. Fenced backticks fail when a harness or a wrapper re-parses the response as markdown and the payload terminates a block early. JSON-in-JSON fails on escaping, and is the one most likely to corrupt the envelope itself rather than just the message. Combining them into one payload would leave the diagnosis to guesswork.

Row order runs cheapest-to-diagnose first. If the unicode row already mangles its output there is little point reading the JSON row's result closely.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-03 | MOSAIC | Initial version |
| 1.1 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16 notation: `EXECUTION.[Stage]` with a `**Blocks:**` table over `Plan.md`, `NEXT` and `COMPLETE` instead of `-` in `On Success`, unique `Subagent` cells through the mode qualifiers `unicode`, `fences`, `json`, `Plan.md` listed in `artifacts`. Dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A fourth row carrying several kilobytes of text, to find truncation limits. Left out of v1 because the failure mode is a threshold rather than a yes/no, so it needs a different readout than "watch the TUI."
- Payloads containing a literal `[checkpoint:...]` marker, to check that marker extraction does not misfire on agent-supplied text.

**Dead ends (tried and rejected):**
- Three non-staged rows. Under 1.x notation this was not authorable (`On Success` resolved by agent name to the first matching row, so rows 2 and 3 were unreachable and row 1 would loop). Mode qualifiers now make it authorable; the block is kept so that the fixture also exercises block iteration.
