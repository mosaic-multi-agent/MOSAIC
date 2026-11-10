---
version: "1.0"
name: "MosaicTest Finished-Work Consultation Workflow"
description: "Runner mode fixture - the finished-work consultation. After the creator succeeds again, its next step (the reviewer) has already succeeded, so the Runner consults the orchestrator instead of dispatching it. Same workflow and fixtures in Auto and Auto-review."
hint: "Mode 2 and 3 test - consultation when the next step already finished"
author: MOSAIC
id: finished-work-consult
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/finished-create.md
  - MosaicTestScript/finished-review.md
  - MosaicTestScript/finished-check.md
  - MosaicTestCreated.md
  - MosaicTestMarker.md
modes:
  - auto
  - auto-review
---

<Workflow type="core" name="finished-work-consult" version="1.0">
## MosaicTest Finished-Work Consultation Workflow

**Use when:** Verifying that, after an engine-routed SUCCESS whose next step already has a SUCCESS in the Execution Log, the Runner consults the orchestrator instead of dispatching that step again.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | mosaictest-scripted(create) | FALSE | NEXT | - | MosaicTestScript/finished-create.md | MosaicTestCreated.md |
| PLANNING | mosaictest-scripted(review) | FALSE | NEXT | - | MosaicTestScript/finished-review.md | - |
| REVIEW | mosaictest-scripted(check) | FALSE | COMPLETE | mosaictest-scripted(create) | MosaicTestScript/finished-check.md | MosaicTestMarker.md |

**Notes:**
- **Run this workflow twice: once in Auto mode, once in Auto-review mode.** The fixtures are identical.
- The same agent holds all three rows, so their `Subagent` cells are told apart by the mode qualifiers `create`, `review` and `check`. Dispatch, instance ids and logs use the bare id `mosaictest-scripted`.
- Row 1 and row 2 always return `SUCCESS`. Row 3 is marker-gated: first invocation (marker absent) writes the marker and returns `COMPLETED_NEEDS_ACTION`; second invocation returns `SUCCESS`.
- Row 3's `On Findings` names the whole cell `mosaictest-scripted(create)`, row 1.
- Seed `Fixtures/finished-work-consult` - the whole directory, not anything inside it.

</Workflow>

---

## Design Rationale

The findings loop sends the run back to row 1. When row 1 then succeeds, its `NEXT` is row 2, which already succeeded in the first pass. Dispatching it again would redo finished work, so the Runner consults the orchestrator (a routing consultation) and the fixture's rule dispatches row 3, the step still outstanding.

The check applies to engine-routed SUCCESS only. The route-back itself is not checked: in Auto the orchestrator's dispatch of row 1 is the orchestrator's own, and in Auto-review the engine's `On Findings` route is exempt. The step after the route-back is checked, in both modes.

---

## Expected Run

| Seq | Agent | Row | Status | Note |
|:---:|---|:---:|---|---|
| 1 | `mosaictest-scripted#1` | 1 | SUCCESS | creator |
| 2 | `mosaictest-scripted#2` | 2 | SUCCESS | reviewer |
| 3 | `mosaictest-scripted#3` | 3 | COMPLETED_NEEDS_ACTION | checker, marker absent |
| 4 | `mosaictest-scripted#4` | 1 | SUCCESS | creator again: Auto by orchestrator re-dispatch (deviation consultation), Auto-review by engine route-back |
| 5 | `mosaictest-scripted#5` | 3 | SUCCESS | checker, marker present; dispatched by the orchestrator after the finished-work consultation |

**Run outcome:** COMPLETE.

**Key observation:** an `orchestrator-script` consultation sits between Seq 4 and Seq 5 in both modes (the finished-work consultation, matching rule `after mosaictest-scripted SUCCESS #3`). In Auto there is one more consultation between Seq 3 and Seq 4 (the CNA deviation); in Auto-review there is none.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| No consultation between Seq 4 and Seq 5; the reviewer runs a second time | The finished-work check is not applied after the engine-routed SUCCESS |
| The stub stops with "no matching rule" and a different `#n` | The count of SUCCESS rows at the consultation differs from 3; check which rows the Runner logged |
| Auto-review shows a consultation between Seq 3 and Seq 4 | The engine treated CNA as a deviation instead of auto-routing via `On Findings` |
| Auto shows no consultation between Seq 3 and Seq 4 | The engine auto-routed CNA in Auto mode |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-13 | MOSAIC | Initial version. Finished-work consultation in Auto and Auto-review. |
