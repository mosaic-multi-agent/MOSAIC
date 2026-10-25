# VS Code GitHub Copilot — Resolved Issues

> Last updated: 2026-10-25 (run 20)

---

### VC-001: Nested runSubagent calls rejected 3 levels deep even when target agent is allow-listed

| Field | Value |
|-------|-------|
| **Source** | https://github.com/microsoft/vscode/issues/335257 |
| **Fixed In / Status** | 1.140.0 (PR #337611, merged 2026-09-24) |
| **Resolution Date** | 2026-09-24 |
| **Original Orchestration Impact** | HIGH |

**Summary:**
In a coordinator->worker->specialist chain, the nested `runSubagent` call (B->C) was rejected with "Requested agent 'C' is not allowed by the current agent" even when C was in the caller's `agents:` allow-list. A commenter bisected it to the allow-list enforcement added in 1.136.0 (#331442), which validated against the top-level request's agent (A) instead of the calling subagent (B).

**Resolution:**
Maintainer-closed as completed (aeschli, milestone 1.140.0; labels bug, regression, verified, insiders-released). PR #337611 identifies the caller by request id and validates against the calling subagent's allow-list; tested to 3 and 4 nesting levels. Side effect worth knowing (relevant to VC-010): a nested subagent calling `runSubagent` without `agentName` now becomes a copy of the calling subagent (instructions, allowlist, model, tools, hooks) instead of a copy of the top-level agent. Nesting still requires `chat.subagents.allowInvocationsFromSubagents`.
