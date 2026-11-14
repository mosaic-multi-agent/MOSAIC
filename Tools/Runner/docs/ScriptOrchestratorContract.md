# Script-Mode Orchestrator Contract

> **Status:** Draft
> **Created:** 2026-08-15
> **Last Updated:** 2026-11-09
> **Scope:** The contract between the Runner (`mosaic-run`) and the script-mode orchestrator agent. Defines when the Runner invokes the orchestrator, what it sends, what it expects back, and the constraints both sides must respect. This is not the orchestrator's internal design (how it reasons about routing) — it is the wire protocol between two systems.

---

## 1. Purpose

The Runner and the orchestrator agent collaborate to execute workflows. The Runner handles mechanical routing (reading the workflow table, building protocol requests, recording results, evaluating triggers). The orchestrator handles intelligent routing (deciding which agent runs next, stating its task, handling deviations) under the Routing Policy it shares word for word with the conversational orchestrator. When the Runner needs a routing decision it cannot make from the workflow table alone — or, in Mode 1, for every routing decision — it invokes the orchestrator through the same harness adapter used for subagents.

This document defines the contract for that invocation: the request the Runner sends, the response it expects, and the guarantees both sides provide.

The Runner requires `orchestrator-script` v3.5.0, the orchestrator version that states this contract. It runs every run sequentially: new runs record `dispatch: sequential` in the artifact frontmatter, and a resumed artifact recording any other `dispatch` value is refused before any dispatch. The wire schema is unchanged by this requirement.

### 1.1 Why a Contract Document

The orchestrator agent is an LLM — it produces natural language and JSON according to its system prompt. The Runner is a Go program — it parses JSON according to a struct definition. If either side's expectation drifts, the invocation fails silently or with an opaque parse error. This document is the single source of truth for what crosses the boundary.

### 1.2 Why Not the Communication Protocol

The Communication Protocol governs orchestrator↔subagent communication. The Runner↔orchestrator boundary is a different relationship: the orchestrator is not a subagent performing domain work — it is a routing advisor that reads `Orchestration.md` and returns a single instruction. Most Communication Protocol fields (`input_artifacts`, `output_artifacts`, `include_result_summary`, `human_in_the_loop`, `status_code`, `error_code`) have no meaningful value in this context. Reusing the protocol would require filling fields that are always the same (or always ignored), adding noise to both sides of the contract.

The Runner↔orchestrator contract defines its own request and response schemas, purpose-built for the routing decision boundary. The harness adapter is the transport — it delivers a JSON message and returns a JSON response, regardless of what schema that JSON follows.

**The orchestrator still carries the Communication Protocol in its deployed prompt**, because it holds `role: orchestrator` and the protocol deploys by role. That is correct and load-bearing — the vocabularies and routing interpretations in it are what the orchestrator reasons with — but it means the deployed prompt also contains dispatch, response-parsing, and HITL-gate-verification obligations that the Runner discharges instead. `orchestrator-script.md` states that division under "What the Communication Protocol Governs Here", which is the authority on it for this boundary.

### 1.3 Scope Boundary

This document covers:
- The invocation contexts (when the Runner calls the orchestrator)
- The request and response schemas for each context
- Constraints on both sides
- Error handling, including the Runner's retries of malformed replies and its mechanical handling of `PARTIALLY_DONE`, `E501` and `HARNESS_ERROR` (§2.3)

This document does NOT cover:
- The orchestrator agent's internal reasoning (its system prompt design)
- The Communication Protocol itself (see `CommunicationProtocol.md`)
- The orchestration artifact format (see `OrchestrationArtifactFormat.md`)
- How the Runner decides when to invoke the orchestrator (see `Design.md` §2)

---

## 2. Invocation Contexts

The Runner invokes the orchestrator in two contexts, each with its own response schema.

### 2.1 Routing Consultation

The Runner needs a routing decision. The orchestrator reads `Orchestration.md`, decides what should happen next, and returns a routing instruction.

This context is used:
- **Mode 1 (every step):** After every subagent completion, regardless of status code. The orchestrator makes all routing decisions, including for `PARTIALLY_DONE`, `BLOCKED` with `E501` and `HARNESS_ERROR`.
- **Modes 2/3 (deviation):** When the engine cannot determine the next step from the workflow table — non-SUCCESS status codes the engine can't auto-route, harness errors whose retry budget is spent (`last_status` `HARNESS_ERROR`, §3.2), ambiguous routing — and after a `SUCCESS` whose next step already finished: the table leads to a row that has a `SUCCESS` in the Execution Log at that iteration, so following it would repeat finished work (`Design.md` §2.10). In these modes the Runner handles two outcomes itself before consulting (§2.3).
- **After orchestration-review:** When the review infrastructure agent fires and produces observations, the Runner follows up with a routing consultation that includes the review's `status_message`. This gives the orchestrator the chance to act on the findings.
- **Damaged artifact (all modes):** When the Runner finds `Orchestration.md` damaged — at run start, on resume, on Retry, or after any consultation — it consults with `artifact_damage` set. The orchestrator repairs the artifact, then routes as usual (§2.5).

The orchestrator does not need to know whether it is being consulted for routine routing (Mode 1) or because something went wrong (Modes 2/3). It reads the artifact, sees the current state, and decides. The `last_status`, `last_status_message` and `last_error_reason` fields (§3.2) carry the triggering response's status, its full verbatim `status_message` and, for `BLOCKED`, its `error_reason`; for a harness error they carry `HARNESS_ERROR` and the Runner's failure description. When the user handed the decision over in the manual routing dialogue, `user_message` (§3.2) carries the user's own words. These are the only pieces of context not already in the artifact: the Execution Log truncates `status_message` and has no column for `error_reason`.

### 2.2 Pre-Consultation

A one-shot invocation at run start (before the dispatch loop) for Modes 2 and 3, enabled by default. The orchestrator returns explicit environment facts from its deployed instructions that apply to every auto-routed subagent. The Runner appends them to subsequent auto-routed dispatches. Pre-consultation can be disabled with `--pre-consult=false`.

