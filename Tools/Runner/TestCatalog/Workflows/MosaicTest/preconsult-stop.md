---
version: "1.0"
name: "MosaicTest Pre-Consultation Stop Workflow"
description: "Runner mode fixture - Auto mode pre-consultation stop. The routing fixture's pre-consultation answer is a stop, so the run ends with exit code 6 before any workflow dispatch."
hint: "Mode 2 test - pre-consultation stop, exit code 6, reason on stderr"
author: MOSAIC
id: preconsult-stop
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/preconsult-stop-unreached.md
modes:
  - auto
---

<Workflow type="core" name="preconsult-stop" version="1.0">
## MosaicTest Pre-Consultation Stop Workflow

**Use when:** Verifying that a pre-consultation answer of `stop` ends the run before any dispatch, as a consultant stop with a reason on stderr.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | mosaictest-scripted | FALSE | COMPLETE | - | MosaicTestScript/preconsult-stop-unreached.md | - |

**Notes:**
- **Run this workflow in Auto mode.**
- The routing fixture's `## Pre-Consultation` section declares a stop whose reason carries `PRECONSULT-STOP-MARKER`. The stub orchestrator returns it as `{"action":"stop","reason":"..."}`.
- Row 1 is never dispatched. Its script exists only so the row has a bound input.
- Seed `Fixtures/preconsult-stop` - the whole directory, not anything inside it.

</Workflow>

---

## Design Rationale

A pre-consultation stop is the one consultant stop that happens before the run has any history. The workflow has a single row on purpose: any dispatch at all is a defect, so the expected dispatch list holds the pre-consultation and nothing else.

The reason is asserted through the sidecar's `stderr_contains`, because a consultant stop writes its reason to stderr and exit code 6 alone does not show that the fixture's reason, rather than some other stop, ended the run.

---

## Expected Run

No Orchestration.md workflow rows. The dispatch log holds one entry, the pre-consultation of `orchestrator-script`. The run exits with code 6 and the reason is printed to stderr.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| Exit code 7 | The stub answered in prose or an unrecognised shape: it must return the JSON `stop` object |
| Exit code 0, a `mosaictest-scripted` dispatch appears | The Runner ignored the pre-consultation stop and dispatched row 1 |
| Exit code 6 but stderr lacks the marker | The stub rewrote or replaced the fixture's reason, or another stop ended the run |
| The stub stops with "no matching rule" at pre-consultation | The stub read the Pre-Consultation section as a routing state; its stop form is not being recognised |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-13 | MOSAIC | Initial version. Pre-consultation stop, exit code 6, reason asserted on stderr. |
