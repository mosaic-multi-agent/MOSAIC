# Unreleased

## Tools

### Runner
#### Changes
- Requires `orchestrator-script` v3.3.0 (dispatch replies name the iteration and the bare agent id; runs are sequential; the orchestrator judges whether finished work must run again)
- **breaking** Infrastructure trigger `STAGE_END` is renamed `ITERATION_END` and fires when an iteration of any block ends; orchestrators must be redeployed
- Refuses to start or resume a run when an infrastructure agent names a trigger the Runner does not know, in the orchestrator or in the run's overrides, before anything is dispatched; for `STAGE_END` the message says it is now `ITERATION_END`
- New runs record `dispatch: sequential`, and resuming a run that records another value is refused
- Commit dispatches carry the files of the iteration that just finished and other trigger dispatches name the trigger(s) that fired; infrastructure log rows show their inputs and error codes
- Accepts only WorkflowFormat 2.16 workflows (1.x workflows are refused at start and on resume) and refuses to resume runs written before OrchestrationArtifactFormat 3.0
- The orchestrator can end a run at pre-consultation with `stop` and a reason: the run ends before any dispatch (exit code 6, reason printed, run resumable; the TUI offers Retry only). Any other pre-consultation `action` and a stop without a reason fail the pre-consultation (exit code 7)
- A routing reply without an `action` field is reported as a missing field, not as an unknown action
- GHCP CLI: a `subagent.deselected` event (GHCP ran its default agent instead of the requested one) fails the invocation with an error naming the requested agent, and a `session.error` event (for example an exhausted quota) fails it with its type, code, status and message; the GHCP version is written to the debug log, and the parser's schema note records the versions these events were observed with
- In Modes 2 and 3 a `SUCCESS` whose next step has already finished goes to the orchestrator instead of repeating that work; with manual resolution the finished step is pre-selected
- The Execution Log `Summary` keeps the first 50 and last 100 characters above 150 characters, counted in characters rather than bytes
- MosaicTest sidecars (`.expected.json`) can assert that the child run's stderr contains a given text (`stderr_contains`)

