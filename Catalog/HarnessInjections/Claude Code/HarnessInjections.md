---
version: "1.4.0"
harness: claude-code
---

# Harness Injections — Claude Code

<HarnessConstraints type="managed">
**Output Artifacts Are Not Report Files:** Every file listed in `output_artifacts` is an orchestration artifact, not a report file. The orchestrator passes it as input to other agents, and your final message never reaches those agents. Write every listed output artifact as a file — review and findings documents included — even where general guidance tells you to return findings in your final message instead of writing them to a file.

**No Heredocs or Stdin Scripts:** Do not use heredocs (`<<`) or feed a script to an interpreter through stdin (`python -`), in any shell, even where tool guidance recommends it, including for commit messages and PR bodies. Write multi-line content to a file with the Write tool, then run or pass that file (`git commit -F`, `gh pr create --body-file`). Without a Write tool, use one `git commit -m` per paragraph.
</HarnessConstraints>

---

## Design Rationale

- **HarnessConstraints:** Claude Code's built-in subagent prompt instructs subagents not to write report, summary, findings, or analysis `.md` files and to return findings in their final message instead (captured in `HarnessKnowledge/SystemPromptCapture/ClaudeCode/Subagent/SystemPrompt-clean.md`). A MOSAIC review artifact matches that description exactly, so review agents returned `SUCCESS` with their findings in chat and the declared artifact never written — a failure the orchestrator does not catch, because a `SUCCESS` review routes onward. The constraint counters the rule at the same level it is delivered, and frames artifacts as inputs to other agents, which fits the harness's own exception for files written as input to another tool. The dispatch's `output_artifacts` list is also the explicit request that the harness's "never create documentation files unless requested" rule asks for.
- **HarnessConstraints (heredocs):** Claude Code's Bash tool description recommends heredocs for multi-line strings and its git instructions require them for commit messages and PR bodies (captured in `HarnessKnowledge/SystemPromptCapture/ClaudeCode/Subagent/`), so the constraint names those cases and overrides the guidance at the same level. On Windows, heredocs and stdin scripts run into several failures: Git Bash truncates long commands, possibly losing the heredoc terminator while still exiting 0 (CC-093); the Bash tool leaves child processes alive after timeout or cancel (CC-100); and Python 3.14 treats an empty stdin as a terminal and loops forever (upstream #101456). macOS has a separate bash pipe deadlock (upstream #92262). Files avoid all of these. The last sentence exists because some agents, such as `commit-manager-git`, have Bash but no Write tool. A `PreToolUse` hook was rejected as enforcement because hooks carry their own Windows freeze (CC-103) and fail open if killed (CC-015).
- **ProtocolExtension:** Not declared in this harness. Removed at the harness level — any orchestration-protocol extensions are authored at the agent level where they belong.
- **ErrorHandlingExtension:** Not declared in this harness. Removed at harness level — error handling guidance is agent-specific.
- **ContextLimits:** Not declared in this harness. Removed at harness level — context window guidance depends on model and agent configuration, not harness.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.1.0 | 2026-07-31 | MOSAIC | Reformat to workflows-style boundary-tag format; add explicit empty sections for HarnessConstraints and LanguagePatterns to maintain declared-but-empty semantics |
| 1.2.0 | 2026-08-08 | MOSAIC | Remove the LanguagePatterns block: it is now a project-authored injection name rather than a tool-managed deployed region, so this harness no longer declares it |
| 1.3.0 | 2026-10-03 | MOSAIC | Fill HarnessConstraints with the output-artifacts constraint, countering Claude Code's subagent rule against writing findings `.md` files, which led review agents to report in chat instead of writing their artifact |
| 1.4.0 | 2026-11-01 | MOSAIC | Add the constraint against heredocs and stdin scripts, overriding Claude Code's guidance that recommends heredocs |
