# GitHub Copilot CLI — Resolved Issues

> Last updated: 2026-10-25 (run 20)

---

### GC-022: `--agent <id>` dropped at session start (`subagent.deselected`); the default agent runs instead (1.0.91 regression)

| Field | Value |
|-------|-------|
| **Source** | MOSAIC experiments, 2026-10-04 to 2026-10-06 (Windows 11); no upstream issue filed or found. Investigation: `MOSAIC/Investigation-GHCP-AgentDeselect.md` |
| **Fixed In / Status** | 1.0.92 (no explicit release-note entry; see Resolution for the related changelog entries) |
| **Resolution Date** | 2026-10-06 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
With `--agent <id>` in non-interactive mode, CLI 1.0.91 emitted `subagent.deselected` (`data: {}`) before the first `session.custom_agents_updated` event and ran the default Copilot agent, which had none of the custom agent's instructions and none of its tool restrictions. In the Runner this looked like unrelated defects: `protocol response not extractable` (BLOCKED/E501), `reply contains no JSON object`, `unknown action ""`. It made 9 of 23 GHCP MosaicTest runs fail on 2026-10-04: 21% of invocations were dropped during the suite, 100% afterwards. 1.0.87 kept the agent.

The default agent then searched the workspace with tools even for an agent declared `tools: []`. In 6 of 20 dropped runs it found the agent's definition in another harness folder (`.claude/agents/`, `.opencode/agents/`), imitated it, and returned a valid-looking protocol response. So this failure cannot be reliably detected from the reply; only the event stream shows it.

Recorded after the fix (the entry never went through `active-issues.md`), because it was diagnosed and resolved within the same investigation.

