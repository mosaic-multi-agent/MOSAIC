---
version: "2.0"
name: "MosaicTest HITL Glob Staged Workflow"
description: "Harness conformance fixture — HITL approval check with a wildcard output artifact. The wildcard is passed to the subagent literally; the approval check must read the concrete per-iteration files the subagent wrote, never the literal wildcard path, so a compliant HITL dispatch is accepted without a false redispatch."
hint: "Harness test — HITL approval read for a wildcard output, concrete per-iteration files, no false redispatch"
author: MOSAIC
id: hitl-glob-staged
referenced_agents:
  - mosaictest-scripted
artifacts:
  - MosaicTestScript/hitl-glob-check.md
  - Stage-*/HITLGlobStage.md
  - Stage-1/HITLGlobStage.md
  - Stage-2/HITLGlobStage.md
  - MosaicTestMarker.md
modes:
  - orchestrated
---

<Workflow type="core" name="hitl-glob-staged" version="2.0">
## MosaicTest HITL Glob Staged Workflow

**Use when:** Verifying that the Runner, when it checks the human-in-the-loop gate of a dispatch whose declared Output holds a wildcard (`Stage-*/HITLGlobStage.md`), reads `human_approved` from the concrete files the subagent wrote that match the pattern, rather than from the literal wildcard path.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| VALIDATION | mosaictest-scripted | FALSE | COMPLETE | - | MosaicTestScript/hitl-glob-check.md | Stage-*/HITLGlobStage.md, Stage-1/HITLGlobStage.md, Stage-2/HITLGlobStage.md, MosaicTestMarker.md |

**Notes:**
- **Run this workflow in Orchestrated mode only.** The routing fixture drives two dispatches of the single VALIDATION row: one with default HITL (false, from the table), then one with the `hitl: true` override. Auto mode would route the SUCCESS on the first dispatch to `COMPLETE`, preventing the second dispatch from running.
- **The wildcard is passed literally.** The Output cell lists `Stage-*/HITLGlobStage.md` next to two concrete paths and the marker. An Output wildcard is never expanded into the dispatch: the subagent receives the pattern as written, and `mosaictest-scripted` skips every declared path that contains `*`, so it never creates a file with `*` in its name. It writes the three concrete paths.
- **Written-output detection matches files against the pattern.** The Runner snapshots the declared paths before a dispatch and, afterwards, counts a file as written when it is new or its content changed; a wildcard matches the files that exist under it. The pattern `Stage-*/HITLGlobStage.md` therefore matches `Stage-1/HITLGlobStage.md` and `Stage-2/HITLGlobStage.md`. A file that exists and was left unchanged is not written and is not checked, which is why this fixture ships none of them: the subagent creates them.
- **Dispatch 1 (HITL=false):** the marker is absent. `mosaictest-scripted` receives `human_in_the_loop: false`, writes the marker and the per-iteration files without an approval stamp, and returns `SUCCESS`. No approval is read for a dispatch without HITL.
- **Dispatch 2 (HITL=true override):** the marker is present. The script declares `HitlBehaviour: proceed`, so the subagent accepts `human_in_the_loop: true`, writes the same paths with different content stamped `human_approved: true`, and returns `BLOCKED` with `E401` (any status but `BLOCKED` with `E503` is checked). The HITL check reads `human_approved` from `Stage-1/HITLGlobStage.md`, `Stage-2/HITLGlobStage.md` and the marker, all approved, and accepts. No HITL redispatch occurs; the orchestrator then stops the run.
- The first row is the run's only row and holds no block, so no `Plan.md` is read and no block schedule exists in this fixture. The `Stage-*` name is only a path pattern.
- Seed `Fixtures/hitl-glob-staged` — the whole directory, not anything inside it.

</Workflow>

---

## Design Rationale

### Why two dispatches of one row rather than two rows