## Catalog
- **orchestrator-script** (v3.1.0): dispatch replies name the block iteration in `iteration` (the Execution Log's `Iteration` value) instead of `stage`, and give the bare agent id without the mode qualifier
- **orchestrator-script** (v3.2.0): runs fork branches and block iterations one at a time, each to its end, in a fixed order
- **orchestrator** (v8.1.0): **breaking** — trigger `STAGE_END` renamed `ITERATION_END`; redeploy orchestrators. Asks whether a run is sequential or parallel (enabling commits makes it sequential), refuses unknown triggers, and gives the commit agent the finished iteration's files
- **commit-manager-git** (v2.3.0): commit messages come from the finished iteration's files whatever the workflow names them; `Mosaic-Stage` trailer replaced by `Mosaic-Iteration`
- **checkpoint-manager-git** (v2.2.1), **checkpoint-restore-git** (v2.2.2): trigger renamed `ITERATION_END`; revert commits name the checkpoint's sequence number
- **orchestration-review** (v1.2.2): uses iteration terminology of the current run format
- **orchestrator-script** (v3.3.0): judges whether finished work must run again when `On Success` leads to a row that already finished, and treats it as a fourth deviation trigger; may end a run at pre-consultation with a `stop`; `Summary` keeps the first 50 and last 100 characters above 150; required by the new Runner
- **orchestrator** (v8.2.0): same finished-work judgement in its Routing Policy; `Summary` handling keeps the first 50 and last 100 characters above 150
- **commit-manager-git** (v2.3.1), **checkpoint-manager-git** (v2.2.2), **orchestration-review** (v1.2.3): wording about the Execution Log `Summary` truncation no longer states the old 50 + 50 cut
- **mosaic-architect** (v1.4.1), **subagent-creator** (v3.1.1): catalog paths corrected (`Catalog/UtilityAgents/subagent-creator.md`, `Catalog/Workflows/Index.md`)
- **MosaicTest catalogue**: stub orchestrator `orchestrator-script` (v1.3.0) can return a pre-consultation `stop` from its fixture; new workflows `preconsult-stop` and `finished-work-consult` cover the pre-consultation stop and the finished-work consultation

# 0.6.2 (2026-10-10)

## Tools

### Runner v1.5.0
#### Changes
- Requires `orchestrator-script` v2.3.0 (dispatch replies now name the workflow row)
- Malformed orchestrator replies are retried instead of stopping the run
- Auto modes retry `PARTIALLY_DONE` and `E501` without consulting the orchestrator
- With manual resolution enabled, you pick the next step yourself when an orchestrator consultation fails, instead of the run ending
- Stopped runs can be continued, retried or manually dispatched without leaving the Runner
#### Bugfixes
- Fix crash when continuing a run
- Fix prompt delivery to OpenCode and GHCP CLI on Windows
- Fix resumed runs continuing from the wrong step
- Fix orchestration artifact tables not rendering in Markdown viewers
- Input fields handle multi-line paste

### Deploy v1.1.4
#### Bugfixes
- Input fields handle multi-line paste

### AgentTest v1.0.3
#### Bugfixes
- Input fields handle multi-line paste

### LogAnalyzer v1.0.2
#### Bugfixes
- Input fields handle multi-line paste

## Catalog
- **Orchestrator** (v7.7.0) / **orchestrator-script** (v2.3.0): orchestration artifact tables render properly, review loop limit resets after the reviewer's `SUCCESS`; `orchestrator-script` names the workflow row in its dispatches (required by Runner v1.5.0)

## Documentation
- Runner Guide: automatic retries, manual routing, resuming from the TUI

# 0.6.1 (2026-10-05)

## Tools

### Deploy v1.1.3
#### Changes
- `Update workspace` updates deployed hooks as well

# 0.6.0 (2026-10-05)

## Tools

### Runner v1.4.0
#### Changes
- Writes MOSAIC logs for runs, covering orchestrator turns and subagent invocations.
- Workflow `On Findings` routing now runs in auto modes without help from the orchestrator.
- New `--review-loop-limit` option (run config and Mode 3 auto-routing)
- Runner settings (mode, pre-consultation, manual resolution) and gated-agent selections persist across resume
- Warns when the working directory is not a trusted folder for GHCP CLI
#### Bugfixes
- Fix incorrect dispatch on route-backs in auto-review mode
- `STAGE_END` / `PHASE_END` triggers fire only after HITL-accepted `SUCCESS`

### Deploy v1.1.2
#### Changes
- `Deploy agents` mode adds infrastructure agent declarations to an existing orchestrator. `update` now refreshes already-declared catalog infrastructure agents whose version, class, triggers or on-failure changed (previously the region was kept as-is)
- Rebuilt workflow tables get a `Row` column (1..N), which the Runner uses
- More tolerant of source file formatting
- Model lists updated (Opus 5.5, Sonnet 5.5, Fable 5.1, GPT-6 family, etc.)

### AgentTest v1.0.2
- Adapted to Claude Code's new subagent hand-back mechanism (2.1.271+, auto mode)

### LogAnalyzer v1.0.1
- Recognizes Runner-produced log shapes

## Orchestration System
- **Communication Protocol v1.13** (from v1.11): subagents should now always choose their status code based only on the outcome of their own task, not on what they think should happen next in the workflow, which makes status codes more deterministic. Stricter artifact access rules, a fuller HITL review request that lists every changed artifact.
- **Deployed Sections Bundle v2.0.0**: managed instruction sections were rewritten to work better across model families, not only Claude. `ProtocolConstraints` was removed because it repeated the protocol.

## Hooks
- **mosaic-logger** (v1.5.1): captures subagent results via Claude Code's `SubagentHandback`, supports Runner mode on all harnesses, and fixes session export on OpenCode

## Catalog
- **Orchestrator** (v7.6.1) / **orchestrator-script** (v2.2.1): updated for protocol v1.13, with improved subagent invocation records in the orchestration log
- **Requirements pair reworked**: **requirements-refinement** (v3.4.1) produces typed requirements at a depth the user declares, with explicit Delegated and Accepted Unknowns sections. **requirements-review** (v5.0.4) reviews against that declared depth and leaves delegated decisions alone, which cuts down on review spirals.
- **build-review** (v4.0.0): tightened instructions on its responsibilities: import, build and deploy, record deployment metadata, and never run tests
- **tests-review-tdd** (v3.4.1), **lean-tdd** skill (v1.2.0): RED-phase verification is now about discriminability (whether the failing tests tell correct and incorrect implementations apart)
- All subagents: status-code guidance aligned with protocol v1.13, plus small fixes (patch/minor bumps across Research, Planning, Validation, Creation, Interface, Audit)
- New model-family-agnostic creators: **subagent-creator** (utility, v3.0.0) and **agent-creator** (standalone, v1.0.0), alongside the existing `anthropic-*` variants
- Workflows bumped for the updated agents: `brownfield-tdd`, `brownfield-tdd-build-verified`, `brownfield-design`, `brownfield-pr-fix`, `kb-correction`

## Documentation
- Deployment Guide: orchestrator declaration refresh, infrastructure injection in `agents` mode, workflow `Row` column
- Runner Guide: workflow rows, `WorkflowRow`, On Findings targeting, and version drift on resume
- New `docs/CODING-GUIDELINES.md`

## Known Issues
- **Runner with GHCP CLI is unreliable.** With recent GHCP CLI versions (observed on 1.0.91), GHCP CLI sometimes ignores the requested `--agent` in prompt mode and runs its default agent instead, so Runner invocations fail with protocol or parsing errors. The failures are intermittent and the cause is not yet confirmed. In limited testing GHCP CLI 1.0.87 did not show the problem, so pinning it may help, but this is not verified. Claude Code and OpenCode are not affected.

# 0.5.0 (2026-09-23)

## Tools

### Runner v1.1.0
#### Changes
- Automated test framework (dev mode): `test` subcommand and TUI test flow for selecting and running orchestration test suites, with test catalog discovery, fixture management, expected-result validation, and multi-mode support (auto, auto-review, orchestrated)
- Reworked agent snapshot mechanism for OpenCode and GHCP CLI: agents are now transformed in-place with automatic backup and crash recovery, instead of requiring a separate runner directory
- Less rigid response parsing from LLMs with automatic retry on non-compliant subagent responses (no user consultation needed)

#### Bugfixes
- Fix delayed infrastructure triggers

### Deploy v1.0.2
#### Changes
- New `--infrastructure` flag on `deploy` subcommand for pre-answering infrastructure agent selection in CI/scripted deployments

## Catalog
- **Planner** (v7.3.0): Reworked stage definition and sizing — stages are now the unit the orchestrator dispatches, with explicit guidance on bounded context, uniform approach, internal coherence, and clean checkpoints. Task sizing reframed as progress checkpoints within execution groups, not session boundaries.
- **Plan Review** (v5.3.0): New stage sizing assessment section validates that execution groups fit in a single agent session. PARTIALLY_DONE guidance improved for large inputs.
- **Contracts Designer** (v4.3.0): Now explicitly scoped to comply with the plan rather than silently overriding plan decisions. Tighter boundary between contract specification and implementation detail — private methods, algorithms, and pseudocode excluded to prevent review spirals.
- **Contracts Review** (v4.4.0): Flags overspecification (implementation detail in contracts) for removal. Aggregates all findings before returning to reduce review-creator round-trips.
- Removed option to skip DESIGN phase in relevant workflows, it was doing more harm than good.

## Documentation
- GettingStarted guide: clarified how to add workflows to an existing deployment (Update workflows mode)
- Deployment Guide: documented Update workflows, Deploy agents, and Deploy hooks modes; removed stale Utility-infra section
- Runner Guide: updated snapshot strategy and crash recovery documentation

# 0.4.0 (2026-09-13)

All tools now carry their own version number, displayed in TUI headers and embedded in logs/reports.

## Tools

### Deploy v1.0.1
#### Changes
- Unused tool mappings are now removed from agents frontmatter on deploy
- Harness constraints injection version is now added even when empty for a given harness
#### Bugfixes
- Fix MCP server deployment for Claude Code harness
- Fix `orchestrator-scripted` not being updated properly during deploy
- Fix `s`/`S` key collision on TUI screen
- Fix some incorrect TODOs at deployment

### Runner v1.0.0
#### Changes
- Improved logging
#### Bugfixes
- Fix graceful stop not working

### AgentTest v1.0.0
#### Changes
- Summary split into user-facing and internal reports
- Scheduling adjusted to leverage Anthropic cross-process prompt caching (cost reduction)
- Improved cost accuracy, infra errors excluded from summary problem areas and cost calculations

### LogAnalyzer v1.0.0
No changes.

## Orchestration System
- Communication protocol updated to prevent routing around HITL
- Orchestrator instructions updated to prevent routing around HITL

## Catalog
- New standalone agents: `spearhead` (exploration), `presentation-creator`
- `CatalogFilesFormat.md` renamed and updated

## Orchestration Test Results
- Completed remaining test suites for Claude Code with Opus 5
- Added results for Sonnet 5, Haiku 4.5, Fable 5

## Harness Knowledge
- First harness issues capture and evaluation completed for all supported harnesses
- System prompt captures created for each harness

## Documentation
- `CONTRIBUTING.md`, `SECURITY.md` created, README updated with Getting Started section

# 0.3.0 (2026-09-01)
## Tools
### Changes
#### Deploy
- No change to tools field in agents on Update anymore, to preserve user customization of them
- `user-invocable` field added to VS code agents deploy
- Multi-level multi-selection screens now show the active selection.

#### Runner
- At start of new run, tool creates own snapshots directory - copy of agents - enabling it to modify agents as needed for execution via CLI without touching user agents.

### Bugfixes
#### Deploy
- Stop detecting changes in user managed instruction regions as local changes, triggering overwrite dialogs. Tool should now show true changes on update.
- Dialogs for TIER model are now triggered only for TIERs used by deployed agents. 

#### Runner
- Engine will not enter loop anymore at dispatches with HITL true and output artifacts with wildcard. Added general anti-loop mechanism as well.
- At create-review pairs, on route back from review to create, add its original output artifacts to its input, to prevent agent overwriting its previous work without realizing.
- Fix hard crash in some scenarios.
- Give agents correct safe tool permissions matching agents tools frontmatter. For GHCP, it is still possible to switch to yolo mode in TUI dialog.

## Catalog
- Removed all Unicode characters from Catalog, all should be ASCII only now.
- Updated Deployed sections bundle version (subagents HITL execution improved, ASCII only constraints to artifacts)

## Notes
- MOSAIC root contains mosaic-helper agent deployed for all harnesses. Agent aims to provide user help with any questions regarding MOSAIC.
- Finished test tool to check orchestrator behavior in various workflow scenarios. Incomplete results in OrchestrationTestResults, will be completed asap.
- Roadmap created in MOSAIC root.

# 0.2.0 (2026-08-21)
## Tools
- Critical: Orchestrator instructions mess fixed
- Many fixes in deploy tool, higlights:
1. Deploy all skill files
2. Fix OpenCode and GHCP CLI value of mode/user-invocable field
3. `Update workspace` does not create orchestrator anymore
4. `version` field changed to `mosaic_version`, harness injection versioning moved from frontmatter to XML tag
5. Preserve user custom tools and infrastructure agents in orchestrator on agents update

## Catalog
- Two new workflows: `Brownfield PR Fix Workflow` and `Product Comparison Workflow`