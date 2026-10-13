---
version: "1.1"
name: "MosaicTest Orchestrated Linear Workflow"
description: "Runner mode fixture — Orchestrated mode end to end. The orchestrator is asked before every step, dispatches the same row three times with three different task descriptions, then stops."
hint: "Mode 1 test — orchestrator consulted every step, task description delivery, stop instruction"
author: MOSAIC
id: orchestrated-linear
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/linear-echo.md
modes:
  - orchestrated
smoke_set:
  - orchestrated
---

<Workflow type="core" name="orchestrated-linear" version="1.1">
## MosaicTest Orchestrated Linear Workflow

**Use when:** Verifying that `mosaic-run` can execute a run in Orchestrated mode against a real harness — that it asks the orchestrator before every step including the first, carries the orchestrator's task description through to the subagent, and ends the run on a stop instruction.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| RESEARCH | mosaictest-scripted | FALSE | COMPLETE | - | MosaicTestScript/linear-echo.md | - |

**Notes:**
- **Run this workflow in Orchestrated mode only.** In Auto or Auto-review the engine routes the single row to `COMPLETE` without ever consulting the orchestrator, and the run proves nothing this workflow exists to prove.
- `On Success` is `COMPLETE` so that a mode confusion fails fast and harmlessly rather than looping.
- No block rows. The workflow has no `**Blocks:**` table, so no block schedule is read.
- Seed `Fixtures/orchestrated-linear` — the whole directory, not anything inside it — as the single seed path.

</Workflow>

---

## Design Rationale

### Why one row, dispatched three times

The obvious shape for this test is three rows, one per step. Under WorkflowFormat 2.16 it is authorable: every `Subagent` cell must be unique, so three rows of one stub agent need three mode qualifiers, and the orchestrator's reply names the row it dispatches in `row`. But it would test row addressing as well as task-description delivery, and a failure could not be attributed to one of them.

One row keeps the test on one mechanism. Three dispatches of that row, each carrying a different orchestrator-written task description, exercise the delivery without depending on row addressing. Multi-row navigation belongs in `orchestrated-backjump`.

### Why the subagent echoes its task description

The whole claim of Orchestrated mode is that the orchestrator writes a task description worth having and the subagent receives it. Nothing in a run shows this unless the subagent reports what it was given.

So the behaviour fixture directs the stub to echo the received `task_description` into its `status_message`. The three log rows then carry three different summaries, which is the only direct evidence that per-dispatch content crossed the harness intact. Without the echo, the three steps would be indistinguishable and the mode's central property would be assumed rather than observed.

### Why the routing fixture counts occurrences

The three dispatches differ only in their task description, so the orchestrator cannot tell them apart from the last agent and status alone — all three states are "after `mosaictest-scripted` SUCCESS". The occurrence qualifier distinguishes them, and the count comes from the execution log rather than from anything the stub remembers.

This is also where an unexpected extra consultation is caught: a fourth SUCCESS would match no rule, and the stub stops and says so instead of dispatching a fourth time.

---

## Expected Run

Three Orchestration.md log rows. Every consultation — including the one before step one and the stop consultation after step three — is NOT logged in Orchestration.md: consultations allocate no `Seq` and leave no row. The routing consultations surface only as dispatch-log entries and the consultant call; the final stop consultation additionally surfaces as a `session.consult.stop` event in RunnerLogs and as the TUI/CLI exit message.

| Log `Seq` | `Agent` | `Phase` | `Status` | `Summary` shows |
|:---:|---|---|---|---|
| 1 | `mosaictest-scripted#1` | RESEARCH | SUCCESS | step one's task description, echoed back |
| 2 | `mosaictest-scripted#2` | RESEARCH | SUCCESS | step two's task description, echoed back |
| 3 | `mosaictest-scripted#3` | RESEARCH | SUCCESS | step three's task description, echoed back |

**Run outcome:** stopped by the orchestrator (`RunStoppedByConsultant`), with the fixture's stop reason surfaced in the exit message. Not `COMPLETE` — in this mode the orchestrator ends the run, and the table's `On Success` column is never consulted.

**RunnerLogs verification:** The debug log must contain a `session.consult.stop` entry with `reason="MOSAICTEST-ROUTING-COMPLETE / three dispatches done as scripted / ending the run on fixture instruction"`.

**Numbering observation to confirm on the first run:** Because no consultation allocates a `Seq`, the three workflow rows are numbered consecutively (1, 2, 3) with no gaps and no disagreement between `Seq` and the `#N` suffix on `AgentInstance`. This is recorded here as a prediction from reading the Runner, not as a settled expectation. If it differs, the difference is a finding about the Runner, not about this fixture.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| Run refused at startup, "no workflow regions found" | The Runner cannot read a deployed orchestrator file (`RUN-1`) |
| First consultation fails or the run dies at its first decision | The code for asking the orchestrator what to run next is missing for this harness (`RUN-4`) |
| Consultation rejected as naming an unknown agent | The routing table is not reaching the consultant (`RUN-3`) |
| Consultation rejected as malformed, but the stub's reply looks correct | The reply is being parsed too strictly (`RUN-5`) |
| Summaries are identical across the three steps | The task description is not reaching the subagent, or the echo fixture is not being honoured |
| A fourth dispatch occurs, or the stub stops with "no matching rule" | The Runner consulted more times than the mode requires |
| Summaries differ but are mangled — wrong characters, truncation | The harness is not carrying dispatch content intact |
| An `orchestrator-script` row appears in `Orchestration.md`, or `Seq` skips a number | Consultations must leave no row and allocate no `Seq`; a routing consultation wrongly called `Store.Apply` |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-17 | MOSAIC | Initial version. First workflow exercising Orchestrated mode. |
| 1.1 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16 wording: the rationale and the rejected three-row shape no longer rely on first-match agent resolution (a reply names `row`; repeated agents take mode qualifiers); the stage-table note became a block-schedule note. Routing table and dispatch sequence unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A variant whose fixture omits the rule for the third SUCCESS, to assert that the stub stops loudly on an unmatched state rather than improvising. Cheap, and it tests the safety property the whole fixture design rests on.

**Dead ends (tried and rejected):**
- Three rows, one per step. Authorable with mode qualifiers, but it adds row addressing to a test about task-description delivery, so a failure would be ambiguous. See Design Rationale.
