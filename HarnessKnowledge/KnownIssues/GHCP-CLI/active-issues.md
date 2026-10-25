# GitHub Copilot CLI — Active Issues

> Last updated: 2026-10-25 (run 20)

---

### GC-001: Memory-pressure watchdog force-compacts conversation at low context usage, can loop until OOM

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4506 |
| **Reported** | 2026-08-16 |
| **Last Activity** | 2026-08-17 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.80 (unknown if fixed since) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:sessions, area:context-memory |

**Summary:**
A process-level memory-pressure watchdog force-compacts the conversation whenever process memory is high, without checking whether the conversation itself is the memory being consumed. In the reported session, memory was dominated by the session's own `events.jsonl` (381 MB, held parsed in memory) — compaction ran repeatedly against a conversation at only 23% of a 400k-token window, recovered ~3 tokens (0.003%) each time, and looped indefinitely because the underlying memory pressure was unrelated to conversation size. The process eventually OOM'd and crashed. A contributing factor (`logLevel: "all"`, non-default) causes unbounded verbatim third-party tracing (`html5ever` tokenizer output) to be written to disk, pushing `events.jsonl` and process memory even higher. No maintainer response or comments; single reporter with exceptionally detailed technical evidence (log line frequency counts, exact memory metrics, V8 OOM stack trace, reproduction steps).

**Impact on Orchestration:**
Long-running MOSAIC sessions (interactive hub-and-spoke orchestration, or a headless Runner stage that stays open across many tool calls) can hit this watchdog and lose conversation history to compaction that provides zero memory relief, then eventually crash the process outright — losing session state entirely rather than gracefully degrading. Because the trigger is unrelated to actual context-window usage, this can strike even a conversation MOSAIC believes is well within budget, undermining the reliability of both primary-agent context management and the assumption that compaction is purely context-driven.

**Evidence:**
- Single reporter, no maintainer acknowledgment, no independent reproduction reports
- Extremely detailed, quantified technical evidence: log line counts, exact memory/heap metrics, `events.jsonl` size, and version/environment specifics — not a vague symptom report
- Meets the Unverified-inclusion bar: threatens a core MOSAIC concern (primary-agent context/session stability), reproduced on stock Copilot CLI infrastructure, and includes a clear, mechanical reproduction recipe

**Workaround(s):**
1. ★ Avoid setting `"logLevel": "all"` in `~/.copilot/settings.json` — this is the largest identified contributor to unbounded `events.jsonl`/log growth that feeds the memory-pressure trigger; default log level does not exhibit the same growth.
2. Periodically restart very long-running sessions (rather than keeping one process alive for many hours/days) to bound `events.jsonl` growth and process memory before the watchdog engages.
3. Avoid heavy `web_fetch` usage against large HTML pages in the same long-lived session, since HTML tokenizer trace volume (under `logLevel: all`) was a major contributor in the reported case.

