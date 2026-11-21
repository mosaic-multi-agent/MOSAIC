# OpenCode 2.x — Active Issues

> Last updated: 2026-11-18 (run 1)

---

### O2-001: `subagent` sessionID resume fails — model emits bare "ses", server returns "Subagent session not found: ses"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/59293 (also dup reports #60679, #60493, #63874, #63987) |
| **Reported** | 2026-10-26 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.1.4, 2.2.1, 2.2.3, 2.2.5, 2.2.7, 2.3.0 (latest confirmed on 2.3.0; not re-checked on 2.3.2) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | triaging |

**Summary:**
Resuming a child via the async `subagent` tool's `sessionID` parameter fails with `Subagent session not found: ses` for several models/providers (GLM via OpenRouter, Qwen, StepFun, DeepSeek, Kimi-like). The model-facing schema is only `pattern: "^ses"`, so the minimal match `ses` is emitted, passes validation and fails at lookup. Core is not truncating: with a full ID the lookup succeeds. Open fix PR #58053 (`^ses.*$`) would still accept bare `ses`; commenters propose `^ses_[0-9A-Za-z]{26}$` plus an example ID in the description. A maintainer-bot reply confirms the schema only checks the `ses` prefix. Issue still open, no maintainer fix.

**Impact on Orchestration:**
Continuation of a subagent by sessionID (resume with prior context) is unusable on affected models; the orchestrator must re-brief a fresh child, losing child context and wasting tokens. A failed resume can look like model error rather than tooling error.

**Evidence:**
- 5+ independent reporters, raw tool-call inputs showing `"sessionID":"ses"`, one with a per-model table (15/15 failures on some models, full IDs on others: model/provider dependent). One reporter saw 6/6 failures on 2.3.0 with a model that works elsewhere, so trigger is also context-dependent.
- Maintainer/bot reply read the code and confirmed schema is prefix-only (`Schema.isStartingWith("ses")`); fix PR #58053 open but insufficient per commenters.
- Many duplicates were closed by bot; verified they all show the same bare-`ses` symptom.

**Workaround(s):**
1. Spawn a fresh subagent and include prior context/handoff brief in the prompt (works, loses child context).
2. Resume via HTTP API: `opencode api post /api/session/<childID>/prompt --data '{"text":"..."}'` (reported working with full context, single reporter, 2.2.7).
3. Pick a model that emits the full ID (varies by model).

**Notes:**
Cross-ref: #55504 (models filling sessionID with made-up values), #58019 (same `^ses` pattern rejected by llama.cpp; fix #58053). 2.2.7 background completion messages reportedly lacked sessionID; reported addressed in 2.3.0 (completion messages carry `sessionID` and `state`). Most relevant to MOSAIC as a reason not to rely on subagent resume.

---

### O2-020: Compaction fails with "Compaction summary did not match the required template" on GPT-6.2 (Responses provider)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/64598 |
| **Reported** | 2026-11-18 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0, 2.3.2 (code unchanged between them) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, area/core, version/v2, platform/windows |

**Summary:**
Manual compaction (`/compact`, `POST /api/session/{id}/compact`) on `gpt-6.2-sol` (xhigh) via an `@ai-sdk/openai` Responses-compatible provider fails with `compaction.failed`. The model returns text (about 3k tokens) but no line exactly matches a `SUMMARY_TEMPLATE` `##` heading, so the validation rejects it. Reproduces on a fresh two-prompt session. A direct request with the same template and no tools succeeds, so the trigger seems tied to the compaction request context (session tools still sent, long history).

**Impact on Orchestration:**
A long-running primary agent (orchestrator) that cannot compact eventually hits the context limit and the session is stuck. Summary validation is strict (exact heading-line match), so any model drift in heading format kills compaction rather than degrading it.

**Evidence:**
- Single reporter with detailed code analysis; a contributor bot could not reproduce (no access to the model) and asked for the raw failed reply.
- Reporter's own workaround confirms the failure is model/provider-specific. Native compaction path is unavailable for `aisdk:@ai-sdk/openai` routes (`UnsupportedCompactionError`).
- Related open items: #57828 (English-only headings), #53242/#53370 (compaction request still exposes session tools, fix PR proposed), #63439 (commentary text kept in summary).

**Workaround(s):**
1. Set `agents.compaction.model` to a different model (reporter used `opencode-go/deepseek-v4.1-flash#max`); the chat model stays unchanged. Reporter-confirmed, contributor agrees it is reasonable.

**Notes:**
Unverified, included because it threatens context management of long-lived primaries on a model MOSAIC may use (GPT-family via Responses API). Related compaction entries (O2-021 and later) cover other failure modes. See also #64553 (unreachable `agents.compaction.model` fails every summary, no fallback to session model), which means the workaround has its own failure mode.

---

### O2-021: Run/turn blocks silently on start snapshot of a huge git work tree (no model request until snapshot ends)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/63904 |
| **Reported** | 2026-11-14 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 and latest `dev` (cbe7159 at time of report); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/core, inference/go, version/v2, platform/windows |

**Summary:**
Issue title reads as a provider hang, but the reporter traced it to the per-step start snapshot: each step waits for `git ... update-index/ls-files` over the whole work tree before publishing the model request. With a stray `.git` in `$HOME` (OneDrive-synced), `opencode run` stalls for minutes or indefinitely: busy session, no log line, no error, no timeout, only the title request completes (`in=438`). A bot reproduced on 2.3.0 and `dev` with 1.5M untracked files (82 s delay; scales with file count). Issue was closed by the reporter as completed, but no fix is referenced and the maintainer-side bot says it still happens on `dev`.

**Impact on Orchestration:**
Headless `opencode run` or subagent turns started in a directory that resolves to a very large git work tree (huge repo, home dir, monorepo with many untracked artifacts) appear hung with no diagnostics, and the orchestrator waiting on completion (or `/session/{id}/wait`) sees no progress. Project identity is keyed by nearest ancestor `.git`, so one stray repo above the workspace affects every project under it.

