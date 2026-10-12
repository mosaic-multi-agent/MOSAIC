---
version: "1.1"
name: "MosaicTest Staged Findings Loop Workflow"
description: "Runner mode fixture — an Auto-review findings loop inside a grouped Stage block, where the same agent fills several rows. Proves the engine still knows which row it is on after a route-back, instead of working the row out from the invocation count."
hint: "Mode 3 test — row identity after an On Findings route-back inside a grouped block, with same-agent rows"
author: MOSAIC
id: staged-findings-loop
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/loop-writer.md
  - MosaicTestScript/loop-gate.md
  - MosaicTestScript/loop-review.md
  - MosaicTestScript/loop-impl.md
  - Stage-{Stage}/MosaicTestLoopWritten.md
  - Stage-{Stage}/MosaicTestLoopGate.md
  - Stage-{Stage}/MosaicTestLoopReview.md
  - Stage-{Stage}/MosaicTestLoopImpl.md
  - Plan.md
modes:
  - auto-review
---

<Workflow type="core" name="staged-findings-loop" version="1.1">
## MosaicTest Staged Findings Loop Workflow

**Use when:** Checking that after an Auto-review route-back inside a grouped `Stage` block, the engine continues from the row that actually ran, not from a row derived from the invocation count.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage].Test | mosaictest-scripted(writer) | FALSE | NEXT | - | MosaicTestScript/loop-writer.md | Stage-{Stage}/MosaicTestLoopWritten.md |
| EXECUTION.[Stage].Test | mosaictest-scripted(gate) | FALSE | NEXT | mosaictest-scripted(writer) | MosaicTestScript/loop-gate.md | Stage-{Stage}/MosaicTestLoopGate.md |
| EXECUTION.[Stage].Test | mosaictest-scripted(review) | FALSE | COMPLETE | - | MosaicTestScript/loop-review.md | Stage-{Stage}/MosaicTestLoopReview.md |
| EXECUTION.[Stage].Implementation | mosaictest-scripted(impl) | FALSE | COMPLETE | - | MosaicTestScript/loop-impl.md | Stage-{Stage}/MosaicTestLoopImpl.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |

**Notes:**
- **Auto-review only.** The route-back under test is the engine's own On Findings auto-route; Auto mode would hand it to the orchestrator instead.
- Row 1 (writer) and row 3 (review) and row 4 (impl) always return SUCCESS. Row 2 (gate) is marker-gated: the first pass writes its marker and returns `COMPLETED_NEEDS_ACTION`; the second pass finds the marker and returns `SUCCESS`.
- Row 2's `On Findings` is `mosaictest-scripted(writer)`, the whole `Subagent` cell of row 1. That is the intended route-back target (writer), standing in for `build-review → test-writer-tdd` in `brownfield-tdd-build-verified`.
- Every row uses the same agent, so the rows' `Subagent` cells are told apart by the mode qualifiers `writer`, `gate`, `review` and `impl`. Dispatch, instance ids and logs use the bare id `mosaictest-scripted`, so a script keyed on the agent name matches every row. That is the condition the test needs: several rows with one agent.
- The `Stage` block runs the `Test` group (rows 1-3) and then the `Implementation` group (row 4) for each plan stage, as the `TDD` approach lists them.
- `Plan.md` is pre-placed. Seed `Fixtures/staged-findings-loop`, the whole directory.

</Workflow>

---

## Design Rationale

### The defect class under test

When several rows share one agent, an engine that works out the row it just ran from the agent name and the global invocation number is wrong as soon as a findings route-back runs an earlier row again: the invocation count moves ahead of the real row position, the engine picks a later row than the one that actually ran, and carries on from there, skipping rows. The defect was found in the 1.x engine and is the reason this scenario exists.

Under WorkflowFormat 2.16 every `Subagent` cell is unique and a dispatch is bound to the row it was made for, so the row that ran is known, not derived. The scenario now guards that the engine keeps using it: after a route-back to the writer row, the next row is the gate that follows the writer, not a row computed from the count. The artifact records the iteration and group (`Iteration` = `1.Test`).

### How this workflow exposes it

| Seq | Row actually run | Row the seq arithmetic picks | Correct next | Buggy next |
|:---:|---|---|---|---|
| 1 | 1 writer | 1 | 2 gate | 2 gate |
| 2 | 2 gate → CNA | 2 | route back → 1 | route back → 1 |
| 3 | 1 writer (route-back) | **3 review** | 2 gate | **4 impl** |

