# Runner Design

> **Status:** Draft
> **Created:** 2026-08-15
> **Last Updated:** 2026-10-13
> **Scope:** The design of `mosaic-run`, the CLI tool that executes orchestration workflows without a human orchestrator in the loop. Covers execution modes (how much routing intelligence the Runner handles autonomously versus delegating to a script-mode orchestrator agent), the architectural layers that implement those modes, the dispatch loop lifecycle, and the contract boundaries between the Runner and the systems it drives (harness adapters, orchestrator agents, the orchestration artifact).

---

## 1. Purpose

The MOSAIC orchestration system was designed around a human orchestrator — an LLM-powered agent that reads a workflow table, dispatches subagents, interprets their responses, and decides what to do next. The orchestrator runs as a persistent LLM session: it creates the orchestration artifact, dispatches a subagent, records the result, reads the response, dispatches the next subagent, records that result... and the context window grows with every iteration. By step 15, the orchestrator is re-processing all prior tool calls, artifact edits, and subagent responses from steps 1–14. Cost scales roughly quadratically with run length.

The orchestrator also does two kinds of work in that loop: **mechanical** work (editing the artifact, tracking sequence numbers, resolving artifact paths, invoking harness commands) and **intelligent** work (deciding which agent runs next, crafting a task description that tells the subagent what to do, handling deviations). The mechanical work is deterministic and does not benefit from LLM reasoning — but it consumes context window and inference cost as if it did.

The Runner (`mosaic-run`) separates these concerns. It takes over all mechanical work: reading workflow tables, invoking subagents through harness adapters, writing the orchestration artifact, tracking sequences, resolving artifacts. The intelligent work — routing decisions and task descriptions — is either handled by the Runner's deterministic engine (when the workflow table prescribes the answer) or delegated to a script-mode orchestrator agent invoked as a fresh, bounded-context session.

### 1.1 The Cost Model

The Runner's three execution modes (§2) represent different points on the cost-vs-quality spectrum:

| Approach | Orchestrator Invocations | Context Growth | Routing Judgment |
|----------|------------------------|----------------|-------------------------|
| **Current (no Runner)** | Every step, persistent session | Unbounded — grows with run length | Every step, in one long session |
| **Mode 1 (Orchestrated)** | Every step, fresh session each time | Bounded — always system prompt + Orchestration.md | Every step |
| **Mode 2 (Auto)** | Only on deviations | Bounded | Non-SUCCESS only, except the mechanical retries of §2.4, plus a SUCCESS whose next step already finished (§2.10); the engine follows the table on every other SUCCESS |
| **Mode 3 (Auto-review)** | Only on unresolvable deviations | Bounded | Unresolvable deviations only |

Mode 1 is not the "expensive" option — it is already dramatically cheaper than the current approach because each orchestrator invocation starts a fresh session (no context window growth) and all mechanical work is offloaded to the Runner. Modes 2 and 3 reduce cost further by eliminating most orchestrator invocations. The price is the orchestrator's judgment on the happy path, and they run only the workflow shapes the engine supports.

**Direction.** Modes 2 and 3 are the intended primary modes. Mode 1 is the fallback: it covers any workflow the script orchestrator can interpret, including table features the engine does not support yet (§1.3), and it remains useful beyond that.

### 1.2 The Dispatch Intelligence Gap

What the orchestrator adds beyond mechanical work has two layers.

**Routing judgment** (where the run must go when the table alone does not say):
- A reviewer's findings implicate upstream work, so the fix belongs to `requirements-refinement` rather than the `On Findings` creator
- A `NEEDS_CLARIFICATION` asks for codebase facts, so a research agent runs before the asking agent is re-dispatched
- A repeated failure or an exhausted review-loop limit calls for a stop, not another attempt
- A run sent back upstream reaches work that already finished, and only judgement can say whether it must run again

**Environment context** (facts that apply to EVERY invocation):
- Skills are located at `.claude/skills/` — read the relevant skill from there by name
- Use `py` not `python` for the Python interpreter
- Harness-specific quirks subagents should be aware of

Task descriptions are not a third layer. Both orchestrators follow a shared minimal rule: state what to accomplish, never how, and never reshape scope from domain content (the Routing Policy in `orchestrator.md` / `orchestrator-script.md`). An orchestrator-stated task therefore differs little from the Runner's generic one. Earlier drafts treated targeted, findings-quoting descriptions as Mode 1's main value, but in practice they over-directed subagents.

Mode 1 supplies both layers on every step. Modes 2/3 lose routing judgment on auto-routed steps and follow the table instead, but never into work that already finished: that step goes to the orchestrator (§2.10). That is their fundamental trade-off, and it is acceptable wherever the table already says the right thing.

The environment context gap is a different class of problem: a subagent that cannot find its skills or uses the wrong Python command fails for plumbing reasons unrelated to its task, and those failures are harder to attribute than task-logic failures. Pre-Consultation (§2.8) addresses this narrow layer with a one-shot orchestrator invocation at run start. It does not close the core dispatch intelligence gap — that remains the fundamental trade-off of Modes 2/3 — but it prevents a category of failure that is difficult to diagnose and easy to avoid.

### 1.3 What the Runner Is Not

The Runner is not a replacement for the orchestrator agent. The orchestrator agent holds domain knowledge (how to recover from a failed test run, when to skip a stage, how to interpret ambiguous findings). The Runner holds workflow-table knowledge (which row comes next, how block iterations are ordered, how groups work). The Runner replaces the orchestrator's *mechanical* work; the orchestrator retains its *intelligent* work and is invoked when judgement is needed.

The Runner is also not protocol-agnostic. It parses subagent responses against the MOSAIC Communication Protocol's status and error vocabularies, which are compiled into it. It does not check which protocol a deployment carries. A catalog-supplied replacement protocol that changes either vocabulary is unsupported and fails at the first subagent response (`Development/Designs/CommunicationProtocol.md` §10.7). The test catalog (`Tools/Runner/TestCatalog`) deploys as a standalone catalog stack with its own copies of the protocol, bundle and harness injections; keeping them in sync with the default ones is a maintenance obligation (see `TestCatalogDesign.md`).

The Runner is also not a workflow engine in the general sense. It executes MOSAIC workflow tables — a specific format with specific semantics. It does not interpret arbitrary DAGs, BPMN diagrams, or pipeline definitions.

The Runner currently invokes one workflow agent at a time. For an admitted workflow, work that a native orchestrator may dispatch concurrently (branches of a fork, iterations of a block) is executed sequentially; this changes scheduling, not which admitted iterations run. True concurrent workflow dispatch is deferred and tracked in `ROADMAP.md`.

**Admission is mode-dependent.** The engine routes in Modes 2 and 3, so admission there refuses table shapes the engine cannot interpret: forks (comma-separated multi-target `On Success`) and joins (`Waits For`), which the engine would have to run in parallel, and a routing cycle (A15). In Mode 1 the engine never routes; it only builds requests for the agent the orchestrator names. Mode 1 therefore admits any table the script orchestrator can interpret, and refuses only what request construction itself needs, such as unresolvable agents or artifact templates. A Mode 1 fork runs its branches one after another: each branch is one routing decision, and the orchestrator judges the join (every `Waits For` branch `SUCCESS`) from the Execution Log. New workflow-table features therefore work in Mode 1 first; Modes 2/3 refuse them until the engine learns them. Mode 1 still refuses a fork branch that is the last row of its phase (which includes the last row of a group), because the last-row helpers answer by table position and such a branch has no join to end on; the refusal names the branch and its row.

### 1.4 Supported Workflow Subset

`Development/Designs/WorkflowFormat.md` defines what a workflow means and the rules that make it valid. This section states which of those shapes the Runner runs and how it resolves the parts the format leaves to the executor.

**Authority.** The Runner implements WorkflowFormat 2.16 and nothing earlier. No 1.x notation is accepted: not `[StageNumber]` or `{StageNumber}`, not an `**Execution Groups:**` heading, not a `*` suffix on a row, not lowercase `next`, not `-` in `On Success`, and no schedule source is inferred from an artifact named `*Plan.md`. A workflow written in 1.x notation is refused at start and on resume. The Runner never reads a block schedule from a file because of its name: each block's schedule comes from the `Source` and `Section` its `**Blocks:**` table declares.

**Supported in every mode.**