Pre-consultation has a different response schema from routing consultation (§5 vs §4); the one routing action it admits is `stop` (§5).

**When orchestration-review is not deployed** (absent from the `<InfrastructureAgents>` region), review-triggered consultations never occur.

**Note on `last_status_message` for reviews:** When orchestration-review fires, its `status_message` is passed in `last_status_message` just like any other agent's. The orchestrator does not need a separate field to know this came from a review — the Execution Log shows the review invocation.

### 2.3 What the Runner Handles Itself

Three outcomes are re-dispatched mechanically by the Runner, without consulting the orchestrator, **in auto and auto-review modes only**:

| Outcome | Runner behavior | When the orchestrator is consulted |
|---------|-----------------|------------------------------------|
| `PARTIALLY_DONE` | Re-dispatches the same assignment (same row and iteration) with the previous output artifacts added to the inputs and the previous `status_message` appended to the task description, up to 3 times in a row | Once the trailing run of `PARTIALLY_DONE` at that row and iteration exceeds 3 |
| `BLOCKED` with `E501` | Re-dispatches the same assignment, with a budget of 3 `E501` attempts per row and iteration since the last `SUCCESS` there | Once the budget is spent |
| `HARNESS_ERROR` (an invocation that ended without a usable subagent response; recorded by the Runner only, not a Communication Protocol status) | Re-dispatches the same assignment, with a budget of 3 `HARNESS_ERROR` attempts per row and iteration since the last `SUCCESS` there, separate from the `E501` budget. An unparseable or empty response first gets one direct re-dispatch (raw-text bypass), which counts against the budget | Once the budget is spent |

Orchestrated mode (Mode 1) never applies these shortcuts: the orchestrator is consulted on every decision, including every `PARTIALLY_DONE`, `E501` and `HARNESS_ERROR` (after the raw-text bypass, where it applies), and applies the shared Routing Policy itself (a fresh invocation for `PARTIALLY_DONE`, the Tiered Error Strategy for `E501`). The orchestrator must know `HARNESS_ERROR` as a `last_status` value and treat it as a transient shortfall of the harness, not as a statement of the subagent. The Runner-only bound of 3 consecutive `PARTIALLY_DONE` re-dispatches does not exist in the Routing Policy; after the bound the orchestrator applies the unchanged policy with judgement. Every other non-`SUCCESS` outcome the engine cannot route from the table reaches the orchestrator in all modes.

### 2.4 Three Row Carriers

The position of a step in the routing table is carried in three places, with different optionality:

| Carrier | Form | Optional? |
|---------|------|-----------|
| Workflow table `Row` column | Row number in the deployed table | Optional. A table without a `Row` column is counted by 1-based data-row position |
| Execution Log `WorkflowRow` column | The row number the step ran | The value `-` is allowed for infrastructure agents, ad-hoc steps and legacy logs written before the column existed |
| Routing reply `row` (§4.1) | The row the orchestrator dispatches | Mandatory on every `dispatch`; there is no optional phase and no fallback that resolves the row from `agent` |

All three use the same numbering: the table's `Row` column value, or the 1-based position among the table's data rows when there is none.

A row inside a block also has an iteration. The routing reply names it in `iteration` (§4.1) in exactly the form the Execution Log writes in its `Iteration` column for that row, so the orchestrator copies the value from the log.

### 2.5 Artifact Repair

The orchestrator edits `Orchestration.md` to write Workflow Notes, and an edit that misses its anchor can damage a Runner-owned section: a split Execution Log row, a note inserted into the log table. A conversational run recovers from this because its orchestrator, an LLM, repairs the artifact and goes on routing. A Runner run recovers the same way, through the same routing consultation, not through a separate repair context: repairing is bookkeeping that precedes the decision, and the decision is still exactly one `dispatch` or `stop`.

**Detection.** The Runner reads the artifact strictly (`Design.md` §4.4): a table that ends before its closing tag, a row with fewer cells than its header, Execution Log `Seq` values that are not integers or not strictly increasing, an `Agent` cell that is not `{AgentName}#{Seq}` with the row's own `Seq`, and a `current_state.last_agent` with no log row are damage, never parsed around. Deliberate tolerances are not damage: a closing tag after content on the same line, CRLF, a `global_sequence` lagging behind the logged `Seq` values (corrected upward), a last `Seq` above `global_sequence`, gaps in `Seq` (sequences consumed but never logged), and an empty `last_agent` (nothing accepted yet). The pre-3.0 layout and run-identity refusals keep their own handling. A run whose frontmatter is itself damaged takes its recorded values from the last-good copy when a usable one exists; without one it records no workflow, so there is no routing table to consult against, and the Runner refuses it without a repair consultation. The Runner never writes after a read that found damage, because its store re-renders the whole artifact from what it read and would make the loss permanent. It reads the artifact after every consultation, including one that failed or timed out, so damage is found where it happened.

**Evidence.** After every successful write the Runner keeps a last-good copy of the artifact outside the run folder (`RunnerLogs/{run_id}/Orchestration.last-good.md`, beside the run folder). Before the first repair consultation for a damage occurrence it saves the damaged file beside that copy; that file is not part of the request, since the damaged artifact is still in place at `orchestration_artifact`. The consultation carries the last-good copy path and, when a usable log exists, the Runner's dispatch log path in `artifact_damage` (§3.2). The Runner restores nothing itself: the copy, the dispatch log and the damaged file in place are evidence for the orchestrator, standing in for what a conversational orchestrator reads from the run's files.

**The consultation.** Everything else in the request keeps its normal meaning. A consultation repeated because the previous one damaged the artifact carries that previous request's trigger fields again; one made at run start, on resume or on Retry carries the resume values (§3.2). On resume and Retry this consultation replaces the Runner's resume-position resolution: the returned `dispatch` says where the run continues. The orchestrator repairs first, within the exception of §6.2, then decides against the repaired artifact and returns the usual reply (§4).