Two rows holding the same agent would be told apart only by a mode qualifier, and both dispatches would then carry that qualifier in their task description. One row dispatched twice — with different orchestrator-controlled `hitl` overrides — keeps the fixture self-consistent with the `orchestrated-linear` and `orchestrated-backjump` patterns and gives two log rows for the same row of the table.

### Why the row's HITL column is FALSE

The table's `HITL` flag controls the default `human_in_the_loop` value sent on every dispatch without an override. Marking the row `FALSE` means the first dispatch (no override) sends `human_in_the_loop: false` and `mosaictest-scripted` takes the marker-absent outcome. The orchestrator then overrides `hitl: true` for the second dispatch to trigger the HITL approval-check path.

### Why the script is marker-gated and the second dispatch must change the files

The approval check reads only the output artifacts a dispatch created or modified. A file that exists before the dispatch and is left byte for byte unchanged is not read. Two consequences shape the fixture:

- The per-iteration files cannot be pre-placed as already approved. A pre-placed file the subagent does not touch is never checked, so the scenario would pass without reading anything.
- The same script is bound to both dispatches, so a single unconditional body would write identical content twice and the second dispatch would leave the files unchanged. The marker gate gives the two dispatches different bodies: the first writes the marker and one body, the second a different body with the approval stamp. The second dispatch therefore modifies every declared concrete path, and the approval check has files to read.

### Why the second dispatch returns BLOCKED E401 and not E503

`BLOCKED` with `E503` is routed directly: it reports that the gate could not run, so no approval is read and the scenario would prove nothing about the wildcard. The script declares `HitlBehaviour: proceed` instead and returns `BLOCKED` with a different code (`E401`), a status the HITL check still inspects. The routing fixture already matches `after mosaictest-scripted BLOCKED` and stops the run, so the dispatch sequence is the same as before the rewrite.

### Why the wildcard is declared next to the concrete paths

The subagent cannot write through a wildcard, so the concrete paths must be declared for the files to exist. The wildcard stays in the Output cell because it is the entry under test: an approval check that reads each declared Output literally would look for a file named `Stage-*/HITLGlobStage.md`, find none, report the approval missing, and redispatch a compliant attempt. The concrete entries make the approval check have something real to read; the wildcard makes a literal-path read visible.

### What the scenario guards against

A HITL check that iterates over the raw declared Output list and calls the approval read on the literal `Stage-*/HITLGlobStage.md` path. A file with `*` in its name cannot exist, so every HITL=true dispatch with a wildcard Output would report `ApprovalFileMissing` and trigger a false HITL redispatch. The check must resolve the declared Outputs to the files that were written and read approval from those.

---

## Expected Run

Two Orchestration.md log rows. Every consultation — before dispatch 1, before dispatch 2, and the final stop — dispatches to the orchestrator but allocates no `Seq` and leaves no row; each surfaces only in the dispatch log.

| Log `Seq` | `Agent` | `Phase` | `Iteration` | `Status` | `Summary` shows |
|:---:|---|---|---|---|---|
| 1 | `mosaictest-scripted#1` | VALIDATION | - | SUCCESS | dispatch 1 / hitl=false / marker absent, wrote marker and per-iteration files |
| 2 | `mosaictest-scripted#2` | VALIDATION | - | BLOCKED | dispatch 2 / hitl=true / marker present / wrote approved per-iteration files / E401 |

**Run outcome:** `RunStoppedByConsultant`, exit_code 6. The orchestrator ends the run after the BLOCKED from dispatch 2.

**Key observation:** The log shows exactly one BLOCKED entry for the HITL dispatch — no additional redispatch row appears. This is the assertion: the approval check resolved `Stage-*/HITLGlobStage.md` to the written `Stage-1/HITLGlobStage.md` and `Stage-2/HITLGlobStage.md`, found `human_approved: true` in both, and accepted without redispatching.

**Files after the run:** `Stage-1/HITLGlobStage.md` and `Stage-2/HITLGlobStage.md` exist and carry `human_approved: true`; no file or directory with `*` in its name exists in the run folder.