On the buggy path, the gate never runs again and the review row never runs. The run still finishes with exit code 0 because every step it did dispatch succeeded. The difference shows up only in the number and order of dispatches, which is what `.expected.json` checks.

### Relation to the production workflow

In `brownfield-tdd-build-verified`, only `build-review` appears in two rows (one per group, told apart by mode qualifiers), so a count-derived row has to reach the other `build-review` row before it causes a misroute. One `tests-review-tdd` findings loop is enough:

`test-writer(1) → build-review(2) → tests-review(3) CNA → test-writer(4) → build-review(5)`

Position 5 is the Implementation group's `build-review`. The engine would then go to `implementation-review` and skip `tests-review-tdd` and `implementation-tdd`. This workflow triggers the same condition with less setup: every row shares one agent, so a single route-back is enough to cause the drift.

### Why one stage

The defect is about position within an iteration. A second iteration adds dispatches without adding a new condition.

---

## Expected Run: Auto-review Mode

Six workflow steps plus the pre-consultation. The pre-consultation takes no `Seq` and leaves no `Orchestration.md` row.

| Log `Seq` | `Agent` | `Phase` | `Iteration` | `Status` | `Summary` shows |
|:---:|---|---|---|---|---|
| 1 | `mosaictest-scripted#1` | EXECUTION | 1.Test | SUCCESS | row 1 / writer |
| 2 | `mosaictest-scripted#2` | EXECUTION | 1.Test | COMPLETED_NEEDS_ACTION | row 2 / gate / marker absent |
| 3 | `mosaictest-scripted#3` | EXECUTION | 1.Test | SUCCESS | row 1 / writer (route-back) |
| 4 | `mosaictest-scripted#4` | EXECUTION | 1.Test | SUCCESS | row 2 / gate / marker present |
| 5 | `mosaictest-scripted#5` | EXECUTION | 1.Test | SUCCESS | row 3 / review |
| 6 | `mosaictest-scripted#6` | EXECUTION | 1.Implementation | SUCCESS | row 4 / impl |

**Run outcome:** COMPLETE. No consultation after the pre-consultation.

**Inputs column check:** Seq 3 (the route-back) should list `Stage-1/MosaicTestLoopGate.md`, the gate's output that the engine injects on the findings route-back, in addition to `MosaicTestScript/loop-writer.md`.

### Buggy run, for recognition

| Log `Seq` | `Iteration` | `Status` | `Summary` shows |
|:---:|---|---|---|
| 1 | 1.Test | SUCCESS | row 1 / writer |
| 2 | 1.Test | COMPLETED_NEEDS_ACTION | row 2 / gate / marker absent |
| 3 | 1.Test | SUCCESS | row 1 / writer (route-back) |
| 4 | 1.Implementation | SUCCESS | row 4 / impl |

Four steps instead of six; the second gate pass and the review row are missing.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| Seq 4 is row 4 (`1.Implementation`), run ends after 4 steps | The defect class under test: the engine identified seq 3 as row 3 instead of row 1. The row after a route-back is derived from the invocation count rather than from the row that ran. |
| Seq 3 is not row 1 | The On Findings route-back target `mosaictest-scripted(writer)` was resolved to the wrong row, or the iteration was lost on the route-back dispatch. |
| A consultation appears after seq 2 | The engine treated the CNA as a deviation. Check that row 2's On Findings holds exactly one target that names a row, and that the run is in Auto-review mode. The routing fixture stops the run here on purpose. |
| Seq 4 returns CNA again | The gate's marker was not written on the first pass, or the route-back did not keep `Stage-1` in the marker path. |
| `FIXTURE_ERROR` naming `MosaicTestScript/` | The seed root was wrong; seed `Fixtures/staged-findings-loop` itself. |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-03 | MOSAIC | Initial version. Reproduces suspected row drift after a findings route-back among duplicate-agent EXECUTION rows. |
| 1.1 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16 notation: `EXECUTION.[Stage].Test` / `.Implementation` rows with `**Blocks:**` and `**Groups:**` tables, `NEXT` and `COMPLETE` instead of `-`, unique `Subagent` cells through the mode qualifiers `writer`, `gate`, `review`, `impl`, `On Findings` names the whole cell, `Plan.md` listed in `artifacts`. Expected Run uses the OrchestrationArtifactFormat 3.0 log values (`Iteration` `1.Test`). Dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A variant whose route-back targets a row in the *Implementation* group while the same agent also has a Test-group row, to cover a route-back target that lies in another group than the row that returned findings.

**Dead ends (tried and rejected):**
- (none yet)
