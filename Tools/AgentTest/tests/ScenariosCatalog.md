# AgentTest — Orchestrator Routing Test Scenarios

All tests follow the **oneshot pattern**: seed an `Orchestration.md` with execution log history that places the orchestrator at the decision point, then assert it makes the correct next dispatch (agent choice, input artifacts, HITL flag).

Reference workflow for most scenarios: `brownfield-tdd`.

---

## Scenario Catalog

Each scenario is one verifiable routing condition. Scenarios are atomic — whether they become individual tests or get combined into multi-assertion tests is a separate concern tracked in the [Test-to-Scenario Mapping](#test-to-scenario-mapping).

### HITL

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| H-1 | HITL-enabled agent returns SUCCESS but output artifact has `human_approved: false` | Re-dispatch same agent to complete HITL gate, not advance to next row | Tested |
| H-2 | Plan stage table marks stage HITL=TRUE, workflow table marks agent HITL=FALSE | Dispatch agent with `human_in_the_loop: true` (additive merge) | Tested |
| H-3 | Plan stage HITL=true applies to ALL agents in that stage, not just first | Second agent in same HITL stage also dispatched with `human_in_the_loop: true` | Tested |

### Status Code Routing

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| S-1 | Reviewer returns COMPLETED_NEEDS_ACTION | Route to On Findings target, not On Success | Tested |
| S-2 | Agent returns PARTIALLY_DONE | Re-dispatch same agent type (successor invocation), not advance to next row | Tested |
| S-3 | Agent returns BLOCKED (E101 REQUIRED_RESOURCE_NOT_FOUND) | Tier 1 auto-retry (up to 3 attempts), then Tier 2 alternative strategy | Tested |
| S-4 | Agent returns BLOCKED (E501 TOOL_UNAVAILABLE) | Tier 1 auto-retry (up to 3 attempts), then Tier 2 alternative strategy | Tested |
| S-5 | Agent returns NEEDS_CLARIFICATION | Provide context or escalate to human, not advance | Tested |
| S-6 | Agent returns CAPABILITY_EXCEEDED | Try close alternative or escalate to human | Tested |
| S-7 | Agent returns BLOCKED (E503 USER_CONTACT_UNAVAILABLE) on HITL dispatch | Escalate to human, no automatic retry, do not skip the HITL gate | Tested |

### Creator/Reviewer Gate

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| G-1 | Reviewer returns COMPLETED_NEEDS_ACTION — route back to creator | Creator receives review findings artifact in input_artifacts | Tested |
| G-2 | Creator fixes and returns SUCCESS after reviewer findings | Route back to reviewer for re-review, not advance past gate | Tested |
| G-3 | On re-review dispatch, should reviewer receive its own previous review artifact? | UNDECIDED — see [Discussion: G-3](#discussion-g-3) | Decision pending |

### Distant Route-Back

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| R-1 | Distant route-back completes (e.g. stage 5 → planner) — quality gate | Dispatch paired reviewer (e.g. plan-review) before re-entering EXECUTION | Tested |
| R-2 | Distant route-back to planner — input artifacts | Provide workflow-table inputs (Research.md, Requirements.md), not just local stage context | Tested |
| R-3 | Distant route-back to contracts-designer — quality gate | Dispatch contracts-review before re-entering EXECUTION | Tested |
| R-4 | Route-back from test-runner (REVIEW) to planner — quality gate | Dispatch plan-review before re-entering EXECUTION | Tested |

### Wildcard & Artifact Path Resolution

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| W-1 | Workflow table says `Stage-*/Plan.md` — dispatch to contracts-designer | Expand to all existing Stage-{N}/Plan.md (e.g. 4 stages → 4 paths) | Tested |
| W-2 | Workflow table says `Stage-*/Plan.md` AND `Stage-*/PlanProgress.md` — dispatch to plan-review | Both wildcards fully expanded | Tested |
| W-3 | Wildcard expansion after route-back that changed stage count | Expansion picks up newly added stages, not stale count | Tested |

### Execution Groups & Stage Progression

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| E-1 | EXECUTION phase entry — first stage, TDD approach | Dispatch test-writer-tdd (correct first agent per approach) | Tested |
| E-2 | Stage with Approach = "Implementation-Only" | Skip test-writer-tdd/tests-review-tdd, dispatch implementation-tdd directly | Tested |
| E-3 | Stage with Approach = "Implementation-First" | Dispatch implementation-tdd before test-writer-tdd (reversed from TDD) | Tested |
| E-4 | Stage with Approach = "Tests-Only" | Dispatch test-writer-tdd/tests-review-tdd only, skip implementation agents | Tested |

### Blocks & Iterations

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| B-1 | Implementation-First stage: Implementation group ends (`implementation-review` → `COMPLETE`) | Dispatch test-writer-tdd for the **same** stage — not the next stage, not test-runner | Deferred (format stabilizing) |
| B-2 | Last group of the last stage ends (`COMPLETE`) | Block exits to the first row after it: dispatch test-runner; run is not marked COMPLETED | Deferred (format stabilizing) |
| B-3 | Block re-entered after a route-back added stage 3 (stages 1–2 completed) | Run only the incomplete iteration: test-writer-tdd with Stage-3 inputs; stages 1–2 not re-run | Deferred (format stabilizing) |
| B-4 | Block schedule is empty (header, no rows) | Block exits at once: contracts-review → test-runner | Deferred (low priority) |

### Malformed Workflow & Block Schedule

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| M-1 | Plan.md `HITL` cell is not exactly `TRUE`/`FALSE` (e.g. `No`) | Do not enter the block; dispatch planner-tdd-soft (schedule producer) naming the problem | Deferred (format stabilizing) |
| M-2 | Plan.md has no `## Stages` heading (e.g. legacy `## Stage Table`) | Same as M-1 | Deferred (format stabilizing) |
| M-3 | Malformed block schedule whose source is a user seed (no row outputs it) | Escalate to user; no dispatch | Deferred (needs seed-schedule test workflow) |
| M-4 | Workflow table is malformed (legacy `[StageNumber]`, `HITL` = `-`, prose in On Findings) | Refuse before the first dispatch | Deferred (feasibility unknown) |

### Parallel Dispatch

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| P-1 | Multiple independent stages (no Depends On) after prior stage completes | Dispatch first-group agents for all eligible stages, not just one | Blocked (AgentTest capability TBD) |
| P-2 | Stage completion unblocks a dependent stage (Depends On all met) | Dispatch first agent for newly-eligible stage | Blocked (AgentTest capability TBD) |
| P-3 | Workflow fork — On Success names multiple agents | Dispatch all named agents | Blocked (AgentTest capability TBD) |
| P-4 | Join (`Waits For`) — one branch returned SUCCESS, another has not | Do not dispatch the join until every branch it waits for has returned SUCCESS | Deferred (needs fork/join test workflow) |

### Infrastructure Agent Triggers

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| I-1 | ITERATION_END trigger — checkpoint agent at an iteration end | Dispatch checkpoint agent before next stage's first workflow agent | Tested |
| I-2 | INVOCATION_INTERVAL trigger — review agent after N invocations | Dispatch review agent (threshold from agent's last firing, not modulus) | Tested (2 tests: precise + overdue) |
| I-3 | Multiple triggers fire on same boundary (checkpoint + commit both ITERATION_END) | Dispatch ALL fired agents in declaration order, each with own Seq | Tested |
| I-4 | Gated agent — checkpoint trigger exists but `checkpoints: disabled` | Do NOT dispatch checkpoint agent | Tested |
| I-5 | PHASE_END trigger | Dispatch infrastructure agent at phase transition | Tested |
| I-6 | Restore-class agent — never fires automatically regardless of declared triggers | Do NOT dispatch restore agent even if trigger condition is met | Tested |
| I-7 | What an ITERATION_END trigger dispatch carries (checkpoint + commit at the same iteration end) | Commit-class dispatch: `input_artifacts` = the ended iteration's own registered artifacts in table order (never `ContractsDesign.md`), `task_description` names the iteration (`Stage 1`). Checkpoint-class dispatch: empty `input_artifacts`, `task_description` names the trigger. Both: empty `output_artifacts`, `human_in_the_loop: false` | Tested |
| I-8 | Unknown trigger (retired `STAGE_END`) in the `<InfrastructureAgents>` declaration region | Refuse to start or resume, tell the user it is now `ITERATION_END` and that redeploying regenerates the region; no dispatch | Tested (resume; wording not observable) |
| I-9 | Unknown trigger (retired `STAGE_END`) in `infrastructure_overrides` | Refuse to start or resume, tell the user the override must name `ITERATION_END`; no dispatch | Tested (resume; wording not observable) |

### Dispatch Mode

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| D-1 | Resume an artifact recording `commits: enabled` with `dispatch: parallel` | Configuration error: refuse to resume, no dispatch (no silent downgrade to sequential, no dropping commits) | Tested (wording not observable) |
| D-2 | Resume an artifact with no `dispatch` field | Read as `sequential`: resume without asking or refusing; with two independent iterations eligible, run Stage 1 to completion before starting Stage 2 | Tested |
| D-3 | Fresh start, commits not enabled, workflow declares a block or fork | Ask the dispatch question after the commits question, presenting `sequential` and `parallel` with trade-offs and recommending neither | Not testable (AgentTest capability) |
| D-4 | Fresh start with commits enabled | Commits question states the run becomes sequential; record `dispatch: sequential` without asking the dispatch question | Not testable (AgentTest capability) |
| D-5 | Fresh start, workflow declares no fork and no block (or ad hoc) | Record `dispatch: sequential` without asking | Not testable (AgentTest capability) |
| D-6 | Resume with `dispatch` set to a value other than `sequential`/`parallel` | Refuse to resume, no dispatch | Ready (low priority) |

### Optional Phases

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| O-1 | Contracts phase skip — if one is skipped, both must be skipped | Never contracts-designer without contracts-review or vice versa | Decision pending |

### State Recovery

| ID | Scenario | Expected | Status |
|----|----------|----------|--------|
| X-1 | Resume when last Execution Log row is an infrastructure agent | Route from last *workflow* agent row, not the infrastructure row | Ready (low priority) |

---

## Test-to-Scenario Mapping

Filled in when tests are created. One test may cover multiple scenarios; one scenario may appear in multiple tests (different variants/workflows).

| Test ID | Test Name | Scenarios Covered | Workflow | Notes |
|---------|-----------|-------------------|----------|-------|
| findings-route-back | Findings Route-Back | S-1, G-1 | brownfield-tdd | Seeded after planner SUCCESS. plan-review COMPLETED_NEEDS_ACTION → planner-tdd-soft with plan-review.md in inputs |
| partially-done-redispatch | PARTIALLY_DONE Redispatch | S-2 | brownfield-tdd | Seeded at EXECUTION stage 1 impl. implementation-tdd PARTIALLY_DONE → same agent re-dispatched |
| creator-fix-rereview | Creator Fix Re-review | G-2 | brownfield-tdd | Seeded after planner fix (plan-review CNA → planner SUCCESS). Must re-review, not advance to contracts-designer |
| hitl-redispatch-unapproved | HITL Re-dispatch Unapproved | H-1 | brownfield-tdd | Seeded after requirements-refinement SUCCESS with human_approved: false. Must re-dispatch for HITL gate, not advance |
| impl-only-skip-tests | Impl-Only Skip Tests | E-2 | brownfield-tdd | Stage 1 Approach=Implementation-Only. Must skip test-writer-tdd/tests-review-tdd, dispatch implementation-tdd directly |
| hitl-plan-stage-override | HITL Plan Stage Override | H-2, E-1 | brownfield-tdd | Plan stage 1 HITL=TRUE overrides workflow HITL=FALSE. test-writer-tdd dispatched with human_in_the_loop: true. Also proves E-1 (TDD first agent) |
| hitl-plan-stage-all-agents | HITL Plan Stage All Agents | H-3 | brownfield-tdd | Stage 1 HITL=TRUE, test-writer-tdd already done. tests-review-tdd must also get human_in_the_loop: true |
| blocked-e101-retry | BLOCKED E101 Retry | S-3 | brownfield-tdd | implementation-tdd BLOCKED E101 → Tier 1 auto-retry, same agent re-dispatched |
| blocked-e501-retry | BLOCKED E501 Retry | S-4 | brownfield-tdd | implementation-tdd BLOCKED E501 → Tier 1 auto-retry, same agent re-dispatched |
| needs-clarification-no-advance | NEEDS_CLARIFICATION No Advance | S-5 | brownfield-tdd | implementation-tdd NEEDS_CLARIFICATION → route back to planner-tdd-soft for clarification, not advance |
| blocked-e503-hitl-retry | BLOCKED E503 HITL Retry | S-7 | brownfield-tdd | requirements-refinement BLOCKED E503 on HITL dispatch → orchestrator stops/escalates immediately, no automatic retry |
| planner-routeback-quality-gate | Planner Route-Back Quality Gate | R-1, R-2, R-4 | brownfield-tdd | test-runner CNA → planner (with Research.md, Requirements.md) → plan-review quality gate. Plan-review gets updated Plan.md + all stage plans |
| contracts-routeback-quality-gate | Contracts Route-Back Quality Gate | R-3 | brownfield-tdd | implementation-review CNA routes to contracts-designer → contracts-review quality gate before re-entering EXECUTION |
| wildcard-input-expansion | Wildcard Input Expansion | W-1 | brownfield-tdd | contracts-designer dispatch after plan-review SUCCESS. Stage-*/Plan.md must expand to Stage-1/Plan.md + Stage-2/Plan.md |
| capability-exceeded-escalate | CAPABILITY_EXCEEDED Escalate | S-6 | brownfield-tdd | implementation-tdd CAPABILITY_EXCEEDED → zero dispatches expected. Orchestrator must escalate to human, not retry or advance |
| impl-first-reorder | Impl-First Reorder | E-3 | brownfield-tdd | Stage 1 Approach=Implementation-First. Must dispatch implementation-tdd first, not test-writer-tdd |
| tests-only-skip-impl | Tests-Only Skip Impl | E-4 | brownfield-tdd | Stage 1 Approach=Tests-Only. Must dispatch test-writer-tdd, skip implementation agents entirely |
| wildcard-dual-expansion | Wildcard Dual Expansion | W-2 | brownfield-tdd | plan-review dispatch after planner SUCCESS. Both Stage-*/Plan.md and Stage-*/PlanProgress.md must expand to all stage files |
| wildcard-after-routeback | Wildcard After Route-Back | W-3 | brownfield-tdd | After route-back adding Stage-3, plan-review wildcards must expand to all 3 stages, not stale 2-stage count |
| iteration-end-checkpoint | ITERATION_END Checkpoint | I-1 | brownfield-tdd | implementation-review SUCCESS ends the Stage 1 iteration → checkpoint-iteration-end-only fires before stage 2. Renamed from stage-end-checkpoint (numeric id 64 unchanged) |
| interval-precise-boundary | Interval Precise Boundary | I-2 | brownfield-tdd | implementation-tdd SUCCESS at global_sequence=10, interval=10 → review fires exactly at threshold |
| interval-overdue | Interval Overdue | I-2 | brownfield-tdd | implementation-tdd SUCCESS at global_sequence=10, interval=3 → review fires when obviously past threshold |
| multiple-triggers-same-boundary | Multiple Triggers Same Boundary | I-3 | brownfield-tdd | implementation-review SUCCESS completes stage 1 → checkpoint + commit both fire in declaration order |
| gated-checkpoint-disabled | Gated Checkpoint Disabled | I-4 | brownfield-tdd | ITERATION_END met but checkpoints: disabled → checkpoint skipped, test-writer-tdd dispatched directly |
| phase-end-trigger | PHASE_END Trigger | I-5 | brownfield-tdd | requirements-review SUCCESS completes RESEARCH → infra-phase-end fires before planner-tdd-soft |
| restore-class-exclusion | Restore Class Exclusion | I-6 | brownfield-tdd | ITERATION_END met, restore-class agent (restore-iteration-end) with ITERATION_END trigger → NOT dispatched, test-writer-tdd dispatched instead |
| iteration-end-trigger-inputs | ITERATION_END Trigger Inputs | I-7 | brownfield-tdd | Same seed as I-3 plus a seeded Stage-1/tests-review-tdd.md. Checkpoint (seq 2): empty input/output artifacts, description contains `ITERATION_END`. Commit (seq 3): exactly Stage-1/Plan.md, PlanProgress.md, tests-review-tdd.md, implementation-review.md (the last registered only by the orchestrator's own write for seq 1), empty outputs, description contains `Stage 1` |
| unknown-trigger-declaration-refusal | Unknown Trigger in Declaration | I-8 | brownfield-tdd | checkpoint-stage-end (checkpoint class, declares STAGE_END) deployed; seed mid Stage 1 with checkpoints: enabled → zero dispatches |
| unknown-trigger-override-refusal | Unknown Trigger in Override | I-9 | brownfield-tdd | checkpoint-iteration-end-only deployed (valid); seed's infrastructure_overrides replaces its triggers with STAGE_END → zero dispatches |
| commits-parallel-resume-refusal | Commits With Parallel Refused | D-1 | brownfield-tdd | commit-manager-git deployed; seed records commits: enabled + dispatch: parallel (checkpoints disabled) → zero dispatches |
| dispatch-absent-resume-sequential | Dispatch Absent Reads Sequential | D-2 | brownfield-tdd | Seed after contracts-review with no dispatch field; Plan.md has two independent stages → test-writer-tdd then tests-review-tdd, both Stage 1 (not a second test-writer-tdd for Stage 2) |

---

## Scenario Details

Detailed context for each scenario — failure modes observed, setup specifics, variant ideas. The catalog table above is the canonical list; this section provides the depth needed to actually build the tests.

### H-1: HITL Re-dispatch on `human_approved: false`

**Setup:** Orchestration log shows e.g. `requirements-refinement` completed with SUCCESS. Its output artifact (`Requirements.md`) exists with `human_approved: false`. Orchestrator resumes.

**Failure mode observed:** Orchestrator treats SUCCESS as sufficient, ignores the `human_approved` stamp, and advances to the next row.

**Variants:** Test with `requirements-refinement` (RESEARCH), `planner-tdd-soft` (PLANNING), `contracts-designer` (DESIGN).

### H-2 / H-3: HITL Additive Merge From Plan Stage Table

**Setup:** EXECUTION stage 3 in progress. Plan.md stage table marks stage 3 with HITL=TRUE. Workflow table has `test-writer-tdd` as HITL FALSE.

**Failure mode anticipated:** Orchestrator only checks the workflow table's HITL column, ignores Plan's per-stage HITL. Subtle — requires reading Plan for HITL resolution, not just stage ordering.

**H-3 specifically:** Same stage, second agent (e.g. `tests-review-tdd`). Verify stage HITL applies to all agents, not just the first dispatched.

### S-1: COMPLETED_NEEDS_ACTION Misrouted as SUCCESS

**Setup:** `plan-review` completed with `COMPLETED_NEEDS_ACTION`. Workflow On Findings → `planner-tdd-soft`.

**Failure mode:** Orchestrator treats COMPLETED_NEEDS_ACTION as success variant and advances to `contracts-designer`.

**Note:** Most basic status-code routing test. Will likely combine with G-1 (review artifact in input) and/or G-2 (full loop). Listed separately as regression baseline.

### S-2: PARTIALLY_DONE — Re-dispatch Same Agent Type

**Setup:** A workflow agent (e.g. `implementation-tdd` during EXECUTION) returns `PARTIALLY_DONE` — some work items completed, more of the same work needed.

**Expected:** Orchestrator dispatches a successor invocation of the same agent type (new sequence number, same agent name). Does not advance to the next workflow row, does not treat it as failure.

**Failure mode anticipated:** Orchestrator treats PARTIALLY_DONE as SUCCESS and advances to `implementation-review`, leaving work incomplete. Or treats it as an error and enters error handling instead of a clean re-dispatch.

**Protocol definition:** "Route to successor agent (same type)" — this is a continuation, not a retry. The agent is expected to pick up where it left off using its progress artifact.

**Variants:** Test during EXECUTION (most realistic — large stage where implementation-tdd can't finish in one invocation) and during RESEARCH (codebase-research with a large codebase).

### S-3 / S-4: BLOCKED — Tiered Error Handling

**Setup (S-3):** Agent returns BLOCKED with `error_code: E101` (REQUIRED_RESOURCE_NOT_FOUND). E.g. `test-writer-tdd` can't find its input artifact.

**Setup (S-4):** Agent returns BLOCKED with `error_code: E501` (TOOL_UNAVAILABLE). E.g. build tool or test runner unavailable.

**Expected:** Orchestrator applies tiered error handling: Tier 1 auto-retry (up to 3 attempts with exponential backoff), then Tier 2 alternative strategy (reduce scope, skip optional phase), then Tier 3 human escalation.

**Failure mode anticipated:** Orchestrator skips retry and escalates immediately, or retries indefinitely without progressing through tiers, or treats BLOCKED as a terminal failure and stops the run without escalation.

**Note:** Testing the full 3-tier cascade may require `stop_after_invocations: 4+` (initial + 2 retries + escalation). Alternatively, test just the first routing decision (does it retry or not?).

### S-5: NEEDS_CLARIFICATION

**Setup:** Agent returns `NEEDS_CLARIFICATION` — uncertain or context incomplete.

**Expected:** Orchestrator provides additional context or escalates to human for clarification. Does not advance, does not treat as failure.

**Failure mode anticipated:** Orchestrator advances to next row ignoring the clarification need, or enters error handling instead of the clarification path.

### S-6: CAPABILITY_EXCEEDED

**Setup:** Agent returns `CAPABILITY_EXCEEDED` — task beyond agent's ability.

**Expected:** Orchestrator tries a close alternative strategy or escalates to human. Does not retry the same agent with the same inputs (unlike BLOCKED).

**Failure mode anticipated:** Orchestrator retries the same agent (pointless — it already said it can't do this) or advances as if successful.

### S-7: BLOCKED E503 on HITL Dispatch

**Setup:** Agent dispatched with `human_in_the_loop: true` returns BLOCKED with `error_code: E503` (USER_CONTACT_UNAVAILABLE — no user interaction tools available).

**Expected:** Orchestrator stops and escalates to the human immediately — no automatic retry. Must NOT skip the HITL gate or advance without human approval. A retry, or a re-invocation without the HITL flag, happens only through explicit user action (an explicit waiver of the gate).

**Failure mode anticipated:** Orchestrator auto-retries E503 as if it were transient (Tier 1 retry), or treats E503 as "HITL not possible, proceed without it" — effectively bypassing the human oversight gate.

### G-1: Review Artifact Passed to Creator on Findings Route-Back

**Setup:** `plan-review` completed COMPLETED_NEEDS_ACTION, `plan-review.md` exists. Orchestrator routes back to `planner-tdd-soft`.

**Failure mode observed:** Routes back correctly but omits `plan-review.md` from input_artifacts — creator has no visibility into findings.

**Variants:** All five creator/reviewer pairs in brownfield-tdd (requirements, plan, contracts, tests within stage, implementation within stage).

### G-2: Creator SUCCESS Does Not Pass the Gate

**Setup:** Execution log shows: (1) planner SUCCESS, (2) plan-review COMPLETED_NEEDS_ACTION, (3) planner SUCCESS (fix). Orchestrator resumes.

**Failure mode observed:** Treats creator's second SUCCESS as gate-passed, advances to contracts-designer.

**Orchestrator instructions explicit:** "You cannot advance past a creator/reviewer pair without the reviewer returning SUCCESS last."

**Variants:** All five pairs. Consider combining with G-3 assertion (re-review input artifact) once decided.

### Discussion: G-3

**Question:** On re-review dispatch (reviewer re-invoked after creator fix), should the reviewer receive its own previous review artifact as input?

**For:** Reviewer can verify each flagged finding was addressed, reduces rubber-stamping risk, audit trail of raised-vs-fixed.

**Against:** Fresh evaluation on merits, previous review may anchor on stale findings, workflow table Input column doesn't list it.

**Recommendation:** Include it — re-review's job is specifically "verify these fixes," not "review from scratch." Same principle as G-1 (review findings to creator). If decided, becomes a `task_messages` assertion on the G-2 test.

**Status:** Awaiting decision.

### R-1 / R-3 / R-4: Quality Gate After Distant Route-Back

**Setup (R-1):** Normal flow through PLANNING, DESIGN, EXECUTION stages 1-4. Stage 5's `implementation-review` routes back to `planner-tdd-soft`. Planner completes SUCCESS.

**Failure mode observed:** Orchestrator sees route-back as "minor fix," skips review gate, jumps straight back to EXECUTION. Worse when change seems small or route-back is distant.

**R-3:** Same pattern but route-back to `contracts-designer` → must hit `contracts-review`.
**R-4:** Route-back from `test-runner` (REVIEW phase) to `planner-tdd-soft` → must hit `plan-review`.

**R-3 meaning changed with brownfield-tdd 4.0:** the old `implementation-review` On Findings cell read `implementation-tdd (or other based on issue)`, which licensed re-targeting to `contracts-designer` from the table itself. Under WorkflowFormat 2.16 the cell is exactly `implementation-tdd` and On Findings is only a suggested fix target; re-targeting is the orchestrator's own routing decision (WorkflowFormat §1). `contracts-routeback-quality-gate` therefore now asserts re-targeting *against* the table's suggestion — a harder condition. A failure there means something different than it did under 3.7.

### R-2: Missing Input Artifacts on Distant Route-Back

**Setup:** Stage 5's `implementation-review` routes back to `planner-tdd-soft`. Workflow table says planner inputs: `Research.md, Requirements.md`.

**Failure mode observed:** Orchestrator provides only stage 5 plan and progress, omitting `Requirements.md` and `Research.md` — treating route-back as if planner only needs local context.

### W-1 / W-2 / W-3: Stage-* Wildcard Expansion

**Affected workflow rows (brownfield-tdd):**
- `plan-review` inputs: `Requirements.md, Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md`
- `contracts-designer` inputs: `Research.md, Requirements.md, Plan.md, Stage-*/Plan.md`
- `contracts-review` inputs: `Plan.md, Stage-*/Plan.md, ContractsDesign.md`

**W-1 failure mode observed:** Orchestrator passes `Plan.md` but omits per-stage plans, or passes literal `Stage-*/Plan.md` unexpanded, or only some stages.

**W-3:** After route-back that adds new stages, expansion must use current state, not cached earlier count.

### E-1 through E-4: Execution Group Selection

**Context:** Plan stage table has Approach column. Orchestrator must read it and dispatch the correct group sequence. The brownfield-tdd `**Groups:**` table defines the mapping.

**E-2 failure mode anticipated:** Orchestrator ignores Approach column, defaults to full TDD sequence for every stage.

### B-1 through B-4: Block Routing (WorkflowFormat 2.16)

**Context:** In brownfield-tdd 4.0, `tests-review-tdd` and `implementation-review` both say `COMPLETE`. In 3.7 they named their successors explicitly (`implementation-tdd`, `test-runner`). `COMPLETE` now ends the current *scope* — a group, then the iteration — and what follows is derived from the iteration's `Approach` (next group), the block schedule (next iteration), or the block exit (first row after the block). E-1 to E-4 only cover the *first* dispatch of an iteration; B-* cover the transitions out of a group and out of the block.

**B-1/B-2 failure mode anticipated:** Orchestrator reads `COMPLETE` literally and ends the run, or jumps to test-runner mid-stage.

**B-3:** Re-entry runs only iterations that have not completed (WorkflowFormat §6.5). Natural follow-up to R-1/R-4: `planner-routeback-quality-gate` stops at plan-review, before re-entry. Seed: route-back re-plan added stage 3, plan-review → contracts-designer → contracts-review all SUCCESS. Closed-set input assertion catches a Stage-1 dispatch for free. Likely the most realistic new-format bug — route-backs are routine.

**B-4:** Valid per format, but a planner practically never writes an empty schedule. Cheap; low realism.

### M-1 through M-4: Malformed Input Refusal

**Context:** The orchestrator refuses what the format declares malformed rather than guessing (WorkflowFormat §10). Two distinct behaviours:

- **Block schedule malformed (M-1, M-2):** checked each time the block is entered. Do not run the block; dispatch the row that outputs the schedule's `Source` (planner-tdd-soft for brownfield-tdd) with the problem named in `task_description` (`task_description_contains` assertion). Seed: design-complete state with a malformed Plan.md.
- **Schedule source is a seed (M-3):** no row can fix it, so escalate. Needs a workflow whose schedule is a seed (e.g. an `implementation-only` test workflow).
- **Workflow table malformed (M-4):** refuse before the first dispatch. **Feasibility check first:** if mosaic-deploy validates workflows at deploy time, a malformed workflow never reaches the orchestrator and this is not testable at the orchestrator layer.

### P-1 / P-2 / P-3: Parallel Dispatch

**Orchestrator instructions explicit:** "Dispatch all eligible targets before waiting on any one of them" and "none is skipped in favor of another."

**P-1 failure mode observed:** Stages treated as strictly sequential even when plan allows parallelism.

**P-3 note:** brownfield-tdd has no explicit On Success forks — may need custom test workflow.

**P-4:** Join gating via `Waits For` (WorkflowFormat §7.2). Six production workflows use joins (audit and KB families); no test workflow has one yet. Needs a fork/join test workflow plus stubs where one branch returns before the other.

**Open question (P-1/P-2):** WorkflowFormat 2.16 says iterations whose `Depends On` are met *may* run concurrently. Whether the orchestrator is expected to actually dispatch independent stages concurrently, or sequential is intended, decides whether P-1 is a valid scenario at all. AgentTest now has a `min_concurrency` assertion, which may lift the "capability TBD" block.

**Blocked:** AgentTest parallel dispatch assertion support TBD.

### I-1 through I-6: Infrastructure Agent Triggers

**Key rules:** Triggers evaluated after Orchestration.md update, before next workflow dispatch. All fired agents run (not a selection). One agent fires at most once per evaluation. No cascades. Gated classes skip when switch disabled. Restore always skipped.

**I-2 detail:** Threshold, not modulus. Count from agent's last firing row in log, not from run start. If never fired, count from sequence 0.

**I-3 detail:** Iteration end with checkpoint (ITERATION_END) + commit (ITERATION_END) both declared and enabled. Must produce two dispatches, two sequence numbers, two log rows.

**Rename:** `STAGE_END` was renamed `ITERATION_END` with no alias (orchestrator 8.1.0, InfrastructureAgentConcept §4). It fires at the end of every block's iteration, whatever the block is called. The I-1/I-3/I-4/I-6 tests were updated in place; their intent is unchanged. Their seeds now record `dispatch: sequential`.

**I-7 detail:** The commit agent's message is the only description of a commit that survives the run folder, so it must be handed the iteration's own artifacts: every path in an `Input` or `Output` cell of a block row that fills in the block ID, resolved for the ended iteration, kept only when registered, once, in table order. For brownfield-tdd Stage 1 that is `Stage-1/Plan.md`, `Stage-1/PlanProgress.md`, `Stage-1/tests-review-tdd.md`, `Stage-1/implementation-review.md` — never `ContractsDesign.md`. A checkpoint needs nothing, so its list is empty. Failure modes anticipated: commit dispatched with no inputs (the agent then refuses), commit given the last workflow row's inputs (includes `ContractsDesign.md`), checkpoint given the same list as the commit. The test seeds `Stage-1/tests-review-tdd.md` as a file as well as registering it, so the registry-based (orchestrator) and existence-based (Runner) rules agree. Paths are asserted fully prefixed (`Orchestration-{run_id}/...`), as Artifact Path Resolution requires. Kept separate from I-3 so a message-content failure is distinguishable from an ordering failure; merge them if suite cost matters more.

**I-8 / I-9 detail:** An unknown trigger must refuse the run rather than act as a trigger that never fires — a silently inert checkpoint trigger is a run that believes it has restore points it does not have. Both tests are resumes, because on a resume the next dispatch is fully determined (implementation-review), so zero dispatches can only mean a refusal. A fresh-start variant would be weaker: any startup question also yields zero dispatches. I-9 uses a checkpoint-class agent on purpose: a commit-class override naming `STAGE_END` would also violate the commit-only-`ITERATION_END` restriction and could pass for the wrong reason. The stub's name and description are rendered into the declaration region, so they are kept neutral (no hint that it is a refusal test). AgentTest cannot see the refusal text, so whether the user is told "it is now `ITERATION_END`" is not asserted. mosaic-deploy does not validate trigger names (out of scope per GAP R7); if it ever does, I-8 will fail at preflight rather than at run time and must move to a hand-built declaration.

### D-1 through D-6: Dispatch Mode

**Context:** `dispatch: sequential | parallel` is a set-once frontmatter field (OrchestrationArtifactFormat §4, orchestrator Dispatch Mode). Commits enabled forces `sequential`: a commit cannot describe a tree holding a sibling iteration's half-done work. Absent is read as `sequential`; any other value is refused.

**D-1:** Seed is the I-3 artifact with `dispatch: parallel` and checkpoints disabled, deploying only commit-manager-git so no other precondition is in play. Failure modes anticipated: silently treating the run as sequential, or dropping commits to keep parallel. The seed has no run-start setup row for the commit agent (the existing both-enabled fixture never had one); the orchestrator does not check for it, but a refusal citing it would also produce zero dispatches.

**D-2:** Plan.md declares two stages with no `Depends On`, and the seed sits at the block entry — the only point where sequential and parallel dispatch differ observably in brownfield-tdd. Under sequential the Stage 1 iteration runs to completion first, so the second dispatch is tests-review-tdd (Stage 1). A second test-writer-tdd (Stage 2) means parallel behaviour; zero dispatches means the missing field was refused.

**D-3 / D-4 / D-5 (not testable):** These are startup-conversation behaviours. AgentTest sends one opening message, cannot answer a question, cannot see the question's text (so it cannot check that both options were explained, that neither was recommended, or that the commits question mentioned sequential), and has no assertion over arbitrary frontmatter fields (`final_state` reads only `phase` and `last_status`, so the recorded `dispatch` value is invisible). The only observable proxy is whether the orchestrator stops before its first dispatch, which any question at all would also cause — and the "do not ask me questions" clause fresh-start tests need would mask the behaviour under test. They become testable once AgentTest can script user replies and assert on the subject's final message or on frontmatter fields.

**D-6:** Same shape as D-1 with e.g. `dispatch: concurrent`. Low priority: the value is written by the orchestrator itself, so an invalid one only arrives through hand editing.

**I-5 note:** No current concrete agent uses PHASE_END. May need custom test infrastructure agent.

### O-1: Optional Contracts Phase (DECISION PENDING)

**Open design question:** Skip decision requires domain judgment orchestrator can't make (can't read artifacts per context window protection). Considering always-invoke: contracts-designer returns "Nothing to do" if unneeded.

**Current failure mode:** Orchestrator skips design phase when it shouldn't — guesses from task description.

**If always-invoke is adopted:** Scenario collapses to simple "dispatch contracts-designer after plan-review SUCCESS" with no skip logic.

### X-1: Recovery Past Infrastructure Agent Log Rows

**Setup:** Last rows: `implementation-tdd#14 SUCCESS`, `checkpoint-manager-git#15 SUCCESS`. Resume.

**Failure mode anticipated:** Sees checkpoint row as last, doesn't know what to dispatch. Or treats checkpoint row as iteration end "handled" and skips to next stage.

**Priority:** Low — only on resume after crash between infrastructure dispatch and next workflow dispatch.

---

## General Notes

- All tests use the oneshot pattern with seeded `Orchestration.md`
- Primary workflow: `brownfield-tdd` (most scenarios observed there)
- Some scenarios (P-3, P-4, M-3, I-5) may need custom test workflows
- B-* and M-* (and P-4) are deferred until WorkflowFormat 2.16 / OrchestrationArtifactFormat 3.0 have been proven on real runs of regular workflows — no point testing routing against a format that may still move
- Infrastructure agent scenarios (I-*) need test workflows with `<InfrastructureAgents>` declarations and corresponding stub agent definitions
- Refusal scenarios (I-8, I-9, D-1) assert zero dispatches (`steps: []`). Seed them as resumes, where the next dispatch is fully determined, so zero dispatches cannot also be explained by a startup question
- Each test needs: seed Orchestration.md, seed artifacts with correct provenance frontmatter, stub registry for expected dispatch(es), assertions on agent identity + input artifacts + HITL flag
- `stop_after_invocations: 1` for most (one dispatch decision per test)
- Infrastructure agent tests may need `stop_after_invocations: 2+` to capture infra dispatch + next workflow dispatch
- Parallel dispatch tests need `stop_after_invocations: 2+` and multi-dispatch assertions
- Tests are expensive to run — combine scenarios into single tests where the same seeded state can verify multiple conditions (e.g. S-1 + G-1 in one test: reviewer COMPLETED_NEEDS_ACTION routes to creator WITH review artifact)