**Evidence:**
- Deterministic A/B by reporter on Windows (moving `~/.git` aside gives `OK` in ~8 s; present hangs > 75 s with `git ls-files` children alive).
- Contributor bot reproduction on Linux with mock provider (`status/repro-simulated`; "a maintainer should confirm"); runtime grows with untracked file count; repro script supplied.
- Earlier slow-snapshot reports all closed (#48657, #57841, #50734). Related stall reports: #53747, #63730, #63533.

**Workaround(s):**
1. Set `"snapshots": false` in config (bot-suggested; undo/revert then lose file snapshots). Not confirmed by the reporter.
2. Remove/move aside a stray `.git` above the working directory (reporter-confirmed).

**Notes:**
Low odds for a normal MOSAIC workspace unless the repo has very many untracked files or a parent directory contains a stray `.git`. Kept active because failure is silent and indistinguishable from a provider hang. Re-verify on 2.3.2.

---

### O2-022: `opencode run` never exits when a child session is still busy or a non-waking inbox row is pending

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/62636 |
| **Reported** | 2026-11-07 |
| **Last Activity** | 2026-11-10 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.6 and `dev` at time of report; 2.3.2 not verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
In `packages/cli/src/run/noninteractive.ts`, the settle loop of `opencode run` treats a busy child session (created with `parentID` but not as a subagent job) as part of its own work, but only wakes on terminal events of sessions in `jobSessions`, so it never exits. A second commenter found a second trigger: a pending inbox row in the run's own session (for example a `resume: false` synthetic/plugin notice admitted after the last boundary) also leaves the loop waiting forever; SIGTERM then reports a stuck finalizer. Bot reproduced the first trigger on 2.2.6 and `dev`; the issue author confirmed the second still hangs even with the fix PR.

**Impact on Orchestration:**
MOSAIC's headless run (`opencode run`) is the completion signal for a dispatched agent. A run that never exits stalls the orchestration step indefinitely unless an external timeout kills it. Plugins that post notices with `resume: false` after the final turn make this reachable without any custom API use.

**Evidence:**
- Bot reproduction with mock model and script (exit 124 vs 0 controls); author-level source analysis of `background()` branches.
- Open fix PR #62643 handles the busy-child case only; the `resume: false` inbox case still hangs (author comment 2026-11-10, asking maintainers whether to split an issue).
- No maintainer comment. Label `reproduced`.

**Workaround(s):**
1. Wrap `opencode run` in an external timeout and treat exit 124/kill as a possible completed-but-hung run (inspect the session for a final assistant message). Not from the thread; the thread uses `timeout 45` only in the repro.
2. A second `opencode run` on the same session promotes the pending inbox row and exits normally in about 2 s (reporter-observed, trigger B).

**Notes:**
Related run-hang reports, open: #62681 (run hangs before creating a session when the initial SSE event is buffered), #62682 (models/session listing hang when cwd does not exist on the server). V1 has no equivalent (the run settle loop is V2-specific).

---

### O2-023: Auto-compaction loops every step when a provider reports input tokens above the model's context limit

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/50474 |
| **Reported** | 2026-09-22 |
| **Last Activity** | 2026-10-28 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.12 and `dev` (fa1260d, 2026-10-28); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
The compaction preflight trusts `usage.input` from the provider even when it exceeds the model's declared context window (reporter: 25-33M against a 1,048,576 limit, after a screenshot-heavy session on `ollama-cloud`). Every following step looks over the limit, so auto-compaction runs again and again (4 compactions in 75 s; a mock repro produced about 5,400 compaction/step requests in 90 s). Assigned to a maintainer (kitlangton); two fix PRs (#50537, #50796) were closed unmerged and the issue is still open.

**Impact on Orchestration:**
An unattended agent session can burn tokens and money in a compaction loop without making progress, and a headless run does not finish. Any provider/proxy whose usage accounting is inflated (cached or image tokens, OpenAI-compatible gateways) can trigger it. The documented "one overflow-recovery attempt" guarantee does not hold here.

**Evidence:**
- Bot reproduction on 2.0.12 and `dev` with a mock provider reporting inflated `prompt_tokens`, plus script. Issue assigned to a maintainer, label `reproduced`.
- Only the original reporter's session showed inflated usage in their DB (not general to the provider), so trigger frequency is unclear. Maintainer-side fix status unknown (both PRs closed).

**Workaround(s):**
1. Set `"compaction": { "auto": false }` (reporter-applied, bot-confirmed to avoid the loop). Trade-off: no automatic compaction, so context overflow must be handled by manual `/compact`.

**Notes:**
Separate sub-finding in the same issue: a provider HTTP 413 (image payload too large) is classified `invalid-request` rather than context overflow, so the session cannot self-recover. Related: #64231 (open, `compaction.auto: false` replays an oversized history and stays stuck on `Prompt is too long`), #58774 (large `compaction.buffer` compacts almost every turn, not read). Check whether 2.3.x changed the preflight.

---

### O2-024: `opencode run --format json` emits only root-session events, no subagent activity

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/62380 |
| **Reported** | 2026-11-06 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.1, 2.2.3 (reporter); `dev` after #61911; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none |

**Summary:**
`noninteractive.ts` in the V2 CLI drops every event whose `sessionID` is not the root session. A subagent's shell calls, text and steps never reach stdout, and the root's `subagent` tool_use appears only after the child finishes. The contributor bot states the filter was kept on purpose in the #61911 runner rewrite and treats the report as a feature request (stream child events marked by session). A community member offered a PR with an opt-in flag; no maintainer response yet.

**Impact on Orchestration:**
A headless supervisor (MOSAIC runner parsing NDJSON) cannot see progress or hang points inside a delegated subagent; a long subagent looks like silence. Combined with V2's asynchronous `subagent` tool, root-level output only shows dispatch and the final report. Downstream user hit an 8-hour timeout inside a subagent with no trace.

**Evidence:**
- Reporter traced the filter to `noninteractive.ts` (`ownsSession` follows `parentID` only for permission prompts) with a stub-provider repro.
- Contributor bot confirmed the by-design reading; same request earlier on 1.18 (#49300, closed for 2.0, so not V2 evidence by itself). Related: #33396/#33397.
- Open proposal to add an opt-in flag emitting subagent events plus a summary `result` event.

**Workaround(s):**
1. Run `opencode serve` and subscribe to the server event stream through the SDK; it includes child-session events linked by `parentID` (suggested by contributor bot, not user-confirmed).

**Notes:**
By design as of now: MOSAIC must obtain subagent progress via the server event stream or session store rather than `run --format json`. Cross-reference O2-022 (run settle loop treats busy children as its own work).

---

### O2-025: Model step stalls indefinitely when the provider stream stays alive with keepalive comments; no inactivity timeout, parent waits forever

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/53747 |
| **Reported** | 2026-10-07 |
| **Last Activity** | 2026-10-07 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.23 (reporter and bot repro); later 2.x reports show the same pattern (#63904 notes a 15-minute default chunk timeout on 2.3.0). 2.3.2 not re-verified. |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none |

**Summary:**
A session's model step can hang with no timeout, error or retry: the assistant message stays open (`time.completed` unset), tool parts stay `running`, nothing is logged until `POST /api/session/:id/interrupt`. Reporter saw 34 interrupted steps across 18 sessions in 3 days (five over an hour, longest almost 4 h) on `github-copilot` with an Opus-class model while background subagents ran. A bot reproduced on 2.0.23: a fully silent stream is bounded by `chunkTimeout`, but a stream that keeps sending SSE `: keepalive` comments never times out even with `chunkTimeout: 15000` (also #43519). Assigned to a maintainer; no fix referenced.

**Impact on Orchestration:**
Unattended multi-agent runs stop silently. A parent (orchestrator) waiting on a stalled subagent waits forever, and after a server restart orphaned steps look like live work. The harness gives no failure signal, so the orchestrator must supply its own watchdog.

**Evidence:**
- Bot reproduction with a simulated provider (keepalive-only stream), maintainer assignee, quantified field data (34 stalls).
- Duplicate-candidate list from the stale-bot verified in a follow-up pass: #53083 (1.18.34), #48675 (1.18.30), #46461 (1.18.x) and #43519 (1.18.x) are all OpenCode 1.x reports and are not V2 evidence. #51291 (2.0.16, closed not_planned after a bot could not reproduce) and #54683 (2.0.26, closed as its duplicate) are V2: a `shell`/`write` tool part sits at `state.input == {}`, `executed: false` (placeholder created when argument streaming starts) while the provider (zai-coding-plan GLM-5.3) goes silent or keepalive-only, session stays `running` for 21-31 minutes on 2.0.26, 2.0.27 and 2.1.0 until interrupted; no timeout or retry logged. A bot showed a fully silent stream is cut by the default chunk timeout (5 min on 2.0.26, 15 min on `dev`), so the unbounded cases are keepalive/empty-event streams. `settleStaleToolCalls` only runs when the next turn drains, so stale `running` parts survive until a client interrupt (#51291 code analysis).
- Related recent reports: #63904 notes that a provider stream that finishes but never closes keeps `opencode run` hanging until the 15-minute chunk timeout (no "response finished" settle path).

**Workaround(s):**
1. External watchdog polling `GET /api/session` and `GET /api/session/:id/message?limit=1`; after about 25 minutes with no progress call `/interrupt` then `/prompt` with "continue" (reporter's script, continues normally afterward).
2. Lower `chunkTimeout` only helps for silent streams, not keepalive-only streams (bot-verified).
3. Set an overall per-request provider `timeout` (suggested by the contributor bot on #54683, e.g. `{"providers":{"zai-coding-plan":{"settings":{"timeout":900000}}}}`; it bounds the whole streamed response regardless of keepalives and ends legitimately long responses too). Not confirmed by the reporter; exact config key path to verify against current docs. An external watchdog keyed on "last message is a `tool_use` with empty input and no stream activity for 60 s, then interrupt and re-prompt" absorbed 6+ occurrences reliably (#54683 reporter).

**Notes:**
Distinct from O2-042 (Code Mode / MCP calls have no timeout) and O2-005 (no default step/time bound): this is the model-stream layer. Related: O2-022 (run never exits), O2-021 (silent snapshot stall).

---

### O2-026: Unreachable `agents.compaction.model` fails every compaction; fallback to the session model only applies when the model cannot be resolved

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/64553 |
| **Reported** | 2026-11-18 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.1 (reporter); behavior is documented design, so 2.3.2 assumed |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/core, version/v2, platform/windows |

**Summary:**
When `agents.compaction.model` is set to a model that resolves in the catalog but whose provider is down (e.g. `ECONNREFUSED`), every automatic and manual compaction fails with `provider.transport`, no summary is written, and the session grows until the provider rejects the request as too long. The documented fallback to the session model only triggers when the configured model cannot be resolved. A contributor bot reclassified the report as a feature request (extend the fallback to request-time failures); a community fix PR (#64554) is open.

**Impact on Orchestration:**
Pinning compaction to a dedicated cheaper or different model (a documented mitigation, see O2-020) adds a single point of failure: if that endpoint is down or rate-limited, long-running primaries stop being able to compact and eventually overflow.

**Evidence:**
- Reporter's session data: session model succeeded before and after an 86 s failed compaction attempt; failed compaction row records `provider.transport` ECONNREFUSED.
- Contributor bot confirms documented behavior scope; related requests #64280 (rewrite a failed summary on another model) and #48991 (configurable model fallback on transient errors).
- Single reporter, open fix PR #64554, no maintainer comment.

**Workaround(s):**
1. Leave `agents.compaction.model` unset (compaction then uses the session model) unless the dedicated model is reliably reachable. Inferred from the docs, not tested in the thread.
2. Manually switch the setting off or compact on the session model when the compaction endpoint is down.

**Notes:**
Treated as Limitation until a maintainer decides on the PR. Related: O2-020 (template mismatch; workaround is the same setting), O2-023.

---

### O2-027: V1 plugin hook names (e.g. `experimental.session.compacting`) are accepted by V2 `ctx.session.hook()` but never fire, with no error or warning

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/62777 |
| **Reported** | 2026-11-08 |
| **Last Activity** | 2026-11-08 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.6 (reporter); design unchanged assumed for 2.3.2 |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none |

**Summary:**
`ctx.session.hook(name, cb)` in V2 registers any string without validation. `experimental.session.compacting` is a V1 hook that does not exist in V2, so the callback never fires while the compaction completes normally; a control `context` hook fired in the same window. A contributor bot explained V2's hook names are `prompt`, `context`, `compaction`, `generate`, `title`, `model.request`, `http.request`, `http.response`, `retry`, `experimental.idle` (plus experimental websocket hooks), and that a plugin `setup()` needs an `id` or it never runs. Name validation on registration was filed as a feature request.

**Impact on Orchestration:**
MOSAIC's logging/observation plugin (hook-based) can silently lose events if it registers V1 or misspelled hook names: logs or guard rails that depend on a compaction/session hook never run, and nothing flags it. Plain-JS plugins get no error; only TypeScript types catch it.

**Evidence:**
- Reporter control experiment (probe plugins on 2.2.6, `strings` of the binary); contributor bot confirms hook names and that the V1 name is simply absent in V2. Single issue, no maintainer comment, open.
- Related per bot: #53706 (V1 plugins do not load on V2), #48138 (V1 plugin backward compat), #57276/#50862 (compaction hook result fields; hook can return only a summary string).

**Workaround(s):**
1. Register the V2 hook name (`compaction` for pre-compaction, with `event.system/messages/tools`, optional `event.result = { summary }` to skip the model request) and verify each hook fires with a positive-control log line. Contributor-provided example, not user-confirmed.
2. Ensure plugin export has an `id`.

**Notes:**
By design: no registration-time validation until the feature request lands. For MOSAIC, a startup self-check that each registered hook produced at least one invocation would expose dead hooks. Related: #60021 (report that `context` hooks do not fire for child sessions; closed not reproduced, bot saw them fire for built-in and custom subagents on 2.2.0, so child-session hooks are expected to work).

---

### O2-028: Service start auto-resumes every suspended session and unfinished background job (no age limit, no consent, no off switch)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/59635 (also #56032, the feature request tracking an off switch) |
| **Reported** | 2026-10-16 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.19 through 2.3.0 (reports); current `dev` per bot on 2026-10-16; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | pending close, triaging (#59635); none (#56032) |

**Summary:**
When the shared background service starts (`serve --service`, including the automatic restart after an upgrade, `reason=version-mismatch`), `SessionRestart.resumeSuspendedSessions` resumes every root session whose `time_suspended` is set, plus children of running persisted `job.background` records, injecting a synthetic notice ("The server restarted while you were working. Continue from where you left off without repeating completed work."). The only limit is a per-turn resume cap of 10. There is no age check, confirmation, or config/env/CLI switch (a contributor bot confirmed on `dev`). Sessions stopped by declining a tool call, or cut off by "Interaction cancelled because the location shut down", are also resumed. Reported outcomes: an elevated destructive uninstall script run 7 hours after last user interaction; an unexpected GitHub review posted; stalled subagent sessions burning tokens; a runaway 1,660-step resume (#55076).

**Impact on Orchestration:**
A MOSAIC run killed or interrupted mid-turn can wake up later, on any service start, and continue with full tool access outside the orchestrator's control, possibly writing into working trees while a new run or another server is active. Two servers sharing one data directory can double-execute a live session (#51213, comment on #59635: claims carry no owner). Declined/rejected tool calls do not stay declined across a restart.

**Evidence:**
- Contributor bot read `restart.ts` and confirmed unconditional resume with no age check; feature request #56032 acknowledged ("maintainers will consider it"), no fix.
- At least four independent user reports on #59635/#56032/#63802 (macOS, Windows, Linux), with DB and log excerpts; #59635 was bot-flagged as duplicate of #56032 but remains open and carries the severity detail (destructive actions, shared data directory) the feature request lacks.
- Related: #55076 (killed `opencode run --standalone` resumed by service, runaway), #54180 (declined call recorded as shutdown, resumes), #51213 (service resumes sessions a live second server runs), #36347 (pending prompts lost at shutdown), #58572 (restart resumes skip `session.prompt` plugin hook). #63802 closed as duplicate of #59635 (read; matches).

**Workaround(s):**
1. Avoid restarting the background service while sessions or background subagents are in flight; export then `DELETE /api/session/:id` orphaned or stale sessions (comment on #59635; destroys history).
2. Run orchestrated work in `opencode run --standalone` or a dedicated server with its own data directory (separates provider logins too); note #55076 reports standalone runs that were killed can still be resumed by the shared service.
3. Use ask/deny `permissions` rules for destructive shell patterns (reporter-applied mitigation, limits blast radius only).
4. Unsupported: SQLite trigger nulling `session_v2.time_suspended` (reporter of #56032 noted it as fragile).

**Notes:**
No fix expected soon; treat as permanent behavior until an off switch ships. MOSAIC should assume any unfinished session can resume on a later service start. Cross-reference O2-003 (declined tool call recorded as "shutdown"; rejected permission leaves a background child stuck) and O2-025 (stalled steps that later wake on restart).

---

### O2-029: Compaction fails terminally with "Compaction summary reached the output token limit" / "Compaction produced no summary" (reasoning models, tight windows, ever-growing summaries)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/56862 |
| **Reported** | 2026-10-18 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.32, 2.1.0, 2.3.0 (confirmed by separate commenters); `dev` ba06ff9e0 on 2026-10-19; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
Compaction ends in `compaction.failed` when the summary stream hits its output cap (`finish_reason: length`): `Compaction summary reached the output token limit` in primaries and `Compaction produced no summary` in subagents (reasoning consumed the whole output budget). Triggers reported: local models with small windows (prompt plus summary output fill the window exactly; OpenCode's token estimate undercounts dense code), thinking-by-default models, and very long sessions where the checkpoint rewrite grows to about 1:1 with its input (29-day session: summary 9.7k chars to 190k chars, then a failure at the 60k output cap). Two consecutive failures kill the drain loop and the session needs manual export/trim/import. A bot reproduced it on `dev`; fix PR #60947 (retry truncated summaries) is open.

**Impact on Orchestration:**
A long-running primary or a subagent reading large files can lose the ability to compact and then stall or overflow. In subagents it appears as a failing compaction while the parent waits. Output-length failures on reasoning models are not recoverable by the harness today.

**Evidence:**
- Bot reproduction with simulated model and script; at least five independent commenters (LM Studio, llama.cpp, a self-hosted DeepSeek variant) across 2.0.32, 2.1.0, 2.3.0, with DB rows and server logs; 3 thumbs-up; open fix PR #60947.
- A commenter reports the shrink-retry ladder `[0.7, 0.5, 0.35]` in the 2.1.0 binary is wired only to a different failure path (comment truncated in this pass; not independently verified). Another notes usage records show `reasoning = 0` even when all output was reasoning, hiding the cause.
- Related: #54797 (token estimate undercounts dense code on local models), #50187 (compaction ignores the context the local server actually serves), #57187 (closed as duplicate; its comment carried the llama.cpp numbers, content appears in this thread).

**Workaround(s):**
1. Raise `compaction.buffer` (about 20% of the window, absolute tokens) so compaction starts with headroom for the summary. Confirmed by two users as necessary but not sufficient (still fails when the summary itself outgrows `limit.output`).
2. Set `agents.compaction.model` to a variant with a large `limit.output` and a non-thinking or low-reasoning setting (partly confirmed, summaries can still grow unboundedly).
3. Recover a dead session by export, trim, re-import (commenter, manual).

**Notes:**
Related compaction entries: O2-020 (template mismatch), O2-023 (inflated usage loop), O2-026 (unreachable compaction model). MOSAIC-relevant when using local or thinking models, or any agent session lasting days. Check whether PR #60947 shipped in 2.3.x.

---

### O2-040: shell tool fails to spawn Microsoft Store PowerShell 7 on Windows (0xC0070005, pipe mode)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/63892 |
| **Reported** | 2026-11-14 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 (regression vs 2.2.7 per reporter; 2.3.2 not re-verified) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, area/service, version/v2, platform/windows |

**Summary:**
Since 2.3.0 local shell commands run through the `opencode-pty` daemon (0.5.2). On Windows, when the configured shell is Microsoft Store PowerShell 7 (`C:\Program Files\WindowsApps\Microsoft.PowerShell_7.x...\pwsh.EXE`), every `shell` tool call fails at spawn with `os error -1073283067` (0xC0070005) before any command runs. The daemon's job-less retry only handles raw error 5, so the fallback is skipped. A commenter traced it to a Windows-side AppX/Helium container job nesting rejection on an Insider build; an open community PR (#64137) widens the retry.

**Impact on Orchestration:**
Every shell call from every agent fails on affected Windows machines, so subagents cannot run builds, tests or git. The failure is at spawn time, so the agent gets an error rather than output, and the model may loop retrying.

**Evidence:**
- Reporter provides an isolated Win32 CreateProcessW repro (JOB_LIST attribute fails, plain creation succeeds) and source pointers.
- Bot (no Windows available) confirmed from code that the retry only matches error 5; second user confirms Windows PowerShell 5.1 works on 2.3.0.
- Open fix PR #64137 linked. No maintainer comment; 3 thumbs-up.
- Third commenter observed the same job-nesting failure only on a Windows 11 Insider build (10.0.26340), so the trigger may be Insider/Store-package specific.

**Workaround(s):**
1. Select Windows PowerShell 5.1 (or presumably a non-Store MSI/ZIP pwsh 7, untested) as the shell. Confirmed by two users (5.1).

**Notes:**
Related introduction of the pty daemon: #63196 / #63346. Differs from closed #62512 (Store alias returning empty output). Materiality depends on MOSAIC machines using the Store build of pwsh 7. Checked 2026-11-18: PR #64137 is still open (not merged, blocked on compliance), not in the v2.3.1 or v2.3.2 release notes, so the bug stands on v2.3.2.

---

### O2-041: Denying `execute` exposes every MCP tool directly on the tool list (prompt size explosion)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/63374 |
| **Reported** | 2026-11-11 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.0 onward (measured on 2.2.6; contributor reproduced on dev 61325d1) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/core, version/v2, platform/linux |

**Summary:**
When an agent's permissions deny the `execute` action (Code Mode), OpenCode lists every MCP tool of default-`codemode` servers as a normal tool with its full JSON schema (plus server instructions). This is deliberate since PR #59315 (so plugin tools stay reachable). Closed as completed by maintainer rekram1-node via docs PR #63566, which only documents the behavior; the behavior itself was not changed.

**Impact on Orchestration:**
A restricted read-only subagent (execute denied) starts with a far larger context than permissive agents: 12k to 29k prompt tokens with 42 MCP tools, 32k in a second user's reader agent versus about 3k for peers, and 863k tokens with 1,274 tools, causing an auto-compaction loop. This is a silent context cost that multiplies across fan-out and can trigger premature compaction.

**Evidence:**
- Reporter token measurements; contributor bot reproduced (simulated model) and confirmed the code does this on purpose; second user independently reports 32k start context.
- Maintainer closed the issue with a docs-only PR, so by-design.

**Workaround(s):**
1. Also deny the MCP tool names on that agent (e.g. deny `*_*`), then restate other underscore-named actions such as `external_directory`. Reporter-confirmed (8 tools, 8,974 tokens).
2. Set `codemode: false` on MCP servers that should not go through Code Mode, or keep `execute` allowed and restrict via other rules (not tested in thread).

**Notes:**
Docs previously claimed denying `execute` removes MCP tools; docs now corrected. A proposed middle ground (hide default-codemode MCP tools when execute is denied) was floated but not committed to. Relevant to MOSAIC agent frontmatter that denies tools on read-only agents.

---

### O2-002: 2.3.0 made `subagent` always-background; no blocking/foreground option, parent wakes per child event

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/64073 (also #64096, closed duplicate-style feature request) |
| **Reported** | 2026-11-15 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 onward (PR #63382); 2.2.x still had `background: false` |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/core, version/v2 |

**Summary:**
PR #63382 (2.3.0) made every `subagent` call return immediately and removed the `background` parameter; the child's result arrives later as a synthetic `<subagent sessionID=... state=...>` message that starts a new parent turn. The tool description tells the model to end its turn and not poll. No blocking call exists. Filed as a feature request with 19 comments, no maintainer reply beyond the bot noting maintainers "will consider this"; the change is deliberate (hence Limitation).

**Impact on Orchestration:**
Sequential or gated delegation (wait for all implementers, then build; wait for research, then plan) can no longer be expressed deterministically. Parents either end the turn (one wasted inference per dispatch and per `state=running` notice) or keep talking and wander into troubleshooting. Reporters see non-deterministic early hand-backs, children's reports triggering parent turns and even compaction, and with nesting (A -> B -> C) each B dispatch/completion wakes A twice (one reporter measured 31 wake-ups, about 22 min wasted over about 30 h). On single-slot backends the parent and child contend for the same inference slot. Headless/unattended runs are especially exposed.

**Evidence:**
- Contributor bot confirms #63382 removed the option; 10+ independent users report the same operational effect; reporters measured time and token costs.
- Related open items: #57157 (plugin hook to suppress subagent notifications), #63894 (background reports wake the parent for hidden/internal turns).

**Workaround(s):**
1. Roll back to 2.2.7 (`opencode upgrade 2.2.7`), which still has `background: false` (reported by several users).
2. Prompt-level: instruct parents that a dispatch returns at once, to do nothing else, end the turn, and treat the child as done only when its result file exists (soft steering; reporter calls it flaky, models ignore it).
3. A proposal in the thread (`wait: true` on the tool, or a `subagents.wait` config) is not implemented.

**Notes:**
By-design per PR #63382; MOSAIC cannot rely on a blocking subagent call on 2.3.x and must design orchestration around child result messages (file-based completion checks, turn-ending after dispatch). See O2-001 for resume problems and the V2 AnalysisReport for the tool description. Update (run 1 follow-up): #48826 (child ends its turn while its own background shell/subagent runs; parent got `completed` with an early reply and the real final report was stranded) was closed completed on 2026-11-11 via PR #63209, which reports to the parent on every child idle (also fixes #54658, resumed-after-interrupt reply lost; #52456 dup, #50751 closed not_planned for a plugin tool). Residual quirk: `state="completed"` can arrive at an intermediate idle and `cancelled` can precede a later `completed` for the same child, so MOSAIC should treat a result file or final report, not the first `completed` notice, as the completion signal. See O2-055 for the unsuppressible report/wake side of the same change.

---

### O2-042: Code Mode `execute` and MCP calls have no effective timeout; one lost MCP reply hangs the session or background subagent for hours

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/54220 (also #64233, closed as duplicate) |
| **Reported** | 2026-10-09 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.26 through 2.3.0 (2.2.0 and 2.3.0 confirmed by later reports; 2.3.2 not re-verified) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced (#54220); type/bug, area/core, version/v2 (#64233) |

**Summary:**
Code Mode `execute` is created without `limits.timeoutMs`, plugin tools have no deadline, and the MCP `timeout.execution` defaults to 12 hours. A slow or never-answered call keeps the step busy until the user interrupts, which ends the entire step. In #64233 an MCP transport warning ("Received a response for an unknown message ID") dropped the reply of a `zai-web-search` call inside `execute`; the tool part stayed `running` for 7-12 hours in background subagents (4+ instances in 2 days). A fix PR (#59217, bound Code Mode execution time) was closed without merge; #54220 remains open.

**Impact on Orchestration:**
A background subagent that hangs on one MCP call silently never reports back, so a parent waiting for child results (file or message based) stalls indefinitely with no error. Batching several MCP calls inside one `execute` increases exposure. Nudging the session does not reliably help.

**Evidence:**
- #54220: bot reproduced on 2.0.26 and v2 source with a never-returning codemode tool; a second user confirms still present on 2.2.0 with an MCP server that accepted but never answered.
- #64233 (verified myself, closed by the bot as duplicate): different trigger (dropped MCP reply), but the same missing-deadline root cause; bot itself says the 12-hour MCP default explains the gaps. Related per bot: #53433 (stuck `running` parts only reaped on next run), #51291, #56741, #61354.
- No maintainer acknowledgment in the threads; contributor-authored issue, proposal discussed.

**Workaround(s):**
1. Lower the MCP execution timeout so a dropped reply ends as an error: `{"mcp": {"timeout": {"execution": 600000}}}` (or per server). Suggested by the contributor bot; not confirmed by the reporter.
2. Reporter practice: hard tool-call budgets per subagent, no batching of MCP calls in one `execute`, cancel and redispatch after about 10 minutes of silence instead of nudging.

**Notes:**
Orchestrator-side idle watchdogs are the only reliable mitigation until a deadline lands. See O2-041 for the related Code Mode / MCP exposure behavior.

---

### O2-003: Declined tool call recorded as "shutdown"; a rejected permission in a background subagent leaves the child job stuck and never notifies the parent

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/54180 (subagent variant reported in #64032, closed as duplicate) |
| **Reported** | 2026-10-09 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.12 through 2.3.0 (subagent variant reproduced on 2.3.0; 2.3.2 not re-verified) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
When a permission request or question is rejected, the step interrupts with no reason and `SessionExecution.terminal` defaults it to `shutdown`: the execution claim is kept, `jobs.cancel` is skipped and no idle marker is projected. Primary effect: the declined turn is resumed after a server restart. Subagent effect (#64032, verified as a distinct and more severe symptom): if a background subagent's `ask` permission is rejected without a message, the child's job stays listed as `cancelled`, no "Subagent cancelled" report is delivered and the parent is never woken. Issue is assigned and open; fix PR #54344 was closed unmerged, replacement PR #58260 is open.

**Impact on Orchestration:**
A parent that waits for a child result message can wait forever when a child hits a rejected permission (headless clients that auto-reject without a message trigger it; `opencode run`'s own rejects carry a message and avoid it). Clients/scripts that wait for background jobs (including `opencode run`, which counts cancelled jobs) can hang. After a restart a declined action may be re-attempted.

**Evidence:**
- Bot reproduced on 2.0.26 and `v2` (restart-resume part); assignee `nexxeln`; two fix PRs linked.
- #64032 reproduced deterministically on 2.3.0 with the repo simulation package (read from source with line references); the bot confirmed the same code path and closed it as a duplicate, but the parent-never-notified symptom was added to #54180 as a comment (verified).

**Workaround(s):**
1. Avoid `ask` rules for subagent-executed tools in unattended runs (allow or deny explicitly); always include a message when rejecting a permission via API.
2. Restart the server clears the stale cancelled job row (reporter note).

**Notes:**
Related: O2-022 (`opencode run` never exits while a child job is outstanding; likely same waiting logic). #54344 (closed unmerged).

---

### O2-004: `subagent` call without (or with a mistyped) `sessionID` silently starts a new context-free child instead of failing

| Field | Value |
|-------|-------|
| **Classification** | Quirk |
| **Source** | https://github.com/anomalyco/opencode/issues/62793 |
| **Reported** | 2026-11-08 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.x, 2.3.x (behavior confirmed in 2.3.0 code by contributor bot; 2.2.7 reported by commenter) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (feature request) |

**Summary:**
The `subagent` tool both starts and continues children, distinguished only by optional `sessionID`. A follow-up that omits `sessionID`, or spells it `sessionId` (unknown parameters are ignored), succeeds and starts a duplicate child with no context while the intended child never gets the message. Contributor bot confirmed the behavior from code and says maintainers "will consider" splitting the tool, a guard or a warning; no implementation committed. Opposite slip (models filling `sessionID` with bogus values) is tracked upstream as #55504.

**Impact on Orchestration:**
Duplicate workers run on the same workspace with a context-free prompt, wasting tokens and potentially conflicting file edits; the follow-up instruction is lost. Hits orchestrators that manage several background children across long sessions, and compounds with O2-001 (resume failing) when models retry.

**Evidence:**
- Reporter saw it repeatedly in one long session; a second user reproduced the mistyped-key variant twice on 2.2.7 on Windows; contributor bot confirms a call without `sessionID` always starts a new child.
- #63298 (2.2.5, DeepSeek model, Windows): same `sessionId` typo spawned four duplicate children; reporter found the cause and closed it invalid. Upstream #59658 tracks telling the model when unknown tool arguments are dropped.
- Related: #56249 (possibly same slip), #59566 (request for tools to inspect/manage child sessions).

**Workaround(s):**
1. Plugin guard using the `tool` `execute.before` hook that rejects a new `subagent` call whose description matches a still-running child (code in the thread by the contributor bot, not run in a live session; does not catch the mistyped-key variant). A reporter branch (ioquatix/opencode `subagent-duplicate-guard`, `new: true` override) is not merged.
2. Orchestrator-side: list running children before dispatching follow-ups; keep the exact `sessionID` key and value verbatim in agent prompts.

**Notes:**
Feature-request-labelled upstream but included because it causes silent duplicate dispatch in orchestration. O2-002 note: 2.3.0 always-background mode also produces early hallucinated parent answers per #63785 (single reporter, Opus 4.6 plan agent, bot could not reproduce, model-dependent; not tracked separately).

---

### O2-043: Code Mode `execute` silently returns `null` when the program body is (or ends in) an uncalled function

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/57484 (also #62923, closed as duplicate) |
| **Reported** | 2026-10-20 |
| **Last Activity** | 2026-11-10 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.1.1 through 2.2.6 reported (2.3.2 not re-verified) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | #57484: none (closed not_planned); #62923: pending close, triaging |

**Summary:**
`execute` treats `code` as a program body; the final top-level expression becomes the result. If the model passes `async () => {...}` or declares `async function main(){...}` without calling it, nothing runs and the result is silently `null` (no error, no log). Maintainer rekram1-node closed #57484 as expected behavior (not planned); a contributor-proposed diagnostic ("program returned a function") is not confirmed merged. #62923 (verified: reporter retested and agreed it was a usage error, same cause) confirms it on 2.2.6.

**Impact on Orchestration:**
Agents using Code Mode MCP tools can believe a call succeeded (or that MCP is broken) when no tool call was made. In one user's sessions 71 of 816 `execute` calls returned `null` for this reason, so it is model-driven and recurring. MCP results are also wrapped as `{ success, data }`, which is easy to misindex.

**Evidence:**
- Maintainer statement of expected behavior with workaround; two independent reporters hit it; session-DB statistics in #57484.
- Both threads mention related #48910 (missing `return` gives `null`) and #50577 (request for clearer Code Mode errors).

**Workaround(s):**
1. Write the program as a body with an explicit `return`, or invoke wrappers: `return await (async () => { ... })()`. Maintainer-provided. Put this rule in agent instructions for agents that use Code Mode.
2. Deny `execute` and restrict MCP via per-tool permissions only if Code Mode is not needed (note O2-041 context cost).

**Notes:**
By design: MOSAIC agent prompts that use MCP tools on V2 should state the body-with-return rule, and orchestrators should treat a bare `null` from `execute` as a failed call rather than an empty result.

---

### O2-044: `edit`/`write`/`patch` silently overwrite a concurrent file change made while the call waits for permission (stale snapshot)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/60509 (edit vs write) and https://github.com/anomalyco/opencode/issues/60595 (patch vs external edit) |
| **Reported** | 2026-10-30 |
| **Last Activity** | 2026-10-30 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Reproduced on dev at 2026-10-30 (about 2.2.x); both issues still open, fix PRs #60552 and #60634 still open and unmerged as of 2026-11-18 and absent from the v2.3.1 and v2.3.2 release notes; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced, severity:high (both) |

**Summary:**
`edit` and `patch` read the file and build the replacement before asking permission, then write that stale content after approval without checking whether the file changed meanwhile. In #60509 a `write` to the same file completes while an `edit` waits for approval, and the edit later overwrites it (both report success). `edit` holds the transaction lock but `write` does not take it. In #60595 an external editor save during a pending `patch` approval is lost the same way, even on a different line.

**Impact on Orchestration:**
With parallel subagents touching the same file (or a human editing alongside an agent), one agent's successful write can vanish with no error, producing silently wrong deliverables. The window is the approval wait, so fully pre-approved headless runs shrink the window but the missing lock between `edit` and `write` remains.

**Evidence:**
- Both issues have deterministic repro scripts that a contributor bot independently reproduced on later dev builds; the bot's code analysis confirms the cause. Labels reproduced / severity:high.
- Two open fix PRs linked (#60552 take the write tool's transaction lock; #60634 reject patch edits when the file changed during approval). No maintainer comment.
- Related and already fixed: #53512 (two parallel `apply_patch` calls on one file, serialized by #53516).

**Workaround(s):**
1. Avoid parallel writers on the same file: assign disjoint files per subagent and pre-approve edit/patch/write permissions so no approval wait exists (inferred from root cause; not confirmed in thread).
2. Re-read the file after write/edit when other writers might exist.

**Notes:**
Distinct from the 2.x `write` truncation report #60561 (closed, 2.2.2, no repro). Re-check on current version before relying on this.

---

### O2-045: `glob` tool (and `grep` `include`) never match absolute patterns: "No files found" for files that exist

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/64296 |
| **Reported** | 2026-11-16 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.7 (bot repro), 2.3.0 code unchanged; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/core, version/v2, platform/linux |

**Summary:**
`glob` passes the pattern unchanged to `rg --files --glob=<pattern>` with the search directory as cwd, so a leading `/` anchors it to that root and an absolute pattern (e.g. `/workspace/src/**`) can never match. The tool returns "No files found". The `include` glob of `grep` has the same flaw per the reporter (not tested by the bot). Contributor bot reproduced on released 2.2.7 and dev; a community member volunteered a fix; no maintainer comment yet.

**Impact on Orchestration:**
Models commonly pass absolute paths. The false empty result makes agents conclude files are missing, so subagents can wrongly report missing inputs or skip work. Notably on Windows, drive-letter absolute paths are likely to hit the same path (unverified).

**Evidence:**
- Reporter root-cause analysis with rg commands; bot reproduced on a real released version with a real `glob` call (simulated model).
- Same report existed on V1 (#46293, closed as not planned in the V1 cleanup); the V2 code path is separate.

**Workaround(s):**
1. Use a relative pattern (`src/**`) or `**` with the `path` parameter set to the directory. Reporter-confirmed, bot-confirmed.
2. Tell agents in their prompts to use relative globs with `path`, or fall back to `shell` listing when glob returns nothing.

**Notes:**
Treat "No files found" from `glob` as unreliable when the pattern was absolute.

---

### O2-046: Generic tool-output truncation keeps only the head, so the tail of large MCP results, skills and logs never reaches the model

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/62718 |
| **Reported** | 2026-11-08 |
| **Last Activity** | 2026-11-08 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | v2 dev at 2026-11-08 (about 2.3.0); default limits `tool_output`: 2,000 lines / 50 KiB |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (bot-marked feature request) |

**Summary:**
When a non-shell tool result exceeds the `tool_output` limit, `ToolOutput.truncate` keeps the first lines only, saves the full output to a file and appends a marker pointing to it. `shell` keeps the end instead. A contributor bot states the head-only behavior is deliberate design and treated the issue as a feature request; PR #62719 (keep head and tail) is open. Models often continue without reading the saved file.

**Impact on Orchestration:**
Final checklists/"do not" rules at the bottom of a large skill file, trailing fields in large MCP results, and errors at the end of build or test output are silently cut, which can make agents miss failures or rules. Likely to matter in test-runner or MCP-heavy subagents.

**Evidence:**
- Reporter plus bot code analysis; no maintainer comment; single report (a V1 equivalent #54148 was closed as not planned for being V1). Related per bot: #35661 (MCP output cut at 50 KB), #54007 (`@` includes share the cap).

**Workaround(s):**
1. Raise limits via config, e.g. `"tool_output": {"max_lines": ..., "max_bytes": ...}` (option exists per the issue; exact keys to verify).
2. Have agents read the saved-output file referenced in the truncation marker, or run commands via `shell` (keeps the tail) and write large outputs to files.

**Notes:**
Treat by-design until PR #62719 lands (still open 2026-11-18, not in v2.3.1 or v2.3.2; the merged #63272 only keeps the preview when the output cannot be saved and does not add a tail). Instruct agents to check for the truncation marker before concluding.

---

### O2-005: No default step, time or cost bound for agents/subagents; `doom_loop` not enforced in V2; subagent fan-out unbounded

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/64733 |
| **Reported** | 2026-11-18 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 (reporter also read 2.3.2 source); 2.x generally |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/core, version/v2 |

**Summary:**
Agents have no step limit unless `steps` is set per agent (built-in `build`, `general`, `explore` have none), there is no cost/token/time budget for a session tree, `doom_loop` is not a V2 core permission action and nothing enforces it, and `subagent` always runs in the background so a primary can spawn repeated waves with nothing bounding the tree. Contributor bot checked the docs and source and agrees this is how V2 works today, treats it as a feature request. Related open requests: #59674 (spend limit incl. children), #59339 (doom_loop, fix PR #61328 open), #27110 (parallel subagent cap), #45442 (subagent identical-call loop).

**Impact on Orchestration:**
In unattended/headless runs, a runaway or over-eager agent or subagent wave runs until aborted manually; the reporter had sessions of 2-3.5 h and about $35 spent in subagents. Aborting the parent did stop in-flight children within about 10 s with no orphans. Mainly a safety/reliability gap for autonomous MOSAIC runs rather than a correctness bug.

**Evidence:**
- Single reporter with local DB statistics; source line references and docs confirmed by the contributor bot. Included as a Limitation because it is an architectural default that MOSAIC must configure around.

**Workaround(s):**
1. Set `steps` on every agent (including `build`, `general`, `explore`) in config or frontmatter; on the final step OpenCode removes tools and asks for a summary (documented, bot-confirmed).
2. Set a spend limit at the provider; add orchestrator-side wall-clock watchdogs and cancel children explicitly.

**Notes:**
Plugins cannot yet refuse a model request before it is sent (per bot, #59674), so enforcement cannot be done fully in a plugin. See O2-002 (always-background) and O2-042 (hang with no timeout).

---

### O2-047: `edit` fails with `path: Missing key` when the model sends the V1 key `filePath`; the error hides the dropped key and failures can persist for a session

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/57051 |
| **Reported** | 2026-10-19 |
| **Last Activity** | 2026-11-06 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.32 reported; bot reproduced on dev 580fcab; production reports from 2.2.x; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
V2 `edit` expects `path`, but many models send V1's `filePath`. OpenCode's input-repair step removes unknown keys before validation, so the error shows `Invalid arguments for tool "edit": - path: Missing key` and "Arguments provided" lists only `oldString`/`newString`, hiding what the model actually sent. A production-scale commenter reports the failure is sticky per session: sessions with pre-upgrade (V1) history keep imitating `filePath`, and every `edit` fails until a new session starts. Same pattern reported for `shell` (`cmd` vs `command`, #49725). Fix PR #57079 (keep dropped tool arguments visible) is open.

**Impact on Orchestration:**
Edit-heavy subagents can burn turns in repeated failed edits (reporter: "constantly happening, different LLMs, different projects"), and agents on resumed or long sessions can effectively lose `edit`. Models retry the same wrong shape because the error does not show the extra key.

**Evidence:**
- Bot reproduced on v2.0.32 and latest dev with a simulated `filePath` call; three independent users show the same error on 2.x.
- Session-store statistics and sticky-per-session behavior from a 625-session install (comment 2026-11-05; also #62130, flagged duplicate by bot, not re-verified).
- Open fix PR #57079 (still open and unmerged 2026-11-18; it only keeps dropped keys visible in the error when a required key is missing, it does not alias `filePath`, and objects with all required keys present still silently drop extras); no maintainer comment.

**Workaround(s):**
1. Start a fresh session rather than resuming one with V1-era history (commenter observation).
2. Instruct agents that `edit` uses `path` (not `filePath`), and fall back to `patch`/`write` after one failed edit.

**Notes:**
A commenter on this issue also claims agent-frontmatter permission rules with an exact (non-glob) `resource` path never match, while wildcard resources do, so exact-path denies are silently unenforced for `opencode run` agents. That claim is single-reporter and not reproduced by anyone else; do not rely on exact-path deny rules without testing, and prefer wildcard resources. Not tracked as its own entry.

---

### O2-006: Agents have no tool to list, inspect or cancel child subagent sessions (cancel only via UI or HTTP API)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/59566 (related: #36423 cancel background subagent, #38966, #59565) |
| **Reported** | 2026-10-27 |
| **Last Activity** | 2026-10-27 (needs re-verification; no activity for 3+ weeks) |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.1.0 through 2.3.x (session tools are only `session_move`/`session_rename`; not re-verified on 2.3.2) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (feature request) |

**Summary:**
The agent-facing toolset has no way to check whether a child is running or stuck, read its messages, fork it, or cancel it. These capabilities exist only in the HTTP API (`opencode api ...`, e.g. `/api/session/{id}/prompt`, `session.interrupt`, `session.fork`) and the UI. Contributor bot says plugins can partly cover status, context read and cancel (a reporter prototype confirmed this on 2.1.0 headless), but not message listing, fork or inbox. Maintainers "will consider it"; no commitment.

**Impact on Orchestration:**
An orchestrator that spawns background children cannot cancel a hung or duplicate child (see O2-004, O2-042, O2-003) or recover its finished-but-unwritten work from inside the agent loop; recovery needs shell calls to the server API or the UI. Since 2.3.0 children are always background (O2-002), the gap matters more.

**Evidence:**
- Contributor bot confirmed the toolset gap and the plugin split; the reporter's plugin test on 2.1.0 (status/context read/interrupt via plugin API) worked, but the plugin has no per-session ownership check. Related #63298 (closed invalid) also hit the lack of list/abort. Single thread, no maintainer reply.

**Workaround(s):**
1. From the shell: `opencode api post /api/session/<childID>/prompt ...`, `GET /api/session/<childID>/message`, `POST /api/session/<childID>/fork`, interrupt via API (confirmed by reporters for prompt/fork/message; cancel via UI close).
2. A custom plugin exposing `session_status`, `context_read`, `subagent_cancel` (prototype by reporter; scoping to own children needs `parentID` check).

**Notes:**
The stuck-child scenario in #59565 (stream cut by a custom gateway with `timeout: false`, continuation hanging) was not given its own entry: custom-backend trigger, reporter disabled the provider timeouts. Re-check #59565 later for a stock-provider repro.

---

### O2-007: Client replaces the shared background service after three 2 s health-probe timeouts (~6 s busy), aborting every in-flight tool call, session and background subagent

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/54181 (also #52049, #51216, #63357, #61061, #58991; open fix PR #58130) |
| **Reported** | 2026-09-24 (first report in set); most recent 2026-11-12 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.16 through 2.2.7 reported (Windows, macOS, Linux); client rule confirmed unchanged on `dev` by a contributor on 2026-10-28; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | severity:high (#54181); pending close (#52049, #51216); type/feature, area/service (#63357) |

**Summary:**
The client (TUI, CLI, Desktop) probes `GET /api/info` with a 2 s timeout; after three consecutive timeouts it SIGTERM/SIGKILLs the shared `serve --service` process and spawns a replacement (log: `Background service is unresponsive; recovery cannot preserve persistent terminals`). Any busy period of about 6 s on the service main thread triggers it while the process is alive: snapshot capture on huge untracked trees, watcher re-subscribe churn on skill directories, a large `opencode.db`, a plugin doing synchronous work in a hook (`@tarquinen/opencode-dcp` in #63357), host CPU saturation, or resume from sleep. A separate 45 s event-stream idle watchdog (`Event stream stalled`) only reconnects the stream, but the stall logs usually precede the replacement. Every replacement aborts in-flight tool calls (`Tool execution interrupted`) and background subagents; sessions then resume via the restart notice (see O2-028). Multiple clients detect the stall in the same millisecond and spawn replacements together (6-14 concurrent `serve --service` starts seen), producing a restart loop every 1-3 minutes. Contributor-bot code reading confirms the rule; the individual trigger fixes landed or are in flight, but the replace-on-three-timeouts rule itself is unchanged. Open community fix PR #58130 (30 s recovery window, liveness classification) is not merged as of 2026-11-11; related PRs #52733, #52963, #53387, #59043 were closed unmerged.

**Impact on Orchestration:**
Any long-running or fan-out MOSAIC session served by the shared service can lose all running tool calls and all background subagents at once, with no failure message beyond "server restarted while you were working", and then burn time re-running the same expensive work (which can trigger the same stall again). In #51216 a loop ran roughly once a minute for hours. Because it kills every session on the service, one heavy session or one slow plugin hurts all of them.

**Evidence:**
- Many independent reporters on Windows, macOS and Linux across #54181, #52049, #51216, #63357 with log excerpts and counts (69 spawns in 8 days; 111 starts in a day). Contributor bot read the client code (`packages/client/src/effect/service.ts`, `requestTimeout: 2_000`, three probes) and reproduced on 2.0.16 with a 200k-file untracked tree.
- Triggers differ per thread and are not all fixed: #51216 snapshot trigger fixed on `dev` by 2026-10-28 (bot repro); #52049/#53416 (watcher churn) closed as circular duplicates (#52049 to #53416 to #53393/#54114; fix PR #58672 open); #63357 trigger traced to a third-party plugin blocking the event loop; #61061 (macOS Desktop, 2.2.5) respawned a healthy service every ~60 s until OS reboot, bot could not reproduce, single report; #58991 (Windows, 2.1.0) four restarts around concurrent `git worktree add`, bot could not reproduce, single report.
- #54181's macOS "port poisoning by TIME_WAIT" analysis (sticky port 49374, server-side 5 s keep-alive close) is a plausible separate cause, reporter-derived, not maintainer-confirmed. The claim that sessions vanish from `opencode.db` after a kill was contradicted by a bot test (sessions are persisted before the create response); treat the data-loss claim as unconfirmed. A related claim of `session_message.seq` UNIQUE collisions after rapid respawns is tracked upstream in #53146 (not read here).

**Workaround(s):**
1. Keep the service responsive: avoid a stray `.git` above the workspace and huge untracked trees (add `.gitignore` entries or `"snapshots": false`), set `OPENCODE_DISABLE_FILEWATCHER=1` when skill-directory watcher churn is suspected (reported effective in several threads), and remove plugins that do synchronous full-history transforms in `context` hooks. Reporter-confirmed in multiple threads.
2. Run unattended work in `opencode run --standalone` or a dedicated server with its own data directory, so a different client's probes cannot kill it (inferred; see O2-028 for the caveat that the shared service can still resume sessions).
3. Do not run the Desktop app and the TUI/CLI against the same service concurrently (reporter observation in #52049).
4. After any restart, treat in-flight tool results as possibly lost but side effects as possibly completed (#58991: `git worktree add` finished while the tool reported aborted); reconcile with `git worktree list` and `POST /api/worktree/refresh`.
5. Unsupported shim approach from #52049 (a wrapper that adopts an existing listener and refuses `service restart|set|unset`) kept a daemon alive 20+ minutes (single reporter).

**Notes:**
Root problem is that liveness is judged by a request on the same busy loop. No setting makes the idle or probe limits configurable today (a contributor-bot reply on #63357 treats configurability as a feature request). Cross-ref: O2-028 (everything is resumed after the restart), O2-021 (huge work tree stalls), O2-025. Related, not individually tracked: #58111 (sleep/resume kills healthy service, PR #58130), #54267 and #58389 (no eviction of an old listener, commands spawning a second service), #37795/#48588 (older watchdog kill loops). Unexplained session loss reports #63218 (Windows 2.2.7, `opencode.db` recreated empty with `database schema bootstrap` after restarting following an API-key auth failure, happened twice for the reporter, closed not_planned, bot could not reproduce) and #62659 (whole data dir recreated empty on service bootstrap, 2.2.5/WSL, closed not_planned, contributor says the bootstrap line is a consequence not a cause and nothing deletes the dir) and #63013 (Desktop 2.2.6: active session and child deleted after connecting a provider, open, trigger unproven, no code path found by the reporter) are single-occurrence, unreproduced and were not given their own entries; if sessions vanish after a service replacement, check the DB and use `POST /api/experimental/session/import` from a backup.

---

### O2-008: V1-to-V2 session import is a one-time latch: V1 sessions and messages written after the first V2 run never appear in V2 (404, invisible history)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/50130 (also #62395, #64439, #58840, #60773, #59517 closed as duplicates; related #57080, #57000, #49827, #58846) |
| **Reported** | 2026-09-20 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.3 through 2.3.1 reported; contributor bot reproduced on 2.0.10 and 2.2.0 and confirmed the code on `dev` 62ae125; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
V2 imports V1 history from the shared `opencode.db` exactly once; afterwards `kv['migration.v1-v2']` is `{"phase":"completed"}` and `run()` returns immediately. Any session a V1 client creates after that (and V1 messages added to already-imported sessions, #57080/#57000) stay in the legacy `session`/`message`/`part` tables only: `GET /api/session/{id}` returns 404 `SessionNotFoundError`, the V2 session list is missing them and desktop tabs break. Data is not deleted. Verified myself: #62395 (closed as duplicate by the bot) has the same root cause, with 37 legacy-only sessions including 16 subagent children; #58840 is also the same latch. Fix PR #54272 (watermark re-import) was auto-closed on 2026-11-10 without merging; #57080 and #58846 (repair) remain open. Latest comment (2026-11-18, v2.3.1) says `run()` still returns early.

**Impact on Orchestration:**
Matters only when V1 and V2 clients write to the same OpenCode data directory (the migration guide recommends running V1 alongside while validating V2, which a MOSAIC machine with both harnesses installed would do). Session IDs recorded by an orchestrator or log tooling for V1-created sessions will 404 from V2, resume/export/`opencode api` lookups fail, and a long child transcript that V1 appended to after import is silently truncated in V2 (one reporter: 308 legacy messages, 4 projected).

**Evidence:**
- Contributor-bot reproduction script (V1 1.18.31 plus V2, shared DB) shows the 404 after the second V2 start; at least eight independent reporters across macOS, Windows, Linux (counts from 5 to 1,406 stranded sessions).
- Several users recovered without data loss; no maintainer fix. The same DB can also carry the related `project_id='global'` shape (#49827, fix #56322 merged in 2.2.0/2.2.5 for fresh imports, #58846 open for already-migrated rows); a SQL diagnostic distinguishing the shapes is in the thread.

**Workaround(s):**
1. Do not run V1 and V2 against the same data directory after the first V2 start; use separate `XDG_DATA_HOME` per harness (inferred from the root cause, not stated in the thread).
2. Recover stranded sessions: back up `opencode.db`, build exports with `transformSession` from the published `@opencode/core` package and import through `POST /api/experimental/session/import` (preserves IDs; stamps `time_updated` to now). Multiple users report success.
3. Alternative (original reporter): after backup, set `kv['migration.v1-v2']` to `{"phase":"sessions"}` and boot a V2 server so the importer resumes. Another user warns the 2.0.27 converter deletes existing V2 message rows before rebuilding, so this can overwrite V2 history.

**Notes:**
Treat as open until a watermark or one-way-cutover change ships (#54272 closed, #60083 proposes an explicit cutover). Never act on a V1 session ID from V2 without checking `session_v2`.

---

### O2-009: A starting `serve --service` sweeps sessions another live server is still running (including `opencode run --standalone`), so two servers drive one session

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/51213 (open fix PR #58929; earlier PR #51215 closed as stale) |
| **Reported** | 2026-09-24 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.15 through 2.3.0 (latest comment 2026-11-18 on 2.3.0, says 2.3.1 and 2.3.2 release notes do not mention a fix); bot reproduced on 2.0.16 and `dev` 9adc42b |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
The managed service's boot sweep (`packages/core/src/session/execution/restart.ts`) treats every unreleased execution claim in the shared `opencode.db` as orphaned; claims store only a timestamp, so a live owner looks like a dead one. When a second server (an app's bundled `opencode serve`, `opencode run --standalone`, another XDG-separated service on the same DB) is mid-turn and a `serve --service` starts, the service marks the live server's in-flight tool calls "Tool execution interrupted", posts "The server restarted while you were working", and starts its own step. Both servers then alternate steps on one session; the real process (for example a `sleep` or a long `shell` call) keeps running while the model is told it was interrupted and repeats it. Triggers observed: starting plain `opencode` (TUI starts a service 70 ms later), an auto-update restarting the service, an agent running `opencode mcp list` (which auto-starts the service), `opencode service restart`. Contributor bot reproduced with a mock provider and a repro script.

**Impact on Orchestration:**
A headless MOSAIC worker started as `opencode run --standalone` cannot be bounded by its caller while any `serve --service` shares the data directory: one reporter's standalone worker was taken over for about 9 minutes, the service's copy ran outside the worker's environment and the caller's timeout process group, and made the final commit. Duplicate tool executions in one working tree, Anthropic request failures from split tool calls ("system updates cannot split a local tool call from its tool result", `messages.N.content: Invalid input`), duplicate `question` prompts, flipping Code Mode tool catalogs (437 catalog notices in one session), and a resumed turn running in the other server's tools, cwd and sandbox.

**Evidence:**
- Five independent reporters on macOS and Linux across 2.0.15-2.3.0 (T3 Code, OpenChamber, `run --standalone` worker); bot reproduction (script in the thread); three fix PRs: #51215 and #60605 closed unmerged, #58929 (records owner PID with the claim, skips claims whose PID is another live process) open as of 2026-11-14 and not part of a release.
- Related per thread: #51784 (recovery loses the client environment), #51764 (tool-result lowering throws), #56127, #53146.

**Workaround(s):**
1. Disable the managed service so CLI commands use a private `serve --stdio` that never sweeps: put `{"disabled": true}` in `~/.config/opencode/service.json` (reporter-verified on 2.0.24: one `sleep`, one output line, no notice). Only `opencode service start/restart`, `pair` and `serve --service` still start it.
2. Use a separate data directory (`XDG_DATA_HOME`) per concurrently running server so they never share `opencode.db`.
3. Do not run `opencode`/`opencode mcp ...` from agents in a session served by another server; never start a TUI while a headless worker is mid-turn on the same data directory.

**Notes:**
Distinct from O2-028 (auto-resume of suspended sessions has no off switch) and O2-010: here the owner is alive and both continue. A related plugin-API request (#58572, awaited preflight hook because restart-resume bypasses `session.prompt`, so plugins that seed per-agent state at admission start the resumed request empty and fail closed; open feature request, bot-confirmed the bypass) belongs with O2-027 for plugin authors. Relevant to MOSAIC mainly because harness behavior differs between a standalone run and a shared service.

---

### O2-010: Service resumes a killed `opencode run --standalone` session without the run's permissions or step limit (runaway steps, denied commands execute)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/55076 (fix PR #55174 closed unmerged) |
| **Reported** | 2026-10-13 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.18 and 2.0.26 reported; bot reproduced on 2.0.26 and `dev` c5aabae; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced, severity:high |

**Summary:**
When an `opencode run --standalone` process is killed mid-turn (timeout, crash), its session stays "working" in the shared DB and the next `serve --service` start resumes it ("The server restarted while you were working..."), with no age limit and again on every later service start. The resumed turn does not carry the config the run was started with (`OPENCODE_CONFIG_CONTENT`): the agent `steps` cap and the bash/shell permission allowlist are gone. Bot reproduction: `steps: 3` with `*: deny, ls *: allow` and a killed run; after `opencode service start` the session ran 9 steps and executed all 9 denied `cat` commands, while the same command without the kill is correctly denied. Reporter's production case: two killed review runs resumed 30 hours later ran 5.5 hours unattended, 1,660 steps and about $24; a second commenter (2026-11-12) sees sessions resume days later after a machine crash. No maintainer comment.

**Impact on Orchestration:**
A MOSAIC headless run that times out or is killed can later come back as an unattended session with default (permissive) permissions and no step bound, on any service restart or auto-update, editing a working tree the orchestrator thinks is idle. Restricted agents (read-only reviewers) lose their restrictions on resume. This is a safety bypass, not only a cost issue.

**Evidence:**
- Contributor-bot reproduction script (mock provider, kill, `service start`) with measured step overrun and executed denied commands; reporter billing data and per-tool counts; second user confirming days-later resumption.
- Fix PR #55174 (exclude standalone executions from restart recovery) closed; no merged fix. O2-028 covers the missing off switch; this entry adds the permission/step-limit loss.

**Workaround(s):**
1. Disable the managed service (`{"disabled": true}` in `~/.config/opencode/service.json`, see O2-009) so a killed standalone session is never swept (inferred; reporter of #51213 verified the service stays out of standalone runs).
2. After killing a run, mark its session finished through the API (interrupt) or delete it before any service start (`DELETE /api/session/<id>`, destroys history); do not rely on the process kill alone.
3. Put restrictions in files the harness loads from disk (agent frontmatter / project `opencode.json`) rather than only in `OPENCODE_CONFIG_CONTENT`, so a service-resumed turn still sees them (inferred; not tested in the thread).

**Notes:**
Closely tied to O2-028 and O2-009. Treat any killed headless run as potentially resumable until the session is explicitly terminated.

---

### O2-011: Permission rules match by literal spelling: absolute/`~`/`**/dir/**` path rules never match in-project files (tool reports relative paths), and shell deny rules are bypassed by `/bin/sh`, `env sh`, `FOO=1 sh`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/58775 (path half also #46358, fix PRs #53365 and #55739; confirmed instances #62346, #57603, #61284) |
| **Reported** | 2026-10-24 |
| **Last Activity** | 2026-11-07 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.32 through 2.2.5 reported on `edit`/`write` paths; shell bypass tested on 2.1.4 and `dev` 17b79f4e0c16; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
`Permission.evaluate` compares the rule resource and the tool's target as plain strings. File tools (`edit`, `write`, `patch`) report in-project targets relative to the session directory (`agents/test.md`) and out-of-project targets by absolute path, so an absolute (`/config/**`), tilde (`~/project/x/**`) or `**/findings/**` rule written for a root-level `findings/` directory never matches a file inside the project, and the preceding `edit * deny` (last-match-wins) applies. Shell deny rules match only the literal command text: with `shell * allow` then `shell "sh *" deny`, `sh -c ...` is denied but `/bin/sh -c`, `env sh -c`, `command sh -c`, `FOO=1 /bin/sh -c`, `./sh -c` all run (bot reproduction with the real scanner and `Permission.evaluate`; a commenter adds `bash -c`, `xargs`, `find -exec`, `nohup`, `timeout`, `time` wrappers bypass `rm *` deny). Another consequence: when the last rule for an action is `* deny`, the tool is removed from the agent's toolset entirely instead of permission-checked (maintainer-bot: intended, not documented). Issues #62346 and #57603 (subagent `edit` rejected despite a matching-looking allow) were resolved or explained by this: relative-form rules fixed #62346; #57603's `**/findings/**` does not match a root-level `findings/` per bot. #61284 (subagent `edit` denied, 2.2.1/2.2.5) was closed without a root cause; the reporter found an `edit * deny` at position 55 of the merged ruleset that survived removal of every user agent's deny, which looks like an ordering/composition issue (see O2-013) and was not confirmed.

**Impact on Orchestration:**
Frontmatter `permissions` that MOSAIC relies on to restrict subagents (read-only reviewers, per-directory writers) can be silently inert for in-project paths, either blocking legitimate writes (subagents fail with `Permission denied: edit` and loop) or, for shell deny lists, failing to block the commands the rule meant to. Failures present as `permission.rejected` with no log of which rule matched (permission evaluations are no longer logged in V2, #56247).

**Evidence:**
- Bot reproduction script on `dev`; #62346 reporter confirmed the relative-form rule fixes the write; #57603 reporter reproduced on 2.0.32 across Windows and Linux and two models (not reproduced by the bot with an `ask` rule on `docs/findings/`, where the prompt showed); issue #58775 labelled `reproduced`, open, no maintainer comment; fix PRs #53365/#55739 (paths) open, shell normalization proposed as a PR by the reporter but not yet filed.
- Maintainer-bot: docs describe literal wildcard matching on the parsed command, so shell normalization needs a maintainer decision.

**Workaround(s):**
1. Write path rules relative to the session directory (`allowed-dir/**`), and keep an absolute rule as well for out-of-project writes (confirmed by #62346 reporter). Prefer `**` patterns that also match a root-level directory.
2. Inspect the effective ruleset with `opencode api get "/api/agent?location[directory]=<dir>"` and check order (last match wins).
3. Do not rely on shell `deny` patterns as a security boundary; restrict shell with an allowlist (`* deny` first, then specific `allow`) and a plugin `tool.execute.before` guard.

**Notes:**
By-design status unknown for shell; treat as a standing limitation until normalization lands. "Always allow" on a wrapped command saves the wrapper pattern (`env *`, `bash *`), widening later grants (commenter on #58775; also #64627 for `sudo *`).

---

### O2-012: A subagent's `ask` permission request cannot be answered: hidden in the child session view, child-session permission API returns 404, call hangs up to 3600 s

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/56907 (also #51815 closed as duplicate; related #44007, #55678) |
| **Reported** | 2026-10-19 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.18 through 2.3.0 (latest comment 2026-11-16 on 2.3.0); bot reproduced on 2.0.27 and `dev` 501b15a; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
When a subagent (child session) triggers an `ask` rule (`shell`, `glob`, `external_directory`), the request is not answerable. In the TUI, the permission list is empty for a viewed session that has a parent, so opening the child hides the prompt and `session.permissions: autoaccept` (which reads the same list) does not reply; the shell call shows a spinner. A server-side analysis on 2.1.2 adds that `GET /api/session/<child>/permission` and `POST .../permission/<id>/reply` both return 404 for child sessions, so API clients cannot answer either, and the call ends only when the engine aborts it after 3600 s (`Tool execution interrupted`); one user counted 47 aborted tool parts in 31 sessions, 36 of them near the 3600 s cap. #51815 (verified myself; closed as duplicate of this) additionally notes the request is not recorded as pending (`GET /api/session/<id>/permission` returns `[]`), auto-accept replies only "allow once" and saves nothing, and 1,399 asks vs 14 replies in a day. Open TUI fix PR #56906 does not address the API 404 half.

**Impact on Orchestration:**
Any MOSAIC subagent definition containing an `ask` rule (or hitting a default ask such as `external_directory`) can block a background child for up to an hour with no message; the parent waiting for the child result stalls. In headless `opencode run` the request is rejected immediately instead (see O2-003 for the follow-on stuck-child behavior).

**Evidence:**
- Bot reproduction with a driver script for both prompt and autoaccept modes; reporter-level source analysis of `packages/server/src/handlers/permission.ts` and database statistics; a third user (2026-11-16) says review agents in the background are "blocked for very long time" on 2.3.0.
- Fix PR #56906 (TUI shows prompts in subagent views) is open; however the v2.3.1 release notes state "Permission prompts from a subagent, and from that subagent's own subagents, now appear while you view the subagent's session" (PR #64343), so the TUI half is addressed from v2.3.1 (not tested here). The API half (child-session `/permission` list and reply returning 404, 3600 s abort) is not mentioned in the release notes and remains unverified on 2.3.2. v2.3.1 also changed rejection scope: rejecting a request now rejects only the other pending requests from the same step (relevant to O2-003).

**Workaround(s):**
1. Avoid `ask` in subagent permission sets: use explicit `allow` or `deny` (reporter practice; one fleet bans any `ask` in a subagent's permissions via a CI check). Add server-side allow rules for known paths, e.g. `external_directory /tmp/** allow` (reporter-confirmed, takes effect immediately).
2. Answer the prompt from the parent view (it is visible there per the bot's repro) before opening the child session.
3. Detect the 3600 s hang: orchestrator-side watchdog on child progress (see O2-025).

**Notes:**
Related: #54524 (since 2.0.25 no built-in `external_directory` ask by default), #52846 (pending request lost on instance disposal). O2-003 covers the rejected-permission variant.

---

### O2-013: Global `permissions` config is appended after built-in and plugin-defined agent rules and overrides them (Plan agent read-only guard silently lost)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/63353 (decided as expected behavior in #55127; request to let agents pin rules open as a feature) |
| **Reported** | 2026-11-11 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.7 (reports); design as of 2026-11-12 |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (bot: feature request) |

**Summary:**
Effective permissions are layered with the user's top-level config `permissions` added after the defaults of built-in agents and after rules a plugin sets via `ctx.agent.transform`; last match wins, so a global `edit * allow` removes the built-in Plan agent's `edit * deny` (a write outside the plan directory succeeds) and a global `external_directory * ask` overrides a plugin agent's narrow allow. The contributor bot states this is expected (same decision as #55127): only rules under `agents.<id>.permissions` in config are appended after the global rules. A feature request lets plugin agents pin their rules. Verified by a second user on the Plan agent (2.2.7), who confirmed the per-agent config workaround.

**Impact on Orchestration:**
Read-only or scoped MOSAIC subagent roles that depend on harness defaults or on plugin-registered rules are not protected if the user's global config grants broader permissions; the guard disappears with no warning (#39715 asks for a warning). Combined with O2-011 (matching quirks) the effective ruleset is hard to predict.

**Evidence:**
- Bot reading of the ordering plus effective rulesets from `GET /api/agent`; maintainer-side decision recorded in #55127 (not read in full here); two independent reproductions.

**Workaround(s):**
1. Put guard rules under `agents.<id>.permissions` in config (appended after global rules), e.g. for `plan`: `edit * deny` then `edit ~/.opencode/plan/* allow` (second user confirmed it restores the guard).
2. Always verify the effective ruleset via `opencode api get /api/agent` after changing global permissions.

**Notes:**
By design: MOSAIC agent definitions should express restrictions in per-agent config (or frontmatter, subject to the same merge semantic, not verified here) and not assume built-in defaults survive global allows.

---

### O2-014: Tool calls emitted as DSML/XML text by DeepSeek-family and other lanes run nothing and end the turn silently (no error, no retry); long contexts

| Field | Value |
|-------|-------|
| **Classification** | Quirk |
| **Source** | https://github.com/anomalyco/opencode/issues/50206 (open fix PR #60575; duplicates #58027, #58932 closed; related #57344, #43385) |
| **Reported** | 2026-09-20 |
| **Last Activity** | 2026-10-29 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.22, 2.0.32, 2.1.x, 2.2.0 on `opencode-go/deepseek-v4.1-flash` (also `-free`, `beam-free`); other lanes (OpenRouter MiMo, DeepInfra DeepSeek) report the same on 1.18.x and 2.x; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | severity:high |

**Summary:**
Intermittently, when the model decides to call a tool, the serving lane returns the invocation as DSML/pseudo-XML inside the assistant message text (`<｜DSML｜ invoke ...>`, `<parameter name=...>`, `<dots_function_call>`, `<function=read>`) rather than structured `tool_calls`. OpenCode has no handling for tool-call markup arriving as text: the existing repair path (`experimental_repairToolCall`, `invalid` tool) only runs for already-structured but malformed calls, so no tool runs, nothing is logged at WARN/ERROR, the step ends `idle` (indistinguishable from a finished answer) or, in one report, the empty turn is not persisted at all. Strong correlation with long, tool-heavy sessions (one session went from 0 to 88 occurrences in a day); shortening or compacting stops it for a while. Once leaked markup is in history the model imitates it on later turns. A contributor bot stated it could not reproduce (needs a Go account), confirmed the client has no handling, and notes the issue is in what the served model returns. Fix PR #60575 (recover DSML into real calls in the OpenAI-Chat parser, strip stray markup) is open and unmerged as of 2026-11-15.

**Impact on Orchestration:**
A subagent or primary whose turn ends this way looks as if it completed with an answer, so an orchestrator that treats an idle step as done receives no result file and no error, and may mark the step finished. In a headless run the process exits normally with prose that contains markup. Any harness-agnostic completion check (result file or artifact present) catches it; a text-only check does not.

**Evidence:**
- At least a dozen independent reporters on Windows, macOS, Linux across Go, OpenRouter and DeepInfra lanes; verbatim samples; a reporter statistic of 424/424 requests carrying `tools=12` rules out a dropped tools array; contributor bot confirmation of the missing client handling.
- Cause is provider-side text-to-tool-call conversion (lane-specific); no maintainer fix; community plugin `opencode-plugin-dsml-recover` exists (single-provider testing).

**Workaround(s):**
1. Compact or restart long sessions before they grow very large; reporters say the leak recurs as context grows again (several confirmations).
2. Orchestrator-side: treat an idle step with markup-like text and no tool part or artifact as failed and re-dispatch; require explicit completion artifacts.
3. Community plugin `opencode-plugin-dsml-recover` (unreviewed; tested by its author on DeepSeek V4.1 via DeepInfra only).
4. Use a different model or lane for tool-heavy long sessions.

**Notes:**
Classified Quirk because the source is model/lane output, but the harness silently swallows it, which is why it is tracked. Related: O2-015 (corrupted or empty arguments at long context). #55378 (turn settles silently) and #59567 (leak repeats to output cap) cited in the thread, not read.

---

### O2-015: At very long context (about 190k-450k+ tokens) some models emit corrupted tool arguments (`{}`, injected tag fragments, mixed schemas); rejected calls stay in history and can loop, unknown properties are dropped and the remainder runs

| Field | Value |
|-------|-------|
| **Classification** | Quirk |
| **Source** | https://github.com/anomalyco/opencode/issues/47902 (also #64481 closed not reproduced; related #56665, #36766, #54792, #57051) |
| **Reported** | 2026-09-08 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0 beta builds through 2.3.0 reported (GPT-5.6 sol, muse-spark-1.3-contributor, GLM); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | 2.0, triaging |

**Summary:**
Reports of tool-call arguments breaking down in long sessions: `patch` text ending in internal markers (`<|DELIM_AE|>...`), fields mixed from other tools' schemas, injected words and bogus paths (a corrupted `read` raised a permission prompt for an unrelated user-profile path), `shell` with `{"background":"true</atem:...>"}`, and repeated `{}` calls rejected with `command: Missing key` (49 empty-input calls in one 1,000-message session, stopped immediately by compaction). In #47902 the reporter observed that parseable calls with a valid required argument plus unknown properties were executed (four shell calls ran, one mutated an external GitHub resource), i.e. unknown properties are dropped before validation and the rest runs (same mechanism as O2-004 and O2-047). #47902 was closed not_planned by the contributor bot for no reply, then reopened by a new comment on 2026-11-17. For #64481 the bot reproduced that a failed provider response (`response.failed`) leaves an errored `{}` tool part that is not replayed, concluded the looping `{}` calls are the model's own output at about 450k tokens, and closed it; the reporter conceded no OpenCode bug was proven.

**Impact on Orchestration:**
Long-running primaries (orchestrators) and long subagent sessions can burn turns on rejected calls and loops; a malformed but partially valid call can still execute its valid part, so a corrupted argument set can cause an unintended action. Mostly model behavior at very large contexts, but the harness keeps rejected calls in history, which reinforces the pattern.

**Evidence:**
- Several independent reporters (GPT-5.6 sol Windows and Linux, muse-spark Linux), byte-verified samples from stored parts; bot analysis attributes it to model output degradation; no maintainer defect acknowledgment. Not reproducible on demand.

**Workaround(s):**
1. Compact earlier (practical workaround accepted by reporter and bot) and start fresh sessions for long tasks; compaction stopped a 49-call `{}` loop instantly.
2. Treat any tool call with unexpected fields as suspect; require strict argument validation in plugins (`tool.execute.before`) for tools with side effects.
3. Orchestrator-side step budgets (see O2-005).

**Notes:**
Cross-ref O2-014 (tool-call markup leaked as text) and O2-004/O2-047 (silent dropping of unknown arguments). Included as Likely/Quirk: model-side, but the silent drop of unknown properties is a harness behavior that makes it dangerous.

---

### O2-048: PowerShell command scanner refuses shell commands with en/em dashes, curly quotes or syntax errors, reporting only `invalid-structure`

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/61704 |
| **Reported** | 2026-11-04 |
| **Last Activity** | 2026-11-09 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.5 and 2.2.6 reported on Windows 11 with pwsh 7; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (open feature request) |

**Summary:**
V2's portable PowerShell scanner (used for permission analysis of `shell` commands) fails closed: any command containing U+2013 to U+2015 dashes, U+2018 to U+201E curly quotes, NUL, U+0085 or BOM, any genuinely malformed PowerShell, or any command over 65,536 characters is refused with `Portable shell scanner cannot analyze command: invalid-structure`, naming neither the character nor the cause. A contributor bot says the refusal is deliberate (fail-closed safety); the thread is an open enhancement request. Three independent users confirm; one measured 7 of 11 refusals in a day were typographic dashes inside string literals.

**Impact on Orchestration:**
Agents writing prose, reports or CJK text through PowerShell hit opaque refusals and, not knowing why, rewrite quoting repeatedly (one session tried four quoting styles for a 6 KB script). On Windows PowerShell setups this wastes subagent turns and can stall work that models routinely produce (em dashes in report text, `gh ... --jq` format strings).

**Evidence:**
- Three independent reproductions on 2.2.5 and 2.2.6, including a real `gh api --jq` command with an em dash and a genuine `EmptyPipeElement` syntax error that gets the same opaque message.
- Commenter analysis: smart quotes must stay refused (PowerShell honors them as delimiters even inside ASCII strings); dashes inside literals could be allowed (proposed 6-line patch, no PR). Related open PR #54218 (explain each reason code).
- No maintainer comment; contributor bot labelled it a design question.

**Workaround(s):**
1. Avoid typographic dashes and curly quotes in shell commands; write such text to a file via `write` and reference it from the command. Confirmed by commenters.
2. Plugin `tool.execute.before` hook that names the offending character so the model fixes it on the first try (reporter's workaround).
3. Instruct agents that `invalid-structure` is a scanner refusal (non-ASCII punctuation, syntax error, or very long command), not a quoting problem.

**Notes:**
By design as of the thread; MOSAIC agent instructions on Windows should tell models to use ASCII punctuation in shell commands and to put long scripts or prose into files. Related Windows shell items checked and not tracked separately: #64573 (cmd escapes double quotes; applies only when `"shell": "cmd"` is configured, open fix PR #64574), #63892 tracked as O2-040.

---

### O2-049: MCP servers (local and remote) that fail once are never retried by the service; only `/mcps` reconnect, a config re-add or a service restart recovers them

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/52237 (also #64095 local docker-daemon variant, closed as duplicate, verified same root cause; #62405 closed duplicate) |
| **Reported** | 2026-09-30 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.20 through 2.3.2 (latest comment confirms still happening on 2.3.2) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none shown on #52237 (assignee nexxeln; 5 thumbs-up, 21 comments); #64095: type/bug, area/mcp, version/v2 |

**Summary:**
When an MCP server's connect attempt fails (DNS down after sleep/wake or VPN, remote app not yet running, a local stdio command whose dependency such as Docker Desktop is still booting, a proxy restart that exhausts the SDK's 2 SSE reconnects), the background service marks it `failed` and never retries. Only a short startup retry (about 1.25 s) and expired-session recovery exist. State is per project directory, so one directory can stay failed while others connect. A contributor bot reclassified the report as a feature request ("maintainers will consider it"); open PRs #61762 (recovery with backoff, one commenter tested it: failed server reconnected unattended in 54 s) and #54378. The original report said local stdio servers were fine, but later commenters on 2.0.22 and 2.3.0 showed local stdio servers stick as `failed` too.

**Impact on Orchestration:**
A headless or long-lived service run whose MCP server fails at start loses that server's tools for the whole session with no retry, and the agent may conclude the tool is "not configured" (#62973). A subagent that needs an MCP tool silently runs without it. Cold-boot races (daemon-backed local servers) are the most plausible way MOSAIC hits this.

**Evidence:**
- 15+ independent data points across macOS, Windows, Linux/WSL on 2.0.20 to 2.3.2, with log timestamps; bot read the code and confirmed no retry; assignee and open fix PRs.
- `ctx.mcp.reload()` does not re-spawn failed servers (verified by #64095 reporter on 2.3.0); `opencode mcp` has no `reconnect` subcommand; `opencode mcp list` can report a stale or `No MCP servers configured` state (#54192).

**Workaround(s):**
1. In the TUI `/mcps`, select the failed server and retry; or `POST /api/experimental/mcp/<server>/connect` with `?location[directory]=<dir>` per directory (bot- and user-confirmed, no service restart).
2. Remove and re-add the server entry in config (user-measured on 2.0.22: a toggle of `enabled` does not reconnect).
3. Gate `opencode` startup on the dependency (wrapper script that waits for `docker info`), or `opencode service restart` (drops attached clients).

**Notes:**
Treated as Limitation until a retry loop ships. Related, not tracked separately: #53226 (local server not restarted after transport error), #62973 (failed servers indistinguishable from unconfigured for the agent). Leads checked and not added: #63876 (resources/list retry storm) and #63847 (config reload leaks MCP processes) were both closed `not_planned` after a contributor bot could not reproduce on 2.0.20/2.3.0 and the reporters did not answer; re-check if new reports appear.

---

### O2-050: Code Mode lists inline MCP tools by the first 120 characters of the first description line only; rules further down are never read by the model

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/63603 |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.7 measured; code unchanged assumed through 2.3.2 |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/mcp, version/v2, platform/macos |

**Summary:**
The Code Mode catalog shows each inline tool by only the first line of its description, cut at `DESCRIPTION_LIMIT` (120 chars), with no marker that more follows. Tools the catalog leaves out are found with `search(...)` and arrive with the full description; inline ones are called without lookup. Reporter measured (Sonnet 4.6, 48-tool server): rule on line 2 followed with direct tools and with search-discovered tools, not followed when the tool was inline. Server `instructions` and a pointer in the first line did not help; with a small complete catalog the header says the catalog is complete and the model never searches. A contributor bot confirmed the truncation is deliberate and refiled it as a feature request (maintainers "will consider it").

**Impact on Orchestration:**
MCP tools whose contract (required field format, "do not use when", success-that-reports-failure) sits past the first line are used wrongly by Code Mode agents without any error. Whether a rule is read depends on catalog path order, so behavior differs between agents and servers.

**Evidence:**
- Controlled table by one reporter (3 to 6 runs per case, consistent); contributor bot read `listing()` in `catalog.ts` and confirmed first-line-only truncation by design. Single reporter, open, no maintainer comment.
- Related: #62316 (server picks inline tools), #58476 (ACP server opt-out of Code Mode), #58708 (Code Mode token budget).

**Workaround(s):**
1. Set `codemode: false` on the MCP server so its tools are direct tools with full descriptions (bot-suggested, matches reporter's `execute`-denied case; context cost see O2-041).
2. Put critical rules in the first 120 characters of the description, or restate them in agent instructions.

**Notes:**
By design for now. Related to O2-041 (direct exposure when `execute` is denied) and O2-043 (Code Mode usage pitfalls).

---

### O2-051: After summary compaction, tool-call inputs in the kept recent text are not truncated (large `write`/`patch`/subagent prompts stay whole and are resent every turn)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/64279 |
| **Reported** | 2026-11-16 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.7 and `dev` (0ac0d7ee); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/core, version/v2, platform/linux |

**Summary:**
`messageToText` (`recent-text.ts`) builds the verbatim-kept `<recent-context>` after compaction. Tool results are cut to 1,250 characters with `[truncated]`, but the tool call input is written whole. A `write` or `patch` of a 40,000-character file keeps all 40,000 characters. The kept text is rebuilt from `metadata.keepID` on every load and sent with each request until the next compaction (per the reporter's code reading; the bot captured only the first post-compaction request). Contributor bot reproduced on 2.2.7 and `dev` with a simulated model; open fix PR #64291.

**Impact on Orchestration:**
A silent context cost: after compaction the freshly freed window can be refilled by one large input from the retained tail (big file writes, long subagent prompts), causing premature re-compaction or overflow in long-running primaries. Separate from where the kept range starts (#43250).

**Evidence:**
- Bot reproduction with script (`status/repro-simulated`), fix PR #64291 open, one thumbs-up. No maintainer comment.

**Workaround(s):**
1. None documented. Avoid passing very large content through tool inputs (write large files via `shell` from a file or in smaller chunks) in sessions expected to compact; compact before large writes (inferred).

**Notes:**
Related compaction content entries: O2-052 (commentary text kept in summary), O2-053 (retained developer messages rejected by native compaction), O2-020. Cost-only lead #63373 (compaction request misses prompt cache) closed not_planned: bot showed the flattened transcript is the documented fallback when the conversation exceeds the model limit; not tracked.

---

### O2-052: Summary compaction on Responses (GPT) models may save `commentary`-phase text into the summary that the next agent reads

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/63439 |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.7 and `dev` (bot mock repro); not seen live with a real model; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/core, version/v2 |

**Summary:**
The compaction `stream` helper appends every text delta to the saved summary regardless of Responses `phase`, unlike title generation (fixed in #62267). The GPT system prompt asks for commentary-channel updates and the compaction request reuses it, so a commentary block before the summary passes the heading check and is stored and re-fed as `<summary>`. Reporter derived this from code; a contributor bot reproduced with a mock Responses endpoint (`COMMENTARY_LEAK` text at the head of the summary). Open fix PR #63505.

**Impact on Orchestration:**
Narration text ("I will now write the structured summary") pollutes the summary the continuing agent starts from, a minor correctness and token cost in compaction-heavy sessions on GPT-family models.

**Evidence:**
- Bot reproduction with a mock (not a real model), second user confirmed in code, fix PR #63505 open. No maintainer comment.

**Workaround(s):**
1. None needed beyond awareness; `agents.compaction.model` pointing at a non-Responses model avoids the commentary channel (inferred, untested).

**Notes:**
Related: O2-020 (strict template validation on GPT-6.2), O2-051, O2-053.

---

### O2-053: Native compaction (OpenAI/Azure Responses) fails with `Invalid compaction response` when the returned window keeps `developer` messages (live AGENTS.md or Code Mode catalog updates)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/63434 |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.6 (reporter, Azure OpenAI, Windows); bot repro on 2.2.6 and `dev` f5ca39a; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/providers, version/v2, platform/windows |

**Summary:**
With `compaction.type: native`, the validator in `packages/ai/src/protocols/utils/responses-compaction.ts` accepts only `user`/`assistant` messages in the compacted window. When OpenCode has injected `developer` messages (Code Mode catalog update, live AGENTS.md change notice) and the endpoint returns them, the whole HTTP 200 response is rejected with `provider.invalid-output`. Bot reproduced with a mock; removing the developer item makes compaction succeed. Open fix PR #63437.

**Impact on Orchestration:**
A long session on Azure/OpenAI native compaction that had its AGENTS.md or MCP catalog change mid-session cannot compact; context eventually overflows. Narrow trigger (native compaction on Responses providers plus live instruction updates).

**Evidence:**
- Reporter's rejected response body; contributor bot mock reproduction with control run; fix PR #63437 open. Real Azure behavior not independently verified.

**Workaround(s):**
1. Use summary compaction instead of `native` for the provider (`compaction.type: "summary"`, inferred from the provider settings shown in the thread; untested).
2. Avoid editing AGENTS.md or changing MCP tool catalogs during a long session (the trigger).

**Notes:**
Related: O2-020, O2-052, O2-029.

---

### O2-054: `edit` matches only exact text, typographic quote/dash/space variants and trailing whitespace; indentation, tab and internal-space differences hard-fail, and the description, error and "closest lines" hint contradict each other

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/62211 (related: #54400 closed duplicate of #42908, same agent-falls-back-to-shell symptom; #53149 line-ending rewrite, fixed by PR #53166 on 2026-10-12) |
| **Reported** | 2026-11-05 |
| **Last Activity** | 2026-11-06 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.22 through 2.2.5 (contributor bot verified code on the v2.2.5 tag and `dev`); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none (bot filed as feature request) |

**Summary:**
`packages/core/src/tool/plugin/edit.ts` tries three tiers: exact, `normalizeForMatch` (curly quotes, dash variants, Unicode spaces) and per-line `trimEnd()`. Different leading indentation, tab vs spaces, and internal space runs never match (reporter table X2, X5, X6, X9, byte-verified on 2.2.5), giving `Could not find oldString ... must match exactly, including whitespace and indentation.` The tool description promises "space variants", while the "closest lines" hint ignores all whitespace, so it points at exactly the line the matcher rejected. `read` prefixes lines with `<n>: ` (one space), which makes models copy one extra leading space. A contributor bot confirmed the code and called it intended behavior, refiling as improvement requests (visible-whitespace diff in the error, unambiguous `read` delimiter, optional indent-tolerant matching); maintainers "will consider" them.

**Impact on Orchestration:**
Edit-heavy subagents (especially weaker models) loop on failed edits or fall back to shell `Set-Content`/redirects, bypassing file-tool permissions and risking encoding/line-ending damage on Windows. Unattended runs stall or waste turns on a whitespace mismatch the error does not explain.

**Evidence:**
- Source-cited report with 8+ deterministic cases; bot confirmed behavior on v2.2.5 and `dev`; no maintainer comment, no fix PR.
- #54400 (Windows, free model, 10 comments) reported the same shell-fallback pattern and was closed as duplicate of #42908 (edit gives no closest-match context); the reporter agreed it was the same. Mixed-line-ending matching is tracked upstream as #45880 (not individually verified here).

**Workaround(s):**
1. Instruct agents to copy `oldString` from the `read` output without the `<n>: ` prefix and without the extra separator space, include only whole lines, and on a failed edit re-read the exact region (and use the "closest lines" hint) before retrying.
2. After one failed `edit`, fall back to `patch` or `write` on the whole file rather than a shell round trip (inferred).

**Notes:**
By design as of the bot analysis; MOSAIC agent prompts that edit files on V2 should state these rules. Related: O2-047 (`filePath` vs `path` edit failures), O2-044 (edit/write concurrency).

---

### O2-055: Every child session with a parent now reports to it on each idle and wakes it; the `subagent` `notify` option and `subagent.notify` metadata were removed ("we decided not to support silencing subagents")

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/63894 (maintainer decision in merged PR #63413; related #63209, #57157) |
| **Reported** | 2026-11-14 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 onward (PR #63413 merged 2026-11-11 by maintainer jlongster); assumed on 2.3.2 |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/plugins, version/v2 |

**Summary:**
Since 2.3.0 any child session created with a `parentID` (via the `subagent` tool or via `client.session.create`) writes a synthetic `<subagent ...>` notice carrying the child's report into the parent and wakes the parent each time the child goes idle, including after follow-ups and restart recovery. PR #63413 deleted the `notify: false` input and stopped honoring `subagent.notify` session metadata (old values are ignored; "We decided not to support silencing subagents"). The only gate left in `prepare()` is "no parent and no background job". No plugin hook runs before delivery (#57157 open). A contributor bot filed #63894 as a feature request.

**Impact on Orchestration:**
Interactive or multi-turn child sessions push full child messages into the parent on each idle, and every notice triggers a parent model turn. Reporters saw about 200 spurious parent wake-ups in 24 h from plugin-owned hidden agents, and a long-lived orchestrator whose context fills quickly from child reports. Hidden helper agents in MOSAIC plugins/logging would also notify the parent.

**Evidence:**
- Maintainer-authored merged PR states the decision; contributor bot read `subagent-report.ts` and confirmed; a second user (long-lived orchestrator pattern) reports the break on a single subtask in 2.3.0 and uses a plugin that blanks the notice (the parent still activates briefly).

**Workaround(s):**
1. For helper agents that need no parent link, create their sessions without `parentID` (bot-suggested; the one case where `prepare()` returns early).
2. Plugin that rewrites the notice to empty text (user-reported; parent still wakes briefly, not recommended as-is).
3. Roll back to 2.2.7 (still has `notify`/`background: false`; see O2-002).

**Notes:**
Permanent by design. Cross-reference O2-002 (always-background `subagent`, no blocking option): that entry covers dispatch semantics, this one the unsuppressible report/wake on every child idle. Related: O2-006.

---

### O2-056: A rate-limited or usage-limited subagent now silently waits for the reset and retries; the parent/orchestrator never sees the error and cannot reroute

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/64749 |
| **Reported** | 2026-11-18 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.2 (reporter, ChatGPT subscription provider; other providers not confirmed) |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/feature, area/core, version/v2, platform/linux |

**Summary:**
When a subagent's model hits a usage limit with a known reset time, V2 now pauses the session and retries after the reset instead of returning the error to the caller (likely from #43126). A reporter with a Claude orchestrator delegating to several providers says earlier versions failed fast, so the orchestrator could redispatch to another model; now the child waits, possibly for hours, and the parent stays blocked. A contributor bot filed it as a feature request ("maintainers will consider it"), pointing to the opposite request #59533 and the configurable-retry requests #58128/#43596.

**Impact on Orchestration:**
Fan-out across providers loses its failure signal: a rate-limited worker looks like a slow worker, and the orchestrator (which cannot cancel children from inside the agent loop, see O2-006) waits on it. Combines with O2-005 (no time bound) and O2-025 (no stall signal).

**Evidence:**
- Single reporter, filed hours before this pass, no maintainer comment; the bot acknowledged the described behavior as current and linked its origin. Reporter's before/after comparison is from experience, not a captured trace.

**Workaround(s):**
1. Plugin hooking `session.retry` that sets `event.decision = { retry: false }` for sessions with a `parentID` when the error type is a rate/usage limit (`provider.rate-limit`, `provider.chatgpt-limit`, `provider.quota`); top-level sessions keep waiting. Contributor-provided, type-checked but not run against a real limit.
2. Orchestrator-side watchdog that cancels and redispatches after a time budget (see O2-025).

**Notes:**
New behavior near 2.3.x; re-verify on providers other than the ChatGPT subscription route. Related: O2-002, O2-005.

---

### O2-057: Every `shell` call stalls 5 to 20 seconds when the `opencode-pty` daemon is unreachable (2.3.0 regression: failed start is retried per command, no latch)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/64080 (#64081 closed duplicate) |
| **Reported** | 2026-11-15 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.3.0 (introduced by commit d4bcde9121 / #62978, first in v2.3.0; v2.2.7 not affected); bot reproduced on `dev` d432a06; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | type/bug, status/repro-simulated, area/service, version/v2, platform/macos |

**Summary:**
Since 2.3.0 `shell` runs through the `opencode-pty` daemon, with a direct-spawn fallback when the daemon is "unavailable". The failed handshake is not remembered: each command re-runs the daemon start/poll (5 s per attempt, new runtime directory each time), so every command arrives 5 to 20 s late (bot measured about 20 s per call with a stand-in helper; reporter about 5 to 10 s, 428 failed handshakes). The reporter's trigger was a macOS `nono` sandbox denying AF_UNIX `connect()` (their sandbox-side correction; a daemon-side defect was ruled out), but any environment where the daemon cannot be reached has the same cost. Fix PR #64087 was closed unmerged; a better candidate (30 s backoff, discovery kept active) exists on a contributor fork (r266-tech/opencode@1def5d6) with no open PR. Issue open, no maintainer comment.

**Impact on Orchestration:**
Shell-heavy subagents (builds, tests, git) pay a fixed multi-second penalty per call in sandboxed or locked-down environments, multiplying across fan-out and eating step/time budgets; commands still succeed via fallback, so it looks like slowness rather than failure. Related to O2-040 (daemon spawn failure for Store pwsh on Windows).

**Evidence:**
- Contributor bot reproduction with a stand-in helper (shell calls 20,195/20,162/20,113/20,132 ms); regression timeline pinned by the reporter to `v2.3.0` (`git merge-base` check against `v2.2.7`); candidate fix benchmarks (second call 3 ms). The idle retry loop could not be reproduced by the bot.

**Workaround(s):**
1. Roll back to 2.2.7 (reporter suggestion from the commit timeline; not confirmed in the thread).
2. Fix the environment so the daemon socket is reachable (reporter's sandbox profile grant); `OPENCODE_PTY_SOCKET_DIR` does not help when `connect()` itself is denied.

**Notes:**
Check whether 2.3.1/2.3.2 include a latch. Related open items: #51796 (dead helper masked as ENOENT), #61750, #63817 (zombies under the daemon).

---

### O2-058: `opencode_worktree_remove` is gated by the `edit` permission, not its own action; an `opencode_worktree_*` deny does not hide or block it, so any editing agent can delete a worktree (uncommitted work included)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/anomalyco/opencode/issues/61121 (also #64136, closed duplicate, verified same cause: user's `opencode_worktree_*` deny did not apply to `worktree_remove` on 2.3.0, Windows) |
| **Reported** | 2026-11-02 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.2.0 (since #59315 exposes pinned `opencode` namespace tools) through 2.3.0; reporter's repro on 2.2.5; 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | none shown (#61121); type/feature, area/core, version/v2, platform/windows (#64136) |

**Summary:**
`worktree_remove` is registered with `permission: "edit"` deliberately ("removing a worktree deletes files, so it requires the same approval as editing them"). An agent with `* deny` plus `edit * allow` is therefore offered the tool and can remove the worktree it runs in, with `force: true` deleting uncommitted files; the reporter's model-free repro (`opencode run --standalone`) removed the worktree and `unsaved.txt`. The docs state the `edit` gating. Contributor bot filed it as a feature request (give it its own action, gate `force` separately); maintainers "will consider it". Open PR #63230 replaces the worktree tools with config-driven worktree guidance, which would change this surface.

**Impact on Orchestration:**
Parallel implementer subagents working in worktrees look locked down by `*` deny rules but can still delete their own or a sibling worktree, destroying uncommitted work. Deny rules written as `opencode_worktree_*` silently skip this one tool, while `worktree_create`/`worktree_list` are covered.

**Evidence:**
- Source-cited report with a deterministic repro; bot confirmed the gating in code and in the docs. Same finding independently hit by #64136 (Windows, 2.3.0).
- No maintainer comment; `worktree_remove` stays `edit`-gated unless PR #63230 or a dedicated-action change lands.

**Workaround(s):**
1. Deny `edit` on `.`, `..`, `../*` and `/*` for builder agents (reporter's workaround; lexical paths only, misses symlinks and nested worktrees). Also a `*` to `ask` rule makes the call ask first (bot reading of the code, untested).
2. Do not run agents that must not delete worktrees with `edit` allowed; clean up worktrees from the orchestrator side.

**Notes:**
By design as of the bot analysis. Related worktree reports not tracked separately: #64162 (plan mode: `worktree_create` bypasses read-only, feature request), #64442 (stale worktree cleanup not a standard step), #60165 (submodule worktrees cannot be deleted).

---

### O2-059: `edit`, `patch` and `write` silently corrupt non-UTF-8 files (every invalid byte becomes U+FFFD) and report success; `read` no longer warns on 2.2.6

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/45924 (#57308 cp1252 on Windows and #49600 closed as duplicates; verified same mechanism) |
| **Reported** | 2026-08-28 |
| **Last Activity** | 2026-11-08 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Confirmed on V2 2.0.32, 2.2.0, `dev` 506ac20 (2026-10-28) and 2.2.6 (Windows); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced, severity:high |

**Summary:**
`edit`, `patch` and `write` read files through `FileMutation.readText`, which decodes with a non-fatal `TextDecoder` (`packages/util/src/bom.ts`). Any file containing non-UTF-8 bytes (cp1252, GBK, Latin-1 legacy sources) has all those bytes replaced by U+FFFD on the next edit, even when `oldString` targets only ASCII, and the tool reports success. A Windows user's one-space edit to a cp1252 + CRLF file changed 745 characters (421,899 to 423,390 bytes). Contributor bot reproduced with `edit` and `patch` on 2.2.0 and `dev`. On 2.2.6 (Windows) `read` returns mojibake instead of failing (UTF-16 with BOM is rejected as binary and is the safe case), so the agent gets no warning. Assigned to a maintainer (neriousy); fix PR #45925 (reject non-UTF-8) closed unmerged, #49881 closed, #58113 (encoding support) open.

**Impact on Orchestration:**
Data loss that is silent and often unnoticed until runtime or review: an agent making a tiny edit on a legacy-encoded file destroys the rest of the file's non-ASCII content. Windows projects with cp1252 sources are the realistic exposure; the damage is only recoverable from git.

**Evidence:**
- Four independent reporters across 1.18/2.0.x/2.2.x with byte counts; bot repro script; severity:high label; maintainer assignee; PRs linked.

**Workaround(s):**
1. Recover with `git checkout -- <file>` and apply the edit with a byte-preserving tool (Python `rb`/`wb` replacement) for legacy-encoded files (reporter-confirmed).
2. Convert legacy files to UTF-8 first, or deny `edit`/`patch`/`write` on directories that hold non-UTF-8 sources; have agents verify encoding (`file`, or check for U+FFFD in `git diff`) after edits (inferred).

**Notes:**
Related: O2-044 (stale-overwrite), O2-060 (CRLF corruption by `patch`).

---

### O2-060: `patch` (apply_patch) rewrites CRLF files into mixed line endings on V2 (touched hunk lines become LF)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/anomalyco/opencode/issues/45926 (#54605 Windows v2.0.26, closed as duplicate by a maintainer: "tracking it there"; verified same behavior) |
| **Reported** | 2026-08-28 |
| **Last Activity** | 2026-10-28 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 2.0.26 and 2.2.0 and `dev` 506ac20 (bot repro on 2026-10-28); 2.3.2 not re-verified |
| **Latest Platform Version** | v2.3.2 |
| **Labels** | reproduced |

**Summary:**
The V2 patch code splits files on `\n` only. The model sees LF-only `read` output, so its context lines match through a `trimEnd()` fallback and the hunk is spliced back with the model's LF lines. Every line inside the hunk, including unchanged context lines, becomes LF while the rest stays CRLF, leaving a mixed-ending file. Assigned to maintainer rekram1-node; fix PRs #58324 and #62477 open (#45931 and #56128 closed). The related `edit` rewrite (#53149) was fixed by PR #53166 on 2026-10-12.

**Impact on Orchestration:**
Windows repos with CRLF files get noisy whole-hunk diffs and possible lint/format failures after agent patches; `patch` is the tool offered to `gpt-` family models, so GPT agents on Windows are the most exposed. Content is otherwise intact.

**Evidence:**
- Bot script reproduction with `cat -A` output on 2.0.26 and 2.2.0/`dev`, unit-level repro by the reporter; maintainer assignee; four fix PRs (two open). Related `edit` mixed-ending matching issue #45880 (not verified here).

**Workaround(s):**
1. Community plugin `Mac-Moneysac/opencode-fix-line-endings` preserves line endings (bot-endorsed; not tested by us).
2. Normalize line endings after agent runs (`.gitattributes`, formatter, `git add --renormalize`), or use `edit` rather than `patch` on CRLF files (the `edit` rewrite was fixed in #53166; inferred).

**Notes:**
Related: O2-059 (encoding corruption), O2-054 (`edit` matching rules).