**Verification.** Before acting on the reply, the Runner reads the artifact again and accepts the repair only if:

- the read finds no damage;
- every Execution Log row present in the last-good copy, when there is one, is present and unchanged;
- the last `Seq`, `last_agent`, `last_status` and `error_code` agree with the dispatch log, counting only responses that advanced the run (skipped without a usable log).

An accepted repair is followed by the reply as usual: a `dispatch` is carried out, a `stop` is a consultant stop. A repair that is not accepted voids the reply, however valid (§7.6).

---

## 3. Request Format

### 3.1 Schema

```json
{
  "orchestration_artifact": "Orchestration-20260815T143000Z-a3f9/Orchestration.md",
  "context": "routing",
  "last_status": "COMPLETED_NEEDS_ACTION",
  "last_status_message": "COMPLETED_NEEDS_ACTION: Found 3 issues — (1) PaymentProcessor.validate() accepts raw string but schema defines Decimal, (2) error response type missing 'retryable' field, (3) retry policy not specified for timeout scenarios. See contracts-review.md for details.",
  "last_error_reason": null,
  "user_message": null,
  "artifact_damage": null
}
```

All seven members of the routing request are always present; the nullable ones are `null`, never absent. The pre-consultation request has no `artifact_damage` member and so carries six. The examples below omit `artifact_damage`, which is `null` in them. A harness error reaches the orchestrator like this (`last_status` `HARNESS_ERROR`, the failure description in both text members):

```json
{
  "orchestration_artifact": "Orchestration-20260815T143000Z-a3f9/Orchestration.md",
  "context": "routing",
  "last_status": "HARNESS_ERROR",
  "last_status_message": "timeout: harness invocation exceeded 30m0s",
  "last_error_reason": "timeout: harness invocation exceeded 30m0s",
  "user_message": null
}
```

A consultation the user handed over in the manual routing dialogue carries the user's text in `user_message`:

```json
{
  "orchestration_artifact": "Orchestration-20260815T143000Z-a3f9/Orchestration.md",
  "context": "routing",
  "last_status": "BLOCKED",
  "last_status_message": "BLOCKED: E101 contracts.md not found",
  "last_error_reason": "contracts.md not found",
  "user_message": "The contracts moved to Design/Contracts.md.\nRe-run the designer with that path."
}
```

A consultation on resume of a damaged artifact carries `artifact_damage` (§2.5):

```json
{
  "orchestration_artifact": "Orchestration-20260815T143000Z-a3f9/Orchestration.md",
  "context": "routing",
  "last_status": null,
  "last_status_message": null,
  "last_error_reason": null,
  "user_message": null,
  "artifact_damage": {
    "problems": [
      "ExecutionLog, line 101: row has 8 cells, the header has 10",
      "ExecutionLog, line 103: non-table line before the closing tag"
    ],
    "last_good_copy": "C:/.../RunnerLogs/20260815T143000Z-a3f9/Orchestration.last-good.md",
    "dispatch_log": "C:/.../RunnerLogs/20260815T143000Z-a3f9/20260815T143000Z-a3f9-dispatch.log"
  }
}
```

The paths are illustrative; where the Runner keeps the copies is its own concern (`Design.md` §4.4).

