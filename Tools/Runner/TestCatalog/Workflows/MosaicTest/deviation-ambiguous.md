---
version: "2.0"
name: "MosaicTest Deviation Ambiguous Workflow"
description: "Runner mode fixture — Auto-review mode with a reviewer row that has no findings target. Proves that a COMPLETED_NEEDS_ACTION with nowhere to route back deviates even in Auto-review, where a row with a findings target would auto-route."
hint: "Mode 3 test — On Findings is - (no route-back target), so auto-review deviates to the orchestrator"
author: MOSAIC
id: deviation-ambiguous
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/ambiguous-produce.md
  - MosaicTestAmbiguousReport.md
modes:
  - auto-review
---

<Workflow type="core" name="deviation-ambiguous" version="2.0">
## MosaicTest Deviation Ambiguous Workflow

**Use when:** Verifying that Auto-review mode does NOT auto-route `COMPLETED_NEEDS_ACTION` when the row names no findings target (`On Findings` is `-`). With no row to send the findings back to, the Runner has no route-back target; the result must become a deviation exactly as it would in Auto mode.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| REVIEW | mosaictest-scripted | FALSE | COMPLETE | - | MosaicTestScript/ambiguous-produce.md | MosaicTestAmbiguousReport.md |

**Notes:**
- **Run in Auto-review mode.** The whole point is to show that Auto-review does NOT auto-route when the row has no findings target. Running in Auto mode would produce the same log (Auto always deviates on CNA) and prove nothing specific.
- `On Findings` is `-`: the row has no suggested fix target, so the engine cannot name a row to dispatch after the CNA and consults the orchestrator. The id `deviation-ambiguous` predates the rewrite (the findings target was once a prose value that could not be resolved) and is kept because the seed paths, the expected file and the catalogue documents refer to it.
- The script is marker-gated: first invocation returns `COMPLETED_NEEDS_ACTION` and writes the marker; second invocation returns `SUCCESS`.
- No block rows. The workflow has no `**Blocks:**` table, so no `Plan.md` is required.
- Seed `Fixtures/deviation-ambiguous` as the single seed path.

</Workflow>

---

## Design Rationale

### Why this needs its own workflow

`findings-loop` proves that Auto-review auto-routes CNA when the row names a findings target. This workflow proves the converse: that Auto-review does NOT auto-route when the row names none. Without both, the suite cannot distinguish "Auto-review auto-routes every CNA" from "Auto-review auto-routes a CNA only when the table gives it a target."

### Why On Findings is `-`

A findings target is one `Subagent` cell, or `-` for none (WorkflowFormat 2.16 sec. 6.5); prose, alternatives and annotations are refused when the workflow is admitted. The only way left for a row to give the engine no route-back target is `-`, which is also what a real reviewer row without a fix target looks like. An earlier version of this workflow wrote a prose value (`mosaictest-scripted (re-check)`) and relied on the Runner admitting it and deviating at runtime. Such a value is now refused at admission, so the scenario would never have started.

### Why marker-gated instead of override

Unlike `deviation-blocked`, this workflow's first invocation returns `COMPLETED_NEEDS_ACTION` (not `BLOCKED`), and a CNA-returning agent legitimately writes output artifacts. The marker file is the output artifact `MosaicTestAmbiguousReport.md`, which doubles as the fixture's state transition. No script override is needed because the second invocation reads the same script but takes a different branch.

---

## Expected Run

Two Orchestration.md log rows. A dispatch-log consultation appears even though the mode is Auto-review — because the row has no findings target — but it leaves no row in `Orchestration.md` and consumes no `Seq`.

| Log `Seq` | `Agent` | `Phase` | `Iteration` | `Status` | `Summary` shows |
|:---:|---|---|---|---|---|
| 1 | `mosaictest-scripted#1` | REVIEW | - | COMPLETED_NEEDS_ACTION | findings produced, marker written |
| 2 | `mosaictest-scripted#2` | REVIEW | - | SUCCESS | marker present, echoed task description |

**Run outcome:** completed normally (`RunCompleted`). The engine routed the recovery dispatch's SUCCESS to COMPLETE via On Success.

**The proof:** The dispatch log shows an `orchestrator-script` consultation between row 1 and row 2, even though the mode is Auto-review. The missing findings target forced the engine to deviate. The consultation still leaves no `Orchestration.md` row, so the re-dispatch is recorded as `mosaictest-scripted#2`, immediately following `#1`.

**Contrast with `findings-loop` Auto-review run:** That run has the same mode and the same status code, but its reviewer row names a findings target (`mosaictest-scripted(create)`) — and produces NO dispatch-log consultation at all. Together, the two workflows prove the engine routes back only when the table gives it a target.

**Note:** `mosaictest-review` (interval 3) may fire after Seq 2 if deployed. Account for an additional infrastructure row.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| No consultation in the dispatch log — run completes in 2 steps identically to `findings-loop` Auto-review | The engine auto-routed a CNA from a row whose `On Findings` is `-`: it invented a route-back target instead of deviating |
| Consultation happens but names an unknown agent | The routing table is not reaching the consultant |
| Second dispatch returns COMPLETED_NEEDS_ACTION instead of SUCCESS | The marker file was not written by the first invocation |
| Run stops after Seq 1 with deviation unresolved | The orchestrator consultation path is not wired for this harness |
| An `orchestrator-script` row appears in `Orchestration.md` | Consultations must leave no row in the artifact; a routing consultation wrongly called `Store.Apply` or allocated a `Seq` |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-17 | MOSAIC | Initial version. Proves ambiguous On Findings forces deviation in Auto-review. |
| 2.0 | 2026-10-12 | MOSAIC | Rewritten for WorkflowFormat 2.16: the prose `On Findings` value `mosaictest-scripted (re-check)` is refused at admission, so the row now has `-` and the scenario proves that a CNA with no route-back target deviates in Auto-review. Phase `REVIEW`, prose and failure table updated; script, routing rule and dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A variant with two reviewer rows where one has a findings target and the other `-`, to show the engine decides per row and not per workflow.

**Dead ends (tried and rejected):**
- Keeping a prose or annotated `On Findings` value (`mosaictest-scripted (re-check)`) as the ambiguity. WorkflowFormat 2.16 refuses it at admission (T6), so no run starts.