**RunnerLogs verification:** The debug log must contain a HITL accept event after seq 2, with no HITL redispatch event. A consultant stop entry must follow naming the fixture's stop reason.

---

## What a Failure Means Here

| Observation | Where to look |
|---|---|
| A third log row appears showing a second BLOCKED from `mosaictest-scripted` | False HITL redispatch: the approval check did not resolve the wildcard to the written files and found the literal pattern missing, so it treated a compliant attempt as unapproved |
| Seq 1 shows BLOCKED (E503) instead of SUCCESS | The first dispatch incorrectly received `human_in_the_loop: true` — the routing fixture's `hitl: true` override may have leaked into dispatch 1 |
| Seq 2 shows `BLOCKED` with `E503` | The marker was not seen as present, or the script's `HitlBehaviour: proceed` was not honoured — check that dispatch 1 wrote `MosaicTestMarker.md` |
| Seq 2 shows SUCCESS | The script's marker-present outcome was not selected, or the stub ignored the declared status |
| Seq 2 shows BLOCKED but the debug log has no approval read | The written files were not detected: the second dispatch did not change them, or written-output detection did not match the wildcard against them |
| `FIXTURE_ERROR` naming a `Write` body with no writable target | The declared Output reached the subagent with only wildcards or without the concrete paths — check how Output paths are passed |
| A file or directory with `*` in its name exists in the run folder | A subagent or the Runner materialised the wildcard literally |
| An `orchestrator-script` row appears in `Orchestration.md` | Consultations must leave no row in the artifact; a consultation wrongly called `Store.Apply` or allocated a `Seq` |

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-28 | MOSAIC | Initial version. HITL glob expansion fixture. |
| 2.0 | 2026-10-12 | MOSAIC | WorkflowFormat 2.16: `COMPLETE` instead of `-` in `On Success`. The premise was rewritten: stage-set re-derivation after a `Stage-*` output and expansion of the pattern over the stage IDs of `Plan.md` no longer exist (an Output wildcard is passed literally, written-output detection matches the files a subagent wrote against it, and a schedule is re-read only when a row produces its block's Source), and an unchanged pre-placed file and `BLOCKED` with `E503` are no longer approval-checked. The fixture now has the subagent write the per-iteration files itself: the script is marker-gated, the second dispatch declares `HitlBehaviour: proceed` and `HitlApproval: true` and returns `BLOCKED` with `E401`, the concrete paths and the marker are declared beside the wildcard, and the pre-placed `Plan.md` and `Stage-N` files are removed. Dispatch sequence and `.expected.json` unchanged. |

---

## Open Ideas / Dead Ends

**Ideas under consideration:**
- A variant where the second dispatch returns `SUCCESS` with the approval stamped, so the accept goes through the ordinary record step instead of a consultation. It needs a second routing rule (`after mosaictest-scripted SUCCESS #2`).
- A variant where one per-iteration file carries `human_approved: false`, to assert the HITL redispatch fires for the concrete file and not for the pattern.

**Dead ends (tried and rejected):**
- Pre-placing `Stage-1/HITLGlobStage.md` and `Stage-2/HITLGlobStage.md` with `human_approved: true` (version 1.0). The approval check reads only files a dispatch created or modified, so an untouched pre-placed file is never read and the scenario passes without testing the wildcard.
- Keeping `BLOCKED` with `E503` for dispatch 2 (version 1.0). `E503` is routed directly without an approval read.
- Using `EXECUTION.[Stage]` rows for the HITL row. A block row's Output fills in the iteration ID (`Stage-{Stage}/...`), so it carries no wildcard and the scenario has nothing to resolve. The check matters for a row outside a block that declares a literal `Stage-*` pattern.
- Designing for Auto mode. Without a routing consultant, a BLOCKED from a HITL row in Auto mode resolves to `RunDeviationUnresolved`. Auto mode cannot complete this fixture cleanly.