### 3.2 Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `orchestration_artifact` | string | Yes | Path to `Orchestration.md` for this run. The orchestrator reads this to understand the run's full state — execution log, current position, workflow notes. This is the orchestrator's single source of truth. |
| `context` | string | Yes | `"routing"` or `"pre_consultation"`. Tells the orchestrator which response schema the Runner expects. |
| `last_status` | string or null | Yes | The status of the response whose message is in `last_status_message`: one of the Communication Protocol statuses a subagent returned, or `"HARNESS_ERROR"` when the Runner recorded the attempt as a harness error. `HARNESS_ERROR` is Runner-recorded only: it is not a Communication Protocol status and no subagent returns it, so an orchestrator that branches on the status must know it as an additional value. It is `null` exactly when `last_status_message` is `null`, and always for pre-consultation. |
| `last_status_message` | string or null | Yes | The full verbatim `status_message` from the agent that triggered this consultation. The artifact does not carry it in full: the Execution Log truncates `status_message` for readability. `null` on the first step of a new run (no prior agent), for pre-consultation, and for a consultation made on resume: a resumed run holds no response from the process that produced the last step, so the Runner has no verbatim message to send, and a finished-work consultation (§2.1) made on resume carries `null` too (the artifact's Execution Log row is then the only record of that step). For harness errors, which the Runner records as `HARNESS_ERROR` (`Design.md` §3.3), this carries the Runner-constructed description: the failure kind followed by its detail, on one line (`timeout: ...`, `process-failure: ...`, `unparseable-response: ...`). |
| `last_error_reason` | string or null | Yes | The full verbatim `error_reason` from the triggering agent's response when its status is `BLOCKED`; for a harness error, the Runner-constructed error description; when the consultation follows a block schedule that stayed defective after the Runner's re-dispatches of the producing row, the Runner-constructed defect description (the rule ID, or that the Source is missing or unreadable, the block, the Source and Section, and the offending row, ID or token), while `last_status_message` stays the producer's verbatim `status_message` (`Design.md` §3.6). `null` for every other status, on the first step of a new run, on resume, and for pre-consultation. Like `last_status_message`, it is not persisted in the artifact. It is what lets the orchestrator correct the defect an `E100` names. |
| `user_message` | string or null | Yes | The text the user entered in the manual routing dialogue after choosing `Ask the orchestrator to decide` (`Design.md` §2.9), verbatim with its line breaks. It is a message from the human in the loop to the orchestrator for this one routing decision and is to be taken into account in it. `null` for every other consultation, for every pre-consultation and when the user left the message empty. It is carried on that one consultation only: not stored by the Runner, not in the artifact and not sent again with later consultations. |
| `artifact_damage` | object or null | Yes | Set when the Runner found `Orchestration.md` damaged (§2.5), `null` otherwise. It is a member of the routing request only: the pre-consultation request has no such member. Members: `problems`, a non-empty array of strings, one per finding, each naming the section, line and problem; `last_good_copy`, the path of the last artifact the Runner wrote, or `null` when it has none (it may be older than the damaged file); `dispatch_log`, the path of the Runner's dispatch log for this run, which holds every request and response verbatim, present as a path only when a usable log exists and `null` otherwise (it is not a required string). There is no member for the damaged file: it is the artifact at `orchestration_artifact`, in place. It cannot be carried in `last_error_reason`, which keeps carrying the triggering response's own reason. |

---

## 4. Response Format — Routing Instruction

For routing consultation (`context: "routing"`), the orchestrator returns a JSON object with one of two actions.

### 4.1 Action: `dispatch`

Route to a specific agent in the workflow table.

```json
{
  "action": "dispatch",
  "agent": "contracts-designer",
  "row": 4,
  "iteration": null,
  "task_description": "Revise ContractsDesign.md to resolve the findings recorded in contracts-review.md.",
  "constraints": null,
  "input_artifacts": ["Requirements.md", "ContractsDesign.md", "contracts-review.md"],
  "output_artifacts": null,
  "hitl_override": null
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | Yes | `"dispatch"` |
| `agent` | string | Yes | The bare agent id, without the mode qualifier: `build-review` for a row whose `Subagent` cell reads `build-review(tests)`. Must equal the bare id of the row named by `row`. The whole `Subagent` cell is rejected as an agent-row mismatch. |
| `row` | integer | Yes | The 1-based routing table row dispatched, in the numbering of §2.4. The Runner takes the defaults from this row. A JSON integer or a decimal string is accepted. A missing `row`, a row outside the table, or a row whose agent differs from `agent` is invalid (§7.1, §7.2). |
| `iteration` | string or null | Required for a row in a block; invalid otherwise | The iteration of the row's block, as a string holding exactly the value the Execution Log writes in its `Iteration` column for that row (`OrchestrationArtifactFormat.md` §4.1: `{ID}` for an ungrouped row, `{ID}.{Group}` for a grouped row), for example `"A"`, `"4"`, `"1.Test"`, `"b-2.Implementation"`. The value is split using the row's `Phase` cell: one ID per block reference, then the row's group when it has one (the Runner admits one block reference per row, so one ID). The ID must exist in the current schedule of the row's block (compared ignoring case; recorded as the schedule spells it). On a grouped row the group segment is required and must equal the row's group; on an ungrouped row a group segment is invalid. `null` or absent on a row outside every block is the only valid form there; any iteration on such a row is invalid, as is `null`, absent or `""` on a row in a block. The `stage` member of earlier contract versions is no longer accepted: a `stage` value other than `null` is a malformed reply. |
| `task_description` | string | Yes | The task for the next subagent, stated minimally under the orchestrators' shared Task Descriptions rule (what to accomplish, never how, never reshaped from domain content). The Runner uses this verbatim in the protocol request's `task_description` field. |
| `constraints` | string or null | No | If non-null, used as the `constraints` field in the protocol request. `null` or absent means the Runner uses any table-level or deployment-level constraints. |
| `input_artifacts` | array of strings or null | No | If non-null, overrides the table row's Input column for this dispatch. The orchestrator specifies exactly which artifacts this invocation should read — e.g., adding a review artifact that isn't in the table's default set. `null` or absent means the Runner uses the table row's Input column. |
| `output_artifacts` | array of strings or null | No | If non-null, overrides the table row's Output column for this dispatch. `null` or absent means the Runner uses the table row's Output column. |
| `hitl_override` | bool or null | No | `true` adds HITL. `false` applies an explicit user waiver recorded in Workflow Notes and applicable to this invocation. `null` or absent defers to workflow and Plan resolution. |

**Why `iteration` and a bare `agent`.** The `iteration` string reuses the vocabulary the orchestrator already reads in the Execution Log, so no second iteration form exists and the orchestrator copies the value instead of translating it; the Runner splits it with the row's `Phase` cell, which it already holds. The bare `agent` id is what is dispatched and logged (agent file, instance ids) and what `orchestrator-script.md` already tells the orchestrator to dispatch, so the qualified table cell is never an accepted spelling.

**Artifact paths are bare and run-relative.** An override names artifacts the way the workflow table, the Execution Log's `Inputs` column, and the Artifacts registry name them — `Requirements.md`, `Stage-2/Plan.md` — without the `Orchestration-{run_id}/` prefix. The Runner adds the prefix when it builds the protocol request, which is the one place the contract requires a fully-qualified path. Keeping the prefix out of everything the run configures or records means `run_id` is stated once, in the folder name, rather than repeated into every path. The Runner normalises an already-prefixed path rather than doubling it, but that is tolerance for a hand-edited or legacy value, not an alternative form to emit.

**Defaults and overrides:** The table row provides the default artifact set — what the workflow author designed for the first happy-path invocation of each agent. On re-invocations (after review loops, deviation recovery, backward jumps), the artifact set often differs: a creator routed back after review needs the review artifact as additional input; an agent re-invoked after upstream changes may need a different output scope. The orchestrator overrides only the fields that differ from table defaults — `null` means "use the table."

**Runner behavior:** The Runner validates `row` and `iteration` against the routing table and the current block schedules, and never corrects them. For each protocol request field, it applies the orchestrator's value if provided, otherwise falls back to the table row's default. Sequence number is always assigned by the Runner. Once that invocation's outcome is accepted, `current_state` takes the dispatched row's position — including phase and iteration changes, even if the dispatch jumps backward in the table.

**Free table navigation:** The orchestrator can name any agent in the routing table, regardless of the current position. A reviewer finding upstream problems (bad contracts, incomplete requirements, wrong plan) is a normal reason to jump backward. The Runner imposes no ordering constraint — the orchestrator's routing decision is authoritative.

### 4.2 Action: `stop`

Stop the run.

```json
{
  "action": "stop",
  "reason": "Unrecoverable test failure: the framework dependency is missing from the project"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | Yes | `"stop"` |
| `reason` | string | Yes | Human-readable reason for stopping. Surfaced in the Runner's exit message. The script orchestrator records any conclusion needed for later resume in Workflow Notes before returning. |

**Runner behavior:** The run ends. The reason is surfaced in the Runner's exit message and is not persisted in the artifact. The artifact is left in its current state (resumable if the underlying issue is fixed).

---

## 5. Response Format — Pre-Consultation

For pre-consultation (`context: "pre_consultation"`), the orchestrator returns field-keyed strings the Runner appends to every auto-routed dispatch.

```json
{
  "task_description": "Skills are located at .claude/skills/ in the project root. When running Python, always use `py`, never `python`."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `task_description` | string | No | Appended to the generic task description on every auto-routed dispatch. Environment facts that apply to all subagents — skills paths, interpreter aliases, harness quirks — go here. |
| `constraints` | string | No | Appended to the `constraints` field on every auto-routed dispatch. Scope or deliverable restrictions only, per the protocol's `constraints` definition; never environment facts. Normally absent. |

Both fields are optional. The orchestrator includes only explicit facts that apply to every auto-routed subagent, without rationale or duplication, and returns `{}` when none exist.

**Runner behavior:** The Runner stores these strings in session state. On every subsequent auto-routed dispatch (Modes 2/3), the Runner appends them to the corresponding fields of the `ProtocolRequest`. The Runner never interprets the content — it appends mechanically.

**Not applied to orchestrator-routed dispatches.** When the orchestrator crafts a dispatch instruction (§4.1), it includes any relevant explicit environment facts itself. Pre-consultation strings are only for auto-routed dispatches where the orchestrator was not involved.

### 5.1 Pre-Consultation Stop

The orchestrator may end the run at pre-consultation, before any workflow dispatch, with the routing `stop` shape of §4.2:

```json
{
  "action": "stop",
  "reason": "The workflow requires a database that is not reachable from this environment"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | Yes for a stop | `"stop"`. No other `action` value is allowed in a pre-consultation reply. |
| `reason` | string | Yes | Human-readable reason, non-empty after trimming whitespace. Surfaced in the Runner's exit message. |

**Runner behavior:** A pre-consultation stop is a decision, not a failure. The run ends before any dispatch as a consultant stop (exit code 6) with the reason on stderr. Pre-consultation runs before the Runner records the advice, so the artifact is left exactly as a pre-consultation failure leaves it: resumable, nothing written for the stop, and a resume runs pre-consultation again. Advice members (`task_description`, `constraints`) sent next to a `stop` are ignored: the stop wins. A reply with no `action` member is advice as before, and any `reason` member in it is ignored. The TUI offers Retry only, because manual dispatch replaces a routing decision and a pre-consultation stop has none; Retry resumes the run, which asks the orchestrator for pre-consultation again.

**Malformed cases (pre-consultation failure, §7.4):** an `action` that is not the string `"stop"` (including `"dispatch"`, `null`, an empty string and any non-string value), and a `stop` whose `reason` is missing, `null`, empty, blank or not a string. Pre-consultation is never retried within the attempt.

---

## 6. Orchestrator-Side Constraints

### 6.1 Response Constraints

| Constraint | Rationale |
|-----------|-----------|
| Return valid JSON conforming to the expected response schema | The Runner parses the response structurally. Malformed JSON and an invalid `row`/`iteration` are retried within the attempt budget (§7.1); a missing required field stops the run. |
| Use only the two defined actions (`dispatch`, `stop`) for routing consultation | Unknown actions are parse errors. |
| Use only agent identifiers from the routing table in `dispatch.agent` | An unknown agent stops the run (§7.2). |
| Always provide `row` in `dispatch`, and `iteration` exactly when the row is in a block; give `agent` as the bare id | Both are validated against the table and the current block schedules; an invalid pair is retried like a malformed reply (§7.1). The row's bare agent id must equal `agent`. |
| Always provide `task_description` in `dispatch` | The protocol requires a non-empty task; the Runner does not synthesize one for an orchestrator-routed dispatch. |

### 6.2 Artifact Constraints

| Constraint | Rationale |
|-----------|-----------|
| May read the full artifact (execution log, current state, workflow notes) | This is how the orchestrator understands the run's history and current position. |
| May write to the Workflow Notes section | This is the orchestrator's scratchpad for recording reasoning across invocations. |
| Must NOT modify the execution log, current state, artifact registry, or frontmatter | These are managed exclusively by the Runner's artifact store. Concurrent writes would corrupt the artifact. |
| Exception: while `artifact_damage` is non-null, may edit those sections to repair them, and only to repair them: fix table structure, restore rows and values the evidence (the last-good copy, the dispatch log when one is given, the damaged file in place) supports, and move a misplaced note to Workflow Notes. Never invent a row, status or sequence number the evidence does not support | The Runner is blocked during the consultation, so the edit races nothing. Without the exception the damage could only be fixed by hand. The evidence bound keeps a repair from becoming a rewrite of history; the Runner checks it (§2.5). |

### 6.3 Behavioral Constraints

| Constraint | Rationale |
|-----------|-----------|
| Must NOT invoke subagents directly | All subagent dispatch goes through the Runner. The orchestrator DECIDES what runs next; the Runner EXECUTES it. |
| Must NOT modify project files | The orchestrator is a routing agent, not an execution agent. It reads the artifact for context; it does not touch the codebase. |
| Judge finished work before repeating it | A consultation can follow a `SUCCESS` because the table's next step already finished (§2.1). Following `On Success` there reruns that work; a creator rewrites approved output that must pass review again. Dispatch it again only when the work since then changed what it depends on; otherwise dispatch the first work not yet finished. This is a Routing Policy judgement and applies to the conversational orchestrator too when a run has been sent back upstream. |
| Decision continuity must live in the artifact | Record every conclusion or routing decision that a later consultation may need in Workflow Notes. Never rely on memory or conversation history from a previous consultation; the current request's `last_status_message` remains current evidence, not persisted decision state. |

---

## 7. Error Handling

### 7.1 Malformed Response

The Runner retries a malformed reply by re-invoking the orchestrator with the same request: three attempts in total, and the run stops with the last parse or validation error when the third attempt also fails. One budget covers both kinds of malformed reply:

- invalid JSON, or a field of an unusable type (for example a `row` that is not an integer or an `iteration` that is not a string), or a `stage` member other than `null`;
- an invalid `row` or `iteration`: `row` missing or not in the table, the row's agent differing from `agent` (§7.2), `iteration` missing on a block row, present on a row outside every block, malformed for the row (too many segments, a group segment on an ungrouped row, an ID outside the iteration ID character set), naming an ID not in the current schedule of the row's block, or carrying a group other than the row's.

An iteration that is valid against the schedule but whose approach does not include the row's group is not retried: the run stops with the consultation's invalid-target message.

These stop the run immediately, without retry: a reply with no JSON object at all, a missing required field (a reply with no `action` field at all counts as one; an explicitly empty `"action": ""` is an unknown `action`), an unknown `action`, a missing or empty `agent` or `task_description`, and an unknown agent (§7.2). A valid reply that routes to the wrong place is carried out and not retried.

### 7.2 Unknown Agent and Agent-Row Mismatch

An unknown agent in `dispatch.agent` stops the run at once. The error message identifies the unknown agent and lists available agents from the routing table.

A known agent whose `row` belongs to a different agent is an invalid row and is retried under the §7.1 budget; the error names the row's actual agent. This includes the whole `Subagent` cell of a mode-qualified row (`build-review(tests)`): only the bare id (`build-review`) matches.

### 7.3 Harness Error Invoking Orchestrator

The run stops without retry. The error message includes the harness error (timeout, crash, connection failure). The malformed-reply retries of §7.1 apply only to a reply that arrived.

### 7.4 Pre-Consultation Failure and Pre-Consultation Stop

A pre-consultation stop (§5.1) is not a failure. It is the orchestrator's decision: the run ends as a consultant stop (exit code 6) and the reason is reported as in §4.2. A pre-consultation failure ends the run as a start failure (exit code 7). Both leave the artifact resumable and both are re-run by a resume.

Failures of the pre-consultation reply: a harness or transport error, a reply with no JSON object, invalid JSON, an `action` that is not the string `"stop"` (§5.1) and a stop without a usable `reason`.

The Runner creates or resumes the artifact before pre-consultation. If pre-consultation fails, the run stops before any workflow dispatch and retains the artifact for inspection and resume; resuming retries pre-consultation. The failed coordinator consultation remains available in Runner diagnostic/MOSAIC logs but does not consume `global_sequence`, create an Execution Log row, or update `current_state`.

### 7.5 All Orchestrator Failures Are Terminal

Unlike subagent failures (which become deviations that the orchestrator can resolve), orchestrator failures are terminal once final. The Runner has no higher authority to escalate to. After the malformed-reply retries of §7.1 are spent, or on any failure that is not retried, the run stops, the artifact is left in a resumable state, and the user must intervene. A repair consultation is the one recovery attempt the Runner has for a damaged artifact (§7.6), and a repair consultation that fails or is stopped stops the run at once: there is no retry and no manual fallback, and the next resume finds the damage and consults again.

### 7.6 Repair Not Accepted

When the verification of §2.5 fails, the Runner discards the reply, even a valid `dispatch`, and consults again with a fresh `artifact_damage`. At most two consultations per damage occurrence; the malformed-reply retries of §7.1 apply inside each. When the second repair is also not accepted, the run stops naming the remaining problems and the paths of the saved damaged copy and the last-good copy, with the artifact as the orchestrator last left it.

The reply may well be sound, since the orchestrator had the full evidence, but the Runner cannot carry it out. It checks a dispatch against the state it reads from the artifact, and it would have to write the step's Execution Log row into a table it still cannot read: re-rendering it would make the damage permanent, and appending to it would leave the damage for the next read to find again. A harness error, failure or stop during a repair consultation stops the run at once (§7.5), without the malformed-reply retries of §7.1 beyond those already spent inside that consultation; the next resume finds the damage and consults again.

---

## 8. Runner-Side Constraints

| Constraint | Rationale |
|-----------|-----------|
| Always provide the current artifact path in `orchestration_artifact` | The orchestrator must read the latest state. A stale or wrong path leads to decisions on wrong state. |
| Write the preceding subagent's result to the artifact BEFORE invoking the orchestrator | The orchestrator reads the artifact for context. If the artifact doesn't reflect the most recent subagent completion, the orchestrator makes decisions on stale state. |
| Always provide the full `status_message` from the triggering agent in `last_status_message` | The Execution Log truncates messages. The orchestrator needs the full response to make informed routing decisions. |
| Provide the triggering response's status in `last_status`, and the triggering agent's `error_reason` in `last_error_reason` when its status is `BLOCKED`, and the Runner-constructed description for a harness error or for a block schedule that stayed defective after the re-dispatch limit; otherwise `null` | The artifact has no column for `error_reason`. Without it, the shared Routing Policy's `E100` rule ("correct the invocation or routing named in `error_reason`") cannot be applied in script mode, and the orchestrator could not tell why a producer that returned `SUCCESS` is being consulted about. |
| Never offer the orchestrator a block schedule the Runner found defective: its consultation request holds no schedule for that block, and a reply that dispatches a row of that block is invalid | A block row dispatched against a defective schedule would run on guessed iterations. The producing row may be dispatched again; the reply is otherwise handled like any invalid dispatch target (§7.1). |
| Re-read the artifact after the orchestrator returns, and after a consultation that failed or timed out | The orchestrator may have updated Workflow Notes. The Runner must pick up those changes before the next dispatch, and must find any damage the edit caused where it happened (§2.5). |
| Never write the artifact after a read that found damage; keep a last-good copy after every write; set `artifact_damage` whenever the artifact is damaged, and `null` otherwise | The store re-renders the whole artifact from what it read, so writing after a damaged read makes the loss permanent. The last-good copy and the dispatch log are the evidence the orchestrator repairs from (§2.5). |
| Keep consultation diagnostics outside `Orchestration.md` | A consultation is a fresh turn of the run's coordinator, not a protocol subagent invocation. It does not consume `global_sequence`, create an Execution Log row, or update `current_state`. Runner diagnostic/MOSAIC logs record the individual call; continuity needed for routing belongs in Workflow Notes. |
| Re-invoke the orchestrator on a malformed reply, within the attempt budget of §7.1 | A reply with invalid JSON or an invalid `row`/`iteration` is a recoverable orchestrator slip; the retry costs one consultation, not the run. |
| Run sequentially and refuse another `dispatch` value on resume | The artifact's `dispatch` field records how the run is scheduled. The Runner writes `sequential` on new runs and refuses a present value other than exactly `sequential`; a missing field is read as `sequential`. The orchestrator's consultation never carries a parallel schedule. |
| Never parse orchestrator responses beyond the defined schema | If the orchestrator returns extra fields, ignore them. Forward compatibility. |

---

## 9. Dead Ends

### 9.1 Communication Protocol as Wire Format

The initial contract reused the Communication Protocol (orchestrator↔subagent schema) for the Runner↔orchestrator boundary. This was replaced by a purpose-built schema because:
- Most CommProtocol fields (`input_artifacts`, `output_artifacts`, `include_result_summary`, `human_in_the_loop`, `status_code`, `error_code`) have fixed or meaningless values in this context — filling them adds noise without information.
- The orchestrator reads everything from `Orchestration.md`. Passing the artifact path in `input_artifacts` rather than a dedicated field obscures the one thing the orchestrator actually needs.
- The CommProtocol response requires `status_code: SUCCESS` always and `result_data` as a JSON-in-string — two layers of wrapping around the routing instruction that carries the actual content.
- The Runner↔orchestrator relationship is fundamentally different from orchestrator↔subagent: one is a routing advisory boundary, the other is a task execution boundary. Different relationships deserve different schemas.

### 9.2 Three-Action Schema (rejoin / custom / stop)

The initial contract defined three actions. `rejoin` named a table agent to route to; `custom` dispatched an off-table agent and specified a rejoin point afterward; `stop` ended the run. This was replaced by the two-action schema (`dispatch` + `stop`) because:
- `custom` encoded two decisions in one response (dispatch + rejoin point), violating the single-decision principle (`Design.md` §2.6). Under single-decision, the orchestrator is consulted again after each dispatch.
- `rejoin` was `dispatch` without a `task_description` — missing the orchestrator's primary value.
- The distinction between table and non-table agents added contract complexity for no gain.

### 9.3 Structured Deviation Payload

Early designs had the Runner send the full `DeviationInfo` as a structured JSON payload in a dedicated field. This was abandoned because:
- The orchestrator can derive everything it needs from the artifact
- Structured payloads risk the orchestrator parsing them instead of reading the artifact, which is fragile
- The `last_status_message` field carries the one thing the artifact doesn't have (the full, untruncated response); everything else is in the Execution Log

### 9.4 Context Hint Field

An intermediate design included a `context_hint` field — a Runner-constructed human-readable string describing why the orchestrator was being consulted (e.g., "contracts-review#4 returned COMPLETED_NEEDS_ACTION at phase DESIGN"). This was replaced by `last_status_message` because:
- The hint duplicated information already in the Execution Log (which agent, what status code, what phase)
- The genuinely useful information — the agent's full `status_message` — was NOT in the hint (it was a summary) and NOT in the artifact (the log truncates it)
- Sending the full verbatim `status_message` gives the orchestrator the one piece of context the artifact doesn't carry, without duplicating what it does

### 9.5 Bidirectional Orchestrator Communication

Considered a multi-turn exchange where the Runner and orchestrator go back and forth (orchestrator asks for more context, Runner provides it). Rejected because:
- The harness adapter is a single-shot invoke-and-return interface
- Multi-turn would require session management between the Runner and orchestrator
- The artifact already carries all context the orchestrator should need

### 9.6 Orchestrator as a Special Subagent

Considered treating the orchestrator as another row in the routing table (a "meta-agent" that routes to other rows). Rejected because:
- It conflates the routing level with the execution level
- The orchestrator's input/output is fundamentally different from subagents (it reads/writes the orchestration artifact, not domain artifacts)
- It would require the engine to special-case one row in the table, defeating the table's uniformity

---

## 10. Changelog

| Version | Date | Summary |
|---------|------|---------|
| 0.10 | 2026-11-09 | **Artifact repair (implemented, Runner v1.12.0).** A damaged `Orchestration.md` is repaired by the orchestrator inside a regular routing consultation, as a conversational orchestrator does, not in a separate context (§2.1, §2.5). The routing request gains `artifact_damage` (findings, last-good copy path, dispatch log path only when a usable log exists), always present and `null` without damage; the pre-consultation request has no such member (§3.1, §3.2). While it is set the orchestrator may edit Runner-owned sections to repair them, bounded by the evidence (§6.2). The Runner verifies the repair before acting on the reply and discards the reply when the repair is not accepted, at most two consultations per damage occurrence (§7.6). The Runner reads strictly, never writes after a damaged read, checks the artifact after failed consultations too, and keeps a last-good copy (§8). A run whose frontmatter is damaged and has no usable last-good copy gets no repair consultation (§2.5). The reply schema is unchanged. Runner v1.12.0 implements this; the `orchestrator-script` version is set by the separate orchestrator change (pending). |
| 0.9 | 2026-11-08 | **`HARNESS_ERROR` and `user_message`.** The Runner records an invocation that ended without a usable subagent response as `HARNESS_ERROR` (Runner-recorded only, not a Communication Protocol status) instead of a synthetic `BLOCKED` with `E501`; it is re-dispatched mechanically in Modes 2/3 within its own budget of 3 (§2.3). The request gains `last_status` (the status of the response whose message is in `last_status_message`, `HARNESS_ERROR` for a harness error) and `user_message` (the text the user entered when handing a routing decision over in the manual dialogue; `null` otherwise), both always present (§3.1, §3.2). For a harness error `last_status_message` and `last_error_reason` carry the failure description. The script orchestrator agent must know `HARNESS_ERROR` and `user_message`; the Runner requires `orchestrator-script` v3.5.0, which ships together with Runner 1.11.0. |
| 0.8 | 2026-10-13 | A pre-consultation reply may be the routing `stop` (§5.1): a decision that ends the run before any dispatch as a consultant stop (exit code 6), leaving the artifact as a pre-consultation failure does. Any other `action`, and a stop with a missing or empty `reason`, is malformed (pre-consultation failure, §7.4). A routing reply with no `action` field is a missing required field and stops at once, not retried (§7.1). A consultation made on resume, a finished-work consultation included, carries `last_status_message: null` (§3.2). §4.2 states that the stop reason is not persisted. No schema field is added. `orchestrator-script` stays at v3.3.0. |
| 0.7 | 2026-10-13 | Modes 2/3 also consult after a `SUCCESS` whose next step already finished (`Design.md` §2.10); §2.1 lists the trigger and §6.3 states the judgement the orchestrator makes there. The wire schema is unchanged: the request carries the succeeding step's message in `last_status_message`, and no deviation kind is sent. The Runner requires `orchestrator-script` v3.3.0, whose Routing Policy states that judgement and whose Deviation Triggers list the case. |
| 0.6 | 2026-10-12 | The Runner requires `orchestrator-script` v3.2.0 and runs every run with `dispatch: sequential`, refusing other recorded values on resume. Section 4.1 speaks of phase and iteration changes. The wire schema is unchanged. |
| 0.5 | 2026-10-12 | **Breaking:** the `dispatch` reply names the iteration of a block row in `iteration`, a string holding exactly the Execution Log's `Iteration` value for that row (`"A"`, `"4"`, `"1.Test"`), split using the row's `Phase` cell; it replaces `stage`, which is no longer accepted. `agent` is the bare agent id, without the mode qualifier; the whole `Subagent` cell is an agent-row mismatch. Validation failures keep their handling classes (retried within the malformed-reply budget, or stopping at once). `orchestrator-script` v3.1.0 states the same contract and the new Runner requires it (§4.1, §7.1, §7.2). |
| 0.4 | 2026-10-11 | `last_error_reason` also carries a Runner-constructed description when the consultation follows a block schedule that a producing row (or a row of the block that writes it) left defective after the Runner's two re-dispatches of that row: the rule ID (or that the Source is missing or unreadable), the block, the Source and Section, and the offending row, ID or token. `last_status_message` stays the producer's verbatim `status_message`. While a block's schedule is defective the request holds no schedule for it, so a reply that dispatches a row of that block is invalid (§3.2, §8; behaviour in `Design.md` §3.6). The request and response schemas are unchanged. |
| 0.3 | 2026-10-08 | **Breaking:** every `dispatch` reply must carry `row` (1-based, numbered as the Execution Log's `WorkflowRow`) and, for a staged row, `stage`; a reply without them is invalid. Added the three row carriers (workflow table `Row` column - optional; Execution Log `WorkflowRow` - `-` allowed; routing reply `row` - mandatory) and which outcomes the Runner handles itself (`PARTIALLY_DONE`, `BLOCKED`/`E501`; auto and auto-review modes only; orchestrated mode consults on every decision). Malformed replies (invalid JSON, invalid `row`/`stage`, agent-row mismatch) are retried up to three attempts in total before the run stops; other orchestrator failures remain terminal. |
| 0.2 | 2026-09-27 | Both orchestrators now share one word-for-word Routing Policy (round-six R6-13): the script orchestrator's `task_description` follows the same minimal rule as native, so "targeted task descriptions as primary value" wording and the findings-quoting example were replaced. The request gains `last_error_reason`: the triggering response's `error_reason` for `BLOCKED` (the Runner-constructed description for a harness error, which is recorded as `BLOCKED`/`E501`), otherwise `null`. It gives script mode the `error_reason` the shared `E100` rule depends on and the artifact never persists (round-seven R7-04, R7-05). Clarified the script orchestrator's continuity boundary. Prior consultation conclusions and routing decisions needed later must be recorded in Workflow Notes rather than retained as conversation memory; the current request's `last_status_message` remains current evidence, not persisted decision state. Script-orchestrator consultations are coordinator turns rather than protocol subagent invocations: Runner diagnostic/MOSAIC logs retain the individual calls, while consultations do not consume `global_sequence` or create Execution Log rows (round-eight R8-04). Pre-consultation now consistently occurs after artifact creation or resume and any required commit setup; failure stops before workflow dispatch while retaining the artifact for inspection and resume (round-eight R8-05). Response schemas are unchanged. §6.1's unknown-agent cross-reference now points to §7.2. §4.1's `current_state` sentence now takes the dispatched row's position once the invocation's outcome is accepted, not at dispatch (round-six R6-05). §5 pre-consultation: environment facts belong in `task_description`; the `py` example moved there, and `constraints` is limited to scope or deliverable restrictions (R6-06). |
| 0.1 | 2026-08-16 | Initial design. Purpose-built wire schema (not Communication Protocol). Request: `orchestration_artifact` + `context` + `last_status_message`. Two-action response: `dispatch` (with optional artifact/constraint overrides) or `stop`. Pre-consultation response for environment strings. Dead ends: CommProtocol reuse, three-action schema, context hint field, structured deviation payload, bidirectional communication, orchestrator as subagent. |