**Notes:**
Run 20 (2026-10-25): still open, one new non-maintainer comment (2026-09-15, a session-rescue script, see #4780); no maintainer response. Release notes show related memory work: 1.0.97 "Emergency memory relief in long sessions can now bypass its cooldown before V8 aborts" and "a session that would have been killed by a JavaScript heap out of memory crash may survive with compacted history", i.e. memory-driven compaction is still deliberate design, and the cooldown aspect may be partially changed. Last activity 2026-09-15 (6 weeks) - Needs re-verification on 1.0.97+. GC-007 commenters also saw "Memory pressure persists but compaction already in progress" right before OOM, so the two are linked. Original note: no maintainer response yet. If confirmed widespread, this argues for MOSAIC's Runner pipeline to bound individual headless session lifetimes (restart between stages rather than reusing one long-lived process) as a structural mitigation independent of any upstream fix.

---

### GC-002: Repeated compaction recursively re-summarizes prior summaries, degrading durable context

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/github/copilot-cli/issues/4441 |
| **Reported** | 2026-08-11 |
| **Last Activity** | 2026-08-19 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.79 (documented/architectural behavior, not version-specific) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:context-memory |

**Summary:**
Filed as a feature request, but the underlying behavior is a confirmed, documented architectural property: each compaction re-summarizes the *previous summary* together with newer turns, rather than re-summarizing from original source material. Official Copilot CLI context-management docs confirm compaction replaces active history with a generated summary and explicitly warn that important context may be lost after many compaction cycles. The reporter's own session logs quantify the effect: a first compaction reduced 196 active messages to 1 replacement message; a second reduced 473 to 1. Early decisions, constraints, and gotchas progressively degrade with every additional compaction cycle since each cycle compresses an already-lossy compression rather than the original record. Reclassified from the GitHub "Feature request" framing to **Limitation** here because the recursive-summarization mechanism is doc-confirmed and by-design, not a defect — but it still poses a real degradation risk for MOSAIC's long sessions.

**Impact on Orchestration:**
Long MOSAIC orchestration runs (many workflow stages, or a single long interactive session) that pass through multiple compaction cycles will progressively lose fidelity on early decisions, constraints, and blackboard-relevant facts — exactly the kind of durable state MOSAIC's `Orchestration.md` blackboard pattern is meant to preserve externally, but any context the primary agent still needs to reason about mid-session (not yet written to a file artifact) is at risk of silent, cumulative degradation the more compactions a session goes through.

**Evidence:**
- Single reporter, but corroborated by official GHCP CLI documentation describing the same recursive-summarization mechanism and its documented risk of context loss over many cycles
- Reporter provided concrete, quantified before/after message counts from an actual session (196→1, 473→1) demonstrating the compounding effect directly
- Confidence set to Likely (not Confirmed) because there is no maintainer acknowledgment specifically discussing whether this is considered a problem to fix, and only one reporter/session is behind the quantified numbers — but the mechanism itself is doc-corroborated, not merely a single user's inference

**Workaround(s):**
1. ★ Maintain a manually/agent-updated `HANDOFF.md` durable-facts file: before each `/compact`, have the agent write/update a running log of decisions, facts, constraints, and gotchas to this file, then have it re-read the file immediately after compaction completes. Reporter (a Copilot CLI user, in a follow-up comment) confirmed this pattern lets them keep using a session across many compaction cycles without losing durable knowledge. Not upstream-confirmed by a maintainer, but a concrete, currently-usable mitigation.

**Notes:**
Classified as Limitation rather than Bug since the recursive-summarization mechanism is a documented architectural property of compaction, not an unintended defect — MOSAIC should treat this as a permanent constraint to design around (e.g., encouraging subagents/orchestrator to externalize durable facts to on-disk artifacts early and often, rather than relying on conversational context surviving many compaction cycles) rather than something to wait on a fix for. The issue also requests first-class `preCompact`/`postCompact` hook support to automate the HANDOFF.md pattern — worth tracking if that ships, since it would turn this from a manual workaround into a built-in mitigation.

Run 20 (2026-10-25): issue still open, no new comments or maintainer response since 2026-08-19 (>9 weeks; no hook support shipped per 1.0.83-1.0.103 release notes searched). Mitigating changes: 1.0.85 "Automatic compaction now saves a checkpoint that appears in /session checkpoints" (passive audit trail only; does not stop recursive summarization). Behavior unchanged; Limitation stands.

Run 20 cross-reference (2026-10-25): open compaction-fidelity reports #1571 (summary drops the "execution cursor": what was running and which action was next; filed on 0.0.413, 3 comments, no maintainer) and #3703 (a summary paraphrased an instruction into its opposite, and the agent trusted the summary over `.github/copilot-instructions.md`; 1.0.60, 2 comments, no maintainer) describe the same lossy-summary property as model/compaction behavior, not separate defects. Thread suggestions: write the current step to a small `NEXT.md` file that the instructions tell the agent to read after compaction; use a `preCompact` hook (exists since 1.0.5) to snapshot the transcript; tell agents to re-read instruction files after compaction. Not tracked as separate entries.

---

### GC-004: Subagent launched with a since-deprecated/unsupported model crashes the entire primary session

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4427 |
| **Reported** | 2026-08-11 |
| **Last Activity** | 2026-08-11 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.79 |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:agents, area:models |

**Summary:**
In a long-running autopilot session, the primary agent (Opus 5) repeatedly dispatched a subagent configured to use `gemini-3.6-flash`. Partway through the ~2 hour session, that model was retracted from the supported model pool server-side. The next subagent dispatch attempt failed with `CAPIError: 400 The requested model is not supported` — and instead of the primary agent catching this and retrying with a different model, the error propagated up and killed the entire primary session. No maintainer or community response yet; single reporter, no comments.

**Impact on Orchestration:**
This is a direct hit on MOSAIC's core dispatch mechanism: the orchestrator (primary agent) invokes subagents via the harness's native agent/task tool, and a subagent's configured model is part of that agent's own definition. If a model referenced by any MOSAIC subagent (or a Runner-driven workflow stage) is deprecated or temporarily unavailable server-side, this failure mode means the entire orchestrator session — not just that one subagent dispatch — terminates. In a long autopilot-style orchestration run (exactly MOSAIC's target use case, whether interactive hub-and-spoke or headless Runner pipeline stages), this converts a single recoverable dispatch failure into total session loss, discarding the blackboard state accumulated in that session unless externally checkpointed.

**Evidence:**
- Single reporter, no maintainer acknowledgment, no independent reproduction — capped at Unverified per validation rules
- Included despite Unverified status because it meets all three inclusion bars: threatens a core MOSAIC pattern (subagent dispatch/invocation reliability), occurs on GHCP CLI's standard, natively-supported multi-provider model catalog (not a custom backend/proxy — using Gemini and Opus together is a first-class, documented capability of this harness, not an unsupported configuration), and provides a clear, concrete reproduction recipe (retract/ban a model mid-session while a subagent is configured to use it)
- The exact error string (`CAPIError: 400 The requested model is not supported.`) is reported directly from the session, not paraphrased
- A much older, related issue (#1237, filed against v0.400) reported the same error string for the built-in `explore` subagent and was closed as fixed in v0.0.407 via a model-fallback mechanism ("subagents now fall back to the session model when their default model is unavailable or blocked by policy"). A later commenter on that same closed issue (v1.0.21) reported the identical `CAPIError: 400 The requested model is not supported.` recurring, including on the *primary* session, not just a subagent. Taken together with this (#4427, v1.0.79) report, the fallback mechanism from v0.0.407 does not appear to fully cover the case of a model becoming unsupported *after* a session/subagent has already started using it — the general failure class (unsupported-model error propagating and breaking the session rather than triggering a graceful fallback) has recurred across CLI versions from v0.400 through at least v1.0.79.

**Workaround(s):**
No confirmed workaround reported. Possible mitigations worth evaluating (not yet community-validated):
1. Avoid pinning subagents to narrowly-available or preview/flash-tier models that are more likely to be deprecated mid-session; prefer models with longer support commitments for long-running orchestration sessions.
2. At the MOSAIC Runner level, consider wrapping stage dispatch so a model-not-supported error triggers a stage-level retry with a fallback model rather than propagating as a fatal session error — this would need to be built into `Tools/Runner` or the harness injection layer since the CLI itself does not do this.

**Notes:**
Run 20 (2026-10-25): still open, zero comments, no maintainer response, last activity 2026-08-11 (>10 weeks) - Needs re-verification. Possibly relevant: 1.0.83 release note "Custom agents can list several models in `model`, tried in order until one is available to you" - a config-level fallback for custom agents (not confirmed to cover mid-session retraction of a model). Single-reporter/Unverified; should be re-checked in a future run for corroboration or maintainer response. See cross-reference to closed issue #1237 in Evidence above — that issue was investigated during this same capture pass and found to have a "fixed" claim that appears incomplete given this and other later reports, but was not itself re-added as a separate KB entry since #4427/GC-004 already covers the currently-active manifestation. If confirmed, this argues for MOSAIC treating subagent model selection as a config value the orchestrator can validate/fall back on rather than a hard dependency, and for the Runner pipeline to checkpoint blackboard state frequently enough that a session-ending crash doesn't lose orchestration progress.

---

### GC-005: Sub-agents with full tool access silently return empty (no error, no log) once configured MCP tool-schema volume crosses an undocumented budget

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4293 |
| **Reported** | 2026-07-29 |
| **Last Activity** | 2026-07-30 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.73 |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:agents, area:tools |

**Summary:**
Sub-agents dispatched via the `task` tool with a full-tool-access agent type (`general-purpose`, `rubber-duck`, `task`) silently return `Agent completed but produced no response.` — no error, no partial output, nothing in the session log — while a restricted-tool agent type (`explore`) launched with the identical model and prompt in the same session succeeds. The reporter ran a controlled test halving their configured MCP server count (12 → 6) and confirmed a direct causal link: the failure tracks total tool-schema volume materialized into the sub-agent's context at session start, not the agent type or model (reproduced identically across `gpt-5.6-sol`, `gpt-5.6-terra`, and `claude-opus-4.8`, in both sync and background dispatch modes). A session restart is required for a server-count change to take effect — schema is fixed at session start. The reporter explicitly reframes the ask: the MCP/tool-schema budget itself is a separate, already-tracked problem (referenced as #3542), but this issue is specifically about the **silent, undetectable failure mode** — no error, no log line, no UI signal — which persists regardless of how the underlying budget is fixed.

**Impact on Orchestration:**
This is a severe, direct hit on subagent invocation and response capture — core to MOSAIC's hub-and-spoke model, where the orchestrator dispatches subagents and depends on their structured JSON status responses to decide routing. If a MOSAIC deployment configures several MCP servers (plausible for tool-rich subagents like research or infrastructure agents) and the combined tool-schema volume crosses this undocumented per-session budget, a subagent dispatch can silently produce empty output that is indistinguishable from a legitimate empty/no-op response. The orchestrator has no signal to retry or escalate — it may interpret "no response" as a completed but empty result and continue routing on a workflow assumption that never actually executed, corrupting the run's blackboard state silently. This risk scales with the number of MCP servers/tools a MOSAIC deployment wires up, which is exactly the trend as more Skills/tools get added to subagents.

**Evidence:**
- Single reporter, but with an unusually rigorous self-directed controlled experiment: identical prompts/models tested with 12 vs. 6 configured MCP servers, isolating tool-schema volume (not agent type, not model, not sync/background mode) as the causal variable
- Reproduced across 3 different models (`gpt-5.6-sol`, `gpt-5.6-terra`, `claude-opus-4.8`) and both `sync`/`background` dispatch modes with consistent results
- Confirmed the session log contains zero `[ERROR]`/`[WARN]` entries during the failure window — genuinely silent, not merely unlogged-but-detectable
- Reporter cross-references related issues (#3542 token-budget/contextTier ignored, #2630 custom agent MCP servers not connected in sub-agent contexts, #3293 extension tools not propagated to sub-agents) suggesting a broader family of MCP-schema-and-subagent-context issues
- No maintainer response yet; capped at Unverified per validation rules despite strength of self-directed evidence, since only one reporter is involved

**Workaround(s):**
1. ★ Reduce the number of simultaneously configured MCP servers (per-session tool-schema volume) until full-tool-access sub-agents start returning output again; requires a full session restart to take effect since schema is fixed at session start, not re-evaluated when servers are toggled off mid-session. Confirmed by the reporter via controlled test.
2. Route sub-agent work through a restricted-tool agent type (e.g., `explore`-equivalent) when many MCP servers are configured, since restricted toolsets stay under the schema budget longer — but this defeats the purpose of specialist/full-tool subagent types and is explicitly called out by the reporter as an unsatisfying workaround, not a fix.

**Notes:**
Related family of issues (not independently tracked as separate GC- entries in this pass, but worth cross-referencing in a future scan): #3542 (MCP tool schema exceeds token limit / `contextTier: long_context` reportedly ignored), #2630 (custom agent `mcp-servers` not connected in sub-agent/`--prompt` contexts — investigated during this same pass and found to have been fixed between v1.0.46 and v1.0.47, confirmed by a community member; not tracked as a separate KB entry since it is resolved and this was a first-ever capture pass), #3293 (extension tools not propagated to sub-agents). If MOSAIC subagents accumulate many MCP tool declarations across Skills/Tools injections, this silent-failure mode should be treated as a standing risk — the orchestrator protocol's status-code contract (SUCCESS vs. no response) has no way to detect this failure mode as currently designed, since the harness itself reports no error.

**Needs re-verification** (run 20, 2026-10-25): still open, still only the reporter's 2 comments, no maintainer response; last activity 2026-07-30 (>12 weeks). Release notes 1.0.83-1.0.103 contain no entry for sub-agent empty responses or an MCP tool-schema budget; 1.0.100 "Sessions no longer stall for minutes when an MCP server hangs while loading tools" is adjacent but not the same. Reporter noted the MOSAIC-relevant point that the silent-failure mode is the real defect. Previous note (run 2, 2026-09-10): body and comments unchanged; last activity 2026-07-30, now exactly 6 weeks stale against a harness that has released multiple versions (1.0.73 → 1.0.83) since the report. No corroboration or refutation found this pass; re-verify again in a future run, ideally with a direct reproduction attempt at MOSAIC given the severity (silent subagent failure with zero error signal).

Run 20 cross-reference (2026-10-25): #3542 (open, last activity 2026-06-03, v1.0.54) reports an enterprise-flight MCP allowlist whose tool schemas (about 573K tokens) exceed the ~136K runtime budget, causing compaction every turn; it appears specific to a Microsoft-tenant server-side flag and is stale, so it is not tracked separately. Release 1.0.100 notes "Avoid false context exhaustion and rollover refusal when tool definitions are deferred" and "Sessions no longer stall for minutes when an MCP server hangs while loading tools", which may reduce this class; see also GC-028.

---

### GC-007: OOM crash (`JavaScript heap out of memory`) on long `--resume` sessions; crash dumps written into user's cwd

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4699 |
| **Reported** | 2026-09-02 |
| **Last Activity** | 2026-09-21 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.79 through at least 1.0.86-2 (interactive TUI sessions); last report 2026-09-21; unknown on 1.0.87+ |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:sessions, area:context-memory |

**Summary:**
Long `--resume` sessions repeatedly crash with a V8 heap OOM (`Allocation failed - JavaScript heap out of memory`), always pinned right at the process's self-imposed heap cap (the CLI passes `--optimize-for-size`, which caps the heap around 4 GiB regardless of available system RAM). Diagnostic dumps show `old_space` (long-lived, survived-GC data) accounts for nearly all heap usage at crash time, consistent with unbounded per-session state retention (conversation/tool history) rather than a transient spike, and correlates with sessions accumulating substantial CPU time (46 min–2h11m). Separately, the resulting Node diagnostic crash-report JSON files are written into the user's current working directory rather than a tool-owned location, so they appear as untracked files inside whatever git repo the user was working in. Two independent commenters confirm hitting the same OOM pattern; one reports the crash persists even after manually raising the heap limit via `--node-options="--max-old-space-size=6000"`, ruling out the heap cap alone as the root cause and pointing to genuine unbounded memory growth. 5 reactions on the original report.

**Impact on Orchestration:**
Directly threatens session/conversation stability for any long-running or resumed MOSAIC session — whether an extended interactive hub-and-spoke orchestration or a headless Runner pipeline stage that reuses a `--resume`'d session across multiple dispatches. A crash mid-session means total loss of in-memory session state unless externally checkpointed, and the raised-heap-limit test suggests this isn't simply a config knob MOSAIC can tune around — no workaround fully prevents the crash, raising the heap cap only delays it. The crash-dump-in-cwd side effect is a secondary but concrete risk: Runner-driven stages operating inside a git-tracked working tree could have crash artifacts appear as spurious untracked/uncommitted files.

**Evidence:**
- Original reporter provided detailed diagnostic data (heap/RSS metrics across 3 separate crashes, `old_space` breakdown, environment specifics ruling out `NODE_OPTIONS` or system memory pressure as the cause)
- Two independent corroborating comments, one from a different reporter confirming the identical failure pattern, another reporting a variant reproduction after attempting the `--max-old-space-size` workaround and still crashing — with cross-references to two additional related issues (#4686, #4725) suggesting a broader OOM-family pattern
- 5 upvotes/reactions on the original report
- No maintainer acknowledgment yet, but multiple independent reproductions with strong technical evidence support Likely confidence

**Workaround(s):**
1. Restart/checkpoint long-running sessions proactively before they accumulate excessive CPU time or session state, rather than relying on a single `--resume`'d session to persist indefinitely — partial mitigation only, since exact trigger conditions for the underlying leak aren't isolated.
2. Set a tool-owned crash-dump directory if possible (or run from a directory outside the working git repo) to avoid crash-report JSON files polluting a tracked repository — not confirmed configurable in the CLI itself; may require an external wrapper that changes cwd before invoking `copilot`.
3. Raising `--node-options="--max-old-space-size=..."` does NOT reliably prevent the crash (confirmed ineffective by one commenter) — do not rely on this as a fix.
4. (added run 20) Run work headless/ACP rather than in the interactive TUI: a commenter's controlled repro (identical build 1.0.86-2, seeded history, scripted replies, 512 MiB cap) aborted the TUI at turn 21 but the headless ACP driver completed 50 turns, and in longer runs held the JS heap flat (~70-90 MiB) across 1,668 turns / 3,424 tool executions (resident memory still grew 1.47 to 2.44 GiB). Single-commenter finding, but encouraging for MOSAIC's headless Runner path.
5. (added run 20) If using `--resume` on a session that no longer resumes, a community script (see #4780 comment) copies/trims the session into a fresh one; the heap climb restarts afterwards, so it is recovery only. On hosts with `ulimit -c unlimited`, the abort also writes multi-GB `core` files into the cwd (data-exposure hazard: contains env vars and conversation) — set `ulimit -c 0` before launching.

**Notes:**
Still being actively reported as of 2026-09-10 (same day as this run 2 re-verification), against version 1.0.82 — essentially current. Cross-referenced by commenters against #4686 (an async-handle-leak pattern) and #4725 as possibly related; worth a dedicated follow-up pass on the broader OOM/memory-retention family in a future run. Note: the v1.0.83 release notes (2026-09-04) include "Long-running sessions on Linux return freed memory to the system instead of holding gigabytes of it" — this is a plausibly-related fix, but no comment in this thread has confirmed it resolves this specific OOM pattern (the 2026-09-10 corroborating comment is against a build still exhibiting the leak with `--max-old-space-size` raised, and doesn't state whether it was on 1.0.83). Flagged for re-verification on 1.0.83+ in a future run. High priority for MOSAIC given direct relevance to long-running Runner-driven or interactive sessions.

Run 20 update (2026-10-25): issue still open, 6 comments, 7 reactions, no maintainer response. Reports continue through 1.0.83, 1.0.84-5 and 1.0.86-2 (Linux HPC, Windows 11), so the 1.0.83 memory-release change did not fix it. Commenters point to out-of-heap memory (RSS 10-15 GiB vs ~3.2 GiB heap), `events.jsonl` growth (1.7-2.8 GiB), and a driver-dependent heap growth that affects the interactive TUI but not headless ACP (see Workaround 4). One crash logged `Memory pressure persists but compaction already in progress` just before OOM, linking this to GC-001. No activity since 2026-09-21 (5 weeks); no report yet against 1.0.87+ - needs re-verification on current 1.0.103.

---

### GC-008: Session hangs after work appears complete; Escape recovery enters permanent "Cancelling"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/2703 |
| **Reported** | 2026-04-14 |
| **Last Activity** | 2026-07-29 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.25 (original report); still reported as of 2026-07-29 |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:sessions, area:input-keyboard |

**Summary:**
After a turn's visible work appears complete, the CLI can hang in a busy/"Thinking" state and never return to a usable prompt. Pressing Escape to try to recover makes it worse: the UI transitions into a permanent `Cancelling` state where normal input and slash commands stop working entirely — the only recovery is killing and restarting the process (`copilot --resume` can then recover the session content, per a corroborating comment). Reproduction is intermittent with no isolated deterministic trigger. A second independent reporter confirms hitting the identical symptom (stuck "Thinking", Escape leads to permanent "Cancelling", process kill required) and links it to a broader cluster of related hang/concurrency reports (#2182 PTY buffer deadlock, #2770, #405, #529, #575), suggesting a shared root cause in stream-draining/abort-signal handling rather than isolated bugs. The reporter's own issue notes this symptom family had previously been closed as fixed through v1.0.21, but recurred on v1.0.25 — an incomplete or regressed fix.

**Impact on Orchestration:**
A hung session with no recovery path except a hard process kill is severe for both MOSAIC execution modes: an interactive hub-and-spoke session would stall indefinitely waiting on a turn that never returns control, and a headless Runner-driven stage invoking the CLI as a subprocess would need external timeout/kill handling to detect and recover from this, since the CLI itself provides no internal escape once stuck. Because the trigger is intermittent and possibly tied to background/sub-agent activity (per the reporter's own note, unconfirmed), this could manifest unpredictably in exactly the multi-tool-call, potentially-subagent-invoking sessions MOSAIC drives.

**Evidence:**
- Two independent reporters describing the identical symptom (post-completion hang, Escape → permanent Cancelling, process kill required)
- Second reporter explicitly frames it as part of a recognized cluster of related hang/concurrency issues with plausible shared root cause (stream-draining, abort-signal propagation) rather than a one-off
- No maintainer acknowledgment or linked fix PR; original report is from April 2026 (1.0.25), corroborating comment from July 2026 — spans several releases with no confirmed resolution
- 2 reactions on the original report

**Workaround(s):**
1. Kill and restart the CLI process when stuck (no clean in-session recovery exists); `copilot --resume` can then be used to recover the session's prior content rather than losing it entirely, per the corroborating commenter.
2. One related/older issue in the cited cluster (#405) reported that using `--disable-parallel-tools-execution` stopped freezes for some users — unconfirmed for this exact issue, but worth testing if MOSAIC hits this pattern, since it may reduce the concurrency conditions believed to trigger the family of hangs.

**Notes:**
Run 20 (2026-10-25): still open, 1 comment, no new activity since 2026-07-29 (>12 weeks), no maintainer response. Several hang-related fixes shipped (1.0.83 "Stopping a timed-out shell command now lets queued messages run and sessions return to idle"; 1.0.85 MCP tool-call cancel and Escape handling; 1.0.100 stall-on-MCP-load fix), none claiming this exact post-completion hang - plausible partial mitigation, unconfirmed. Still Needs re-verification on 1.0.103. Earlier note: flagged **Needs re-verification** on the current version (1.0.83) — last confirmed sighting was 2026-07-29, roughly 6+ weeks and several releases behind current at time of this scan, and the issue's own history shows this symptom family has been closed-then-recurred at least once already. Given the severity (unrecoverable hang, process kill required) and the plausible link to background/sub-agent activity, this is a high-priority candidate for direct reproduction testing at MOSAIC, especially in Runner-driven headless invocations where an external stage-level timeout/kill-and-retry wrapper would be a prudent structural mitigation regardless of upstream fix status.

---

### GC-009: Non-interactive (`-p`) sessions: tool-call approval silently and permanently revoked mid-session, unrecoverable "Permission denied and could not request permission from user"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4433 |
| **Reported** | 2026-08-11 |
| **Last Activity** | 2026-08-27 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.78, 1.0.79 |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | none applied |

**Summary:**
In non-interactive `-p`/`--prompt` mode, long sessions (~4-8 minutes, many tool calls) begin silently and permanently denying every write-capable tool call — and, inconsistently, some read-only/diagnostic commands and all MCP tool calls — partway through, even with `--allow-all` plus explicit `--allow-tool` grants already in force. There is no recovery for the remainder of that session. The GitHub issue is marked **Closed / completed**, but it was closed by the reporter themselves with no maintainer engagement, no linked fix/PR, and no confirmation the underlying cause was addressed — the single comment on the thread is unrelated third-party solicitation, not a resolution. Per the "verify closures yourself" rule, this closure does not represent a real fix and the behavior should be treated as still present.

**Impact on Orchestration:**
MOSAIC's Runner pipeline (`Tools/Runner`) drives headless, non-interactive CLI invocations per workflow stage — exactly the `-p`/non-TTY invocation pattern this bug targets. If a workflow stage runs long enough (many tool calls: file reads/writes/edits, subagent-equivalent shell invocations, MCP calls), tool execution could silently and permanently fail partway through with no way to recover within that session, corrupting or truncating the stage's work with an opaque, unrecoverable permission error — even though `--allow-all` was passed. This is a severe, silent tool-execution reliability risk specifically for headless orchestration, MOSAIC's primary automated execution path.

**Evidence:**
- Single reporter, no maintainer or third-party technical confirmation
- Exceptionally thorough reproduction: exact version/environment info, multiple independent runs isolating the variable (long vs. short session, same directory/flags/agent), ruled out workspace trust, permission flags, session/call volume alone, disable-MCP flags, working directory, and active managed/org policy (confirmed via the CLI's own startup log lines)
- Reporter decompiled the shipped Windows bundle and identified a plausible internal code path (`beginManagedSettingsResolution()` / `bypassPermissionsDisabledByPolicy` / a fail-closed branch zeroing `approveAllToolPermissionRequests`) consistent with the observed behavior, though could not fully confirm it fires for this account
- Meets all three Unverified-inclusion bars: threatens a core MOSAIC pattern (headless `-p` tool execution reliability), observed on stock Copilot CLI infrastructure (no custom backend/proxy), and includes concrete, detailed reproduction steps
- Issue closed by the reporter (not a maintainer) with `state_reason: completed` but zero evidence of an actual fix — the only comment is an unrelated product pitch, not a resolution confirmation

**Workaround(s):**
No confirmed workaround. The reporter found none — `--allow-all` combined with explicit `--allow-tool` grants does not prevent it, and the issue includes no maintainer-suggested mitigation.

**Notes:**
Run 20 (2026-10-25): no new activity since 2026-08-27 (still the self-closure + one unrelated solicitation comment); no related release-note entry in 1.0.83-1.0.103 (1.0.85/1.0.87 yolo/managed-policy startup fixes - "Keep --yolo enabled after startup policy checks for authenticated unmanaged sessions" - touch the fail-closed managed-settings path the reporter suspected, so a silent fix is plausible but unconfirmed). Needs re-verification. Tracked as ACTIVE despite the GitHub "closed/completed" status because the closure is self-administered by the reporter with no maintainer acknowledgment or fix evidence — per CaptureGuide's "don't trust duplicate/closure labels" rule, a stale/unverified closure must be investigated, not taken at face value. This should be re-verified in a future run: check if the reporter left an explanation elsewhere, if related issues have since surfaced, or if a newer CLI version (currently at 1.0.83, three-plus versions past the 1.0.78/1.0.79 report) has addressed it silently. Given the severity for MOSAIC's headless Runner pipeline, recommend prioritizing a re-check and/or direct reproduction attempt at MOSAIC. Thematically adjacent to GC-016 (hook `deny` ignored) and GC-019 (managed setting bypassed) — all concern permission/approval enforcement breaking down in non-interactive mode — worth a combined re-check.

Cross-reference (run 20, discovery pass): the process-log line `[rust:permissions_engine::orchestrator] permission request denied: unusable session working directory` is the diagnostic marker for this error text ("Permission denied and could not request permission from user ... This is not an OS or sandbox error") in at least two other reports. #5173 (hard-deny of `mkdir X && cd X && ...` compound commands under `/allow-all`, 1.0.99-0 to 1.0.100, reporter confirmed fixed in 1.0.101) and #5206 (open, 2026-10-20, single reporter, Linux: in RESUMED interactive sessions with allow-all active, escalated or out-of-cwd commands are silently denied with the same message and log line, first seen 1.0.99-2, still on 1.0.101-0; `copilot help config` states `defaultPermissionMode` is ignored on resumed sessions, so resume may intentionally drop the bypass while session state still shows allow-all on). If a headless `-p` stage using `--resume` shows this approval loop, grep the process log for that marker; it is not yet established that GC-009's `-p` cause is the same code path.

---

### GC-010: No approval required for any tool execution inside Docker sandboxes, even with `/yolo off`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4609 |
| **Reported** | 2026-08-26 |
| **Last Activity** | 2026-09-10 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.83 (Docker Sandboxes feature v0.39.0) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions |

**Summary:**
When running Copilot CLI inside a Docker sandbox (`sbx run copilot`, a Windows-only opt-in sandboxing feature), no tool execution requires approval regardless of the `/yolo` setting — file create/edit/delete, arbitrary shell/curl calls, and running generated scripts all execute without prompting, even though `/yolo show` reports `off` and states individual approval is required. The same CLI running directly on Windows (outside the Docker sandbox) correctly prompts for approval with identical settings. Open, single reporter, no maintainer response yet.

**Impact on Orchestration:**
This is a complete bypass of the tool-approval/permission layer, scoped specifically to the Docker sandbox execution environment. It does not directly affect MOSAIC's primary headless execution path (Runner invokes `copilot` as a plain subprocess, not via the Docker sandbox feature) but would matter significantly if MOSAIC or a user ever adopts Docker sandboxing for isolation — any workflow relying on permission gating (e.g., HITL approval checkpoints, restricted tool scopes for a subagent) would silently have no enforcement at all inside that environment, with tools executing exactly as if `--allow-all` were passed even when it wasn't.

**Evidence:**
- Single reporter, no maintainer acknowledgment or third-party reproduction yet
- Labeled `area:permissions` and `Bug` issue type by the reporter/repo triage, but this alone is not maintainer confirmation
- Clear, specific reproduction steps (install `winget Docker.sbx`, `sbx run copilot`, `/yolo off`, ask for file/network/script operations) and relevant environment details (`permissions.disableBypassPermissionsMode: disable`, no `permissions-config.json`)
- Meets Unverified-inclusion bar: concrete repro steps provided, not a custom backend, and touches a core safety mechanism (tool execution permission gating) — though the Materiality bar is weaker here since MOSAIC's headless Runner does not currently invoke the Docker sandbox feature, hence MEDIUM rather than HIGH impact

**Workaround(s):**
No confirmed workaround reported. Avoiding the Docker Sandboxes feature entirely (running Copilot CLI directly on the host) is the only known mitigation, per the reporter's own comparison.

**Notes:**
Run 20 (2026-10-25): still open, zero comments, no maintainer response, last activity 2026-09-10 (6+ weeks) - Needs re-verification on 1.0.103 (sandbox-related entries exist in recent release notes but none name Docker or approvals). Scope is currently narrow — this only reproduces inside the opt-in Docker Sandboxes feature (`sbx run copilot`), which MOSAIC's Runner pipeline does not use by default. Flagged for awareness in case MOSAIC adopts container-based sandboxing for subagent isolation in the future, at which point this would become a HIGH-impact permission-bypass concern. Thematically close to GC-019 (managed policy bypass on non-interactive path) — both show permission enforcement failing to apply in a specific execution mode. Re-check in a future run for maintainer response.

---

### GC-012: `/allow-all` slash command does not suppress bash/shell tool execution prompts

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/2955 |
| **Reported** | 2026-04-24 |
| **Last Activity** | 2026-10-20 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.35 (reported); confirmed again on Linux 2026-08-29 without a version stated; 1.0.101 (Linux, managed-policy variant, 2026-10-20) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions |

**Summary:**
Running the interactive `/allow-all` slash command mid-session does not suppress the "Do you want to run this command?" approval dialog for bash/shell tool calls — every shell invocation still prompts individually, even after `/reset-allowed-tools` followed by `/allow-all` again. Independently reproduced on both macOS (original reporter) and Linux (later commenter), no maintainer response. Reported against a fairly old version (1.0.35); the Linux confirmation is recent (2026-08-29) but doesn't state a version, so it's unconfirmed whether this persists on the current 1.0.83.

**Impact on Orchestration:**
This is the interactive `/allow-all` slash command, not the CLI launch flag (`--allow-all`/`--allow-all-tools`, tracked separately under GC-009's context) — so it mainly affects MOSAIC's interactive/harness-native orchestration sessions where a human or the orchestrator issues `/allow-all` mid-session expecting subsequent shell tool calls to proceed without prompts. If subagent delegation or tool-heavy stages rely on this to avoid prompt pile-up during an interactive run, they would instead hit repeated approval dialogs, stalling any unattended portion of an otherwise-interactive session. Headless Runner-driven sessions that pass allow flags at launch are less likely affected, since this bug is specific to the slash command.

**Evidence:**
- Two independent reporters on different OSes (macOS, Linux) report the identical symptom with matching repro steps
- Clear, minimal reproduction steps in both reports
- No maintainer acknowledgment or fix PR
- Confidence set to Likely rather than Confirmed: independent reproduction across platforms supports it, but reporter count is low (2) and no maintainer engagement exists

**Workaround(s):**
1. ★ Prefer native built-in tools (`view`, `grep`, `glob`) over bash/shell tool calls where possible, since built-in tools are not subject to the same prompt — per the original reporter (unconfirmed by others, but the only workaround offered).
2. Use launch-time flags (`--allow-all`/`--allow-tool=shell`) instead of the in-session `/allow-all` slash command, since GC-009's investigation suggests the launch flags are handled through a different code path (though that path has its own separate bug — see GC-009).

**Notes:**
Run 20 (2026-10-25): still open, no maintainer response or fix PR. Two new comments (2026-10-20, same user) on 1.0.101 Linux narrow a reproducing trigger: with an MDM/device-managed policy, the presence of ANY managed `permissions` rules (even deny-only, or a single unrelated `ask` rule) puts every shell command into one-shot Yes/No approval (`managedApprovalRequired: true`); allow-all and saved approvals in `permissions-config.json` are ignored, even for commands matching no managed rule. A `sandbox`-only managed block does not cause it. This confirms the symptom persists on a recent version (1.0.101) but may be a distinct managed-policy cause rather than the original unmanaged macOS repro; the same user also sees `permission.requested` delivery timeouts (cross-ref #5100). Relevant to MOSAIC only in org-managed environments. The original report on 1.0.35 remains unverified on current builds.

---

### GC-013: Agent Skill `allowed-tools` frontmatter is ignored in non-interactive (`-p`) mode, causing permission-denied loops

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/3699 |
| **Reported** | 2026-06-05 |
| **Last Activity** | 2026-07-14 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.60 |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:non-interactive, area:plugins |

**Summary:**
A Skill's `allowed-tools` frontmatter (e.g., declaring `bash`, `shell`, `powershell` as pre-approved) is honored in interactive Copilot CLI sessions but ignored in non-interactive `-p` mode — the declared tools still trigger repeated `Permission denied and could not request permission from user` errors, burning tokens, time, and cost on a mode that can never resolve the denial (no interactive prompt channel exists). Open, single reporter; the one follow-up comment is a non-maintainer design suggestion (propose a preflight capability check unifying interactive/non-interactive policy evaluation) rather than a confirmation of the underlying behavior.

**Impact on Orchestration:**
This directly threatens a core MOSAIC pattern: Skills (`Catalog/Skills/`) are deployed with frontmatter that can declare tool scoping, and MOSAIC's Runner pipeline drives GHCP CLI specifically in non-interactive `-p` mode for headless workflow stages. If a deployed Skill's `allowed-tools` declaration is silently ignored in this mode, any subagent or workflow stage using that Skill would repeatedly hit unrecoverable permission denials on tools it was explicitly declared to have — wasting tokens/turns on futile retries with no way to grant approval (there is no user to prompt), and potentially causing the stage to fail outright or produce degraded partial output.

**Evidence:**
- Single reporter, no maintainer acknowledgment
- One community comment proposes a design fix (unified preflight capability check across interactive/non-interactive modes) but does not independently confirm the reported behavior — it accepts the premise and argues for a specific remediation approach
- Reproduction steps are clear in principle (any skill with `allowed-tools`, compare interactive vs. `-p` invocation) though not maximally detailed
- Meets Unverified-inclusion bar: this squarely threatens a core MOSAIC orchestration pattern (Skills + headless non-interactive execution), is not tied to a custom backend, and the repro condition is concrete and easy to verify directly

**Workaround(s):**
No confirmed workaround reported. The suggested remediation (unified preflight policy evaluator) is a proposed fix design, not a user-side workaround. Possible mitigation to test directly: passing the same tools explicitly via `--allow-tool=<tool>` launch flags in addition to the Skill's `allowed-tools` frontmatter, though this is untested here and GC-009/GC-012 suggest launch-time allow-flags have their own separate reliability problems in `-p` mode.

**Notes:**
Run 20 (2026-10-25): re-checked; issue and its single comment unchanged, still open, no maintainer response or fix PR; no activity since 2026-07-14 (~15 weeks). Latest release is now v1.0.103 (report is against 1.0.60, ~43 releases behind) — still flagged **Needs re-verification**. Given the direct relevance to MOSAIC's Skill-deployment + headless-Runner combination, this is a high-priority candidate for direct reproduction testing at MOSAIC rather than relying solely on upstream resolution. Cross-reference GC-009 (broader non-interactive permission-approval failures) and GC-012 (`/allow-all` not suppressing prompts) — all three point to a recurring theme of GHCP CLI's tool-approval system behaving unreliably specifically in automated/non-interactive contexts.

---

### GC-014: allowed_directories in permissions-config.json never loaded

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4398 |
| **Reported** | 2026-08-07 |
| **Last Activity** | 2026-08-08 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.79-6 (unknown if fixed since) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:configuration |

**Summary:**
`allowed_directories` entries configured per-location in `~/.copilot/permissions-config.json` are silently ignored — `/list-dirs` never shows them and file reads under those paths still trigger a permission prompt. No maintainer response or independent confirmation yet; reporter gave a precise, minimal repro (create directory, add to config under the location key, observe it's absent from the loaded allowlist).

**Impact on Orchestration:**
MOSAIC's headless Runner pipeline drives non-interactive CLI invocations per workflow stage; pre-configuring allowed directories is the intended way to avoid permission prompts blocking automated file access across subagent working trees. If this config section is never loaded, any headless stage touching a directory outside the default allowlist will hit a permission prompt that cannot be answered non-interactively — the run stalls or fails rather than proceeding.

**Evidence:**
- Single reporter, no maintainer acknowledgment, no independent reproduction yet.
- Reporter provided exact, reproducible CLI steps (mkdir, jq-edit the config, restart, `/list-dirs`, then a file-read prompt) — concrete and mechanical, not environment-specific.
- Labeled `area:permissions`, `area:configuration` by the tracker (filing-time label, not a confirmation).

**Workaround(s):**
No workaround identified in the issue thread. The only directories that reliably appear are the ones auto-derived from the cwd/workspace and a small hardcoded set (`/tmp`, plugin install dir) — pre-staging files inside those default directories is the only currently-known avoidance.

**Notes:**
Related to GC-015 and GC-018 by theme (permission/allowlist config-matching failures in the same subsystem) but the root cause is distinct: this is a config *section* (`allowed_directories`) apparently not being read at all, versus GC-015's tokenization failure on multi-word `commandIdentifiers` and GC-018's CLI-flag pattern matching gap. Not a duplicate of either — tracked separately per CaptureGuide guidance to verify rather than assume from surface similarity. Needs re-verification against a current 1.0.8x build once user has hands-on access.

Run 20 (2026-10-25): re-checked; body unchanged, zero comments, still open, no maintainer response or fix PR. No activity since 2026-08-08 (~11 weeks) against latest v1.0.103 (report on 1.0.79-6) — flagged **Needs re-verification**.

---

### GC-015: commandIdentifiers with spaces still require approval

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4150 |
| **Reported** | 2026-07-16 |
| **Last Activity** | 2026-10-15 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.71 (reported); independently confirmed on 1.0.98 (2026-10-15) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:configuration |

**Summary:**
A `commandIdentifiers` entry containing a space (e.g. `"make fix"`) in `permissions-config.json` does not match the invoked command — the tool still prompts for approval. None of the documented pattern variants tested (`make:fix`, `make(fix)`, `shell(make fix)`, `bash(make fix)`) work either. Only the bare single-word identifier (`"make"`, matching the whole command regardless of arguments) is honored. No maintainer response; 3 upvotes. A 2026-10-15 comment independently reproduces it on 1.0.98 (Windows) and generalizes: ANY identifier containing a space (including documented `gh pr:*` / `mkdir -p:*` forms) is silently dropped and never matches, so `commandIdentifiers` is effectively limited to a bare program name.

**Impact on Orchestration:**
Headless Runner stages that need to pre-approve a *specific* subcommand invocation (rather than blanket-approving an entire executable) cannot do so reliably — forcing operators to either accept broader wildcard approval (weakens the permission boundary MOSAIC relies on for safe automation) or leave the narrower rule in place and eat an unanswerable permission prompt in non-interactive runs.

**Evidence:**
- No maintainer acknowledgment or fix PR.
- Second, independent reproduction (2026-10-15, v1.0.98, Windows) with a minimal config and control matrix showing single-word identifiers work and spaced ones are silently dropped (not a looser first-token match); cites the official docs promising `gh pr:*`-style matching. Upgraded to Likely.
- Original reporter tested every pattern variant documented in the official CLI reference for this exact feature, which is a strong, mechanical repro.

**Workaround(s):**
1. ★ Use the bare command name without arguments as the identifier (e.g. `"make"` instead of `"make fix"`) — reporter confirmed this matches and auto-approves, at the cost of approving *all* invocations of that command rather than just the intended subcommand.
2. Use a `permissionRequest` hook (fires before the permission service; receives `toolName` and `toolInput.command`) that returns `{"behavior":"allow"}` for an allow-list of command-string regexes and falls through otherwise (fail-closed). Posted by the 1.0.98 commenter (single user, community-sourced, not maintainer-confirmed); the hook's input payload is undocumented upstream (docs issue github/docs#46334). Worth considering for MOSAIC since it is also the likely path for GC-018.

**Notes:**
Thematically related to GC-014 and GC-018 (permission-matching engine gaps) but a distinct trigger — this is specifically about multi-word/argument tokenization in `commandIdentifiers`, not location-scoped directory config or the `--allow-tool` CLI flag.

Run 20 (2026-10-25): re-verified as still present on 1.0.98 by an independent commenter (latest release now v1.0.103); the earlier "Needs re-verification" flag is cleared. Still open, no maintainer response.

---

### GC-017: preToolUse hook `ask` decision auto-approved by TUI since v1.0.53

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/3590 |
| **Reported** | 2026-05-30 |
| **Last Activity** | 2026-07-14 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.53–1.0.56 confirmed broken; 1.0.52 confirmed working (regression window; unknown if fixed since 1.0.56) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:plugins |

**Summary:**
When a `preToolUse` hook returns `permissionDecision: "ask"`, the TUI permission dialog renders for roughly 50-79ms and is auto-resolved as "approved" without any user interaction — confirmed via `events.jsonl` timing evidence (79ms resolution on v1.0.56 vs. 3-5 seconds of actual user wait time on v1.0.51/52). Reporter pinpoints this as a regression introduced in v1.0.53, tied to a TUI session-initialization/component-mounting change (also correlated with broken selection-highlight and question-background colors starting the same version). One follow-up comment (non-maintainer) frames the needed fix (tri-state hook verdict normalized before dispatch, binding responses to tool_use_id) but is not an independent reproduction.

**Impact on Orchestration:**
For MOSAIC's interactive/harness-native execution mode, a hook that intends to force a human decision point (`ask`) is silently converted into automatic approval — the exact permission gate the hook was installed to enforce never actually pauses execution. This defeats hook-based human-in-the-loop guardrails in interactive sessions using this version range.

**Evidence:**
- Single reporter, but with unusually strong quantitative evidence (before/after timing comparison across versions using the CLI's own event log, precise version bisection 1.0.52 good / 1.0.53+ broken).
- 1 upvote; one non-maintainer comment discussing likely fix approach, not an independent repro.
- No maintainer acknowledgment or linked fix PR.

**Workaround(s):**
1. ★ Downgrade to v1.0.52 (or earlier) — reporter confirmed the permission dialog correctly waits for user input on that version, and that sessions *created* under 1.0.52 continue to behave correctly even after an auto-update to a broken version, suggesting the regression is tied to session/TUI initialization rather than a global runtime toggle.

**Notes:**
This is TUI-specific (interactive mode) — MOSAIC's headless Runner pipeline doesn't render the TUI, so this exact symptom (dialog flashing) wouldn't manifest there, but any interactive harness-native orchestration session using hooks with `ask` verdicts on an affected version is exposed. See GC-016 for the related `deny`-not-enforced variant; needs re-verification on current 1.0.8x line since the regression window (1.0.53-1.0.56) is well behind the current v1.0.83.

**Needs re-verification** (added run 2, 2026-09-10): Re-checked via GitHub MCP — issue body and its single comment unchanged from the original capture, still open, no maintainer response. Last activity remains 2026-07-14, now roughly 8 weeks stale; the regression window (1.0.53-1.0.56) is now three-plus months behind current v1.0.83, so it remains unconfirmed whether this still reproduces on a current build. Re-verify again in a future run, ideally with a direct reproduction attempt on the current version.

Run 20 (2026-10-25): re-checked; issue and its single comment unchanged, still open, no maintainer response, no activity since 2026-07-14 (~15 weeks); latest release is v1.0.103. Related partial change: v1.0.97 release notes state "Autopilot denies a preToolUse hook's 'ask' decision instead of stopping on a permission dialog nobody is watching" — an autopilot-only behavior that does not address the TUI auto-approve regression described here. Still **Needs re-verification**.

---

### GC-018: --allow-tool='shell(docker ps)' pattern matching fails for non-git commands

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/2610 |
| **Reported** | 2026-04-09 |
| **Last Activity** | 2026-04-16 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.21 (unknown if fixed since) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:tools |

**Summary:**
The documented `--allow-tool='shell(<subcommand>)'` pattern for pre-approving a specific shell subcommand works for `git` (reporter confirmed `shell(git push)`-style patterns match) but does not work for other commands such as `docker` — `shell(docker ps)` and `shell(docker ps:*)` both fail to match, and the run is denied with "Permission denied and could not request permission from user." No maintainer response; 4 upvotes, no further comments.

**Impact on Orchestration:**
This flag is a primary mechanism for granting narrow, non-interactive permission in headless Runner invocations (`-p`/`--prompt` mode with no human available to answer a prompt). If subcommand-scoped patterns only work for `git` and fail silently for other tools, any headless workflow stage that needs to pre-approve a specific non-git subcommand cannot do so — the stage will hit an unanswerable permission denial and fail outright rather than degrade gracefully.

**Evidence:**
- Single reporter, 4 upvotes (just under the 5+ "Likely" threshold), no maintainer acknowledgment or comments.
- Reporter isolated the difference cleanly: identical syntax works for `git` subcommands, fails for `docker`, suggesting command-specific (possibly hardcoded `git`-only) handling in the pattern matcher rather than a generic mechanism.

**Workaround(s):**
1. Use the bare command name without a subcommand pattern (e.g. `--allow-tool='shell(docker)'` or similar blanket form) if broader approval of the whole tool is acceptable — not explicitly tested in the issue thread, but consistent with the same class of workaround reported for GC-015.
2. For `git` specifically, subcommand-scoped patterns are confirmed to work as documented.

**Notes:**
Related by theme to GC-014 and GC-015 (permission-matching engine gaps across different config surfaces — CLI flag vs. config file location scoping vs. commandIdentifiers tokenization) but distinct root cause: this looks like command-specific parsing logic (git works, others don't) rather than a config-loading or tokenization defect. Tracked separately per CaptureGuide "verify yourself" guidance. Needs re-verification on current version — reported against 1.0.21, current is v1.0.83, a large version gap.

**Needs re-verification** (added run 2, 2026-09-10): Re-checked via GitHub MCP — issue body unchanged, still open, zero comments, no maintainer response. Last activity remains 2026-04-16, now roughly 21 weeks (5 months) stale against a harness many releases past the reported 1.0.21. No corroboration or refutation found this pass; this is now the most stale entry in the harness's active list and a high-priority candidate for direct reproduction at MOSAIC to resolve the long-standing uncertainty.

Run 20 (2026-10-25): re-checked; body unchanged, zero comments, 4 upvotes, still open, no maintainer response or fix PR. No activity since 2026-04-16 (~27 weeks) against latest v1.0.103 (report on 1.0.21) — remains **Needs re-verification**. Related finding from GC-015's thread: the CLI config reference documents `gh pr:*`-style prefix matching for `commandIdentifiers`; the same spaced-pattern matching gap may underlie this. Candidate workaround (from the GC-015 thread, community-sourced): a `permissionRequest` hook returning `{"behavior":"allow"}` for allow-listed command strings, since it sees the full `toolInput.command`.

---

### GC-019: Non-interactive `--yolo` bypasses `disableBypassPermissionsMode` managed setting

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4528 |
| **Reported** | 2026-08-19 |
| **Last Activity** | 2026-08-20 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.80 (unknown if fixed since) |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:permissions, area:non-interactive, area:enterprise |

**Summary:**
When an admin-managed setting (`.github-private/copilot/managed-settings.json` with `"disableBypassPermissionsMode": "disable"`) is configured to forbid bypass-permissions mode, the interactive `/allow-all`/`/yolo` command correctly refuses ("Bypass permissions mode has been disabled by policy"), but passing `--yolo -p {prompt}` (non-interactive) silently ignores the managed setting and grants full permission bypass anyway. No maintainer response, no reactions/comments.

**Impact on Orchestration:**
For MOSAIC deployments in enterprise/managed environments, this means an org-level control intended to force human oversight (or at least block full auto-approval) can be silently circumvented simply by driving the CLI non-interactively — exactly the invocation style the Runner pipeline uses. This is a governance/security-posture gap rather than a workflow-breaking defect: it makes headless automation *more* permissive than the administrator intended, which could let an automated run take actions an org explicitly tried to disallow.

**Evidence:**
- Single reporter, 0 reactions, no maintainer acknowledgment, no independent confirmation.
- Reporter provided a precise two-step repro (set the managed setting, invoke with `--yolo -p`) and contrasted it with the correctly-enforced interactive path — internally consistent and mechanical.

**Workaround(s):**
No workaround identified in the issue thread; the managed setting appears to have no effect on the non-interactive path at all. Organizations relying on `disableBypassPermissionsMode` should not assume it constrains headless/`-p` invocations until this is confirmed fixed.

**Notes:**
Distinct from GC-016/GC-017 (which are about hook-issued verdicts not being enforced) and from GC-014/GC-015/GC-018 (config/flag pattern-matching gaps) — this is specifically about an enterprise policy toggle not being consulted at all on the non-interactive code path. Relevant primarily to MOSAIC deployments operating under org-managed Copilot CLI settings; low relevance for unmanaged/individual setups.

Re-checked run 2 (2026-09-10): Re-fetched via GitHub MCP — issue body unchanged, still open, zero comments, no maintainer response. Last activity 2026-08-20, only ~3 weeks stale — under the 6-week re-verification threshold, so no "Needs re-verification" flag added this pass; still worth re-checking on a future run given the enterprise-governance severity.

Run 20 (2026-10-25): re-checked; body unchanged, zero comments, still open, no maintainer response or fix PR. Now ~9 weeks without activity against latest v1.0.103 (report on 1.0.80), several managed-policy related release-note items since (e.g. 1.0.99 "Managed account policy and bypass-mode warnings stay accurate after switching accounts") but none explicitly addressing `-p --yolo` ignoring `disableBypassPermissionsMode`. Flagged **Needs re-verification**.

---

### GC-020: Repo-level hooks (`.github/hooks/*.json`) are silently not loaded in `-p` mode unless the working directory is a trusted folder

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | MOSAIC experiments, 2026-10-03 (Windows 11); no upstream issue filed |
| **Reported** | 2026-10-03 (MOSAIC) |
| **Last Activity** | 2026-10-03 |
| **Confidence** | Confirmed (reproduced at MOSAIC) |
| **Orchestration Impact** | HIGH for hook-based logging and guardrails in headless runs; no effect on runs that do not rely on repo hooks |
| **Reproduced at MOSAIC** | Yes |
| **MOSAIC Response** | Mitigated: the Runner checks trust (read-only) at every GHCP CLI session start and warns; in the TUI it asks whether to proceed |
| **Version(s) Affected** | 1.0.87 (unknown for other versions) |
| **Latest Platform Version** | v1.0.87 (2026-10-03) |
| **Labels** | `area:hooks`, `area:non-interactive`, `area:trust` |

**Summary:**
Repo-level hooks in `.github/hooks/*.json` are not loaded in `-p` mode unless the working directory equals or is a descendant of a `trustedFolders` entry in `$COPILOT_HOME/config.json` (else `~/.copilot/config.json`). The untrusted case exits 0 with no warning, so every hook silently never runs.

Matching rules found by probing (Windows 11):
- Case-insensitive and separator-tolerant.
- An ancestor entry grants trust to its descendants; a child entry does not cover its parent.
- `trustedFolders` in `settings.json` is ignored; only `config.json` counts.
- `config.json` starts with `//` comments (JSONC), so a plain JSON parser fails on it.
- `--yolo` and `--add-dir` do not grant trust.
- Not probed: segment-boundary prefixes (a sibling folder sharing a name prefix), UNC paths, Linux/macOS, policy files.

**Impact on Orchestration:**
A Runner run in an untrusted folder produces no hook-written MOSAIC logs and gives no sign of the problem; missing logs look like a logger bug.

**Evidence:**
- Same repo and hook file run in `-p` mode, differing only in whether the working directory was covered by a `trustedFolders` entry: hooks fired only when trusted.
- Entry-placement variants (ancestor, child, other case, other separators, `settings.json`) probed to derive the rules above.

**Workaround(s):**
Trust the project folder (or an ancestor) in Copilot CLI before running. The Runner never edits the Copilot configuration; it only warns.

**Notes:**
Related to GC-003 (workspace `.mcp.json` not wired in an untrusted workspace; one reporter saw it work once the workspace was marked trusted) and to GC-014 (another configuration surface that is never loaded): all are "configuration silently not applied" cases, and folder trust is plausibly the shared precondition for GC-003. Not investigated whether trust also gates other repo-level artifacts. See also CC-082 and OC-099 for the same class of hook-output-cannot-be-assumed behaviour in the other harnesses.

Upstream check (run 20, 2026-10-25; MOSAIC findings unchanged): still no upstream issue filed. Copilot CLI v1.0.100 (2026-10-19) added `--trust-folder` for one-off prompt-mode sessions started with `-p` or piped stdin, i.e. an upstream CLI-flag route to trust a folder for a single run without editing `config.json` (not yet probed at MOSAIC; behaviour for repo hooks unverified). Latest release v1.0.103.

Cross-reference (run 20, discovery pass): folder trust has wider effects than hook loading. GC-037 (#5124): `--agent` selection silently falls back to the default agent in an untrusted workspace (reported for the Copilot app; CLI `-p` unverified). #5232 (open, 1.0.102, Windows, single reporter plus one comment): in `manual` permission mode, shell commands whose referenced paths are all inside a trusted folder run with no `permission.requested` event (a PowerShell script created a database), i.e. trust acts as a standing shell approval. #4878 (feature request for a temporary per-run trust flag) was addressed by `--trust-folder` in 1.0.100 per release notes. No separate entry for #5232: interactive-mode security behavior, no reliability effect on headless runs.


---

### GC-021: No non-argv prompt input; `-p <text>` is mangled by cmd.exe through the npm `copilot.cmd` shim on Windows

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | MOSAIC experiments, 2026-10-06 (Windows 11); no upstream issue filed |
| **Reported** | 2026-10-06 (MOSAIC) |
| **Last Activity** | 2026-10-06 |
| **Confidence** | Confirmed (reproduced at MOSAIC) |
| **Orchestration Impact** | HIGH for Runner dispatch on Windows when `copilot` resolves to the npm shim; none on native binaries |
| **Reproduced at MOSAIC** | Yes |
| **MOSAIC Response** | Mitigated: the prompt is piped on stdin and `-p` is never emitted |
| **Version(s) Affected** | 1.0.91 and 1.0.92 (stdin routes verified: direct exe on 1.0.91, `copilot.cmd` shim on 1.0.92); the cmd.exe mangling itself is version-independent |
| **Latest Platform Version** | v1.0.92 (2026-10-05) |
| **Labels** | `area:non-interactive`, `area:windows`, `area:prompt-delivery` |

**Summary:**
The CLI documents no flag that reads the prompt from a file or stdin; only `-p <text>`. On Windows, `copilot` usually resolves to `%APPDATA%\npm\copilot.cmd`, whose last line forwards `%*` to node. A `-p` value therefore passes through cmd.exe: defined `%VAR%`, `^`, `&`, `|`, `<`, `>` and quotes are mangled, and a JSON request exits 1 with no output.

Stdin behaviour matrix (live runs, ground truth is the `user.message` event in `--output-format json`). Running code version taken from the `pkg\win32-x64\<version>` paths in the event stream, not from `copilot --version`: the CLI updated itself from 1.0.91 to 1.0.92 in the background mid-session (2026-10-06 20:36 local) while `--version` still reported 1.0.91. The direct-exe cases ran 1.0.91; the `cmd /c copilot.cmd` cases ran 1.0.92:
- `-p` omitted + piped stdin: the prompt equals stdin exactly, on the direct binary and through `cmd /c copilot.cmd`, with `--yolo --no-ask-user`, `--agent`, `--agent` + `--model`, a payload without trailing newline, and a 38 568-byte payload (past the 8 191 cmd.exe and 32 767 CreateProcess limits).
- `-p ""` + piped stdin: same result.
- A non-empty `-p` value takes precedence and stdin is silently dropped.
- A bare `-p` with no value is a parse error (exit 1).

**Impact on Orchestration:**
Orchestrator requests (single-line JSON with quotes, Windows paths) never reach the model intact through the shim when sent via `-p`.

**Evidence:**
Byte-for-byte comparison of stdin payloads (special characters, paths, trailing backslash) against the CLI's `user.message` event across the routes above.

**Workaround(s):**
Pipe the prompt on stdin and omit `-p`. Not live-probed: the partial-allowlist mode; prompt delivery is independent of those flags, but `--allow-tool` and extra-argument values still travel in argv through cmd.exe.

**Notes:**
Same class as the Claude Code and OpenCode argv-delivery problems fixed by stdin delivery. Raw probe outputs: `C:\AI\MOSAIC\HarnessProbes\GhcpAgentDeselect\20261006-stdin-probes\` (`secondpass-direct\live\` = 1.0.91 direct exe, `thirdpass-shim\out\` = 1.0.92 through the shim). Because of the silent self-update, `copilot --version` does not reliably give the running version; read it from the event stream. In the Runner, GC-021 can look like GC-022 (resolved: agent dropped at session start), since both surface as `reply contains no JSON object` / `BLOCKED E501`. Tell them apart by the JSONL: `subagent.deselected` means GC-022; exit 1, empty output, or a `user.message` differing from the request means GC-021.

Upstream check (run 20, 2026-10-25; MOSAIC findings unchanged): still no upstream issue filed for argv mangling through `copilot.cmd`. Release notes now treat piped stdin as a supported prompt-mode route: v1.0.100 "--trust-folder for one-off prompt-mode sessions started with -p or piped stdin" and v1.0.102 "Prompt mode now honors managed Auto model selection for -p and piped stdin runs". Latest release v1.0.103 (2026-10-22); the stdin routes were verified at MOSAIC only on 1.0.91/1.0.92.

---

### GC-033: Subagents started in the same turn as an approved `exit_plan_mode` get none of the built-in tools (regression after 1.0.95)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5243 |
| **Reported** | 2026-10-23 |
| **Last Activity** | 2026-10-24 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.100-1 (as bundled in VS Code 1.143.0 SDK host); 1.0.92 and 1.0.95 reported not affected |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:agents` |

**Summary:**
When `exit_plan_mode` is approved with the `interactive` action and the agent continues in the same turn, every subagent it starts via `task` runs without any built-in tools (no shell, view, create, edit, grep, glob, sql, task, skill, web_fetch); only host-registered external tools remain. The main agent keeps its tools, and subagents from later turns get the full set. No comments, no maintainer response yet.

**Impact on Orchestration:**
A plan-then-implement flow that delegates to subagents right after plan approval silently yields subagents that cannot read, write or run anything; they improvise with whatever external tools exist. Affects only plan-mode-to-implementation transitions via an SDK host, not plain `-p` runs.

**Evidence:**
- Reporter's own session-log counts over 20 days: 6 failing calls (1.0.100-1, approval turn), 34 working calls (1.0.100-1, normal turn), 78 working calls on 1.0.92/1.0.95 in the approval turn.
- Concrete repro with a `Probe` custom agent; session log shows `session.mode_changed` plan to interactive before `subagent.started`.
- Single reporter, BYOK Claude model, SDK host (VS Code Agents window); only `interactive` action tested (not autopilot).

**Workaround(s):**
1. Start delegating subagents in a turn after the approval turn (e.g. let the plan approval turn end first). Reporter data only; not tested by others.
2. Pin to 1.0.95 or earlier (reporter data: not affected).

**Notes:**
Included as Unverified because it hits the core subagent-dispatch pattern with concrete repro, though trigger (plan mode via SDK host) is narrow for MOSAIC's headless runs. Companion VS Code report microsoft/vscode#343716. Related: GC-005 (silent tool-less/empty subagents).

---

### GC-023: Failed/never-completing compaction OOMs the process and leaves the session permanently unresumable (compaction retried on every `--resume`)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4780 |
| **Reported** | 2026-09-09 |
| **Last Activity** | 2026-09-24 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.83 (Linux); related OOM reports through 1.0.93 (see GC-024); unknown whether 1.0.97 "emergency memory relief" changes this |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | triage |

**Summary:**
A session reaching the compaction threshold (about 161k of 200k tokens) emitted `session.compaction_start` four times with no completion event; each aligned with a V8 heap OOM (about 4.3 GB cap, 56 GB RAM free). After a failed compaction the turn proceeds anyway and context keeps growing, so each retry is larger. Every `--resume` re-enters the same compaction and crashes, so the session cannot be reopened (a 1.6 MB / 437-event session needed more than 4 GB of heap). A second user saw the same with `--max-old-space-size=6000` (crash merely delayed).

**Impact on Orchestration:**
A long MOSAIC primary or Runner session that crosses the compaction threshold can die in a loop and become unrecoverable, losing the in-flight run state. Resume-based recovery in the Runner would loop on the same crash.

**Evidence:**
- Single reporter with unusually detailed data (four `--report-on-fatalerror` reports, event-log timeline of compaction starts without completions); one corroborating comment from another user (heap raise only delays crash, though without a confirmed compaction marker)
- 3 reactions; label `triage` only, no maintainer response
- Included as Unverified because it threatens context-management/session stability for MOSAIC and has concrete mechanical evidence on stock infrastructure

**Workaround(s):**
1. Community rescue script (gist linked in the issue's comment 5677177668) copies the stuck session into a fresh, optionally trimmed session (`--keep-last N`/`--max-bytes`); never modifies the original. Not tested by us.
2. `--node-options="--max-old-space-size=N"` (the CLI ignores `NODE_OPTIONS`); only delays the failure.
3. Keep Runner stage sessions short and persist progress on disk so a lost session can be restarted rather than resumed.

**Notes:**
Distinct from GC-001 (watchdog compacts at low context usage and loops) and GC-007 (heap growth on `--resume`): here the compaction step itself is the memory sink and the loop is made permanent by resume. Release 1.0.97 notes: "a session that would have been killed by a JavaScript heap out of memory crash may survive with compacted history" - may mitigate; Needs re-verification on 1.0.97+ (last activity 2026-09-24, over 4 weeks).

---

### GC-024: Interactive sessions hit V8 heap OOM (about 4.3 GB cap) after minutes to hours even without `--resume`; heap raise only delays; reports through 1.0.93

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4725 (also https://github.com/github/copilot-cli/issues/4686) |
| **Reported** | 2026-09-01 |
| **Last Activity** | 2026-10-09 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.82 through 1.0.93 (Linux, macOS, Windows); unknown on 1.0.94+ |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:platform-linux (#4725), area:sessions (#4686) |

**Summary:**
Multiple independent users report `Ineffective mark-compacts near heap limit ... JavaScript heap out of memory` in sessions that were never resumed: after a few minutes of agentic work (#4725), after about 37 min with about 32k active unreferenced `async` libuv handles (#4686, original reporter's theory of a per-turn async-context leak; other reporters had only about 6 such handles so it is not the universal cause), or after 24-42 h in autopilot with hundreds of compactions (#4725 comment 2026-09-26: 29 of 37 compactions were `memory_pressure`; #4686 comment 2026-10-09 on 1.0.93: 825 compactions that did not help, GC freed about 9 MB, memory is retained). Heap raise via `--node-options` only delays the crash. Crash reports (and on some hosts core dumps) are written into the cwd.

**Impact on Orchestration:**
Any long-lived MOSAIC session, especially one with many tool calls and sub-agents (the 42 h macOS session had 24k tool calls and 175 sub-agents), can die with total loss of in-memory state. Headless Runner stages that stay alive across many dispatches are exposed. Complements GC-007 (resume) and GC-001 (watchdog).

**Evidence:**
- Seven or more independent reporters across #4725/#4686 with quantified Node diagnostic reports; reports span 1.0.82 to 1.0.93 and three OSes
- No maintainer response on either issue (both open, no linked PR), so Likely rather than Confirmed
- Also seen: `NODE_OPTIONS` is ignored by the SEA binary; `--node-options=...` is the working knob (reporter comment, measured)

**Workaround(s):**
1. Bound session lifetime: restart long sessions and persist state on disk; the 42 h session lasted 7.5 h before the first memory-pressure compaction.
2. `copilot --node-options="--max-old-space-size=6000"` (or larger) buys time only.
3. Prefer headless/ACP drivers over the interactive TUI: GC-007 workaround 4 reports flat JS heap under ACP (single-commenter finding).
4. Set `ulimit -c 0` and run from a non-repo cwd so crash reports/core files do not land in the working tree.

**Notes:**
Run 20: #4725 last comment 2026-09-26 (1.0.88, macOS), #4686 last comment 2026-10-09 (1.0.93, Windows). Related: GC-001, GC-007, GC-023. Release 1.0.97 added "emergency memory relief" so later versions may behave differently; Needs re-verification on 1.0.97+.

---

### GC-034: Selecting a custom agent (`--agent` / `/agent`) disables deferred MCP tool loading from the second model call; all MCP tool schemas flood context and invalidate the prompt cache

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5195 |
| **Reported** | 2026-10-19 |
| **Last Activity** | 2026-10-25 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.98 (reporter); 1.0.102-0.unstable (second reporter, desktop app) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:agents`, `area:context-memory`, `area:mcp` |

**Summary:**
With tool search and MCP dynamic retrieval enabled, the default agent keeps MCP tools deferred (MCP Tools 0 in `/context`). With a custom agent selected, deferral holds only for the first model call; from the second call every tool of every configured MCP server is loaded (26.9k tokens for one ~100-tool server in the original report; ~150k for 13 servers/~390 tools in the follow-up). Reproduces in `copilot -p "Hi" --agent <name>`. A second user reports the deferral flips back and forth (restored on user prompts, lost on agent follow-up requests), rewriting the prompt cache twice per turn. No maintainer response yet.

**Impact on Orchestration:**
Every MOSAIC subagent run through `--agent` on a machine with many MCP servers silently pays tens of thousands of extra tool-schema tokens per call, shrinking usable context and defeating prompt caching. Also makes `tool_search_tool` return "No tools found" (tools already loaded; see #4773). This is a silent context-window cost, not just a billing one.

**Evidence:**
- Original reporter: `events.jsonl` `session.usage_record` data (toolTokenCount 11485 on call 1 vs 40069 on call 2; constant 11207 without `--agent`), reproduced interactively and with `-p`.
- Second independent reporter (1 comment, 2026-10-25) with per-request token table on a different CLI build and a custom agent that has a `tools:` list.
- No maintainer comment or fix PR.

**Workaround(s):**
1. `--disable-mcp-server <name>` for servers the agent does not need (reporter-suggested; not independently confirmed).
2. Keep configured MCP servers to the minimum for headless runs.

**Notes:**
Related: GC-005 (MCP tool-schema volume breaching a budget causes silent empty subagents) - the same schema bloat may push a custom-agent subagent over that budget. Related: #5237 (plugin custom agent cancels its own HTTP MCP calls; not tracked, plugin-specific) and #5191 (GC-035).

---

### GC-035: Child custom agent's MCP tool discovery (`tool_search_tool`) appears scoped to the parent agent's tool list

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5191 |
| **Reported** | 2026-10-19 |
| **Last Activity** | 2026-10-20 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.100-1 |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:agents`, `area:mcp` |

**Summary:**
A parent agent with `tools: ['agent', 'read']` delegates to a child agent declaring `tools: ['microsoft-learn/*']`. The child cannot execute the MCP tool; its tool search returns only tools permitted to the parent, not those allowed by its own frontmatter. Reported in the Copilot harness (VS Code host, CLI 1.0.100-1). Zero comments, no maintainer response.

**Impact on Orchestration:**
Hub-and-spoke orchestrators typically have only the delegation tool while specialist subagents carry their own tool/MCP scopes. If child scopes are intersected with the parent's, specialists silently lack the MCP tools their definitions grant.

**Evidence:**
- Single reporter with a complete, minimal two-file repro (parent + child agent definitions).
- Behavior described as "appears"; reporter did not confirm root cause. Host is VS Code's Copilot harness rather than a plain CLI run; not verified for `-p`.

**Workaround(s):**
1. Add the child's MCP tools to the parent's `tools` list (inferred from the report; untested).

**Notes:**
Included as Unverified under the core-pattern criterion (subagent tool scoping with concrete repro). Needs a direct headless reproduction. Related: GC-034 (custom-agent MCP tool loading), GC-005.

---

### GC-025: Session host event-delivery acknowledgement timeouts (`permission.requested` 5 s, other events 120 s) permanently break `session.send` until exit and `--resume`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5100 (also https://github.com/github/copilot-cli/issues/5018) |
| **Reported** | 2026-09-30 |
| **Last Activity** | 2026-10-12 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.88 (#5018); 1.0.90 through 1.0.96-2 (#5100); unknown on 1.0.97+ |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:sessions (#5100); area:permissions, area:sessions (#5018) |

**Summary:**
Two independent reporters (interactive sessions, macOS and Windows) describe the same failure class. In #5018 (several concurrent interactive sessions on one machine) the host does not ack a `permission.requested` event within 5 s, the permission prompt is auto-cancelled (`permission_prompt_delivery_failed`, fail-closed) and bursts of failures end in `session.error ... session.send failed`. In #5100 (15+ h resumed sessions, large contexts) the Node main thread stalls (`Node memory snapshot timed out`, `Native session event flush exceeded 10000ms`), one 120 s host-ack timeout occurs and every later delivery fails instantly; the agent loop keeps running unseen (about 6,500 streaming events in 12 min, still persisted to `events.jsonl`) while the user cannot steer or approve. Recovery in both cases requires exit and `copilot --resume`.

**Impact on Orchestration:**
A long-lived or concurrent MOSAIC session can wedge: tool calls needing approval fail instantly, the primary agent may continue unobserved, and input is rejected until restart. Relevant to multi-process fan-out (several `copilot` processes on one host) and long-lived stages; headless `-p` use was not reported, so exposure there is unknown. Failure is fail-closed (no unsafe execution).

**Evidence:**
- Two independent reports with log excerpts and counts (#5018: 13-39 auto-cancelled prompts per session over 3 sessions; #5100: 1,645 repeated failure lines in one process, 20 `session.error`s across two sessions)
- No maintainer response on either; both open; #5018 untouched since 2026-10-01
- Included as Unverified because it concerns session/permission-flow stability and mechanism is clearly logged, but triggers are non-deterministic and interactive-only

**Workaround(s):**
1. Exit and `copilot --resume` (or `/restart`) after the first failure.
2. Avoid keeping a single interactive session alive for many hours and avoid many concurrent interactive sessions on one host; pre-approve commands (`--allow-tool`) so fewer prompts depend on the 5 s ack path.
3. #5100 reporter did not see it on 1.0.95-3 initially but then saw it on 1.0.96-2, so upgrading is not a confirmed fix.

**Notes:**
Related: GC-007/GC-024 (the same long sessions often show heap pressure, and the Node-main-thread stall in #5100 looks memory/GC related). #4259 and #4469 (orphaned `permission.requested` replayed on resume) are cited by #5018 as the same event path but a different symptom; not checked. Needs re-verification on 1.0.97+.

---

### GC-026: Failed compaction is silently retried on nearly every turn with no backoff or user-visible error (context keeps growing, billed retries)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4663 (also https://github.com/github/copilot-cli/issues/5054) |
| **Reported** | 2026-08-30 |
| **Last Activity** | 2026-10-21 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.81-3 (38 and 16 consecutive failures, #4663); retry-without-backoff still observed on 1.0.83; #5054 on 1.0.89; unknown on 1.0.97+ |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:context-memory, area:models |

**Summary:**
When a compaction model call fails (any cause), the CLI re-issues essentially the same request on the next turn (observed 1 s later), with no backoff, no fallback and no transcript warning; `success: false` appears only in `session-state/<id>/events.jsonl`. #4663 (reporter-verified on 1.0.83) saw 38 consecutive failures while context grew 751k to 909k tokens; the originating 400 (deferred MCP tool not declared in the compaction request) stopped reproducing on 1.0.82, but the retry path is unchanged. #5054 is a second independent trigger: on a 1.05M-token window with xhigh reasoning the summarizer exceeds its 300 s budget ("summarizer did not settle within 300s"), 110 of 131 compactions failed, 15 in a row on one day. Open compaction-400 reports #4519 and #4572 would feed the same loop (not checked).

**Impact on Orchestration:**
A long MOSAIC primary or stage session whose compaction fails persistently silently burns credits and latency, and context grows toward the hard limit without any signal to the orchestrator or user. For headless runs there is no visible warning at all.

**Evidence:**
- Two independent reporters with event-log counts; #4663 reporter re-verified on later versions; #5054 has a second user confirming ("same issue on my sessions") and 2 reactions
- No maintainer response on either; both open; no linked PR

**Workaround(s):**
1. #4663: `copilot --resume=<id>` reloads the full tool catalogue and let compaction succeed (reclaimed 471k and 568k tokens) for the deferred-tool trigger.
2. #5054: manual `/compact` after switching to a faster model with medium reasoning effort (second user's report); or avoid sessions near the 800k+ token range.
3. Detect via the reporter's script: count `session.compaction_complete` events with `success: false` in `~/.copilot/session-state/*/events.jsonl` (match on event `type`, not raw line text).

**Notes:**
Related: GC-001 and GC-002 (compaction behavior), GC-023 (compaction that OOMs). #4663 itself distinguishes #4506/#4639 (compaction succeeds but reclaims nothing) from this hard-fail retry. Last activity of #4663 is 2026-09-23 (over 4 weeks); Needs re-verification on 1.0.97+.

---

### GC-036: Non-interactive MCP tool call hangs after the initial progress notification until a ~5 minute transport idle timeout; no result or structured error reaches the model

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4910 |
| **Reported** | 2026-09-19 |
| **Last Activity** | 2026-09-20 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.87-0 (Windows 11) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `triage` |

**Summary:**
In `-p` mode with `--allow-all-tools --no-ask-user`, an already-discovered Azure MCP tool (`azure-get_azure_bestpractices` with `learn: true`) emits one progress notification and then never returns. Debug log shows the request enqueued, a single progress notification, and a WebSocket close with `idle timeout` about 5 minutes later; the model remains blocked and the CLI surfaces no timeout. The same call succeeds immediately from VS Code's MCP client. Reproduced in three independent sessions by the one reporter; no maintainer response (reporter answered a request for details only). About 5 weeks stale, not re-checked against later releases.

**Impact on Orchestration:**
A headless stage that calls a slow or stuck MCP tool can stall for minutes with no error the orchestrator can detect, until an external watchdog kills it. MCP-server-specific trigger (Azure MCP beta), so exposure depends on which MCP servers MOSAIC stages use.

**Evidence:**
- Single reporter, three runs, debug-log timelines, direct-client control showing the tool works elsewhere.
- No `timeout` configured in the MCP server entry; the CLI does not appear to apply a bounded per-request timeout.
- Related: reporter references a maintainer-side finding about repeated progress notifications resetting the timeout; not applicable here (only one notification).

**Workaround(s):**
1. Wrap headless invocations in an external wall-clock timeout (reporter's harness watchdog eventually killed the session).
2. Avoid the affected MCP tool/server in unattended stages.

**Notes:**
Needs re-verification on current release. Related: GC-008 (session hang with no internal escape). Cited by reporter: #4678 (MCP server startup blocking), #3257 (dead pooled HTTP connection reuse); not tracked.

---

### GC-027: Tool output above roughly 20 KB is replaced by a short preview plus an OS temp-file path; no stable artifact id or paging interface

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/github/copilot-cli/issues/5003 |
| **Reported** | 2026-09-29 |
| **Last Activity** | 2026-09-30 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.89-7 (observed); presumed unchanged on current versions |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:tools (issue type: Feature) |

**Summary:**
When a tool (shell, or an MCP tool) returns more than about 20 KB, the CLI does not pass the result to the model. It substitutes a message ("Output too large to read at once (52.4 KB). Saved to: <path>. Consider using rg, head/tail, view with view_range, or jq...") and writes the full output to an OS temp file. The path exists only in display text; there is no session artifact id, continuation token or structured paging, and the temp file's lifetime is undocumented. Filed as a feature request, but the behavior itself is plainly deliberate (the message tells the agent what to do), so it is tracked as a Limitation.

**Impact on Orchestration:**
Large grep, git diff or log output and large MCP responses reach subagents as a preview only. An agent that does not follow up by reading the file path works from partial data without any error; the extra read costs a tool call and context. Tool-permission scoping that excludes file-reading tools from a subagent would make the full output unreachable.

**Evidence:**
- Single reporter, no maintainer comment, no comments at all; the behavior is visible in the CLI's own message text, which supports the by-design reading
- Included because silent partial results in tool output are a core MOSAIC tool-execution concern

**Workaround(s):**
1. Instruct agents to limit output at the source (`| head`, `rg -l`, `git show --stat`, paged MCP queries) and to follow the temp-path message with a `view`/`rg` call on that path.
2. Ensure subagents that run shell/MCP tools also have a file-read tool.

**Notes:**
Exact threshold not documented in the issue ("approximately 20 KB"). Temp-file retention not verified. Unrelated to create-file size failure (resolved GC-006).

---

### GC-037: `--agent <name>` selection silently dropped right after startup (`subagent.deselected`); an untrusted workspace falls back to the default agent with no warning

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5124 |
| **Reported** | 2026-10-12 |
| **Last Activity** | 2026-10-20 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.95 (CLI, macOS, interactive); 1.0.99-2 (Windows Copilot app, plugin agent) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:agents` |

**Summary:**
Starting `copilot --agent my-agent` shows "Selected custom agent", then about 1 s later `events.jsonl` records `subagent.deselected` and the session runs as the default agent (generic system prompt, agent's MCP tools never attached). Original reporter (interactive, macOS) saw 5/6 dropped for a user-level agent and 4/6 for a plugin agent, and hypothesized a race with sign-in / MCP reload. A second reporter (Windows Copilot app) saw 9/9 sessions dropped, then corrected their analysis: the decisive variable was folder trust. After the project folder was added to `trustedFolders`, the agent stayed selected across multiple sessions; untrusted workspaces silently fell back to the default agent with no trust prompt or error. Interactive mode only; `-p` not tested. No maintainer response; open.

**Impact on Orchestration:**
MOSAIC dispatches every role via `--agent`; a silent fallback to the default agent runs the wrong instructions and tool set while the process exits normally. Same signature as the resolved GC-022, and a second "untrusted folder silently degrades the session" behavior alongside GC-020.

**Evidence:**
- Two independent reporters with `events.jsonl` timelines; the second ran controlled variants (YAML fix, MCP removed, stub MCP, permission mode) and isolated folder trust.
- The first reporter's race hypothesis is unconfirmed; trust finding is from the app, not verified in the CLI.

**Workaround(s):**
1. Trust the working folder (or an ancestor) in `config.json` `trustedFolders` before launching (second reporter; also what GC-020 requires). Since 1.0.100, `--trust-folder` exists for `-p`/piped stdin runs (release note; not verified here).
2. Run `/agent my-agent` after startup (interactive only; first reporter).
3. Verify in `events.jsonl` that no `subagent.deselected` event follows `subagent.selected`.

**Notes:**
Cross-reference GC-022 (resolved 1.0.92: `--agent` dropped at session start, MOSAIC-observed, same `subagent.deselected` signature) and GC-020 (repo hooks only load in trusted folders). If the Runner's trust check (GC-020 mitigation) already warns on untrusted folders, it likely also covers this variant; whether the untrusted case triggers deselect in `-p` mode is unverified.

---

### GC-038: Autopilot stops after a fixed 600 s wait for background subagents even though they are still running; the cap cannot be disabled

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5223 |
| **Reported** | 2026-10-21 |
| **Last Activity** | 2026-10-23 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.102 and 1.0.103 for interactive autopilot (reported regression; 1.0.101 and 1.0.100 reportedly fine); prompt mode (`-p`) had the same cap earlier (see Notes) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:agents` |

**Summary:**
Since 1.0.102 interactive autopilot ends the run with "Autopilot stopped after 600s waiting for background tasks" while a background subagent is still working. A second user reproduced it on pinned v1.0.103 (Windows, SDK harness with autopilot mode, `COPILOT_TASK_WAIT_TIMEOUT_SECONDS=2`: `session.autopilot_stopped` reason `background_timeout` fired mid-stream; a 120 s control completed normally). The limit is env-configurable but not removable: `COPILOT_TASK_WAIT_TIMEOUT_SECONDS=0` gives a 0 ms wait, negative or non-numeric values fall back to 600. No maintainer response; open.

**Impact on Orchestration:**
Any long-running background subagent (over 10 minutes) ends an autopilot parent turn prematurely, leaving work in flight and a parent that thinks the run is over. Only relevant to workflows that use background-mode `task` calls with autopilot; MOSAIC's per-dispatch `-p` stages are less exposed, but see Notes on exit status.

**Evidence:**
- Original reporter with version bisect (1.0.101 vs 1.0.102) from behavior and changelog; second reporter's scripted reproduction plus control on v1.0.103.
- The older-runtime A/B comparison was not independently established by the second reporter.

**Workaround(s):**
1. Set `COPILOT_TASK_WAIT_TIMEOUT_SECONDS` to a large value (for example 3600) before launching (reporter-suggested; the control run confirms the variable is honored).

**Notes:**
Related earlier report #4628 (prompt mode, 1.0.80; closed by bot as resolved in 1.0.91 with no reporter confirmation): the 600 s timer terminated the whole process with exit code 0 even though the subagent had finished and the parent was active, so a timeout was indistinguishable from success. Not tracked separately because of the weak closure, but if MOSAIC uses background subagents under `-p`, check the exit code and an explicit completion marker rather than exit 0 alone.

---

### GC-039: Background sub-agent model-stream failure exhausts 5 retries in ~5 s and the parent receives an ordinary completion/idle signal instead of a failure

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/4911 |
| **Reported** | 2026-09-19 |
| **Last Activity** | 2026-10-01 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.79-9, 1.0.81-0, 1.0.84-5 (persisted terminal event was `subagent.completed`); 1.0.86-2 persists `subagent.failed` for the same fault (retry behavior unchanged) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `triage` |

**Summary:**
When a model response stream ends early (no `response.completed`, or chunked body cut), the CLI retries 5 times with about 1 s gaps (reported total wait 5-6 s) and then ends the turn with "Failed to get response from the AI model; retried 5 times". For a background sub-agent on 1.0.84-5 the parent got a normal completion/idle notification with no failure status; the persisted `subagent.completed` event carries usage/timing but no success/error field. A loopback fault-injection script reproduces it deterministically on 1.0.84-5 and 1.0.86-2 (on 1.0.86-2 the child's persisted terminal event is `subagent.failed`). Real-world occurrences were reported on both OpenAI and Anthropic paths, five in sub-agents and three in top-level sessions. One additional comment (suggestions only). No maintainer response; open, last activity 2026-10-01.

**Impact on Orchestration:**
A subagent that dies mid-task from a transient stream fault can look like a finished one, so the orchestrator may accept a missing or partial deliverable as complete. Tool actions already performed are not rolled back, so partial work is left behind. The short fixed retry window also gives no time for transient provider outages to clear.

**Evidence:**
- Single reporter with a self-contained reproducer and results on two CLI builds; fault is injected client-side, so the original upstream cause is not established.
- Not verified against current v1.0.103; the parent's handling of `subagent.failed` was not tested.

**Workaround(s):**
1. Do not treat a subagent's completion/idle signal as success; require an explicit deliverable check (file exists, completion marker in the reply) before accepting a stage.
2. Retry the stage once from the orchestrator when the deliverable is missing.

**Notes:**
Needs re-verification on current release (reporter's own data suggests `subagent.failed` is written from 1.0.86-2). Related: GC-005 and GC-004 (other silent or abrupt subagent failure modes), #3350 and #4101 (background agent lifecycle/status; not tracked), #5143 (follow-up to an idle background agent is delayed minutes to hours, interactive and unusual session conditions; not tracked).

---

### GC-028: Large MCP fleets connect only partially at startup with no retry, and `tool_search_tool` reports a clean "No tools found" while servers are still registering

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5069 (also https://github.com/github/copilot-cli/issues/4598) |
| **Reported** | 2026-08-25 |
| **Last Activity** | 2026-10-07 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.80 (#4598), 1.0.90-0 (#5069); 1.0.100 release notes say "Sessions no longer stall for minutes when an MCP server hangs while loading tools", unclear whether that covers this |
| **Latest Platform Version** | v1.0.103 (released 2026-10-22) |
| **Labels** | area:mcp, area:tools (#5069); triage (#4598) |

**Summary:**
With a large MCP fleet (reporter: 18 servers merged from user, repo and plugin configs), #4598 found the runtime emitting `session.mcp_servers_loaded` about 9 s after start with only 3 of 18 servers connected, never retrying, and a status subscription from another session tearing down and respawning all MCP connections (`NativeMcpHostHandle has been destroyed`). #5069 (same reporter, 1.0.90-0, reproduces on a bare `copilot` session with only `mcp-config.json`) shows the agent-facing effect: within the first 1-3 minutes (once also a 3 h 43 m window), `tool_search_tool` returns `No tools found matching pattern ...` with success status for tools whose server has not registered yet, indistinguishable from a real no-match; the agent then concludes the capability does not exist. `copilot plugins list --kind mcp` shows configured, not connected, servers, and the connected set is not persisted in `events.jsonl`.

**Impact on Orchestration:**
A MOSAIC agent or subagent that discovers MCP tools through search early in a session can silently decide a capability is missing and proceed on a false premise, with no error in headless output. Exposure scales with the number of MCP servers; MOSAIC setups with few or no MCP servers are largely unaffected.

**Evidence:**
- Same single reporter on both issues, with log-level evidence and a timed before/after table; no confirmations from others, no maintainer comment, both open, no PR
- Included as Unverified: tool discovery is a core delegation concern, stock Copilot CLI (#5069 reproduces without the desktop app), concrete steps given
- Related sibling #4731 (closed by bot as fixed in 1.0.92, not verified): server's tools permanently dropped after a cancelled tool call; a gh-aw CI user saw `Tool '...' does not exist` for all MCP tools on 1.0.83 in headless Actions runs (worked on 1.0.80)

**Workaround(s):**
1. Wait or retry `tool_search_tool` after startup settles (about 1-3 minutes for a large fleet); instruct agents never to treat a first "No tools found" as authoritative.
2. Keep the MCP fleet small and per-task; load only the servers a stage needs.
3. For CI users hit by the 1.0.83 tool-vanishing variant, pinning 1.0.80 was reported to work (gh-aw user, #4731; closed upstream).

**Notes:**
Relates to GC-005 (silent empty results with heavy MCP schema volume). Release 1.0.100 also lists "Avoid false context exhaustion and rollover refusal when tool definitions are deferred" and 1.0.101 "Headless and SDK sessions without an OAuth handler skip MCP connections when saved tokens require sign-in" (a by-design silent skip in headless runs; worth knowing for MCP servers using OAuth). Needs re-verification on 1.0.100+.

---

### GC-040: Headless mode exits 0 mid-task when a model response has `finish_reason: "tool_calls"` but no tool calls (empty response treated as successful completion)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5140 |
| **Reported** | 2026-10-14 |
| **Last Activity** | 2026-10-14 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.90 (single occurrence, Linux CI runner) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:non-interactive` |

**Summary:**
In `-p`/`--prompt-file` mode with `--no-ask-user`, after 62 healthy turns the Copilot API returned one malformed response (`finish_reason: "tool_calls"`, `content: null`, no `tool_calls`, 193 completion tokens billed). The CLI logged `runHeadlessMode: succeeded=true`, `exiting with code 0` with no retry, no error and no nudge, so the wrapper's retry logic never ran and the run produced no outputs. The tool call was probably lost between API and CLI, but the CLI turns it into a silent clean exit. One occurrence, not deterministic, no comments or maintainer response.

**Impact on Orchestration:**
Exit code 0 does not prove the stage finished. A headless MOSAIC stage can end mid-task with no deliverable and nothing to flag it except the missing artifacts. Rare trigger (one in about 63 turns on one run).

**Evidence:**
- `--log-level all` excerpt of the malformed response and the exit-0 log lines; model `claude-sonnet-5.5` on GitHub-hosted runner via gh-aw; stock Copilot API.
- Single reporter, no reproduction steps because the trigger is an API response.
- Related older, stale report #3239 (open, 1.0.44, macOS interactive): a text-only assistant turn after an action-requesting message is treated as a valid stop, ~1 per hour in a 6.5 h session, with no auto-continue; no comments, 5+ months without activity, not tracked separately.

**Workaround(s):**
1. Do not rely on exit status alone; verify the stage deliverable (expected file or completion marker in the reply) and retry the stage when absent (reporter's wrapper-retry approach, which failed only because exit was 0).

**Notes:**
Included as Unverified because it concerns exit-status reliability for unattended `-p` runs, which MOSAIC depends on. Needs re-verification on current release. Related: GC-008 (session hang with no internal escape), GC-039 (subagent failure surfacing as normal completion).

---

### GC-041: `view` refuses files whose full path is 257+ characters in non-interactive runs with a misleading "Permission denied and could not request permission from user"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/github/copilot-cli/issues/5095 |
| **Reported** | 2026-10-09 |
| **Last Activity** | 2026-10-11 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 1.0.86-0, 1.0.89-3, 1.0.89 (Windows 11, long paths enabled) |
| **Latest Platform Version** | v1.0.103 (2026-10-22) |
| **Labels** | `area:permissions`, `area:non-interactive`, `area:platform-windows` |

**Summary:**
With `-C <dir> --no-ask-user --available-tools=view,rg,glob --deny-tool=write --deny-tool=shell`, the `view` tool succeeds on files whose full path is 256 characters or fewer and fails on files inside the working directory whose path is 257 or more characters, returning "Permission denied and could not request permission from user". `rg` over the same directory still reads the file, and Windows `LongPathsEnabled=1` was set. Reporter's production data: 227 `view` calls over 7 sessions, all 218 successes at 256 or fewer characters, all 7 denials of real in-directory files at 257-264 characters, consistent across sessions. No controlled minimal repro, no comments or maintainer response.

**Impact on Orchestration:**
On Windows, a headless stage working in a deep project tree silently loses access to over-long files through `view`; the agent cannot prompt and continues without the file. The permission-flavored error hides the real cause (path length) and looks like GC-009/GC-013 permission loops. Interactive behavior and `glob` were not checked.

**Evidence:**
- Single reporter with call-level statistics from real unattended runs; no independent confirmation, no controlled repro.
- Same error string as GC-009 and GC-013 but a deterministic, path-length-correlated trigger.

**Workaround(s):**
1. Keep the working directory and artifact paths short enough that full paths stay at 256 characters or fewer (MOSAIC deployments in deep Windows trees are at risk).
2. Read over-long files via `rg` (reporter observed it works), if that tool is available to the stage.

**Notes:**
Windows path-length threshold (256) suggests an internal MAX_PATH-style check in the permission path-resolution step, not an OS limit; unconfirmed. Related: GC-009, GC-013, GC-014 (permission/path configuration failures that surface as "Permission denied and could not request permission from user"); see also #5173/#5206 cross-reference in GC-009.
