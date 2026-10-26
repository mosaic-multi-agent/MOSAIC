# Unreleased

## Tools

### Deploy v1.2.1
#### Bugfixes
- Agents deployed by recent versions are recognised as MOSAIC agents again. Before, `transform` and `retarget` skipped them as "not agent", `promote` refused them, and `update` silently left them out of the harness-only refresh. No migration is needed: affected workspaces are recognised on the next run

### Runner v1.8.0
#### Changes
- The loop guard can be switched off for a session with `g` on the progress screen, or when resuming after a guard stop
- Pre-consultation guidance is saved with the run, can be reviewed and edited (also during a run with `e`), and is reused on resume
- Resuming asks fewer questions and shows a summary of the recorded settings
- Infrastructure failures are visible on the progress screen and listed at the end of the run
#### Bugfixes
- Requests no longer send `null` instead of an empty artifact list
- Row numbers are now consistently 1-based in logs and messages
- Resume no longer asks again for settings the run already recorded

# 0.7.2 (2026-10-25)

## Tools

### LogAnalyzer v1.1.0
#### Changes
- New ways to look at runs: an overview, a timeline, breakdowns by agent, status and tool use, and a side-by-side comparison of several runs
- More accurate cost estimates for Claude models
- Runs can be saved to a small summary file and opened again later, without keeping the original logs

### AgentTest v1.1.0
#### Changes
- Several new safeguards to ensure that tests run at selected model.
- Test results show which model actually ran. Runs where that can't be confirmed are flagged and left out of the pass rate
- Cost is no longer shown in summaries

### Runner v1.7.1
#### Bugfixes
- Fix commits at the end of an iteration failing in some runs

## Catalog
- **mosaic-logger** hook: logs now also capture reasoning tokens, reasoning effort and the model provider, for Claude Code and OpenCode
- **orchestrator** (v8.2.2) / **commit-manager-git** (v2.4.0): the commit dispatch at an iteration end names the run's recorded commit branch, so the commit agent can check it instead of always blocking
- **commit-manager-git** (v2.4.0) / **checkpoint-restore-git** (v2.2.3): commits and rollback reverts also leave out MOSAIC's log folders (`OrchestrationLogs/`, `RunnerLogs/`), and no longer fail when those folders are gitignored

## Known Issues
- **VS Code GHCP and GHCP CLI logs don't include the new details yet.** LogAnalyzer works for these harnesses, but without the new information from this release. An update is planned within 2-3 days

# 0.7.1 (2026-10-20)

## Tools

### Runner v1.7.0
#### Changes
- The progress screen lets you step into a running run: `m` requests manual dispatch at the next routing decision, `h` runs the next agent with HITL on. Both are one-shot and pressing the key again cancels them
- Orchestrator consultations show as a temporary row on the progress screen while they run
- The progress screen shows the same rows during a run as after resuming it
- Claude Haiku 5.5 added to the model list of every harness
#### Bugfixes
- GHCP CLI model IDs corrected (for example `claude-opus-5.5` instead of `claude-opus-5-5`)

### Deploy v1.2.0
#### Changes
- **Catalog stacks:** deploy from several catalogs layered on top of each other (`--catalog-folder`, `--include-default-catalog`), in every subcommand and the TUI. A higher layer replaces agents, workflows, skills, hooks and the Communication Protocol of the layers below it. Every catalog needs a `catalog-identity.yaml`
- `update` reuses the catalog stack saved with the workspace, and compares protocol versions against the protocol the stack supplies
- `promote` asks which layer receives a new agent when the stack has more than one layer
- Claude Haiku 5.5 added to the model list of every harness
#### Bugfixes
- GHCP CLI model IDs corrected

## Catalog
- The Communication Protocol moves into the catalog (`Catalog/CommunicationProtocol.md`), so a catalog layer can supply its own. The Runner still expects the default status and error codes
- **planner-tdd-soft** (v7.5.0) / **plan-review** (v5.4.0): plans no longer decide interface or data design. Plan review goes stage by stage and records its progress in a ledger, so a revised plan only gets its changed stages re-read. The planner can decline review findings with a reason
- **contracts-designer** (v4.4.0) / **contracts-review** (v4.5.0): designs define only the public surface, not internals. The design keeps a Revision Log, and review goes section by section with a ledger, re-reading only sections a revision changed. The designer can decline findings with a reason
- **subagent-creator** (v3.1.2): reads the protocol from its new catalog location

## Documentation
- Deployment Guide: Catalog Stacks section
- Runner Guide (Design): progress-screen controls, in-run requests and consultation rows

# 0.7.0 (2026-10-13)

> **Breaking: the workflow format is finalized, and the new orchestrators accept nothing else.** The format is close to what 0.6.2 workflows already used. It is now fully specified, and "stages" are generalized into "blocks" and "iterations". Workflow logic is unchanged and all catalog workflows have been migrated. The new `orchestrator` (v8), `orchestrator-script` (v3) and Runner (v1.6) refuse workflows in the old format and cannot resume runs started with earlier versions.
>
> **To upgrade:** redeploy the orchestrator and workflows together, and finish any runs still in progress before upgrading. To migrate a custom workflow, use `workflow-creator`. The logic carries over as is, so every workflow is expected to migrate without problems.

## Tools

### Runner v1.6.1
#### Changes
- Accepts only the finalized workflow format. Requires `orchestrator-script` v3.3.0
- Workflows with parallel branches (forks and joins) run only in Mode 1, one branch at a time
- GHCP CLI: clear errors when GHCP runs its default agent instead of the requested one, or when the session fails (for example an exhausted quota)
#### Bugfixes
- Fix finished work being repeated after a route-back in Modes 2 and 3

### Deploy v1.1.5
#### Changes
- `subagent-creator` and `workflow-creator` are no longer offered by default when deploying a workspace. They ship ready to use in the MOSAIC repository root, so run them there
#### Bugfixes
- VS Code GHCP model list is fixed and now offers the same models as GHCP CLI

## Catalog
- **orchestrator** (v8.2.1) / **orchestrator-script** (v3.3.0): finalized workflow format only. Stage-based wording becomes iterations, including the infrastructure trigger `STAGE_END`, renamed `ITERATION_END`
- All catalog workflows migrated to the finalized format
- **product-comparison** (v2.0): a new **comparison-planner** agent plans the products, research topics and comparison dimensions up front
- **workflow-creator** (v2.1.0): authors workflows in the finalized format
- **commit-manager-git** (v2.3.1): commits once per finished iteration. The `Mosaic-Stage` commit trailer is replaced by `Mosaic-Iteration`
- Infrastructure, planning and research agents updated for the finalized format

## Documentation
- Workflow format fully specified in `Development/Designs/WorkflowFormat.md`
- Getting Started and Deployment Guide: creator agents are run from the MOSAIC repository root
- Orchestration and Runner guides use iteration terminology

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