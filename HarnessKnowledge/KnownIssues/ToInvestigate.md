# Harness Behaviors To Investigate

> Last updated: 2026-10-25

Watch list for behaviors that someone observed first-hand but that are **not yet reproducible, not yet filed upstream, or not yet evidenced well enough** to become a KB entry (`active-issues.md`). Items here are leads, not findings.

**Lifecycle:** an item leaves this file when it is (a) promoted to an `active-issues.md` entry (note the new ID here until the next cleanup), (b) disproven, or (c) found to be an existing entry. Never put these items in the harness `index.md` tables.

**Entry rules:** Source is usually "Direct observation". Only a human sets `Reproduced at MOSAIC`. Always record what is still needed to verify, so the next person knows what to try.

---

## Open Items

### TI-001: VS Code — subagents see the parent's tool descriptions but not its permissions

| Field | Value |
|-------|-------|
| **Harness** | VS Code GitHub Copilot |
| **Source** | Direct observation (Tomas, week of 2026-10-19, on a different PC) |
| **Added** | 2026-10-25 |
| **Status** | Observed once, **not reproducible on the current setup** |
| **Suspected Classification** | Bug |
| **Suspected Impact** | HIGH (subagent tool execution; wasted calls or stalled subagents) |
| **Upstream issue** | None found (searched `microsoft/vscode`, 2026-10-25) |

**Observation:**
Subagents repeatedly tried to use the terminal tool, and every call was rejected immediately by the harness. The rejection itself was correct, since terminal was never configured for those subagents. The bug is that they were *offered* the tool at all. They received the terminal tool's description because the orchestrator had it configured. In effect they inherited tool descriptions from the primary agent but not its permissions. After terminal was removed from the orchestrator, the subagents never tried to use it.

**Still needed to verify:**
- VS Code version, Copilot Chat extension version, model and harness (Local vs Copilot/Agent Host) on the PC where it happened. Not yet recorded.
- Reproduction: orchestrator with `tools: [agent, execute/runInTerminal, ...]`, subagent with a tool list that excludes terminal. Check whether the subagent's tool inventory or prompt mentions terminal, and whether the subagent attempts calls.
- Whether it only occurs in one harness or on one version. The current PC may be on a different version or setting set, which could explain the non-repro.

**Workaround (observed, single report):** remove the affected tool from the orchestrator's tool list. This costs the orchestrator that tool. MOSAIC orchestrators need terminal only if they run commands themselves.

**Related, but not the same behavior:**
- [#284304](https://github.com/microsoft/vscode/issues/284304) — nested subagents get the root agent's tools instead of the spawner's (open, assigned, `bug`).
- [#336376](https://github.com/microsoft/vscode/issues/336376) — subagent prompt mandates `tool_search` while the request omits it. This is a prompt/request mismatch, the same type of defect (fix PR #340701 open).
- [#341550](https://github.com/microsoft/vscode/issues/341550), [#313520](https://github.com/microsoft/vscode/issues/313520) — subagents missing tools their frontmatter declares (opposite direction).

**Promotion criteria:** reproduced on any setup, or a matching upstream issue appears (then add it as a `VC-` entry with `Reproduced at MOSAIC` = Partial/Yes as set by the user).

---

## Template

```markdown
### TI-NNN: {Harness} — {short title}

| Field | Value |
|-------|-------|
| **Harness** | {harness} |
| **Source** | {Direct observation (who, when, which PC) / URL} |
| **Added** | {YYYY-MM-DD} |
| **Status** | {Observed once / Not reproducible / Waiting on upstream / ...} |
| **Suspected Classification** | {Bug / Limitation / Quirk} |
| **Suspected Impact** | {HIGH / MEDIUM / LOW} |
| **Upstream issue** | {URL or "None found (searched date)"} |

**Observation:** {what happened}
**Still needed to verify:** {bullets}
**Workaround (observed):** {if any}
**Related, but not the same behavior:** {links}
**Promotion criteria:** {what moves it to active-issues.md}
```