**Resolution:**
Deciding probes on 2026-10-06 (40 runs, cells interleaved per round): 1.0.92 through `copilot.cmd` kept the agent 0/20 dropped, in both the cluttered test workspace and a clean copy. 1.0.91 run directly dropped it 20/20, with stdin and with `-p` prompt delivery alike. The full GHCP MosaicTest suite on 1.0.92 then had 0 drops in 118 invocations. The CLI self-updates in the background and `copilot --version` can lag the code that actually runs; the running version is the `pkg\win32-x64\<version>` path in the JSONL events. Raw data: `C:\AI\MOSAIC\HarnessProbes\GhcpAgentDeselect\`. Changelog context (bundled `changelog.json` in `%LOCALAPPDATA%\copilot\pkg\win32-x64\1.0.92\`; the release page shows only the latest version):
- **1.0.88** fixed: "Custom-agent startup now distinguishes model-list load failures from an empty catalog, preventing false unavailable warnings and silent required-agent deselection." So the CLI deliberately deselects a requested agent at startup when it judges the agent's model unavailable, and that check depends on the model list having loaded. Our drop came before the agent list arrived and was intermittent, then constant. That fits the same check running against a model list that had not loaded yet: the 1.0.88 fix covered failed loads, and 1.0.91 apparently still deselected when the list was merely late. **Likely, not confirmed:** the deselect event carries no reason (`data: {}`). Every MOSAIC agent sets `model:` in its frontmatter, so all were exposed to this check.
- **1.0.92** has no entry for it. Changes that touch this path: "Custom agent model entries keep model-bound reasoning effort only when that model is selected" (copilot-agent-runtime #23934), "Remove retired models from the model picker and supported CLI selections" (#24575), "Custom agents launched through ACP task calls now resolve and run correctly" (#24626; matches upstream copilot-cli #5030, "ACP task tool cannot launch custom agents since 1.0.89"), and "Improve first-run startup by extracting the bundled CLI package in a child process" (#24128; related to the package folder still being written during the 10-04 suite). The fix is probably one of these or an unlisted change; the PRs are in a private repository.

MOSAIC follow-up: Runner fails an invocation on `subagent.deselected` (`MOSAIC/Requirements-runner.md`, addendum), so a recurrence after a future self-update is reported as a harness error.

---

### GC-003: Workspace `.mcp.json` detected by `mcp list`/`mcp get` but not wired into actual agent session (interactive, `-i`, `-p`)

| Field | Value |
|-------|-------|
| **Source** | https://github.com/github/copilot-cli/issues/4542 |
| **Fixed In / Status** | 1.0.85 (partial `.mcp.json` loading fixes) / 1.0.89 (closed as resolved by copilot-cli-bot) |
| **Resolution Date** | 2026-09-30 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
Workspace `.mcp.json` servers showed as Enabled in `copilot mcp list`/`get` but were silently not connected into the agent session (interactive, `-i`, `-p`) on 1.0.75-1.0.80. Reproduced on macOS and Windows.

**Resolution:**
A community comment (2026-09-16) reported the reporter's example working again as of 1.0.85; the bot closed the issue 2026-09-30 as resolved as of 1.0.89. Residual caveat from the same comment: the workspace must have *persisted* trust in the Copilot config (temporary trust via interactive "Yes" is not enough for `copilot mcp list` / `-p` MCP connection); `--additional-mcp-config @file` has highest load priority and works even in an untrusted workspace. For MOSAIC headless Runner use, either pre-trust the workspace directory in Copilot config or keep passing `--additional-mcp-config`. Not independently verified on a current build.

---

### GC-006: Create-file tool returns invalid/null response and retries forever when content is too large

| Field | Value |
|-------|-------|
| **Source** | https://github.com/github/copilot-cli/issues/4626 |
| **Fixed In / Status** | 1.0.85 (per copilot-cli-bot closure; no linked PR) |
| **Resolution Date** | 2026-09-16 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
The create-file tool failed with a generic `"path": Required` / `"file_text": Required` validation error on very large single-file content, and the agent retried the identical failing call indefinitely (1.0.80). Single reporter.

**Resolution:**
Closed by the automated copilot-cli-bot as resolved as of v1.0.85 (no PR linked, no reporter confirmation). Accepted as resolved on that basis; not independently verified. If MOSAIC stages ever stall on large artifact writes again, splitting the file remains the known workaround.

---

### GC-016: preToolUse hook `deny` decision does not block tool execution

| Field | Value |
|-------|-------|
| **Source** | https://github.com/github/copilot-cli/issues/3874 |
| **Fixed In / Status** | 1.0.86 (bot closure; no PR linked, not in the 1.0.86 release notes) |
| **Resolution Date** | 2026-09-20 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
A `preToolUse` hook returning `permissionDecision: "deny"` (exit code 2) or `behavior: "deny"` did not stop the tool call from executing (reported on v1.0.65). Single reporter; two non-maintainer comments were technical speculation.

**Resolution:**
Closed 2026-09-20 by the automated copilot-cli-bot as resolved as of v1.0.86, with no PR linked, no reporter confirmation, and no matching line in the v1.0.86 release notes. Accepted as resolved on that basis (same convention as GC-006); not independently verified. Until MOSAIC verifies hook deny enforcement on a current build (latest v1.0.103), do not treat `preToolUse` deny as the sole hard gate.

---

### GC-011: Steering message typed into a `preToolUse` "ask" denial prompt is silently dropped, never reaches the agent

| Field | Value |
|-------|-------|
| **Source** | https://github.com/github/copilot-cli/issues/4237 |
| **Fixed In / Status** | 1.0.86 (per copilot-cli-bot closure; no linked PR) |
| **Resolution Date** | 2026-09-19 |
| **Original Orchestration Impact** | MEDIUM |

**Summary:**
When a `preToolUse` hook returned `permissionDecision: "ask"`, the free-text a human typed when denying never reached the agent (only a bare denial was registered), unlike Claude Code's hook contract (1.0.74-4). Single reporter.

**Resolution:**
Closed by the automated copilot-cli-bot as resolved as of v1.0.86 (no PR linked, no reporter confirmation). Accepted as resolved on that basis; not independently verified. Related GC-016/GC-017 (other `preToolUse` verdict-handling bugs) are tracked separately.