- **Block schedules** read from each block's declared `Source` and `Section`, whatever the artifact is named, including a seeded source (an artifact the run is started with).
- **Blocks and groups.** Blocks on any phase, several sibling blocks, and grouped blocks per the `**Groups:**` table (a group-qualified row runs in the iteration's approach order).
- **Iteration IDs** of letters, digits and `-`, compared ignoring case and used in paths and in the log as the schedule spells them.
- **Mode-qualified `Subagent` cells** (`agent(mode)`): the agent is dispatched under its bare id, and the qualifier reaches the subagent as a focus paragraph in its task description.
- **Fill-ins.** A `{Name}` in an `Input` or `Output` entry is replaced by the current iteration ID of block `Name`.
- **Wildcards.** An `Input` entry containing `*` lists every existing artifact that matches (`*` matches within one path segment); an `Output` entry containing `*` is passed to the subagent literally.
- **The deployed `Row` column** is accepted when each number equals the row's 1-based position, and refused (T4) otherwise.

**Routing in Modes 2 and 3.** The engine follows the `On Success` entries (a target, `[Name]`, `NEXT`, `COMPLETE`) with their 2.16 scopes (`WorkflowFormat.md` sec. 6.5), including routing inside a group or an iteration: a target names a row of the row's own scope, `NEXT` and `COMPLETE` end the scope, and ending a group moves the iteration on to its next group. A block is entered at its first row and exits to the row after it. Iterations run one at a time, in schedule order: the next iteration is the first whose `Depends On` iterations have all completed, and an iteration is complete once its last group, in its approach order, has ended with a `SUCCESS`. Entering a block again (re-entry, for example after a routing back) runs only the uncompleted iterations. In Mode 3 an `On Findings` entry routes a reviewer's `COMPLETED_NEEDS_ACTION` back to the row it names (§2.4); in Mode 2 findings are always a deviation. In Mode 1 routing and iteration order stay with the script orchestrator, which the Runner validates against the table and the block schedules (§2.9).

**No parallel execution.** The Runner dispatches one agent at a time. Forks and joins are refused in Modes 2 and 3. Mode 1 runs a fork's branches one agent at a time (§1.3), and each entry of a fork list must match a `Subagent` cell (T9).

**Shapes the Runner refuses.** A valid workflow outside this subset is refused before the first dispatch:

| Shape | Modes 2/3 (engine routes) | Mode 1 (orchestrated) |
|---|---|---|
| Sequential table, blocks on any phase, several sibling blocks, grouped or ungrouped | Runs | Runs |
| Fork (comma in `On Success`) / join (`Waits For`) | Refused (B1 on a block row) | Runs, one agent at a time (§1.3), except a fork branch that is the last row of its phase or group |
| Routing cycle among `On Success`, `NEXT` and block entry (A15) | Refused | Runs |
| Nested blocks (a row with two block references, or a block `Source` that fills in another block's ID) | Refused | Refused |
| `On Success` / `On Findings` / `Waits For` target that is prose or matches no `Subagent` cell (T6, T9), `[Name]` or a target outside the row's own scope (A11, A12) | Refused | Refused |
| Block `Source` that is neither a seed nor output by a row outside the block before it is entered (A7) | Refused | Refused |
| Seeded block whose schedule is missing, unreadable or breaks V1-V6 at start or resume | Refused | Refused |
| Any 1.x notation | Refused | Refused |

Each refusal names the rule ID where the format has one, the 1-based row or the token, and the workflow id. The rule IDs the Runner checks, as implemented:

| Where | Rules |
|---|---|
| Parsing the workflow region | T1-T9 and T11-T14, P1-P12. T4 covers the deployed `Row` column; T10 (a source workflow has no `Row` column) is not checked, because the Runner reads deployed tables |
| Admission (step 4 of §4) | A1-A7, A11, A12, and the no-cycle part of A15 (Modes 2/3); B1 (Modes 2/3). Nested blocks are refused by name without a rule ID |
| Reading a block schedule | V1-V6 |

A rule the Runner does not check (A8, A9 and A16, which concern nested blocks; A13 and A14, which concern artifact inputs; the reachability part of A15; and B2-B6) is not enforced: a workflow that breaks it is not refused for it, and the Runner's behaviour on it is not specified.

**Defective produced schedules.** A block schedule that a row produces and that is defective is never used and never replaced by the previous one: the Runner re-dispatches the producing row up to 2 times (3 attempts in all) and then consults the routing consultant; §3.6 states the limit, the counting and the Mode 1 behaviour.

**Run artifacts.** The Runner writes OrchestrationArtifactFormat 3.0: the Execution Log has an `Iteration` column whose value is `{ID}` for an ungrouped block row, `{ID}.{Group}` for a grouped one (for example `1.Test`) and `-` outside every block, and `current_state` has an `iteration` key. A run artifact written before this format (an Execution Log `Stage` column, a `current_state.stage` key, or a grouped value in the `{Group}.{ID}` form) is refused on resume.

---

## 2. Execution Modes

The Runner supports three execution modes that control how much routing intelligence is handled autonomously versus delegated to a script-mode orchestrator agent. The modes form a spectrum from full delegation to maximum autonomy.

### 2.1 Mode Overview

| Mode | Name | Engine Routes | Orchestrator Decides | Workflow Shapes | Cost vs. Current |
|------|------|---------------|---------------------|-------------------|-----------------|
| 1 | **Orchestrated** | Nothing | Everything | Any the orchestrator can interpret (§1.3) | Much cheaper (bounded context) |
| 2 | **Auto** | SUCCESS onto work not yet finished (§2.10) + mechanical retries (§2.4) | Everything else | Engine-supported only | Cheapest for deviation-heavy runs |
| 3 | **Auto-review** | Same as Mode 2 + creator/reviewer loops | Remaining deviations only | Engine-supported only | Cheapest overall |

All three modes are cheaper than the current approach (persistent orchestrator session with unbounded context growth). The modes differ in how much further they reduce cost, and what quality they trade for it.

### 2.2 Mode 1 — Orchestrated

**Routing rule:** The Runner never decides "what next." After every subagent invocation, the Runner invokes the script-mode orchestrator agent as a fresh session. The orchestrator reads Orchestration.md, makes a routing decision, and returns both the target agent AND a task description for the next dispatch.

**Why this is cheaper than the current approach:** The current orchestrator accumulates context across the entire run — every tool call, every artifact edit, every subagent response stays in the context window. In Mode 1, each orchestrator invocation starts fresh: the context is always just the orchestrator's system prompt + the compact Orchestration.md artifact. No growth. The Runner handles all mechanical work (artifact writes, harness invocations, sequence tracking) that previously consumed orchestrator context.

**What it adds over Modes 2/3:** The orchestrator judges every step. It can send work upstream when a finding implicates it, pick the agent that can answer a clarification, or stop. It states each task minimally, with the environment facts its instructions carry, and that remains useful guidance for the subagent. Because the engine never routes in Mode 1, it also runs workflow shapes Modes 2/3 do not support yet (§1.3).

**Engine role:** The engine is used only for dispatch construction (building `ProtocolRequest` objects, resolving artifact paths) — never for routing decisions. The routing decision, task description, and effective HITL all come from the orchestrator's instruction.

**When to use:**
- Fallback: workflows using table features the engine does not support yet (e.g., parallel forks), which Modes 2/3 refuse at admission
- Workflows with complex conditional routing that depend on artifact content (e.g., "skip the design phase if the research shows no architectural changes needed")
- When per-step routing judgment matters more than cost
- Developing or debugging the orchestrator agent (the Runner becomes a test harness for the orchestrator's routing logic)

**Dispatch loop:** See the unified loop in §3.1. In Mode 1, step 1 always consults the orchestrator — the engine is never asked for a routing decision. The orchestrator returns a dispatch instruction (agent, task description, and optionally artifact overrides and constraints) or `stop`. The Runner applies orchestrator-provided fields and falls back to table-row defaults for anything the orchestrator omits. See `ScriptOrchestratorContract.md` for the full schema.

**Difference from Modes 2/3:** In Modes 2 and 3, the engine makes routing decisions first and only falls through to the orchestrator on deviation. In Mode 1, there is no engine routing — the orchestrator decides everything and states every dispatch's task.

### 2.3 Mode 2 — Auto

**Routing rule:** The engine handles SUCCESS routing autonomously as long as the next step is work that has not finished yet (§2.10), and re-dispatches `PARTIALLY_DONE` and `BLOCKED` with `E501` mechanically within the bounds of §2.4. A SUCCESS whose next step already finished, and all other non-SUCCESS responses — including COMPLETED_NEEDS_ACTION from review agents — are delegated to the orchestrator.

**Engine role:** The engine reads the workflow table, determines the On Success target, resolves groups and iteration order, and produces a `DispatchDecision`. For the mechanical retries it produces a `DispatchDecision` for the same row. For other non-SUCCESS responses, and for retries whose bound or budget is used up, the engine produces a `DeviationDecision`, and the session invokes the deviation resolver (orchestrator delegate or stop).

**Trade-off:** On the happy path (SUCCESS → next row), the Runner sends a generic task description and follows the table's On Success target without judgment. Orchestrator-stated descriptions are minimal by policy, so the description itself loses little. What is lost is the chance to route differently after a SUCCESS. Deviations still reach the orchestrator.

**When to use:**
- When happy-path subagents perform well with generic task descriptions (their instructions and artifact inputs are sufficient)
- When deviations require orchestrator judgement — review findings sometimes warrant more than routing back to the creator (e.g., escalating to the user, adjusting the plan, or stopping the run)
- When minimizing orchestrator invocations on the happy path matters more than per-step routing judgment

**Dispatch loop:** See the unified loop in §3.1. In Mode 2, step 1 asks the engine first. SUCCESS is auto-routed (generic task description) unless its next step already finished (§2.10), and the mechanical retries of §2.4 are auto-routed too. Every other non-SUCCESS — including COMPLETED_NEEDS_ACTION — produces a Deviation, which triggers orchestrator consultation (orchestrator-stated task description).

**Key difference from Mode 3:** COMPLETED_NEEDS_ACTION with an unambiguous On Findings target is NOT auto-routed. It produces a Deviation and goes to the orchestrator. The value is routing flexibility — the creator reads the review artifact regardless and derives its focus from there. What the orchestrator adds is the ability to override the On Findings target: when a reviewer's findings point to an upstream problem (e.g., "the contracts are wrong because the requirements are incomplete"), the orchestrator can route to `requirements-refinement` instead of `contracts-designer`. Mode 3 would blindly auto-route to the On Findings target, which may cause the creator to attempt a local fix when the real problem is upstream.

### 2.4 Mode 3 — Auto-review

**Routing rule:** The engine handles SUCCESS routing and creator/reviewer loop routing autonomously. Only truly ambiguous or unexpected situations are delegated to the orchestrator.

**Engine role:** Same as Mode 2, plus: when a review agent returns COMPLETED_NEEDS_ACTION and the workflow table row has an unambiguous On Findings target, the engine auto-routes back to the paired creator without invoking the orchestrator.

**Trade-off:** Review-loop re-dispatches are auto-routed too, with generic descriptions and table targets. The creator reads the review artifact, which is added to its inputs, to learn what to fix; creators are designed to read review feedback, so this usually works. What Mode 3 gives up relative to Mode 2 is the orchestrator's chance to route a finding upstream instead of back to the paired creator. The review loop limit (`review_loop_limit`) still applies: before auto-routing, the engine counts that reviewer's COMPLETED_NEEDS_ACTION iterations. The count is kept per reviewer, phase and iteration and starts over after the reviewer's last SUCCESS there. A row of the reviewer directly after a COMPLETED_NEEDS_ACTION row of the same reviewer is a re-dispatch, so it neither counts nor resets the count. When the count reaches the limit, the engine produces a deviation of kind review-loop-limit instead of auto-routing. With no limit it never stops.

**When to use:**
- Maximum automation and minimum cost: the orchestrator is invoked only when something genuinely unexpected happens
- When review-loop routing is fully captured by the workflow table's On Findings column
- When subagents reliably derive their task from instructions + artifacts without orchestrator routing

**Auto-routed cases:**

| Subagent Status | Condition | Engine Action | Task Description | Artifacts |
|----------------|-----------|---------------|-----------------|-----------|
| SUCCESS | The next step has not finished (§2.10) | Route to the On Success target, with block entry and iteration order | Generic | Table defaults |
| SUCCESS | The next step already finished (§2.10) | Deviation → orchestrator | Orchestrator-stated | Orchestrator specifies |
| PARTIALLY_DONE | Fewer than 4 consecutive PARTIALLY_DONE rows at this row and iteration | Re-dispatch the same row and iteration | Generic + the previous status message | Table defaults + the previous output artifacts added to input |
| BLOCKED with `E501` | E501 budget not spent (fewer than 3 attempts at this row and iteration since the last SUCCESS there) | Re-dispatch the same row and iteration | Generic | Table defaults |
| COMPLETED_NEEDS_ACTION | Row has unambiguous On Findings, review loop limit not reached | Route to On Findings target | Generic | Table defaults + review artifact added to input |
| COMPLETED_NEEDS_ACTION | Row has no/ambiguous On Findings, or review loop limit reached | Deviation → orchestrator | Orchestrator-stated | Orchestrator specifies |
| Any other non-SUCCESS, or a retry whose bound or budget is used up | Always | Deviation → orchestrator | Orchestrator-stated | Orchestrator specifies |

The COMPLETED_NEEDS_ACTION rows apply in Mode 3 only. In Mode 2 COMPLETED_NEEDS_ACTION is always a deviation. The two retry rows apply in Modes 2 and 3 only; in Mode 1 the orchestrator is consulted after every status.

**Mechanical retries (Modes 2 and 3):** `PARTIALLY_DONE` and `BLOCKED` with `E501` (`TOOL_UNAVAILABLE`) are transient shortfalls the shared Routing Policy answers with a fresh invocation of the same agent, so the engine does it without consulting the orchestrator. The retry is checked before any other non-SUCCESS routing and always targets the row and iteration that just ran. All counting comes from the Execution Log, so it survives a resume:

- **PARTIALLY_DONE bound.** With k the length of the trailing run of PARTIALLY_DONE rows at the last step's row and iteration (rows without a recorded row are skipped, any other row ends the run), the engine re-dispatches while k is at most 3 and produces a deviation once k exceeds 3. The bound is Runner-only; the Routing Policy has no cap, so after the bound the orchestrator applies the unchanged policy with judgement. The re-dispatch hands the previous output over: the previous output artifacts join the inputs and the previous status message is appended to the generic task description.
- **E501 budget.** At most 3 `BLOCKED`/`E501` attempts per row and iteration since the last SUCCESS there. The engine re-dispatches while fewer than 3 are recorded and produces a deviation when the budget is spent. A harness error is recorded as a `BLOCKED`/`E501` row (§3.3), so it spends the same budget, and so does a failed raw-text bypass re-dispatch. Only rows whose Summary carries the `[error:E501]` marker count, see below.
- **Anything else.** BLOCKED with another or no error code, COMPLETED_NEEDS_ACTION outside Mode 3's findings loop, and an exhausted bound or budget are deviations: the orchestrator is consulted.

**The `[error:CODE]` Summary marker:** A BLOCKED Execution Log row ends its Summary cell with a marker such as `[error:E501]`, appended after truncation so it is always complete. The log has no error-code column, and the marker is how a later read of the artifact (including resume) knows a BLOCKED row's code. The Runner recognises it only as the last token of the cell, in the shape `[error:` followed by ASCII letters or digits and `]`; other bracket markers such as `[commit:abc]` are left alone. Rows of other statuses carry no marker. The E501 budget above counts BLOCKED rows whose marker reads `E501`.

**Anti-loop guard:** The session counts consecutive dispatches of the same agent at the same workflow row, in memory only. The count restarts when the row or the agent changes, and on every run start including resume. A dispatch the orchestrator requests is blocked when the same agent has already been dispatched 4 times in a row at that row. A blocked dispatch is not executed and writes no Execution Log row; instead the session consults the orchestrator again with a synthetic BLOCKED deviation that states the guard fired. The guard bounds the orchestrator only: dispatches the engine itself produces are counted but never blocked, and mechanical retries are not counted at all, because their own bounds (the PARTIALLY_DONE bound and the E501 budget, both read from the Execution Log) own them and they must not eat into the allowance for dispatches the orchestrator requests afterwards. In Mode 1 the guard is also the only bound on the raw-text bypass re-dispatch; in Modes 2 and 3 the E501 budget owns it.

**Guard escalation bound:** If the orchestrator, consulted after the guard fired, again requests the dispatch the guard blocks, the guard escalates again, but only twice. The third consecutive blocked request ends the run as a resumable stop (outcome `RunStopped`) with a message naming the agent and row. Because nothing was written, the artifact is still resumable. An allowed dispatch, or any engine dispatch, resets the escalation count.

**Row identification:** The engine identifies the row that ran from the `WorkflowRow` value recorded in the Execution Log (the column directly after `Iteration`, matching the table's `Row` column), not from seq arithmetic or invocation counts. This holds for every row type, block row or not: whenever the entry records a row, that row is the position. Row and iteration are derived by one routine that both live routing and resume use, so the two cannot disagree. The recorded row is validated against the table: it must exist and hold the entry's agent (compared by bare agent id), and for a block row the entry must carry an `Iteration` value in the form the row accepts (`{ID}`, or `{ID}.{Group}` for a grouped row with the row's own group). Live routing and resume stop and report when it does not. A block row whose log entry records no iteration is refused with a message that names the row. Infrastructure, out-of-band and ad-hoc steps record `-`; only an entry that records no row (these steps) falls back to resolving by agent and phase, and an agent that fills several rows of one phase cannot be resolved that way and is reported rather than guessed. The three places that carry a row are stated in "Row carriers" below. The table's `Row` column is added by mosaic-deploy; the Runner accepts tables with or without it and refuses one whose `Row` numbers do not match row positions. The `On Findings` target is the row the entry names (whole-cell match against a `Subagent` cell, T9); there is no nearest-row search, and a table whose target matches no cell is refused at parse. A run is pinned to its workflow only by `workflow_version`, and the table is re-read on every start including resume, so authors bump the version on every table edit; a mismatch refuses resume unless version drift is allowed.

**Review artifact injection (COMPLETED_NEEDS_ACTION auto-routing):** When the engine auto-routes back from a reviewer to the On Findings target (typically the paired creator), it adds the reviewer's output artifact to the target's `input_artifacts`. The table row for the creator lists the creator's normal inputs — it does not anticipate review loops. The engine knows which artifact the reviewer produced (from the table row's Output column) and adds it to the creator's input set. This ensures the creator can read the review findings without the orchestrator having to specify it manually.

**Creator/reviewer pair recognition:** The engine does not need to understand the `-review` naming convention from `OrchestrationSemantics.md`. It operates purely on the workflow table's On Findings column: if the column is present, non-empty, and contains a plain agent identifier (no spaces, no parentheses), the target is unambiguous and the engine routes to it. The semantic meaning (that this is a creator/reviewer pair) is established by the workflow author when they write the table; the engine just follows the instruction.

**Row carriers:** The position of a step in the routing table is carried in three places, with different optionality (`ScriptOrchestratorContract.md` §2.4):

| Carrier | Optional? |
|---------|-----------|
| Workflow table `Row` column | Optional. A table without it is counted by 1-based data-row position |
| Execution Log `WorkflowRow` column | The value `-` is allowed, for infrastructure agents, ad-hoc steps and logs written before the column existed |
| Routing reply `row` (§2.9) | Mandatory on every `dispatch`; nothing resolves the row from `agent` |

All three use one numbering: the table's `Row` column value, or the 1-based position among the table's data rows when there is none.

### 2.5 The Task Description Spectrum

Task descriptions differ little across modes, because orchestrator-stated descriptions are minimal by policy. Pre-consultation (§2.8) adds environment facts to auto-routed ones. To make this concrete:

**Mode 1 dispatch (orchestrator-stated, minimal + environment):**
```
Revise ContractsDesign.md to resolve the findings recorded in
contracts-review.md.
Skills are at .claude/skills/ — read the relevant skill by name. Use `py`
not `python`.
```

**Mode 2/3 dispatch WITH pre-consultation (generic + environment, default):**
```
Proceed with your task.
Skills are at .claude/skills/ — read the relevant skill by name. Use `py`
not `python`.
```

**Mode 2/3 dispatch WITHOUT pre-consultation (--pre-consult=false):**
```
Proceed with your task.
```

The generic message is deliberately minimal. The subagent already has its instructions (which define its role and scope), its `input_artifacts` list (which tells it what to read), and its `output_artifacts` list (which tells it what to produce) — the Runner has no domain understanding to add. What Modes 2/3 give up is routing judgment, not description content.

Pre-consultation adds environment plumbing on top of the generic content.

### 2.6 The Single-Decision Principle

Regardless of mode, the orchestrator makes exactly ONE routing decision per invocation. It reads Orchestration.md, decides what should happen next, and returns. The Runner executes that decision, records the result in the artifact, and — if needed — invokes the orchestrator again for the NEXT decision.

This applies uniformly:
- **Mode 1 happy path:** Orchestrator reads artifact → "dispatch contracts-designer with this task" → Runner executes + records → orchestrator reads updated artifact → "dispatch contracts-review" → ...
- **Mode 2/3 deviation:** Engine can't route → orchestrator reads artifact → "dispatch contracts-designer to fix the issues" → Runner executes + records → engine can now route (SUCCESS onto work not yet finished) → continues autonomously
- **Complex deviation chain:** Engine can't route → orchestrator reads artifact → "try codebase-research first" → Runner executes + records → orchestrator reads artifact → "now dispatch test-writer-tdd" → Runner executes + records → engine can route → continues
- **Routing back (§2.10):** a step asks for upstream work → orchestrator: "dispatch planner-tdd-soft" → SUCCESS, but the table's next step `plan-review` already finished → orchestrator: "dispatch plan-review" → SUCCESS, next step `contracts-designer` already finished → orchestrator: "dispatch implementation-tdd at stage 4" → SUCCESS onto work not yet finished → engine continues. The handback is never declared: the engine takes over at the first SUCCESS whose next step is new work

The orchestrator never resolves a full deviation chain internally. It never invokes agents itself. It makes one decision, returns, and is consulted again if needed. Every intermediate step is executed by the Runner and recorded in the execution log.

**Why this matters:**

| Property | Single-decision | Multi-step resolution |
|----------|-----------------|----------------------|
| Orchestrator context | Bounded: system prompt + Orchestration.md, always | Grows within the resolution chain (same cost problem the Runner exists to solve) |
| Infrastructure triggers | Fire after each step, even during deviation resolution | Don't fire: orchestrator dispatches agents directly, Runner's trigger evaluation is bypassed |
| Contract complexity | Simple: orchestrator returns ONE instruction | Complex: needs `rejoin_after_custom`, chain orchestration logic |

Note: the real orchestrator already maintains Orchestration.md in real time during deviation resolution — it logs every dispatch and updates current_state properly. The single-decision model does NOT improve audit trail or resumability over the current approach. Its value is cost (bounded context) and infrastructure integration (triggers fire).

The orchestrator does lose cached attention state between invocations — a multi-step recovery plan held in KV cache is discarded. But this is minor: when the orchestrator reads the updated artifact next time, the execution log shows what it already tried, and it will near-certainly derive the same continuation. If it doesn't, it's because the intermediate result genuinely changed the picture.

The multi-step resolution model is documented as a dead end in §7.4.

### 2.7 Mode Selection

The mode is selected at run start. Both interaction surfaces — the CLI (non-interactive) and TUI (interactive) — expose the same mode selection with identical semantics. The flag/option name and the interaction with `--on-deviation` are captured in Open Items (see §7).

### 2.8 Pre-Consultation

Pre-consultation is enabled by default for Modes 2 and 3. At run start, before the dispatch loop, the Runner invokes the orchestrator agent once and receives generic environment-level strings to append to every subsequent dispatch. The default is on because a subagent in Modes 2/3 that lacks the orchestrator's environment context can fail for plumbing reasons unrelated to its task — skills not found, wrong interpreter command, harness quirks not accounted for — and those failures are difficult to attribute without the missing context. The cost of the single extra orchestrator invocation at run start is small relative to a full run with stale or absent guidance. Pre-consultation can be disabled with `--pre-consult=false`.

**Invocation context:** Pre-consultation is a distinct invocation context from routing consultation (§2.9). The orchestrator returns field-keyed strings (§2.8's response shape), not a routing instruction; the one routing action it may return is `stop` (see the stop bullet below). The contract document defines both response schemas under their respective invocation contexts.

**What it fixes:** Environmental plumbing — project-specific conventions, tool configurations, harness quirks, and other generic facts that apply identically to all subagents. Without pre-consultation, a subagent in Modes 2/3 might fail because it doesn't know a project-specific convention that the orchestrator's deployed instructions carry. Pre-consultation does NOT add routing judgment — that remains the fundamental trade-off of Modes 2/3.

**How it works:**

1. After the artifact is created or resumed and any required commit setup completes, but before the workflow dispatch loop, the Runner invokes the orchestrator via the normal `HarnessAdapter`
2. The orchestrator extracts explicit environment facts from its deployed instructions that apply to every auto-routed subagent, without rationale or duplication; it returns an empty object when none exist
3. The Runner stores these strings in session state
4. On every subsequent dispatch, the Runner appends the stored strings to the corresponding fields of the `ProtocolRequest` (e.g., appending to `task_description`, appending to `constraints`)
5. The Runner never interprets the content — it appends mechanically

**What the orchestrator produces (example):**

```yaml
task_description: |
  Skills are located at .claude/skills/ in the project root.
  When running Python, always use `py`, never `python`.
```

**Key design decisions:**

- **Uses the real orchestrator, not a dedicated advisor agent.** The orchestrator already has all the context through deployment. A separate agent would need the same context assembled and passed explicitly — duplicated context with drift risk.
- **Generic, not per-agent.** The orchestrator doesn't know agent internals. It knows environment facts (paths, command aliases, harness quirks) that apply equally to all subagents. The output is flat strings, not a per-agent map.
- **On by default, hard failure on error.** A failure to obtain a usable reply is not a decision: if pre-consultation fails (transport error, no JSON object, invalid JSON, an `action` other than `stop`, or a stop without a reason), the Runner stops before any workflow dispatch and retains the already-created or resumed artifact for inspection and resume; resuming retries pre-consultation. The coordinator call remains in the Runner diagnostic log and the MOSAIC log's orchestrator sessions but creates no Execution Log row and changes neither `global_sequence` nor `current_state`. Silent degradation — proceeding without environment guidance — is worse than stopping, whether the feature was explicitly requested or merely defaulted on. Pre-consultation can be disabled with `--pre-consult=false`.
- **A stop is a decision, not an error.** The orchestrator may answer pre-consultation with `{"action":"stop","reason":"..."}` when its deployed instructions show the run cannot work in this environment (`ScriptOrchestratorContract.md` §5.1). The run ends before any dispatch as a consultant stop: exit code 6 with the reason on stderr, not the start failure of exit code 7. The artifact is left as a pre-consultation failure leaves it: resumable, nothing written for the stop, and a resume runs pre-consultation again. Advice sent next to a stop is ignored. A stop needs a non-blank string `reason`; any other `action` value is a malformed reply and a failure as above. The TUI offers Retry only (§3.4).
- **Not cached across runs.** Project context, CLAUDE.md, and harness injections can change between runs. One extra LLM invocation at startup is cheap relative to a full run with stale guidance.

**Mode interaction:**

| Mode | Pre-consultation value |
|------|----------------------|
| 1 — Orchestrated | Unnecessary — orchestrator already includes environment context in every dispatch it states |
| 2 — Auto | Provides environment context for auto-routed dispatches; adds no routing judgment |
| 3 — Auto-review | Provides environment context for all auto-routed dispatches; adds no routing judgment |

### 2.9 Orchestrator Consultation

There is no separate "deviation resolver" concept. Consulting the orchestrator is a single operation used identically across all modes:

- **Mode 1:** Called after every step — "what's next?"
- **Modes 2/3:** Called when the engine cannot determine the next step from the workflow table — "the engine can't decide, what's next?" — and after a SUCCESS whose next step already finished (§2.10) — "the table would repeat finished work, what's next?"

The call is the same either way: invoke the orchestrator agent as a fresh session via the harness adapter, it reads Orchestration.md, and it returns one of two instructions:

- **Dispatch:** `{agent, row, iteration?, task_description, constraints?, input_artifacts?, output_artifacts?, hitl_override?}` — execute this routing-table agent at this row next, with this task description. `agent` is the bare agent id, without a mode qualifier. `row` is mandatory and names the 1-based routing-table row (the third row carrier of §2.4); `iteration` is mandatory for a block row and invalid on any other, and holds the value the Execution Log's `Iteration` column writes for that row (`"A"`, `"1.Test"`). Optional fields may override artifact and constraint defaults; `hitl_override: true` adds HITL, while `false` applies an explicit user waiver recorded in Workflow Notes. When omitted, the Runner uses table defaults and Plan resolution.
- **Stop:** `{reason}` — end the run.

**Validation and retries:** The Runner validates the dispatch target and never corrects it. `row` must exist in the table and its bare agent id must equal `agent`; for a block row `iteration` is split by the row's group and its ID must exist in the block's schedule in force at consultation time (including iterations added mid-run), a grouped row's value must carry the row's group, and the iteration's approach must include the row's group. An unknown agent is a terminal failure of its own kind. A reply that is not valid JSON or fails the row, iteration or agent-row check is retried, up to three attempts in total, before the consultation fails; a reply containing no JSON object at all, a missing required field (a reply with no `action` field at all is one; an explicitly empty `"action": ""` is an unknown action), an unknown action or agent, and a transport failure, are not retried. A failed consultation ends the run as a resumable stop (`ScriptOrchestratorContract.md` §7). The dispatch the Runner then executes carries the validated row and iteration: the recorded `WorkflowRow` and `Iteration` come from them, and nothing is inherited from the step the consultation was entered after.

This is the complete action vocabulary. The orchestrator never returns a multi-step plan, never assigns sequence numbers (that's the Runner's mechanical work), and never invokes agents itself. It names an agent, describes the task, optionally adjusts the artifact set and constraints for this specific invocation, and the Runner does the rest.

**Free table navigation:** The orchestrator can dispatch any agent in the routing table, regardless of the current position. This is normal operation, not an exception — a reviewer may find upstream problems (wrong contracts, incomplete requirements, bad plan), and the orchestrator routes back to wherever the fix belongs. The Runner looks up the named agent in the table, finds the corresponding row, and builds the `ProtocolRequest` — applying orchestrator-provided fields (task description, artifacts, constraints, HITL) where specified, falling back to the table row's defaults where not. Once that invocation's outcome is accepted, `current_state` takes the dispatched row's position, even if that means jumping backward to an earlier phase.

**Recording:** Orchestrator consultations are turns of the run's coordinator, not protocol subagent invocations. They do not consume `global_sequence`, create Execution Log rows, or update `current_state`. Each consultation call is recorded for troubleshooting in the Runner diagnostic log and, through the mosaic-logger hooks, as its own session-scoped orchestrator session (events in `00_orchestrator_events.jsonl`, transcript in a session-scoped `00_orchestrator_session__{scope}.raw`) in the run's MOSAIC log; see `Development/Designs/MosaicLogFormat.md` §4.6 (Runner-hosted mode) for what Runner mode records and the accepted differences from a native run. Conclusions and decisions a later consultation needs are appended by the script orchestrator to Workflow Notes, using the artifact's current `global_sequence` to associate the note with the most recent completed recorded invocation (`0` during run initialization).

The orchestrator never needs to know whether it's being consulted for routine routing (Mode 1) or because something went wrong (Modes 2/3). It reads the artifact, sees the current state, and decides. The distinction is the Runner's concern, not the orchestrator's.

**Stops around a consultation:** A graceful stop request (TUI) is checked at the start of every consultation, before any consultant is called; if one is pending the consultation does not start, and since it writes nothing a resume derives the same decision again. If the stop arrives while the consultation is in progress, the completed decision is not dispatched: it is discarded visibly (a warning notice, a debug log entry, and a stop message naming the agent, row and iteration) and re-derived on resume. The artifact is untouched in both cases.

**Manual routing dialogue:** The user can take the orchestrator's place for a routing decision, with the same instruction vocabulary. It is used in two ways: as the fallback when a consultation fails and the run has manual resolution enabled (`runner_manual_resolution`, the `--manual-resolution` flag), and for one decision on demand through the TUI stop screen's manual dispatch (§3.4). It is a dialogue over the interaction port, one question per step, in this order:

1. **Row** — the table rows, labelled by their 1-based number, with a mandatory stop option; the row that deviated is pre-selected (for a finished-work consultation, §2.10, the already-finished next step the engine would have dispatched). Choosing stop returns a stop instruction.
2. **Iteration** — only for a block row; the iteration of the deviation is pre-selected when the row's block schedule holds it.
3. **Task** — the task description, mandatory.
4. **Inputs** and 5. **Outputs** — multi-select lists with the row's defaults (as the engine resolves them for that row and iteration) pre-checked; the Artifacts registry supplies the candidates and free-form entries are allowed.
6. **Constraints** — optional text.
7. **HITL** — default (no override), on, or off.

Esc steps back to the previous step; Esc at the row step abandons the dialogue. Choosing another row or iteration discards the artifact selections made for the previous one. The finished instruction is checked with the same target validation as an orchestrator reply; a rejected one is never returned. The user sees the reason and is taken back to the step that can correct it. Two bounds guarantee the dialogue ends: after 3 invalid results the 4th ends it, and no more than 64 questions are asked in one attempt.

**Ending outcome:** A dialogue that produces no dispatch ends the consultation with a classified failure, and every such ending is a resumable stop (outcome `RunStoppedByConsultant`) whose stop reason names the case: the user cancelled at the row step, the interaction was unavailable, or a bound was exceeded. The interaction counts as unavailable when it errors, when the context ends, or when an answer has any status other than answered or cancelled, for example the unsupported status the non-interactive CLI returns. The CLI therefore cannot answer a manual routing dialogue: with `--manual-resolution` a failed consultation ends the run as a resumable stop instead of dispatching, and never blocks on input. Nothing is written for the abandoned decision, so a resume derives it again.

### 2.10 Routing Back: Finished Work Is Not Repeated Automatically

**The problem.** The engine is memoryless: it knows where the run is, not where it was. A run can be sent back, for example when the orchestrator sends a step's question to the planner, or a Mode 3 findings route sends `test-runner`'s failures there. The engine then used to follow the table forward from that row on every SUCCESS, rerunning work that had already finished. The contracts designer redesigned approved contracts, with a second human stop, and a stage's passed Test group ran again. Repeating finished work is not just waste. A creator without a progress artifact re-reads its inputs and rewrites reviewed output, and the result must pass its review cycles again. The orchestrator's intent ("return to the asker once the plan passes review") is lost, because it makes one decision per consultation (§2.6).

**The rule (Modes 2 and 3).** After a SUCCESS, the engine computes its next step exactly as before. Before dispatching it, the engine looks it up in the Execution Log:

- **Not yet finished:** there is no `SUCCESS` entry for that row (by recorded `WorkflowRow`) at that `Iteration` value. The engine dispatches it.
- **Already finished:** such an entry exists. The engine dispatches nothing and consults the orchestrator. The consultation carries the full `status_message` of the step that just succeeded in `last_status_message`. There is no new request field and no deviation kind on the wire.

The orchestrator's dispatch is authoritative and is not checked against this rule: only the orchestrator decides to rerun finished work. It may also follow the table's `On Success`, which reproduces what the engine would have done. That is why the judgement must be stated in the orchestrator's Routing Policy: rerun finished work only when the work since then changed what it depends on, otherwise dispatch the first work not yet finished (`ScriptOrchestratorContract.md` §6.3). An orchestrator without it may follow `On Success` and repeat the work anyway, so the engine change ships together with that policy text, not before it.

"Finished" is judged per row at an `Iteration` value. The same definition applies inside and outside blocks, and it never covers a whole group or iteration. The engine's block re-entry is unchanged: an entered block runs only its uncompleted iterations, starting from their first row. The rule checks only the single step about to be dispatched, wherever re-entry places it.

**What counts as finished.** The check is internal to the engine: a deviation of its own kind carries the already-finished step as its planned continuation, and nothing of it is serialised or sent to the orchestrator. A log entry counts only when its status is `SUCCESS`, it records a `WorkflowRow` (the `-` of legacy and infrastructure rows never matches) and its iteration value can be read for the row; an entry whose iteration value cannot be read is skipped, never counted. On resume the run's last SUCCESS is not held in memory, so the consultation it leads to carries `last_status_message: null` (`ScriptOrchestratorContract.md` §3.2). The manual routing dialogue (§2.9) pre-selects the finished step.

**Not affected:** the mechanical retries (§2.4), schedule-defect re-dispatches (§3.6), HITL compliance re-dispatches (§3.5) and Mode 3 findings routes keep their behaviour. A findings route target is usually a finished creator, and dispatching it is what the route exists for. After it succeeds, the rule applies again to whatever comes next.

**When the engine takes over again.** Nothing declares a handback. The engine takes over at the first SUCCESS whose next step is work that has not finished. That is typically the step that originally asked, or the first row of a new iteration the planner added. It holds even when the detour removed or moved the original step. A stage reworked to Tests-Only whose tests already finished is complete, and the run continues at the next stage without anyone returning to the asker.

**Why it holds for any workflow.** The rule never looks at the workflow's shape: no creator/reviewer pairs, groups or `On Findings` semantics. It depends only on "row + `Iteration` value" naming the same work for the whole run, and the formats guarantee that for every valid workflow:

- The table is pinned by `workflow_version`.
- An iteration that has started can be neither removed nor renamed. A completed one is frozen, and further work on it becomes a new iteration (`WorkflowFormat.md` §9.3, R2; a schedule that breaks this is refused, §3.6).

A workflow's shape can change how often the orchestrator is consulted, never whether finished work is repeated without it. On a run that never routes back, the rule never fires: Modes 2/3 refuse routing cycles among `On Success` (A15), so a forward path never reaches finished work.

**Why the orchestrator decides.** Whether finished work must run again is a judgement about content, for example whether a changed plan still fits the approved contracts. A native orchestrator makes the same judgement when it resumes a run from `Orchestration.md` alone. The consultation sees exactly what that orchestrator would see: the Execution Log, Workflow Notes, and the triggering message in full. The decision often rests on outcomes several rows back, so the log's `Summary` keeps the first 50 and last 100 characters of a status message (§4.4). The Runner adds no further protection here, such as a bound on consecutive consultations. Native orchestration has none, and this rule creates no risk that native orchestration does not have.

**Example (brownfield-tdd, Mode 3).** Stages 1-3 are complete and stage 4's Test group has passed. `implementation-tdd @ 4.Implementation` returns `NEEDS_CLARIFICATION` about the plan.

| # | Step | Decided by | Why |
|---|---|---|---|
| 1 | `planner-tdd-soft` revises stage 4 | Orchestrator | Not a SUCCESS |
| 2 | `plan-review` | Orchestrator | SUCCESS, but `plan-review` already finished. The plan changed, so it runs again |
| 3 | `implementation-tdd @ 4.Implementation` | Orchestrator | SUCCESS, but `contracts-designer` already finished. The contracts still fit |
| 4 | `implementation-review @ 4.Implementation`, then stage 5 onward | Engine | Work not yet finished |

The engine alone would have run `contracts-designer`, `contracts-review` and stage 4's Test group again before reaching step 3.

**Resume.** The rule reads only the Execution Log, so a resumed run makes the same decision the live loop would have. No detour state is persisted.

---

## 3. Dispatch Loop Lifecycle

The Runner has ONE dispatch loop. The mode determines who makes the routing decision at step 1 — the engine or the orchestrator — but everything else is identical.

### 3.1 The Unified Loop

```
┌─────────────────────────────────────────────────────┐
│ 1. DECIDE NEXT STEP                                 │
│                                                     │
│    Mode 1:  Always consult orchestrator              │
│    Mode 2:  Engine first; orchestrator if Deviation  │
│    Mode 3:  Engine first (incl. On Findings);        │
│             orchestrator if Deviation                │
│                                                     │
│    Result: {agent, task_description} or stop/complete│
├─────────────────────────────────────────────────────┤
│ 2. BUILD REQUEST                                    │
│    Engine resolves artifacts, assigns sequence       │
│    number. Fields come from:                         │
│    Orchestrator-routed:                              │
│      task_description, constraints from orchestrator │
│      input/output artifacts: orchestrator if         │
│        specified, else table row defaults            │
│      HITL: table/Plan, plus add or recorded waiver   │
│    Auto-routed (Modes 2/3):                          │
│      task_description: generic + pre-consultation    │
│      input/output: table defaults (+ review artifact │
│        on CNA auto-route back)                       │
│      HITL: table + Plan resolution                   │
├─────────────────────────────────────────────────────┤
│ 3. INVOKE SUBAGENT                                  │
│    Harness.Invoke(agent, request) → response        │
│    Harness error → treat as deviation, back to 1    │
├─────────────────────────────────────────────────────┤
│ 4. VERIFY HITL GATE                                 │
│    Only when this invocation was dispatched with     │
│    human_in_the_loop: true. Read human_approved      │
│    from each output the invocation wrote (§3.5)     │
│    Rejected attempt: log it without current_state     │
│    or Artifacts updates, then re-dispatch             │
├─────────────────────────────────────────────────────┤
│ 5. RECORD ACCEPTED OUTCOME                          │
│    Store.Apply(state, completedStep)                │
│    → Execution log row appended                     │
│    → current_state updated                          │
│    → global_sequence bumped                         │
├─────────────────────────────────────────────────────┤
│ 6. INFRASTRUCTURE TRIGGERS                          │
│    Evaluate checkpoint/commit/etc. triggers          │
│    May dispatch infrastructure agents (each recorded)│
├─────────────────────────────────────────────────────┤
│ 7. SCHEDULE RE-READ                                 │
│    If an output artifact is a block's declared       │
│    Source, re-read that block schedule (§3.6)        │
├─────────────────────────────────────────────────────┤
│ 8. LOOP                                             │
│    Back to step 1 with updated state                │
└─────────────────────────────────────────────────────┘
```

### 3.2 Step 1 in Detail: Routing Decision

Step 1 is the ONLY step that varies by mode. Everything else is identical.

**Mode 1 — Always orchestrator:**
The orchestrator is invoked as a fresh session. It reads Orchestration.md, decides what's next, and returns `{agent, task_description}` or `stop`. On the first iteration of a new run, the artifact has no prior subagent result — the orchestrator reads the fresh artifact and decides the first step.

**Modes 2/3 — Engine first, orchestrator on deviation:**
The engine is called with the current state and last response. Three outcomes:
- **Dispatch** → proceed to step 2 (with generic task description)
- **Complete** → end run
- **Deviation** → consult orchestrator (same call as Mode 1), proceed to step 2 with orchestrator-stated task description. This includes a SUCCESS whose next step already finished (§2.10)
- **Stop** → end run (precondition failure)

The mode only affects WHICH deviations reach the orchestrator: both modes first re-dispatch `PARTIALLY_DONE` and `BLOCKED`/`E501` within the bounds of §2.4 (a Dispatch with the same row and iteration); beyond those, Mode 2 sends all non-SUCCESS (including COMPLETED_NEEDS_ACTION); Mode 3 auto-routes COMPLETED_NEEDS_ACTION with unambiguous On Findings before producing a Deviation.

**Deviation chains resolve naturally:** If the orchestrator-dispatched agent's result is itself a deviation (Modes 2/3), step 1 simply consults the orchestrator again on the next iteration. No special chain logic. Each step is recorded, triggers fire, and the orchestrator reads the updated artifact each time.

### 3.3 Harness Errors

A harness-level error (timeout, crash, malformed output) at step 3 is fed back into step 1 as a deviation. The session constructs a synthetic response with `StatusCode=BLOCKED`, `ErrorCode=E501` (`TOOL_UNAVAILABLE`: the harness is the external tool that failed), and the error message as both status message and error reason. It records the failed attempt as a BLOCKED Execution Log row for the row and iteration that was being dispatched (so its Summary ends with `[error:E501]`, §2.4), and the next iteration's routing decision accounts for it. In Modes 2 and 3 the engine re-dispatches the same row and iteration while the E501 budget of §2.4 lasts, and consults the orchestrator once it is spent; in Mode 1 the orchestrator is consulted straight away. A harness error that is a raw-text protocol failure (output received but no valid protocol JSON extracted) gets one direct re-dispatch (the raw-text bypass) first; a failed bypass is recorded as its own BLOCKED/E501 row and counts against the same budget. With no routing consultant wired, a harness error ends the run as an unresolved deviation. The GHCP CLI adapter classifies two events of its JSON stream as harness errors, whatever else the stream holds: `subagent.deselected` (GHCP ran its default agent instead of the requested one; the error names the requested agent) and `session.error` (a failed model call such as an exhausted quota; the error carries the event's type, code, status and message, which appear nowhere on stderr). Both take the `E501` path above for a dispatch and are a transport failure for a consultation. The adapter's debug-log entry for each invocation also records the GHCP version (`ghcp_version`) read from the `pkg/<platform>/<version>` paths in the stream, because the CLI self-updates between invocations. The code matters because the shared Routing Policy picks its recovery tier by `error_code`: `E501` retries the same agent within the Repeated Failures cap and then escalates. The consultation request carries the description in `last_error_reason` (`ScriptOrchestratorContract.md` §3.2).

### 3.4 Stop Handling

The run ends resumably in several ways, and the Runner's reaction depends on the interaction surface. In every case the artifact is left in its current state and resumable.

| Ending | Outcome | Cause |
|--------|---------|-------|
| Pre-consultation stop | `RunStoppedByConsultant` | The orchestrator answered pre-consultation with a `stop` (§2.8). Exit code 6, reason on stderr; not a start failure |
| Consultant stop | `RunStoppedByConsultant` | The routing decision at step 1 is `stop` (from the orchestrator or the manual dialogue), or a consultation failed: transport, unparseable or invalid reply after the retries of §2.9, an invalid dispatch target, or a manual dialogue that ended without a dispatch (cancel, unavailable interaction, exceeded bound; §2.9) |
| Engine stop | `RunStopped` | The engine's precondition failure (for example a recorded row that no longer matches the table, §2.4) |
| Graceful stop | `RunStopped` | The user confirmed a stop in the TUI |
| Guard stop | `RunStopped` | The orchestrator kept requesting a dispatch the anti-loop guard blocks (§2.4) |

**Graceful stop** is checked at fixed checkpoints and never interrupts an invocation in flight: before an engine-routed dispatch, before a HITL re-dispatch (engine-routed and consultant-routed), before a consultant-routed dispatch, before an infrastructure dispatch, and at the start of every consultation (§2.9). A decision completed while the stop was pending is discarded and surfaced, not dispatched. Nothing is written at a checkpoint, so a resume re-derives what was about to happen.

**CLI:** Terminal. The Runner prints the stop reason, exits with a non-zero code. The artifact is left in its current state — resumable if the underlying issue is fixed.

**TUI:** After a consultant stop the Runner presents the stop reason and offers two actions:

| Action | What it does |
|--------|-------------|
| **Retry** | Resumes the run, so the engine or orchestrator is asked for the same decision again. Useful when the user fixed something externally (environment config, missing dependency, network issue) and wants the orchestrator to reconsider. |
| **Manual dispatch** | Resumes the run with the user as resolver for this one decision: the manual routing dialogue of §2.9 runs in place of the first consultation. The run then continues normally (the next routing decision goes back to the configured resolver). Triggered on-demand rather than configured at run start. |

After a pre-consultation stop the screen is worded for pre-consultation and offers Retry only: manual dispatch replaces a routing decision and a pre-consultation stop has none. Retry resumes the run, which runs pre-consultation again.

Esc on the stop screen quits. After a graceful stop or an engine stop the done screen offers Continue. Retry, Manual dispatch, Continue and the executable-override retry all use one restart path: it disarms the stop, resumes the existing run (a run whose artifact exists is never started as new, so the seed inputs are not applied again; a run that stopped before its artifact was written starts as new), rebuilds the session when needed, and keeps the progress screen, replacing its rows with the run history rebuilt from the Execution Log (agent, phase, iteration and status of every row, in log order). A resumed run therefore shows what already ran rather than an empty list.

The stop action on the wire (`ScriptOrchestratorContract.md` §4.2) is the same regardless of surface — the Runner's reaction to it is surface-specific behavior.

### 3.5 HITL Gate Verification (Step 4)

`CommunicationProtocol.md` §9.7 makes verifying the human-in-the-loop gate an obligation of the party that dispatched the invocation. In Runner mode that party is **the Runner**, not the orchestrator agent: the orchestrator returns a routing instruction and cannot re-dispatch, so leaving the check to it would leave it undone. The check's semantics and the response to a gate that was not discharged are the protocol's — this section records only that the Runner owns them here, so the obligation is not read as unassigned.

What the Runner does:

- **Reads `human_approved`** from the frontmatter of each output artifact that an invocation dispatched with `human_in_the_loop: true` created or modified, whatever status it returned except `BLOCKED` with `E503`, which is routed directly because it already reports that the gate could not run — the same set recorded in the Artifacts registry, with wildcards expanded. A listed output that does not exist or that the invocation left unchanged is not checked. The read distinguishes the failure conditions — unreadable, no frontmatter, malformed value, field absent — rather than collapsing them into "not approved", and it never errors the run itself.
- **Refuses to start** a run whose workflow requires HITL when the interaction surface cannot read approvals at all. A surface that cannot verify the gate cannot honour it, and discovering that mid-run would leave a run that believed it was gated and was not.
- **Verifies before accepting the workflow outcome.** A discharged gate proceeds to the ordinary record step, which updates `current_state` and the Artifacts registry. An undischarged attempt is appended to the Execution Log and advances sequence/time, but leaves both structures unchanged before the protocol-required re-dispatch. Infrastructure triggers do not run for that rejected attempt.
- **Routes on the original outcome after a repaired gate.** When the re-dispatch discharges the gate and returns `SUCCESS`, the Runner routes on the original invocation's status and error code and records them as `current_state.last_status` / `error_code`, with `last_agent` naming the re-dispatch (protocol "Routing after the re-dispatch"). Any other re-dispatch status is routed as returned.

This is the only place the Runner inspects artifact content beyond the orchestration artifact, and it reads frontmatter only — the same narrowing the protocol applies to an LLM orchestrator, for the same context-discipline reason.

### 3.6 Defective Produced Block Schedules

A block's schedule is read from its declared Source and re-read after a row whose Output produces that Source. When such a produced schedule is defective (the Source is missing or unreadable, or the table breaks a block-schedule rule V1-V6 of `WorkflowFormat.md`, for example a malformed or duplicate ID or an unsatisfiable `Depends On`), the Runner never uses it, and never falls back to the previous schedule.

**The defect is the producer's.** The producer is the row whose completion triggered the re-read: a producing row outside the block (a planner before it) or an in-block writer row (a row of the block that writes its schedule while it runs). Both are taken from the per-block admitted model, never from the artifact overrides of a dispatch; a completed row the model does not list for the block triggers no re-read and no re-dispatch. The Runner re-dispatches exactly that row, at the iteration value at which it just ran (none for a row outside every block; the current iteration and group for an in-block writer), with the row's declared inputs and outputs and the non-compliant Source among its input artifacts. The task description states that the block schedule is non-compliant and names the rule ID (or that the Source is missing or unreadable), the block, the Source and Section and the offending row, ID or token, and asks for the schedule to be corrected in place. While an in-block writer is re-dispatched no other row of the block runs. A block classified seeded is handled the same way after a re-read that follows one of its in-block writers; the refusal of a defective seeded schedule applies only to the read at run start and at resume. A corrected schedule on re-dispatch lets the block run.

**Every mode, and Mode 1 in particular.** The re-dispatch happens in Modes 1, 2 and 3 and ends only the producing row's own attempts. In Mode 1 it is a deliberate exception to orchestrator-driven dispatch: the Runner does not consult the script orchestrator for it, because an orchestrator handed a broken schedule could name a next agent while the Runner still has no usable schedule. In every mode no block row is dispatched against the defective schedule: while the block is defective its consultation requests, the manual dialogue and the validation of a reply hold no schedule for it, so a reply that dispatches a row of the block is an invalid dispatch target. Only the Runner's own re-dispatch of an in-block writer runs on the block's last accepted schedule, to resolve that row's defaults and paths.

**Limit and counting.** At most 2 re-dispatches follow the first attempt: 3 producer attempts in total, matching the 3-attempt limits of §2.4. The count is derived from the Execution Log alone, so it survives a restart: the attempts are the `SUCCESS` entries of the producing row (by recorded `WorkflowRow`) at the same recorded iteration value in the trailing run of the log, that is the longest suffix without an entry of any other routing-table row; infrastructure-agent entries are skipped and do not end the run, a `PARTIALLY_DONE` or other non-`SUCCESS` entry of the row is not an attempt, and a block row's entry ends the run, so a later defect of the same producer starts a fresh count. Re-dispatches used = attempts - 1 (`engine.ScheduleDefectAttempts`, `engine.ScheduleDefectRedispatchLimit`). The limits of §2.4 cannot be reused: they count `PARTIALLY_DONE` and `BLOCKED`/`E501` entries, not a producer that returns `SUCCESS` with a defective schedule. The re-dispatch is marked with its own mechanical retry kind (`schedule-defect`), so the anti-loop guard (§2.4) neither counts nor blocks it; with 3 attempts the bound is below the guard's cap in any case.

**HITL producer rows.** The human review happens inside the subagent's session, so the Runner's HITL compliance loop (§3.5) is ordered before the schedule judgement. The schedule is judged only for an attempt the loop accepted: a defect that a re-read before the HITL gate sees only feeds written-output detection and the approval check, and when the loop re-dispatches a non-compliant attempt the accepted re-dispatch's schedule is the one judged. When the loop escalates or ends the run no schedule judgement follows. A schedule-defect re-dispatch is an ordinary dispatch with the row's effective HITL, so its result passes the HITL compliance loop again (including that loop's own single re-dispatch). A HITL-rejected attempt is logged as an entry of the producing row at the same iteration value with the status the subagent returned, so it counts as an attempt: a HITL producer row gets at least 1 and at most 2 schedule-defect re-dispatches before the consultation.

**Consultation after the limit.** When the schedule read after the latest attempt is still defective and the limit is used up, the session enters an ordinary routing consultation itself (the engine would otherwise route onwards after a `SUCCESS`), with a deviation of kind `schedule-defect` (not on the consultation wire). `last_error_reason` carries the Runner-constructed defect description (the same content as the fix message) and `last_status_message` the producer's verbatim status message (`ScriptOrchestratorContract.md` §3.2). The orchestrator then replies as usual: a dispatch of the producer, or a stop. With no routing consultant wired (and no manual dispatch pending) the run ends `RunDeviationUnresolved` with a message naming the kind `schedule-defect` and the defect; the re-dispatches before the limit need no consultant. Blocks of a legacy table keep the warning and the previous schedule, and no re-dispatch, limit or consultation applies to them.

**Resume.** The attempt count is read from the log, so a resumed run continues where it stopped. At resume a defective read of a produced block whose last routed log entry is a `SUCCESS` of one of its producing rows is acted on as a judged attempt of that row (re-dispatch while re-dispatches remain, otherwise the consultation). A resume whose log holds no routed entry yet treats a defective stale Source as no schedule yet, since the producer runs first. A defective produced schedule in any other log state, and a defective seeded schedule, refuses the resume.

**Only `SUCCESS` is judged.** A producer completion with another accepted status (for example `COMPLETED_NEEDS_ACTION`) is not judged: a defective read keeps today's warning and the previous schedule. A schedule-defect re-dispatch of an in-block writer that returns `PARTIALLY_DONE` or `BLOCKED`/`E501` (or fails at the harness) is retried by the ordinary mechanical retry of §2.4 over the block's last accepted schedule, which the Runner hands to the engine for that re-dispatch only (`NextInput.RetrySchedules`); routing and consultation still see no schedule for the block. Those retries carry the PARTIALLY_DONE or tool-unavailable text, not the fix message. The re-dispatch's eventual `SUCCESS` is judged as any other.

---

## 4. Run-Start Sequence

The run-start sequence validates configuration and compatibility preconditions before creating the artifact. When commits are enabled, commit setup is the first invocation after creation so its outcome is recorded like every other invocation. A setup failure stops the run but leaves an inspectable artifact rather than erasing the failure from the shared run record.

| Step | What | Failure → |
|------|------|-----------|
| 1 | Load orchestrator file, extract selected workflow region | Refusal |
| 2 | Parse routing table from workflow region | Refusal |
| 3 | Read existing artifact (if resuming) or verify none exists (if new) | Refusal |
| 4 | Admit workflow for the selected mode (compat checks, block and group resolution; engine-routing shape checks in Modes 2/3 only — §1.3) | Refusal |
| **4b** | **Recovery check: scan for orphaned `.agents-backup/` directory next to the agents directory; restore originals if found and no active runs are detected** | **Refusal if restore fails** |
| 5 | Resolve every agent identifier to a definition file | Refusal |
| **5b** | **Create snapshot (copy-and-invoke) or backup and transform originals (backup-and-transform), per harness strategy** | **Refusal** |
| 6 | Read each block's schedule from its declared `Source` and `Section` (seeds applied first on a new run) | Refusal when a seeded block's schedule is missing or defective; a produced block whose `Source` does not exist yet is normal |
| 6b | Enumerate declared infrastructure agents | Refusal |
| 6c | Validate per-class agent selections (gated classes: one active per class) | Refusal |
| 7 | Settle run configuration (checkpoints, commits, commit variant) | Refusal if precondition fails |
| 7a | Build and validate seed plan (new runs only) | Refusal |
| 8 | Create (new) or resume (existing) artifact | Failure |
| 8a | Commit setup dispatch when commits are enabled and `commit_branch` is absent | Stop with the attempt recorded if setup fails |
| 8b | Pre-consultation in Modes 2/3 when enabled | Failure: stop before workflow dispatch; retain the artifact for inspection and resume. An orchestrator `stop` is a consultant stop (§2.8) |

### 4.1 Run Configuration (Step 7)

The Runner collects three configuration decisions that mirror what the human-driven orchestrator asks the user at run start. Both interaction surfaces (CLI and TUI) must support all three; the CLI uses flags, the TUI uses interactive prompts.

**Checkpoints** (`enabled` / `disabled`):
- Asked unconditionally — even when no checkpoint-class agent is declared. A user who wanted rollback capability and is told this deployment cannot provide it has learned something useful while they can still choose a different deployment.
- Precondition: `checkpoints: enabled` requires at least one `Class = checkpoint` agent in the infrastructure declaration region. `Class = commit` and `Class = restore` do not satisfy this — a commit agent writes to the user's history (not restorable), and a restore agent only consumes checkpoints (doesn't create them).

**Commits** (`enabled` / `disabled`):
- Asked only when a `Class = commit` agent is declared in the infrastructure region. If no commit-class agent exists, commits default to `disabled` silently — the capability doesn't exist in this deployment, so there is no choice to make.
- Precondition: `commits: enabled` requires at least one `Class = commit` agent.

**Commit branch variant** (when commits enabled):

| Variant | Where stage commits go |
|---------|----------------------|
| **MOSAIC-owned** (recommended) | A branch created for this run (`mosaic/run/{run_id}`), merged by the user when satisfied |
| **User's own** | The branch the user is already on |

MOSAIC-owned is recommended because an abandoned stage on a run-owned branch can be discarded cleanly, while on the user's own branch the failed attempt and its undo both stay in history permanently.

**Review loop limit** (positive integer or no limit):
- Asked at run start in both surfaces, suggesting `3`, and recorded as the set-once `review_loop_limit` frontmatter field; no limit omits the field. Resume reads it and never asks again. Both orchestrators' shared Routing Policy and the Mode 3 engine escalate instead of routing a reviewer's `COMPLETED_NEEDS_ACTION` back once that reviewer has hit the limit. Iterations are counted per reviewer, phase and block iteration since the reviewer's last `SUCCESS` there (§2.4).

**Gated infrastructure selections** (`checkpoint`, `commit`, `restore`):
- When a gated class has more than one declared agent, the user selects exactly one at run start. A class with exactly one declaration is auto-selected and needs no stored entry.
- The Runner records each required class-to-agent choice in the artifact's set-once `infrastructure_selections` map. On resume it reads and validates that map instead of asking again; an absent, unavailable, or reclassified required selection refuses the resume rather than choosing by declaration order.
- Non-gated classes such as `review` are not selected; all their declarations remain active.

**Runner-owned settings** use optional `runner_*` frontmatter fields in the shared orchestration artifact (executor-owned fields, `OrchestrationArtifactFormat.md` §4), all set once on first Runner use: `runner_mode` (`orchestrated`, `auto` or `auto-review`), `runner_pre_consultation` (`enabled` or `disabled`; whether each session reconstructs environment advice before auto-routing, §2.8), and `runner_manual_resolution` (`enabled` or `disabled`; whether the user is the fallback deviation resolver, §2.9). A Runner-created run writes them at creation. On first Runner resume of a native-created artifact that omits them, the user supplies the settings and the Runner records them once; later Runner resumes reuse them. Native orchestration preserves these fields but does not act on them. `commit_branch_variant` is not stored: the artifact schema defines the variant from `commit_branch`, so a second field would be free to disagree with the destination it describes.

### 4.2 Commit Setup Dispatch (Step 8a)

When `commits: enabled` and the artifact has no `commit_branch`, the Runner dispatches the commit-class agent as an out-of-band invocation after creating or loading the artifact. This setup dispatch establishes the target branch — for MOSAIC-owned, the agent creates it; for user's-own, the agent reports the current HEAD branch. The same condition applies on resume, so an artifact retained after failed setup retries setup before any workflow agent can run.

The setup dispatch returns the branch name in a `[branch:{name}]` marker at the end of its `status_message`. The Runner extracts this and records it as `commit_branch` in the artifact frontmatter. If the marker is missing or the dispatch fails, the run stops before its first workflow invocation and retains the setup attempt in the artifact — proceeding without a known branch destination would risk committing to the wrong place.

The setup dispatch is an ordinary invocation: it receives the next sequence number, returns a standard protocol response, and gets an Execution Log row whether it succeeds or fails. On a new run it is sequence 1. Append the row before writing `commit_branch`, following the artifact's normal log-first ordering. A successful response with a readable branch marker sets `commit_branch` and permits the workflow loop to begin. A non-success response or missing marker leaves `commit_branch` absent, keeps the setup row and advanced sequence in the artifact, and stops the run with the setup reason. `current_state` remains unchanged because setup is an out-of-band non-workflow invocation.

This ordering matches native orchestration and preserves one audit rule across executors: once a protocol invocation has occurred, its outcome exists in `Orchestration.md`. Configuration or compatibility refusal still happens before artifact creation; commit setup is not a precondition check but an agent invocation whose failure belongs in the run record.

Setup is dispatched by explicit instruction, not by a trigger, so the `ITERATION_END`-only restriction on the commit class does not apply to it.

### 4.3 Resume

On resume, the session first requires a valid artifact `run_id` matching the enclosing `Orchestration-{run_id}/` folder. An absent, empty, malformed, or mismatched value refuses the resume; the Runner never mints identity for an existing artifact. It then computes a `ResumePoint` from the execution log. If the trailing workflow row does not match `current_state.last_agent`, the row is an unaccepted or interrupted attempt: the session preserves the prior accepted `current_state` and re-dispatches that workflow assignment rather than routing on its recorded status. Run configuration — including checkpoints, commits, commit branch, and any gated `infrastructure_selections` — is read from the existing artifact's frontmatter; it was set at original run start and is never modified.

**Row and iteration on resume:** The row and iteration of the last workflow step are derived by the same routine that live routing uses (§2.4), from the `WorkflowRow` and `Iteration` the log recorded for that step. Resume therefore cannot place the run differently from the live loop did. A recorded row that no longer exists in the table, holds another agent, or records an `Iteration` the row does not accept stops the resume with a message naming the row, and so does a block row whose log entry records no iteration; the Runner never guesses a row. An entry that records no row (infrastructure and ad-hoc steps) is resolved by agent and phase only where that is unambiguous. The position after a completed row is the one the table's declared routing gives (`On Success` with its scopes, block entry and exit, iteration order from the block schedule and the log), computed by the same routing as live dispatch. A run artifact in the pre-3.0 layout (an Execution Log `Stage` column, a `current_state.stage` key, or a grouped value in the `{Group}.{ID}` form) is refused before any dispatch.

**Counters on resume:** The PARTIALLY_DONE bound, the E501 budget and the review loop count are all read from the Execution Log, so they carry across a resume. The anti-loop guard is in memory and starts again from zero.

**Restart from the TUI:** The TUI's continue, retry and manual-dispatch paths resume the existing run in the same process by the mechanism described in §3.4; the progress screen is seeded with the run history from the Execution Log.

### 4.4 Artifact Table Layout and Path Form

The Runner renders `Orchestration.md` itself, so its tables follow one layout:

- **Regions.** The body holds three tag-delimited regions, `ExecutionLog`, `Artifacts` and `WorkflowNotes`, each with a blank line between its open tag, its table and its close tag so the table renders as a table in CommonMark viewers.
- **Columns.** Execution Log: `Seq`, `Agent`, `Phase`, `Iteration`, `WorkflowRow`, `Status`, `Timestamp`, `Summary`, `Inputs`, `Checkpoint` (OrchestrationArtifactFormat 3.0; `Iteration` holds `{ID}` or `{ID}.{Group}`, for example `1.Test`). Artifacts: `Artifact`, `Created In`, `Created By`. Workflow Notes: `Seq`, `Note`. An empty Iteration, WorkflowRow, Inputs or Checkpoint is written as `-`, and `-` is read back as empty.
- **Compact tables.** Tables are written without column padding, so line length follows row content.
- **Sanitising.** Every cell is made safe before it is written: each line break becomes a space, surrounding whitespace is trimmed and each `|` is escaped as `\|`; reading a cell restores the pipe. A cell can therefore never break its table. A status message goes through its own step before that: line breaks and `|` are removed, and a message longer than 150 characters is shortened to its first 50 and last 100 characters, joined by ` ... ` (OrchestrationArtifactFormat 3.1). The limits count characters, not bytes, so a cut never splits a multi-byte character. A BLOCKED row's Summary then ends with the error marker of §2.4.
- **Path form.** Artifact paths in the `Inputs` column and the Artifacts registry are recorded run-relative with forward slashes, without the `Orchestration-{run_id}/` prefix (for example `Stage-2/Plan.md`); `run_id` is stated once, in the folder name. A project file outside the run folder is recorded unchanged. The prefixed form exists only in the protocol request, where the subagent needs a fully-qualified path; the Runner adds the prefix when it builds the request, and prefixed and unprefixed forms of one path denote the same artifact (a duplicate input is dropped, a registry entry is replaced in place).

### 4.5 Text Entry in the TUI

Single-line text fields in the TUI are paste-safe: a paste can never submit the field. The behavior lives in the shared `Tools/Common/tui/pastesafe` component, and `widgets.TextInput` delegates to it. The component's package comment records the decision; in short:

- **Why a heuristic.** On Windows the terminal layer delivers a paste as individual key events without the paste flag, so a pasted line break arrives as an Enter key that cannot be told apart from a typed one. A strict mechanism is therefore not available and the component uses timing.
- **Detection rule.** A key carrying the paste flag is always pasted text. An unmarked Enter that arrives within the paste window (80 ms, inclusive) after the previous key event seen by the same field is pasted: it becomes a space and the field stays open, and the user presses Enter again to submit. An Enter after the window submits. A multi-rune key message containing CR or LF is pasted text as well.
- **Line break rule.** One line break is one space whatever its encoding: CR followed by LF yields one space, a lone CR or lone LF yields one space, and Ctrl+J never submits.
- **Accepted edge cases.** An Enter right after the value is set programmatically, and a paste whose very first event is a line break, have no preceding key in the window and submit.
- **Test clock.** The field never reads wall time itself; it reads a clock once per key message and decides inside the update, with no timers. Tests replace the process-wide clock or give a field its own clock.

---

## 5. Infrastructure Agent Integration

Infrastructure agents are declared outside workflow tables and reached either by automatic trigger evaluation or explicit dispatch. Checkpoint, commit, and review agents may fire automatically according to their declarations; restore agents are manual and never participate in automatic trigger evaluation. All are declared in the orchestrator file's infrastructure agent region.

**No-cascades rule:** Infrastructure agent completions do not trigger further infrastructure evaluations. Only workflow step completions trigger the evaluation pass.

**Trigger types:**

| Trigger | Fires When |
|---------|-----------|
| `INVOCATION_INTERVAL` | Updated `global_sequence` minus this agent's most recent Execution Log `Seq` is at least N; when it has no prior row, `global_sequence` is at least N. Counts globally allocated invocations, not only workflow steps |
| `ITERATION_END` | The completed step is the last step of an iteration of its block, whatever the block is named, and returned `SUCCESS`, accepted by HITL verification where it applies. Any other status, including a reviewer's `COMPLETED_NEEDS_ACTION` loop-back, does not fire. This is the only trigger a commit-class agent may name in an `infrastructure_overrides` entry |
| `PHASE_END` | The completed step is the last step of its phase and returned `SUCCESS`, accepted by HITL verification where it applies; for a phase holding a block this means the last step of the block's last iteration (phase-wide scope, not per iteration) |
| `MANUAL` | Never fires automatically; dispatched by explicit instruction only |

`ITERATION_END` was named `STAGE_END` before orchestrator-script v3.2.0. The old name has no alias: it is outside the vocabulary and refused like any other unknown trigger.

**Unknown triggers refuse the run.** A trigger outside the four names above (matched exactly after trimming surrounding whitespace, case-sensitive; a blank trigger included) refuses start and resume before any dispatch. The check covers every row of an agent's table in the orchestrator's declaration region, whatever its class, filter or selection, and every trigger of an `infrastructure_overrides` entry. Overrides are validated before the commit setup dispatch, so a refused override leaves the run untouched. For `STAGE_END` the refusal says it is now `ITERATION_END`: redeploy the orchestrator in the declaration case, make the override name `ITERATION_END` in the override case.

**What a trigger dispatch carries.** A review-class dispatch, on any trigger, carries the run-scoped `Orchestration-{run_id}/Orchestration.md` as its only input artifact; it is sent without a file-existence check because the Runner writes that file before evaluating triggers, and its task description names all triggers that matched. A commit-class dispatch fired by `ITERATION_END` carries the ended iteration's own artifacts: every `Input` and `Output` cell entry of the block's rows that fills in the block ID, resolved to the iteration, with a `*` wildcard in such an entry expanded against the run folder, kept only where the file exists, de-duplicated, in table order and run-scoped on the wire. File existence on disk stands in for the Artifacts registry the native orchestrator consults. Its task description names the block and iteration (for example `Stage 2`). Checkpoint dispatches and commit dispatches on other triggers carry no input artifacts and its task description names all triggers that matched. All trigger dispatches have empty output artifacts and `human_in_the_loop: false`. The commit setup dispatch (§4.2) is unchanged. If the iteration or its artifacts cannot be determined the dispatch is built as for any other trigger and does not fail.

**Infrastructure log rows.** The row of a trigger dispatch records its input artifacts in `Inputs`. A BLOCKED response records its error code in the row's Summary marker as on workflow rows; a harness-level failure or an agent definition that cannot be resolved is recorded as BLOCKED with `E501`.

**Sequential running.** The Runner runs every run sequentially. New runs record `dispatch: sequential` in the artifact frontmatter after `global_sequence`. Resume refuses a present `dispatch` value other than exactly `sequential` (a blank value or a case variant included) with a message naming `dispatch` and saying the Runner runs sequentially. A missing field is read as `sequential` and is not written back; a present value survives artifact rewrites. The Runner requires `orchestrator-script` v3.3.0 (§2.10).

**Recording:** Infrastructure steps are recorded in the execution log like any other step, but they do not update `current_state`. The artifact's recorded workflow position always names the last workflow step, ensuring the engine's row-lookup stays correct.

---

## 6. Runner Agent Snapshot

Regular orchestration and Runner execution have different deployment requirements. In regular orchestration, the orchestrator agent (`mode: primary`) spawns subagents via the harness's task/subagent tool, and the subagent's `mode: subagent` frontmatter enforces isolation. The Runner invokes every agent via the harness CLI (`opencode run --agent <id>`, `claude --agent <path>`, etc.), and some harnesses block CLI invocation of agents marked `mode: subagent`. OpenCode is the first harness with this constraint, but others may follow.

### 6.1 The Problem

The deployed agents directory (e.g., `.opencode/agents/`) serves both interactive orchestration and the Runner. These two execution models have conflicting requirements for the same agent files. The Runner needs transformed copies (e.g., `mode: primary` instead of `mode: subagent`), but the regular agents must remain unchanged for interactive orchestration.

Harnesses differ fundamentally in how they load agent definitions:

| Harness | Agent loading | Mechanism |
|---------|-------------|-----------|
| Claude Code | **Path-based** | `--append-system-prompt-file <path>` — loads the file at the given path directly |
| OpenCode | **Name-based** | `--agent <name>` — resolves the name from its canonical agents directory |
| GitHub Copilot CLI | **Name-based** | `--agent <name>` — resolves the name from its scoped agent directories |

Path-based harnesses can be pointed at an arbitrary file — a snapshot copy works naturally. Name-based harnesses always read from their canonical directory regardless of what path the Runner has resolved internally. A snapshot sitting in a sibling directory is invisible to them.

### 6.2 Dual-Strategy Snapshot

The Runner selects a snapshot strategy per harness based on its agent loading mechanism:

| Strategy | How it works | Used by |
|----------|-------------|---------|
| **Copy-and-invoke** | Copy agents to a run-scoped snapshot dir, apply transforms, invoke from snapshot path. Originals untouched. | Path-based harnesses (Claude Code) |
| **Backup-and-transform** | Copy originals to a shared backup dir, transform originals in-place, restore from backup on completion. | Name-based harnesses (OpenCode, GHCP CLI) |

Both strategies create sibling directories next to the regular agents directory:

| Strategy | Harness | Regular directory | Sibling directory | Role |
|----------|---------|------------------|-------------------|------|
| Copy-and-invoke | Claude Code | `.claude/agents/` | `.claude/agents-runner-{run_id}/` | Working snapshot (invoked from here); one per run |
| Backup-and-transform | OpenCode | `.opencode/agents/` | `.opencode/.agents-backup/` | Shared safety backup (restore source); one per workspace |
| Backup-and-transform | GHCP CLI | `.github/agents/` | `.github/.agents-backup/` | Shared safety backup (restore source); one per workspace |

Copy-and-invoke creates a **per-run** directory (scoped by run ID) because each run needs its own independent copy. Backup-and-transform creates a **shared** backup directory because all concurrent runs share the same transformed originals and need the same original-state backup to restore from.

#### 6.2.1 Copy-and-Invoke (Path-Based Harnesses)

This is the current mechanism, unchanged:

1. Copy agents directory to `agents-runner-{run_id}/`
2. Apply harness-specific transformations to the copies
3. Re-resolve all agent paths to point into the snapshot
4. Invoke agents using the snapshot paths
5. On completion: delete snapshot directory

**Crash safety:** Inherent. Originals are never modified. An orphaned snapshot directory is harmless.

**Concurrency:** Naturally concurrent — each run has its own snapshot directory. No coordination needed.

#### 6.2.2 Backup-and-Transform (Name-Based Harnesses)

For harnesses that resolve agents by name from a canonical directory, the Runner must transform the originals in-place so the harness naturally reads the correct content. Multiple concurrent runs share one backup and coordinate via per-run lock files (§6.3).

**First run to start (backup directory does not exist):**

1. **Recovery check:** Scan for orphaned backup state from prior crashes (§6.3.2). Restore if found.
2. **Create backup directory:** Attempt `os.Mkdir(".agents-backup", 0755)`. This is atomic on both Windows and Linux — exactly one caller succeeds when multiple runs race. If the call fails with "already exists," this run is not first — fall through to the concurrent join protocol below.

   **Creator failure handling:** If lock creation (step 3) fails because the directory was concurrently deleted (by a recovery check that saw the partial backup), the creator must not proceed with copying or transforming — it retries from step 2 (re-attempt `os.Mkdir`). Similarly, if copy (step 4) or manifest write (step 5) fails because the directory vanished mid-operation, the creator must not transform. The invariant is: a creator never transforms unless its own lock is held AND the manifest write succeeded.
3. **Acquire lock:** Create `.lock-{run_id}` inside the backup directory and hold an exclusive OS-level file lock on it for the duration of the run. This must happen **before** copying files, so that a concurrent recovery check or joiner sees "lock held" and does not treat the in-progress backup as orphaned.
4. **Copy originals:** Copy all agent files into the backup directory.
5. **Write manifest:** Write `recovery-manifest.json` into the backup directory. The manifest lists the files that will be transformed, computed in memory by a dry-run of the transformation rules (no disk writes yet). The manifest's presence is the completion signal for backup creation — a concurrent joiner that sees the backup directory but no manifest knows the creator is still copying and must wait.
6. **Drop recovery marker:** Write a human-readable recovery file into the agents directory itself (§6.3.3).
7. **Transform in-place:** Apply harness-specific transformations to the original agent files.
8. **Write setup-complete signal:** Write a `.setup-complete` file into the backup directory after all transforms have been applied. A concurrent joiner that sees the manifest but no `.setup-complete` file knows the creator is still transforming and must wait for it before proceeding to run. This prevents a joiner from running against untransformed agents between manifest write and transform completion.
9. **Run.**

**Subsequent concurrent runs (backup directory already exists):**

1. **Wait for manifest:** If `recovery-manifest.json` does not yet exist in the backup directory, the creator is still copying. Poll with a bounded timeout (refuse the run if exceeded) until the manifest appears. During each poll iteration, also check: (a) if the backup directory has disappeared, start fresh as a first run (go to step 2 of the first-run protocol above); (b) if `.restoring` becomes held, enter the `.restoring` re-poll loop described in step 3 below instead of continuing to wait for the manifest.
2. **Wait for setup-complete:** If `.setup-complete` does not yet exist in the backup directory, the creator has finished copying but is still transforming the originals. Poll with a bounded timeout (refuse the run if exceeded) until `.setup-complete` appears. This prevents a joiner from running against untransformed agents. During each poll iteration, also check: (a) if the backup directory has disappeared, start fresh as a first run; (b) if `.restoring` becomes held, enter the `.restoring` re-poll loop described in step 3 below instead of continuing to wait for setup-complete. This prevents a joiner from timing out and refusing the run when a last-out teardown deletes the manifest or backup directory mid-wait.
3. **Check for active restore:** Attempt to acquire an exclusive lock on `.restoring` inside the backup directory. If the lock is held, a restore or exit serialization is in progress — poll in a loop until either: (i) `.restoring` becomes acquirable (acquire it briefly, then re-check backup directory existence: if the backup directory is gone, release `.restoring` and start fresh as a first run; if the backup directory still exists with its manifest, release `.restoring` and proceed to step 4), or (ii) the backup directory disappears (meaning restore completed and the directory was deleted — start fresh as a first run). If the lock is acquirable on the first attempt (or the file does not exist), acquire it briefly, re-check backup directory existence, release it, and proceed.
4. **Acquire lock:** Create `.lock-{run_id}` inside the backup directory and hold an exclusive lock on it.
5. **Post-lock re-verification:** After acquiring the lock, re-check that `.restoring` is not held and that the backup directory and manifest still exist. This closes the check-then-act window between step 3 and step 4: a last-out run could have acquired `.restoring` and begun restoring in that interval. If re-verification fails (`.restoring` is now held, or the backup directory/manifest no longer exists), release and delete the lock file, wait for the backup directory to disappear (bounded timeout), then start fresh as a first run (go to step 2 of the first-run protocol above).
6. No backup or transform needed — originals are already transformed (transforms are idempotent) and the backup already holds the true originals.
7. **Run.**

**On clean completion (any run):**

1. **Acquire restore sentinel:** Create `.restoring` inside the backup directory and acquire an exclusive **blocking Lock** (not TryLock) on it. This serializes all exits: when multiple runs exit simultaneously, they queue on `.restoring` — the second exit blocks until the first completes. This prevents new runs from joining mid-restore (they will see the held `.restoring` lock and wait). This must happen **before** releasing the run's own lock, so that no window exists where a joiner sees "no `.restoring` held, backup exists" and joins between this run's lock release and `.restoring` acquisition.
2. Release the lock on `.lock-{run_id}` and delete the lock file.
3. **Last-out check:** List remaining `.lock-*` files in the backup directory. For each, attempt to acquire an exclusive lock:
   - Lock acquired → owner crashed → delete the lock file.
   - Lock held → another run is still active → **release `.restoring`** (do NOT delete it), leave everything in place, done. Deleting `.restoring` while another run is blocked on it breaks mutual exclusion: on Unix the waiter holds a lock on the unlinked inode while the next arrival creates a fresh file (two holders simultaneously); on Windows the delete fails because the waiter has the file open.
4. If no lock files remain → this is the last active run → restore originals from backup, remove recovery marker, then perform **Windows-safe backup directory deletion**: (a) delete `recovery-manifest.json` while `.restoring` is still held (a joiner's post-lock re-verification checks manifest existence and will detect its absence), (b) delete `.setup-complete`, (c) close and release the `.restoring` file handle, (d) call `os.RemoveAll` on the backup directory. This ordering is necessary because Go opens files on Windows without `FILE_SHARE_DELETE`, so `os.RemoveAll` would fail if `.restoring` were still held. On Linux the ordering is harmless.

### 6.3 Concurrency and Crash Recovery

The backup-and-transform strategy modifies user files in-place. Multiple concurrent runs share the same transformed originals and the same backup. Coordination uses **per-run lock files** inside the backup directory — OS-level file locks (`flock` on Linux, `LockFileEx` on Windows) that are released automatically when a process exits or crashes. No heartbeats, no PID checks, no staleness thresholds.

#### 6.3.1 Race Resolution

Two concurrency races require explicit handling:

**Race (a): Simultaneous first-run backup creation.** Multiple runs start when no backup directory exists. Each attempts `os.Mkdir(".agents-backup", 0755)` — this is atomic on both Windows and Linux: exactly one succeeds, others receive "already exists." The winner creates the backup; losers fall through to the concurrent-join protocol (§6.2.2, "Subsequent concurrent runs"). The winner acquires its `.lock-{run_id}` immediately after `os.Mkdir` and before copying files, so that any concurrent recovery check or joiner sees "lock held" rather than treating the in-progress backup as orphaned. The manifest (`recovery-manifest.json`) is written **last** and serves as the completion signal: a joiner that sees the backup directory but no manifest knows the creator is still copying and polls until the manifest appears before joining.

**Race (a) — partial backup from crash:** If the creator crashes after `os.Mkdir` but before writing the manifest, the backup directory contains files (or is empty) with no manifest and no held lock (the OS released it on crash). This is a partial backup; originals are still untouched because transforms have not been applied (transforms follow the manifest write). The recovery check (§6.3.2) detects this state — backup exists, no manifest, no held locks — and safely deletes the partial backup directory.

**Race (a) — creator failure before lock acquisition:** If N runs start simultaneously and the `os.Mkdir` winner crashes or has the directory removed between `os.Mkdir` and lock acquisition, a recoverer may delete the partial backup. A retrying creator re-attempts `os.Mkdir`. The invariant is that exactly one creator advances to file copying; all others join or retry.

**Race (b): A run joining while a last-out run is restoring.** The last-out run acquires an exclusive **blocking Lock** on a `.restoring` sentinel file inside the backup directory **before** releasing its own `.lock-{run_id}` and before probing other locks or starting the restore (§6.2.2, "On clean completion," step 1). A joining run checks for `.restoring` before creating its own lock: if `.restoring` is held, the joiner polls until either `.restoring` becomes acquirable or the backup directory disappears (see §6.2.2 step 3). After acquiring its own lock, the joiner performs a **post-lock re-verification**: it re-checks that `.restoring` is not held and that the backup directory and manifest still exist. If re-verification fails, the joiner releases its lock, deletes its lock file, and re-enters the `.restoring` poll loop or waits for the backup directory to disappear before starting fresh. This two-step check (pre-lock + post-lock) closes the check-then-act window and ensures a joiner never creates a lock inside a backup directory that is being deleted, and never runs against agents mid-restore.

**Race (b) — two simultaneous exits:** When two runs exit at the same time, both attempt to acquire `.restoring` as a blocking Lock. Exactly one acquires it first; the other blocks. The first performs the last-out check — if it detects the second run's `.lock-{run_id}` as still held (because the second run has not yet had a chance to release it), it releases `.restoring` without restoring. The second run then unblocks, acquires `.restoring`, re-runs the last-out check, finds no remaining locks, and performs the restore. Exactly one of the two exits performs the restore.

**Race (c): A run joining while the creator is still transforming.** After the manifest is written (step 5) but before `.setup-complete` is written (step 8), the originals are not yet transformed. A joiner that sees the manifest but no `.setup-complete` must wait for it before proceeding. The `.setup-complete` file is the signal that transforms are complete and it is safe to run.

#### 6.3.2 Automatic Recovery (Runner Startup)

Every Runner start begins with a recovery check — before any other run-start logic:

1. Scan the agents directory's sibling for the `.agents-backup/` directory. If not found → no recovery needed → done.
2. List all `.lock-*` files inside the backup directory. For each, attempt to acquire an exclusive lock:
   - Lock held → another Runner is actively running → **do not recover**. Leave the backup in place and proceed (this run will join as a concurrent run per §6.2.2).
   - Lock acquired → owner crashed → delete the lock file (release the lock first).
3. If no held locks remain (all lock files were orphaned or none existed), acquire `.restoring` as a **blocking Lock**. Two simultaneous recoverers serialize on `.restoring` — the second blocks until the first finishes, then finds no backup directory and no-ops.
4. **Re-probe all `.lock-*` files under `.restoring`:** A joiner may have acquired a lock between the initial probe (step 2) and `.restoring` acquisition (step 3). For each `.lock-*` file found now, attempt to acquire an exclusive lock:
   - Lock held → a run became active between steps 2 and 3 → release `.restoring` and **skip recovery** (leave the backup in place; this run will join per §6.2.2).
   - Lock acquired → orphaned → delete the lock file.
5. If any held lock was found in step 4 → recovery skipped (released `.restoring` in that step). Done.
6. If still no held locks → no active runs → check the manifest:
   - **Manifest present and readable** → **recover:** restore modified files from backup copies, remove the recovery marker from the agents directory, delete the backup directory, release `.restoring`.
   - **Manifest absent** → **partial backup** from a crash during backup creation (§6.3.1, Race (a) — partial backup). Originals are still untouched (transforms follow the manifest write). Delete the partial backup directory, release `.restoring`, and proceed as if no backup existed.
   - **Manifest present but corrupt/unreadable** → release `.restoring`, **refuse the run** with a clear error. The backup may contain the only copy of original field values, and restoring from a corrupt manifest risks data loss.

This is idempotent: running it when no recovery is needed is a no-op. Running it when another run is active correctly detects the live run and skips recovery.

#### 6.3.3 Manual Recovery (User Self-Service)

If the user abandons the Runner and returns to native harness usage, they need to be able to recover without Runner knowledge. The recovery marker file placed in the agents directory serves this purpose:

- **Location:** Inside the agents directory (e.g., `.opencode/agents/RUNNER-RECOVERY.txt`)
- **Visibility:** Not hidden, not dot-prefixed — visible to anyone who lists the directory
- **Content:** Plain-language explanation of what happened, where the backup is, and step-by-step restore instructions: (1) copy the `.md` files from the `.agents-backup/` directory back into the agents directory (overwrite the modified files), (2) delete the `.agents-backup/` directory, (3) delete this marker file. The instructions must NOT tell the user to rename or replace the agents directory with the backup directory, because the backup contains only `.md` files and renaming would destroy any subdirectories or non-`.md` files the user has in the agents directory.
- **Format:** `.txt` — empirical testing shows that a stray `.md` file in the agents directory is listed as an available agent by OpenCode, while `.txt` files are ignored by all harnesses (Claude Code, OpenCode, GHCP CLI)

#### 6.3.4 What "Corrupted" Means in Practice

Today's only transformation is `mode: subagent → mode: primary` for OpenCode. If left unreversed:

- The agents become directly invocable as primary agents — **more permissive, not broken**
- Interactive orchestration still works (the orchestrator can still spawn them)
- The semantic meaning is wrong but the functional impact is minimal

This is a fortunate property of the current transformation set, not a design guarantee. Future transformations could be more destructive, which is why the recovery mechanism exists regardless of current severity.

### 6.4 Snapshot Transformations

Both strategies apply the same harness-specific transformation rules to make agent files compatible with CLI invocation:

| Harness | Field | Regular value | Runner value | Reason |
|---------|-------|---------------|--------------|--------|
| OpenCode | `mode` | `subagent` | `primary` | `mode: subagent` blocks `opencode run --agent` CLI invocation |

This table is expected to grow as new harness-specific constraints are discovered. The transformation set is hardcoded per harness. The transformation mechanism is currently limited to frontmatter field rewrites; broader file transforms can be introduced when needed without changing the snapshot strategy selection.

### 6.5 Orchestrator Resolution

The Runner derives the script-mode orchestrator path from the harness convention -- it looks for `orchestrator-script` in the regular agents directory (e.g., `.opencode/agents/orchestrator-script.md`). The `--orchestrator-file` flag is removed; the path is fully determined by harness selection. If the orchestrator is not found at the expected path, the Runner refuses to start with a clear error indicating the workspace is not properly deployed.

This requires the deploy tool to place the script-mode orchestrator in the regular agents directory alongside all other agents. The regular orchestrator and script-mode orchestrator coexist in the same directory -- the regular orchestrator is used by interactive orchestration, the script-mode orchestrator is used by the Runner.

### 6.6 Run-Start Integration

The snapshot step is inserted into the Run-Start Sequence (§4). Recovery runs before agent resolution (step 5) so that agent files are in their original state when ResolveAll reads them:

| Step | What | Failure |
|------|------|---------|
| **4b** | **Recovery check: scan for the `.agents-backup/` directory next to the current harness's agents directory, restore if found** | **Refusal if restore fails** |
| 5 | Resolve every agent identifier to a definition file | Refusal |
| **5b** | **Create snapshot (copy-and-invoke) or backup + transform (backup-and-transform), per harness strategy** | **Refusal** |
| 6 | Read each block's schedule from its declared `Source` and `Section` | Refusal when a seeded block's schedule is missing or defective |

Step 4b runs unconditionally for CLI harnesses — it is a no-op when no recovery is needed (and correctly detects active concurrent runs via lock files, skipping recovery in that case). Step 5b creates the snapshot or backup depending on the harness's agent loading mechanism. For copy-and-invoke, resolved paths are rewritten to point into the snapshot. For backup-and-transform, resolved paths remain unchanged (originals are now transformed).

**Cleanup:** On run completion (any terminal outcome):
- **Copy-and-invoke:** Delete the snapshot directory. Cleanup failure is non-fatal.
- **Backup-and-transform:** Release lock, delete own lock file, perform last-out check (§6.2.2). If last out: restore originals, remove marker, delete backup directory. If not last out: leave everything for the remaining active runs. Restore failure is logged as an error but does not change the run's exit code — the run itself succeeded, and the next Runner start will retry recovery automatically.

---

## 7. Dead Ends

Approaches considered and rejected during implementation:

### 7.1 Engine as a Stateful Object

Early designs had the engine as a struct holding mutable state (current row index, stage counter, pending deviations). This was replaced by the pure-function design because:
- Mutable state made the engine hard to test (required setup/teardown)
- Resume required serializing/deserializing engine state alongside the artifact
- The artifact already carries all state needed for routing decisions — duplicating it in the engine was redundant

The pure function receives everything as parameters and returns a decision. The session manages all mutable state.

### 7.2 Single Deviation Mode

The original `--on-deviation` flag had only `stop`. The `delegate` mode (calling the script-mode orchestrator) was added because stopping on every deviation was too disruptive for real workflows — review agents returning `COMPLETED_NEEDS_ACTION` is normal operation, not an error. The current three-mode system formalizes this spectrum further.

### 7.3 Orchestrator as HTTP Service

Briefly considered having the script-mode orchestrator run as a persistent service that the Runner calls via HTTP. Rejected because:
- The orchestrator agent runs in the same harness (Claude Code, etc.) as subagents — there is no separate service infrastructure
- Statefulness would require session management between the Runner and orchestrator
- The current approach (invoke-and-parse) is simpler and uses the same harness adapter as everything else

### 7.4 Multi-Step Deviation Resolution

The initial deviation resolver design had the orchestrator resolve an entire deviation chain in a single invocation: the Runner hands off the deviation, the orchestrator invokes however many agents it needs internally (updating Orchestration.md along the way), and returns only when it can rejoin the happy path. Rejected in favor of the single-decision principle (§2.6) because:
- **Context growth:** The orchestrator's context grows with each internal agent invocation during the chain — the same unbounded cost problem the Runner exists to solve. A complex deviation chain (research → re-design → re-test) inside one orchestrator session accumulates all intermediate tool calls and responses.
- **No infrastructure triggers:** The Runner's trigger evaluation (checkpoints, commits) runs after each step in the dispatch loop. When the orchestrator dispatches agents internally, the Runner never sees those steps, so triggers don't fire during deviation resolution — precisely when checkpoints may matter most.
- **Contract complexity:** Required `rejoin_after_custom` fields and chain-orchestration logic that the single-decision model eliminates.

Note: audit trail and resumability are NOT advantages of single-decision — the real orchestrator already maintains Orchestration.md properly during multi-step resolution. Those properties hold either way.

The single-decision model has the orchestrator make one routing decision per invocation. Complex deviation chains become a sequence of single decisions, each passing through the Runner's dispatch loop (harness → record → triggers → next decision). The orchestrator loses cached attention state between invocations, but this is minor: the execution log in the updated artifact shows what was already tried, and the orchestrator will near-certainly derive the same continuation plan.

### 7.5 Other Ways to Route Back (§2.10)

These alternatives were considered on 2026-10-12/13, before the finished-work rule of §2.10 was chosen:

- **Group-level re-entry alone.** Resume a reopened iteration at its first unfinished group instead of its first group. The run still goes forward through finished rows outside the block (the contracts designer), so this only moves the problem.
- **Handback flag.** The orchestrator marks each dispatch with whether the engine routes its outcome.
  - It is a prediction made before the outcome is known, and it requires the orchestrator to know the engine's block semantics.
  - Its likely mistake, handing back too early, brings back exactly the repeated work, and nothing catches it.
  - It needs a new wire field, persisted detour state, and Runner behaviour explained in the orchestrator's instructions.
- **Detour plan.** The orchestrator returns the whole path back. Any step can redirect the detour, task descriptions cannot quote answers that do not exist yet, and iterations may not exist until the schedule is re-read. It is also a multi-step decision (§7.4).
- **Skip rows whose inputs did not change.** The planner did change the plan. Only judgement can say that the change does not affect downstream work.
- **Rejoin points declared in the workflow table.** Detours are open-ended, and handling them is the orchestrator's job (`WorkflowFormat.md` §1).
- **Run the fix row's creator/reviewer pair, then jump to the origin.** A failing `test-runner` sent to the planner would rejoin at `test-runner` and skip the new fix stage.
- **Origin anchor.** Store where the detour began and hand back when the run reaches it.
  - A successful detour can remove or move the origin (a stage reworked to Tests-Only or Implementation-Only), and the run then never hands back.
  - Anchoring on an area instead (the origin's iteration or anything after it) fails the other way: a fix stage the planner adds lies before the area.
- **Let checkers through without asking.** An early draft of §2.10 let the engine rerun a finished row whose `On Findings` target had run again since, treating `On Findings` as "the row whose work this row checks".
  - The format defines `On Findings` only as a suggested fix target. In real workflows it can belong to a row whose own work is creation (a generator relaying missing declarations to a writer), which the shortcut would have rerun unasked.
  - The shortcut only saved consultations. The Routing Policy's Quality Gate already makes the orchestrator re-dispatch a reviewer after a fix.
- **A bound on consecutive consultations after a SUCCESS.** Every consultation ends in a dispatch that is written to the log, so each consultation sees its predecessor's decision and outcome. A loop would require ignoring both. Native orchestration has no such bound either.
- **Consulting before returning to a step that asked a question,** so that its task quotes the answer. The asker may be one or several steps away from the answer, through review and rework, so a rule only for the adjacent case adds a special case. Workflows put answers into artifacts, which the asker reads.

---

## 8. Open Items

No open design items remain. All items from the initial draft have been resolved — see below.

**Resolved:**

- **Manual deviation resolution:** The user can be the routing resolver instead of the orchestrator — making routing decisions when the engine cannot. This is a separate option from the mode, not a mode itself. When `runner_manual_resolution` is enabled, a consultation that fails (transport, unparseable or invalid reply after its retries) falls back to the manual routing dialogue (§2.9) instead of ending the run; the orchestrator is still consulted first. The TUI stop screen can also run the dialogue once on demand (§3.4). It combines with any mode: in Modes 2/3 the user handles what the engine can't auto-route, in Mode 1 the same fallback applies to every consultation. The dialogue needs an interactive surface: the non-interactive CLI answers every question as unsupported, so there the dialogue ends without a dispatch and the run stops resumably instead of blocking.

- **Orchestration-review in Runner context:** The `orchestration-review` infrastructure agent is deployed optionally — injected into the orchestrator file during deployment when the user selects it. The Runner detects its presence in the `<InfrastructureAgents>` region. After orchestration-review fires, the Runner invokes the orchestrator with the review output as additional context (one extra LLM invocation per review firing). This is a known cost trade-off — the user accepts it by choosing to deploy orchestration-review. Available in all modes.

- **Mode selection flag:** Implementation detail — the mode must be explicitly selected at run start (no default), same as checkpoints and commits.

- **Orchestrator instruction schema:** The contract collapses to two actions: `dispatch {agent, task_description, hitl_override}` or `stop {reason}`. The old `rejoin` / `custom` / `stop` three-way split is eliminated — `custom` was a multi-step instruction (dispatch + rejoin point) that violates the single-decision principle (§2.6), and `rejoin` was just `dispatch` without a `task_description`. The orchestrator names an agent in the routing table and provides a task description; the Runner builds the rest of the request from the table row. Free table navigation — dispatching any agent regardless of current position — is the normal mechanism for both happy-path routing and deviation recovery. See §2.9.

- **Generic task description content:** The Runner generates a minimal generic message ("Proceed with your task"). The subagent already has its instructions, `input_artifacts`, and `output_artifacts` — the Runner has no domain understanding to add. See §2.5.

- **Mode 1 dispatch construction:** The Runner uses the engine for request construction. The orchestrator returns `{agent, task_description}` plus optional overrides (`constraints`, `input_artifacts`, `output_artifacts`, `hitl_override`). The Runner looks up the agent in the routing table; the engine builds the `ProtocolRequest` using orchestrator-provided fields where present, falling back to the table row's defaults where not. Sequence numbers are always assigned by the Runner.

- **Orchestrator consultation recording:** Individual calls remain in the Runner diagnostic log and the MOSAIC log's session-scoped orchestrator sessions rather than the orchestration artifact (`Development/Designs/MosaicLogFormat.md` §4.6). They are coordinator turns, so they consume no `global_sequence` value and create no Execution Log row. Workflow Notes preserve conclusions needed by later consultations. See §2.9.

- **Mode 2 engine suppression:** Implementation concern — how Mode 2 suppresses the engine's On Findings auto-routing. Not a design decision; the engine will be rewritten to support all three modes.

- **Default mode:** No default. The user must explicitly choose a mode at run start, same as checkpoints and commits. The mode is a quality-vs-cost trade-off that should not be assumed.

- **Mode switching mid-run:** Not supported. While technically feasible (the mode affects routing decisions, not artifact format), there is no clear use case. A run starts in a mode and stays there. If a different mode is needed, start a new run or resume with the new mode as a future consideration.

- **Mode naming:** The names "orchestrated," "auto," and "auto-review" are accepted as final.

---

## 9. Changelog

| Version | Date | Summary |
|---------|------|---------|
| 0.11 | 2026-10-13 | **Pre-consultation stop and stream errors documented as implemented.** §2.8 and §3.4: the orchestrator may answer pre-consultation with `stop`, a decision that ends the run as a consultant stop (exit code 6, reason on stderr) before any dispatch; the artifact is left as a pre-consultation failure leaves it; advice sent with a stop is ignored; any other `action` and a stop without a usable `reason` are a pre-consultation failure (exit code 7); the TUI offers Retry only. "Hard failure on error" now names only failures. §2.9: a reply without an `action` field is a missing required field and is not retried. §2.10: what counts as finished and the manual dialogue's pre-selection. §3.3: GHCP `subagent.deselected` (error names the requested agent) and `session.error` are harness errors; the GHCP version is logged. §4.4: Summary limits count characters. Contract 0.8. |
| 0.10 | 2026-10-13 | **Routing back without repeating finished work (§2.10).** In Modes 2/3 the engine dispatches the next step after a SUCCESS only when that step (a row at an `Iteration` value) has no `SUCCESS` in the Execution Log; otherwise it consults the orchestrator, whose dispatch is authoritative. The handback to the engine is implicit: the first SUCCESS whose next step has not finished. No wire or artifact change; mechanical retries, schedule-defect and HITL re-dispatches and Mode 3 findings routes are unchanged. §2.1, §2.3, §2.4 (auto-routed cases), §2.6, §2.9 and §3.2 follow; rejected alternatives in §7.5. §4.4: `Summary` keeps the first 50 and last 100 characters above 150 (OrchestrationArtifactFormat 3.1). Implementation follow-up: the finished-work check and its consultation trigger in the engine, shipped together with the Routing Policy judgement in both orchestrators (`ScriptOrchestratorContract.md` §6.3; `orchestrator.md` 8.2.0, `orchestrator-script.md` 3.3.0, which the Runner now requires); `TruncateSummary`. |
| 0.1 | 2026-08-16 | Initial design. Three execution modes (Orchestrated, Auto, Auto-review) with cost model and dispatch intelligence gap as central tensions. Single-decision principle. Two-action orchestrator contract (dispatch + stop) with free table navigation. Dispatch instruction carries optional artifact/constraint overrides (table row defaults, orchestrator overrides on re-invocations). Mode 3 engine injects review artifact on CNA auto-route back. Pre-consultation for environment plumbing (Modes 2/3). Run-start sequence with run configuration (checkpoints, commits, branch variant, commit setup dispatch). Stop-action UX: CLI terminal, TUI offers retry + manual dispatch. Infrastructure agent triggers. All open items resolved. |
| 0.2 | 2026-08-26 | Runner Agent Snapshot (§6). Runner creates a run-ID-scoped snapshot of deployed agents (`agents-runner-{run_id}/`) at every run start, with harness-specific transformations applied (e.g., `mode: primary` for OpenCode). Run-scoped directories enable safe parallel execution. Snapshot cleaned up on run completion; orphaned snapshots from crashes are harmless. Orchestrator file auto-discovered from harness convention, `--orchestrator-file` flag removed. Snapshot step inserted into Run-Start Sequence as step 5a. |
| 0.4 | 2026-09-26 | **HITL verification, Runner run-state corrections, and current sequential scheduling documented.** §3.5 and the unified loop put HITL verification before acceptance: a rejected attempt is logged without changing `current_state` or the Artifacts registry and cannot fire infrastructure triggers. §4.2 aligns commit setup with native orchestration: create the artifact first, record every setup outcome as an ordinary out-of-band invocation, and retain a failed attempt for inspection and resume. §4.3 rejects an existing artifact whose `run_id` is absent, malformed, or inconsistent with its run folder rather than preserving obsolete pre-identity behavior. §5 corrects `INVOCATION_INTERVAL` to its implemented global-sequence threshold and distinguishes automatically trigger-fired checkpoint/commit/review agents from manual restore agents. §4.1 and §4.3 persist gated-agent selections across resume. Runner-only execution policy is namespaced as `runner_mode`, `runner_pre_consultation`, and `runner_manual_resolution`; redundant `commit_branch_variant` storage is rejected. Consultation rows are described by their persisted form rather than an internal flag absent from the artifact. Admitted staged workflows currently execute one invocation at a time; concurrent workflow dispatch remains deferred in `ROADMAP.md`, while existing compatibility admission continues to reject workflow shapes the Runner cannot interpret. Setup ordering, legacy identity rejection, selection, and Runner-field persistence require the implementation follow-ups recorded by the review. §3.3 records harness errors as `BLOCKED`/`E501`, so the Routing Policy's error tiers apply (implementation follow-up). HITL verification applies to every returned status except `BLOCKED`/`E503`, which is routed directly because it already reports that the gate could not run, and checks only output artifacts the invocation created or modified; a listed but absent or unchanged output is not a gate miss. Mode rationale rewritten after both orchestrators adopted a shared minimal task-description rule. Mode 1's value is per-step routing judgment plus coverage of any workflow the orchestrator can interpret; it is no longer "targeted task descriptions". Modes 2/3 are the intended primary modes, and Mode 1 is the fallback. Admission is mode-dependent: engine-routing shape checks such as the parallel `On Success` refusal apply only in Modes 2/3, and Mode 1 runs forks sequentially with the orchestrator judging joins. The review loop limit is added to run configuration and to Mode 3 auto-routing. §5 `STAGE_END`/`PHASE_END` fire only when the last step of the stage or phase returns HITL-accepted `SUCCESS`, matching `InfrastructureAgentConcept.md` §4. §2.9 moves `current_state` to the dispatched row's position once the outcome is accepted, not at dispatch. The pre-consultation example carries every environment fact in `task_description`; `constraints` is for scope or deliverable restrictions only. §3.5: after a gate-discharging HITL re-dispatch that returns `SUCCESS`, the Runner routes on and records the original invocation's status (implementation follow-up). |
| 0.5 | 2026-10-08 | **Runner behavior documented as implemented.** §2.4: PARTIALLY_DONE (bound of 3 re-dispatches) and BLOCKED/E501 (budget of 3 attempts) are re-dispatched mechanically in Modes 2 and 3 and consult in Mode 1; the `[error:CODE]` Summary marker and how the E501 budget is counted from it; the review loop is counted per reviewer, phase and stage since the reviewer's last SUCCESS; row identification from the recorded `WorkflowRow` for every row; the in-memory anti-loop guard, its relation to the mechanical retries and its escalation bound (a resumable stop); the three row carriers. §2.9: dispatch replies carry `row` and `stage`, are validated and retried; the manual routing dialogue (steps, Esc navigation, bounds) and its ending outcome as a resumable stop; stops around a consultation. §3.3 and §3.5: harness errors carry `E501` and a gate-repaired step routes on the original outcome (both implemented). §3.4: stop endings, graceful-stop checkpoints, restart paths resuming the existing run with the history rebuilt. §4.3: resume derives row and stage like live routing and refuses a staged row without a stage. New §4.4 (artifact table layout, sanitising, path form) and §4.5 (paste-safe text entry). |
| 0.6 | 2026-10-09 | **Supported workflow subset (§1.4).** `WorkflowFormat.md` became executor-neutral; the Runner-specific parts moved here: which validity rules are checked when, which valid shapes each mode refuses, the `Plan.md`-only stage source as a known gap, and how the engine resolves `On Success` (named targets, block entry, the ignored in-block values, `next`, `-`). |
| 0.7 | 2026-10-11 | **Defective produced block schedules (§3.6).** A produced block schedule that is missing, unreadable or breaks a block-schedule rule is never used: in every mode the Runner re-dispatches the producing row (or the in-block writer row) itself, at the iteration it ran at, with a fix message and the non-compliant artifact as input; Mode 1 does this without consulting the orchestrator. Bound: 2 re-dispatches counted from the Execution Log alone, HITL-rejected attempts included, judged only after the HITL gate; the re-dispatch carries its own mechanical retry kind and is not counted by the anti-loop guard. Afterwards a routing consultation of kind `schedule-defect` carries the defect in `last_error_reason`, or the run ends `RunDeviationUnresolved` without a consultant. |
| 0.8 | 2026-10-12 | **Conformance to WorkflowFormat 2.16 and OrchestrationArtifactFormat 3.0.** §1.4 rewritten: the Runner accepts only 2.16 notation (no `[StageNumber]`, `**Execution Groups:**`, `*` suffix, lowercase `next`, `-` in `On Success`, no schedule source inferred from a `*Plan.md` name); block schedules come from each block's declared `Source` and `Section`; blocks on any phase, siblings, groups, mode-qualified `Subagent` cells, `{Name}` fill-ins and wildcards are supported; the checked rule IDs are T1-T9 and T11-T14, P1-P12, A1-A7, A11, A12, A15 (cycles, Modes 2/3), B1 (Modes 2/3) and V1-V6; the 1.x stage-table and routing rules and the 1.x numbering of P and A rules no longer exist. §1.3 admission wording, §2.9 reply (`iteration` replaces `stage`; bare agent id), manual dialogue (iteration step), §2.4 row identification (`On Findings` names its row), §3.1 and §4 steps (schedule read from declared sources, schedule re-read), §4.3 resume (pre-3.0 artifacts refused) and §4.4 (`Iteration` column) follow. |
| 0.9 | 2026-10-12 | **Infrastructure triggers and sequential running (§4.2, §5).** Trigger `STAGE_END` renamed `ITERATION_END` (no alias; fires at the end of an iteration of any block). Unknown triggers in the declaration region or in `infrastructure_overrides` refuse start and resume before any dispatch. Trigger dispatches: a commit-class `ITERATION_END` dispatch carries the ended iteration's artifacts, others name the matched triggers; infrastructure log rows show `Inputs` and the error code. New runs record `dispatch: sequential`; other values are refused on resume. Requires `orchestrator-script` v3.2.0. |
| 0.3 | 2026-09-20 | Dual-strategy snapshot (§6 rewrite). Harnesses that resolve agents by name (OpenCode, GHCP CLI) cannot read from a snapshot directory — the copy-and-invoke mechanism only works for path-based harnesses (Claude Code). New backup-and-transform strategy for name-based harnesses: backup originals, transform in-place, restore on completion. Concurrent runs coordinate via per-run OS-level file locks in a shared backup directory — no heartbeats or PID checks. Crash recovery via startup reconciliation (automatic, lock-aware) and user-visible recovery marker file (manual self-service). Run-start sequence gains step 4b (recovery check, before ResolveAll at step 5) and step 5b (snapshot/backup creation, after ResolveAll). |
