# OpenCode — Resolved Issues

> Last updated: 2026-10-25 (run 20)

---

### OC-089: V2 `subagent` tool does not enforce per-target subagent permissions - actual runtime execution bypass, distinct from OC-007's legacy Task-tool description mismatch

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anomalyco/opencode/issues/35238 |
| **Fixed In / Status** | N/A (not relevant) - 2.x-only (`packages/core` V2 `subagent` tool); fixed in 2.x, closed `completed` 2026-10-01 |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
In the 2.0 rewrite the V2 `subagent` tool only hid the whole tool and neither asserted per-target permission before spawning a child session nor filtered the model-facing target list, so a "only allow `explore`" restriction was not enforced at runtime (independently confirmed by a user).

**Resolution:**
Re-read in run 20: issue is labeled `2.0`/`core` and concerns `packages/core` only; it is not a 1.x defect and falls outside the OpenCodeV1 scope. Maintainer `neriousy` closed it `completed` on 2026-10-01: V2 now enforces per-target `subagent` permission on each call and omits denied targets from the list shown to the model (implementation in `tool/plugin/subagent.ts`, #35794). Fix PR #41100 was closed unmerged; the fix landed via other work. The related 1.x issues remain tracked separately: OC-007 (legacy Task tool advertises subagents ignoring session permission overrides) and OC-090 (Task subagents inherit stale parent denies / caller-dependent permission bypass).

### OC-087: `opencode run --auto` auto-approval does not cascade to subagent sessions - subagent permission prompts hang indefinitely

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anomalyco/opencode/issues/41730 |
| **Fixed In / Status** | v1.18.20 (PR #43675, merged 2026-08-21) - main case fixed; one residual edge case reported unfixed |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
Under `opencode run --auto`, permission requests raised by subagent (child) sessions were never answered, so the run hung forever on any `ask` rule hit inside a subagent. Reported on v1.18.14.

**Resolution:**
PR #43675 ("answer subagent permissions in run", merged by `rekram1-node` 2026-08-21) tracks child and nested child sessions created during a non-interactive run and auto-approves (with `--auto`) or auto-rejects (without) their permission requests, only within that run's session tree. Shipped in v1.18.20 (2026-08-21 release notes: "Answer permission requests triggered by subagents during `opencode run`"). Issue #41730 was later closed `not_planned` in the 2026-10-11 v1 mass-close, which is unrelated to the fix. **Residual edge case (still unfixed on 1.x):** a commenter (2026-09-09) reports that when `task_id` reuses an earlier child session and no `session.created` event fires, the hang can still occur; the follow-up PR #42310 (follow `parentID` for unknown children) was closed unmerged by the stale-PR bot on 2026-09-13. Not independently reproduced by us. Keep an external timeout/watchdog around `opencode run --auto` fan-out, and avoid resuming prior subagent sessions via `task_id` in headless runs. Versions before v1.18.20 remain affected (workaround there: explicit `allow` for all subagent-needed permissions). Related still-active entries: OC-084, OC-085.

### OC-086: Plugin-registered agent-specific permissions cannot override a root-level default deny

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anomalyco/opencode/issues/47946 |
| **Fixed In / Status** | N/A (not relevant) - 2.x-only issue; closed `completed` by reporter 2026-09-19, no linked PR |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
Plugin-registered agent permission rules (via the 2.0-line `context.agent.transform(...)` API) lost to a root-level wildcard deny because of flattened last-match-wins evaluation.

**Resolution:**
Re-read in run 20: the report was filed against `0.0.0-beta-19242` (2.0 beta line), carries the `2.0` label, and uses the V2 plugin API (`context.agent.transform`, `agent.permissions.push({action, resource, effect})`), which does not exist on 1.x. It was never a 1.x defect, so it falls outside the OpenCodeV1 scope; closed by the reporter (`stevoland`) as completed on 2026-09-19. The general last-match-wins/specificity evaluator flaw on 1.x remains tracked in OC-085 (still active, no 1.x fix). Sibling #39022 is tracked separately as OC-097.

### OC-096: No timeout/idle-watchdog on any LLM stream (subagent or primary) — a stalled model response hangs the session forever with no recovery

| Field | Value |
|-------|-------|
| **Source** | https://github.com/anomalyco/opencode/issues/13841 (closed as duplicate of #11865); https://github.com/anomalyco/opencode/issues/11865 (closed completed 2026-10-02) |
| **Fixed In / Status** | v1.18.27 (PR #46890, merged 2026-09-02) - partial: per-chunk idle timeout + provider header timeout only |
| **Resolution Date** | 2026-10-25 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
OpenCode had no request timeout, stream-idle watchdog or subagent timeout, so a stalled LLM stream (mid-stream silence or no first byte) hung a subagent or primary session indefinitely. Reproduced across Anthropic, OpenAI/Codex, DeepSeek, GLM, Minimax; 3 earlier fix PRs (#13846, #16599, #36650) were closed unmerged.

**Resolution:**
Maintainer `rekram1-node` (COLLABORATOR) stated on #11865 (2026-10-02): "The timeout and retry problem here is fixed in #46890 (v1.18.27) and #49229 (v2.0.22)". The v1.18.27 release notes list "Default provider header timeouts to five minutes" and "Default streamed chunk timeouts to five minutes, with `false` supported to disable them". PR #46890: default `chunkTimeout` of 300,000 ms between SSE chunks; stalled requests are aborted and retried via the existing retry policy; configurable per provider via `provider.<name>.options.chunkTimeout` (number of ms, or `false` to disable). **Scope limits (keep mitigations):** this bounds inactivity between chunks (and header wait) at 5 min only. It is NOT a total-request timeout and NOT a subagent/Task-level wall-clock timeout; a subagent that keeps emitting chunks (or loops on tools) is still unbounded, and the earlier-reported unbounded session-level retry loop (`processor.ts`) was not addressed by the PR text. Keep an external per-dispatch watchdog around `opencode run`/subagent fan-out, and verify behaviour on v1.19.x before dropping it. Related hang entries (OC-001, OC-006, OC-044, OC-067) are not covered by this fix. Versions before v1.18.27 remain affected (workaround there: explicit `provider.<name>.options.timeout = 300000`).
