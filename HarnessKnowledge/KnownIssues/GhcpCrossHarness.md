# GHCP cross-harness compatibility

How GitHub Copilot CLI and the two VS Code chat harnesses interact when they share a workspace. It sits here, not in `GHCP-CLI/` or `VsCodeGHCP/`, because each finding involves more than one of them.

**Last checked:** 2026-11-01, with VS Code 1.144.0, Copilot CLI 1.0.105 and the Copilot SDK build bundled with VS Code, 1.0.27-unstable.

**Evidence labels:** **Tested** means we observed it locally. **Stated** means an official source says it. **Maintainer** means a VS Code maintainer said it in a PR or issue. Official pages change often, so check a quote at its source before relying on it. Most were read through a summarising fetch tool, and only the main retirement, default-harness and hook quotes were checked directly.

**How we tested:** on Windows, in a throwaway workspace. Each small agent replied with its own filename or carried out a fixed task. A hook probe logged every event, its payload keys and its process chain. VS Code session files and CLI logs showed which model ran.

**MOSAIC decision:** a workspace holds one GHCP target at a time. The `ghcp-cli` target also serves the Copilot harness, and `vscode-ghcp` serves Local only. A shared agent file worked in testing but was rejected, because Local is being retired.

---

## The three runtimes

| Runtime | Process that runs hooks (Tested) | Status |
|---|---|---|
| **GHCP CLI** | `copilot.exe` from the npm package | Current |
| **VS Code Copilot harness** | `copilot-runtime.exe` from VS Code's bundled `copilot-sdk-win32-x64`, child of `Code.exe` | Default for new sessions since 1.143 |
| **VS Code Local harness** | `Code.exe` | Being retired, no date given |

**Is the Copilot harness a third harness?** It's the CLI's engine hosted inside VS Code, so it's a separate target but not a true hybrid. On every behaviour tested here, it matched the CLI and not Local, apart from the extra `rename_chat` tool.

| Tested behaviour | Copilot harness | GHCP CLI | Local |
|---|---|---|---|
| Built-in tool names | `create`, `view`, `task`, `ask_user` | Same | `create_file`, `read_file`, `runSubagent`, `vscode_askQuestions` |
| File chosen from `X.md` and `X.agent.md` | `.agent.md` | Same | `.md` when selected, `.agent.md` as subagent |
| Hook events fired | Both casings | Same | PascalCase only |
| Runs `.claude/settings.json` hooks | Yes | Yes | No |
| Ask tool in a subagent | No | No | Yes |
| Session store | `~/.copilot/session-state` | Same | VS Code `workspaceStorage` |

**Official position:**
- The SDK "exposes the same engine behind Copilot CLI" ([SDK README](https://github.com/github/copilot-sdk)).
- VS Code says "We combined our Local VS Code harness with the Copilot CLI harness to create the new Copilot harness" ([VS Code blog, 2026-10-28](https://code.visualstudio.com/blogs/2026/10/28/vscode-wrapped)).
- "The shared SDK runtime also does not imply identical tools, models, permissions, or customizations across products" ([harness docs](https://code.visualstudio.com/docs/agents/run/agent-harnesses)).

**The installed CLI isn't used (Tested):** the Copilot harness ran VS Code's own bundled runtime, not the npm-installed `copilot.exe`. The two have separate version numbers, so their behaviour can drift.

**Older names:** the "Copilot CLI" session type and background agents pages now redirect to the harness page.

### Copilot SDK and Agent Host

- **SDK (Stated):** `github/copilot-sdk`, public preview 2026-04-02, GA 2026-06-02. "All SDKs communicate with the Copilot CLI server via JSON-RPC", and the SDK manages that process. The CLI is bundled for Node.js, Python and .NET, and you can override the binary.
- **Agent Host (Stated):** "a dedicated process that runs agent harnesses and manages their sessions", either a local utility process or a remote server. It runs the Copilot, Claude and Codex harnesses. It arrived as opt-in in 1.129 through `chat.agentHost.enabled`.
- **Sessions (Stated):** Copilot sessions survive closing the window and can be continued from the Chat view, Agents window, remotely or in the CLI. Local stops with its window and appears only in the Chat view.
- **Separate install (Stated):** 1.143 added an installer option, "Install GitHub Copilot CLI with VS Code", which adds a `copilot` executable to PATH on Linux and prompts on macOS.

### Copilot harness versus Local

| Area | Copilot harness | Local |
|---|---|---|
| Tool picker | Tools tab in the Agent Customizations editor | "Configure Tools" in the chat input |
| Available tools | "do not have access to every built-in or extension-provided tool". Limit of 128 tools | Built-in, MCP and extension tools |
| Client tools | Available only while the VS Code client is connected | Always in the same window |
| Handoffs, `argument-hint`, prompt files | Not supported | Supported |
| Sandbox and permission rules | Enforced | "Local does not enforce runtime-only sandbox and permission rules". Windows terminal sandboxing deprecated in 1.142 |
| SDK policy hooks | Loaded | Not loaded |
| BYOK models | Experimental | Supported |
| Copilot-only features | Connectors, Copilot Memory, computer use, canvases, remote hosts | None |

Stated, apart from Local's client tools cell, which we inferred from Local running in the extension host. Handoff works from Local to Copilot, not back. The tool picker row is the likely reason Configure Tools disappeared from the chat input on Copilot.

---

## Default harness and Local's retirement

- **Default (Stated):** gradual rollout in 1.142, "now the default for new sessions" in [1.143](https://code.visualstudio.com/updates/v1_143).
- **Observed (Tested):** every new chat opened on Copilot, even after using Local in the previous chat.
- **Retirement (Maintainer):** "Local will eventually be completely retired and unavailable" ([PR #342623](https://github.com/microsoft/vscode/pull/342623)). The 1.143 notes call the admin controls "temporary while Local is retired".
- **No date (Stated):** the docs say only "Local remains an editor harness option while you adapt integrations that depend on it."
- **Local's settings text (Tested):** VS Code 1.144 says "This setting and the Local agent harness will be removed in a future release." This wording isn't on any public page we found.
- **Keeping Local visible (Stated):** for an unmanaged editor, set `chat.editor.localAgent.enabled: true`, `chat.defaultToCopilotHarness: false` and `chat.editor.preferCopilotHarness: false`. All three are experimental.
- **Enterprise controls (Stated):** `vscode.temporaryDefaultToLocalHarness: true` keeps Local available and suppresses automatic Copilot routing. `vscode.temporaryAllowLocalHarness: false` blocks Local, Claude and Codex, and takes precedence. Managed `sandbox.enabled` or `permissions` settings can require the Copilot harness. VS Code 1.144 also contains `chat.temporaryDefaultToLocalHarness`, which isn't publicly documented.
- **What decides the default:** gradual rollout, settings and policy. A rollout experiment is implied by PR text. No source ties the default to anything installed locally.
- **Remembered choice:** VS Code's code suggests explicit and remembered choices are preserved, but on the test machine each new main-window chat returned to Copilot. [Issue #337676](https://github.com/microsoft/vscode/issues/337676) reports the same.

---

## Custom agent files

### Discovery

- **Locations (Stated):** `.github/agents` and `~/.copilot/agents` on all three. VS Code also reads `.claude/agents` and `~/.claude/agents` in Claude format.
- **`.claude/agents` in the CLI (Stated):** the 1.0.62 changelog says agents in nested `.github/agents` and `.claude/agents` folders are discovered. So a workspace with a Claude Code deployment may expose those agents to the CLI too. Not tested.
- **Identity (Stated):** the CLI's agent ID is the file name without `.agent.md` or `.md`, and subfolder files get `--` in the ID. A personal agent beats a repository agent with the same ID. `name` doesn't affect deduplication.
- **Plain `.md` (Tested):** the CLI loads it, and lists each agent once when both files exist. [Issue #2187](https://github.com/github/copilot-cli/issues/2187) reports that every `.md` in the folder loads, including a README.

### Which file wins when `X.md` and `X.agent.md` both exist

No official source documents this. Both extensions are documented as valid in `.github/agents`, and neither is called deprecated.

| Tested | Copilot harness | Local harness | GHCP CLI |
|---|---|---|---|
| Selected agent | `.agent.md` | `.md` | `.agent.md` |
| Subagent | `.agent.md` | `.agent.md` | `.agent.md` |

Local's split between selection and dispatch rules out side-by-side files. File age doesn't decide the CLI's choice or Local's selection. Local's subagent choice was only tested with `.md` newer.

### Frontmatter

| Field | Finding |
|---|---|
| `target` | The CLI ignores it with `unknown field ignored: target` (Tested). VS Code uses it to choose which harness's schema reads the file. It doesn't switch the session harness (Stated). With `target: vscode`, instructions didn't appear to load on Local (Tested). Don't write it |
| `model` | The CLI resolves VS Code display names such as `Claude Haiku 5.5` (Tested; Stated since 1.0.24). The CLI tries arrays in order since 1.0.83, and the Copilot harness takes a preference-ordered array (Stated). Unknown values aren't documented; see below |
| `model` format | Copilot uses runtime model IDs, which "differ from Local's qualified display names". CLI docs ask for provider-qualified IDs in BYOK examples, while its changelog accepts display names. `model-policy: required` makes the model mandatory (Stated) |
| `user-invocable`, `disable-model-invocation` | Honoured by the CLI, since 0.0.412 and 0.0.411 (Stated). `disable-model-invocation` isn't described for the Copilot harness |
| `agents` | Local only: an allowlist of subagents, with case-sensitive names (Stated) |
| `handoffs`, `argument-hint` | Local only (Stated) |
| `mcp-servers` | Supported on the CLI and Copilot harness, not on Local (Stated). VS Code doesn't forward it from IDE agents |
| `include-custom-instructions` | On the CLI, subagents don't follow repository custom instructions unless this is `true` (Stated) |
| Unknown fields | The CLI warns and still loads the agent, since 0.0.402 (Stated) |

GitHub says properties "can vary across environments, and some may be ignored". Nothing promises that one file behaves the same everywhere.

**Unknown `model` value (Tested):** no runtime refused the agent.
- **CLI:** warned and used its default, `claude-sonnet-5.5`.
- **Copilot harness and Local:** no warning seen, and each used the model already selected in the chat.

### Tool names

Each list ran as a subagent, which had to create a file, read it back and ask the user a question.

| Tested list | Copilot harness | GHCP CLI | Local |
|---|---|---|---|
| Union of both deployed lists | File tools work, no ask tool | Same | All work, including ask |
| VS Code names only | File tools work, no ask tool | Same | Not run |
| CLI names only | File tools work, no ask tool | Same | File tools work, no ask tool |

The `merged` agent, selected directly, asked successfully through `ask_user` on the Copilot harness and the CLI. So **the ask tool is missing only from subagents on the CLI engine, and the CLI and Copilot harness behave the same**. Our system prompt capture agrees: `ask_user` is in the GHCP CLI primary agent's tool list and not in its subagent list (see `SystemPromptCapture/GHCP-CLI/`). No official source states this. Claude Code background subagents have the same gap (CC-086).

- **Local needs the VS Code name:** `ask_user` doesn't give Local an ask tool, so a CLI-only list loses it there.
- **Unknown names:** ignored, not rejected (Stated). A fake tool caused no error on any harness (Tested).
- **Editor hint:** VS Code greys out names the active harness doesn't know, with the message "not recognized by the matching GitHub Copilot harness". It's hint severity and undocumented (Tested).
- **VS Code names on Copilot harness:** `vscode/<tool>` is valid only when the session provides that tool (Stated). Copilot uses the aliases `execute`, `read`, `edit`, `search`, `agent`, `web` and `todo` (Stated).
- **Subagent tool:** Copilot uses its native `task` tool and Local uses `runSubagent`. Local's `agent/runSubagent` isn't portable (Stated).
- **Alias mapping (Stated):** `execute` covers shell, Bash and powershell. `read` covers Read and NotebookRead. `edit` covers Edit, MultiEdit, Write and NotebookEdit. `search` covers Grep and Glob. `agent` covers custom-agent and Task. `web` covers WebSearch and WebFetch. `todo` covers TodoWrite. They're case-insensitive.
- **MCP tools (Stated):** written as `server-name/tool-name` or `server-name/*`.
- **Maintainer view:** "VS Code, Copilot CLI, and Claude run as separate agent harnesses with different tool sets and names" ([#313437](https://github.com/microsoft/vscode/issues/313437), before the harness merge).

---

## Hooks

**Shared format (Stated):** "VS Code, the GitHub Copilot app, and GitHub Copilot CLI share this harness and its hook format." Local isn't part of that: it "sends the payloads documented here even when it accepts a compatible configuration format" ([Local reference](https://code.visualstudio.com/docs/agents/reference/hooks-reference)).

**Limits (Stated, [hooks docs](https://code.visualstudio.com/docs/copilot/customization/hooks)):**
- "Sharing a hook file location does not establish runtime compatibility."
- "Event names also select the input format." `preToolUse` gets `toolName` and `toolArgs`, and `PreToolUse` gets snake_case and Claude-style tool names.
- The VS Code hooks experience is in Preview.

**Locations (Stated):** the Copilot harness reads `.github/hooks/`, `~/.copilot/hooks/` and `.github/copilot/settings.json`. It also reads `.claude/settings.json` and `.claude/settings.local.json`.

**Other CLI locations (Stated):** policy files, `.github/copilot/settings.local.json`, `~/.copilot/settings.json` and plugins. Cloud agent loads only `.github/hooks/*.json`. A new repository hook doesn't replace a user or plugin hook for the same event.

**Format on the CLI engine (Stated):**
- `"version": 1` is documented as required, and a bad version rejects the file. MOSAIC's `ghcp-cli` registration has no `version` and still loaded (Tested).
- Command hooks use `bash`, `powershell` or `command`, or `exec` plus `args`, with `cwd`, `env` and `timeoutSec` (default 30). `timeout` is an alias used only when `timeoutSec` is absent. Also `http` and `prompt` hooks, and `matcher`.
- A malformed item is dropped and logged. Invalid JSON or a non-array event list rejects the whole file.
- Events: `sessionStart`, `sessionEnd`, `userPromptSubmitted`, `userPromptTransformed`, `preToolUse`, `postToolUse`, `postToolUseFailure`, `permissionRequest`, `preCompact`, `subagentStart`, `subagentStop`, `agentStop`, `errorOccurred` and `notification`.
- PascalCase is deliberate: since 1.0.6, hook files "work across VS Code, Claude Code, and the CLI without modification", and since 1.0.21 PascalCase gets snake_case payloads.
- Timestamps are Unix milliseconds for camelCase and ISO 8601 for PascalCase.

**Local (Stated):** events are `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `SessionStart`, `Stop`, `SubagentStart`, `SubagentStop` and `PreCompact`. It accepts nested Claude command groups but ignores `matcher`. It needs `chat.useHooks` and a trusted workspace. Don't set both `timeout` and `timeoutSec`, because Local and the Copilot runtime give them opposite precedence.

**Trust:** the CLI runs repository hooks only in trusted folders, and fails silently otherwise.

**What fired (Tested),** with one hook file registering both casings of each event, plus a `.claude/settings.json`:

| | Copilot harness and GHCP CLI | Local |
|---|---|---|
| camelCase events | Fire, with `toolName`, `toolArgs` and `sessionId`. Tool names are native, such as `create` and `task` | Didn't fire |
| PascalCase events | Fire as well, with `tool_name`, `tool_input` and `session_id`. Tool names are Claude-style, such as `Write` and `Agent` | Fire, with native tool names such as `create_file` |
| `.claude/settings.json` hooks | Fire, with the PascalCase payload | Didn't fire |

- **Double firing:** a file that registers both casings runs twice for each event on the CLI engine. Adding `.claude` hooks makes it three times for events registered there.
- **Local's aliases:** its docs say it accepts Copilot event aliases. We only tested a file that also had PascalCase entries, so Local alone with camelCase events is untested.

**`ghcp-cli` logger on the Copilot harness (Tested):** the unmodified adapter wrote orchestrator events, a subagent folder and transcripts for one subagent run. A real MOSAIC run wasn't tested.

**MOSAIC collisions:**
- The `ghcp-cli` and `vscode-ghcp` deployments both write `.github/hooks/mosaic-logger.json` and `.github/hooks/mosaic_logger.py`, with different adapters.
- MOSAIC's Claude Code registration in `.claude/settings.json` fires under the CLI, but the logger doesn't run and writes no logs (Tested on the CLI with the real logger). The registration uses `command` plus `args`. A probe registered that way never ran, while the same probe placed fully in `command` did. The CLI documents `args` only alongside `exec`, which would explain it.
- What's left is a side effect. The CLI starts the interpreter with no script. With `py`, one `Hook command failed with code 1` error appeared in the log. With `python3` on the test machine, a hook hung until its 30-second timeout and delayed the run.
- `${CLAUDE_PROJECT_DIR}` didn't work inside a `command` string on Windows, although the CLI does set that variable.

---

## Skills and instructions

All Stated.

| | Copilot harness | Local | GHCP CLI |
|---|---|---|---|
| Project skills | `.github/skills` | `.github/skills`, `.claude/skills`, `.agents/skills` | `.github/skills`, `.agents/skills` |
| Personal skills | `~/.copilot/skills` | `~/.copilot/skills`, `~/.claude/skills`, `~/.agents/skills` | `~/.copilot/skills`, `~/.agents/skills` |
| Instructions | `.github/copilot-instructions.md`, `AGENTS.md`, `.github/instructions/**/*.instructions.md` | Depends on the session | `AGENTS.md`, `copilot-instructions.md`, `CLAUDE.md`, `~/.copilot/instructions` |

- Skills use `SKILL.md`. An invalid name fails to load silently.
- "Applicable instruction sources are additive", and "there is no general precedence order between these files".
- Agent body instructions are added after custom instructions and take precedence on the Copilot harness.
- The CLI stopped loading user-level agents, skills and commands from `~/.claude/` in 1.0.35 and 1.0.36. Project `.claude` folders are separate.

---

## Open questions

1. Does an MCP ask tool work in a CLI-engine subagent? It's the workaround for Claude Code (CC-086), and very likely works here, but it hasn't been tested.
2. Does Local honour a CLI model ID such as `claude-haiku-5.5`? Our tests couldn't tell it apart from the model already selected.
3. Does a real orchestrator work on the Copilot harness? The test agents were small, and terminal and search tools weren't called.
4. In a workspace with a Claude Code deployment, do the CLI and Copilot harness pick up `.claude/agents`, and which file wins on a name clash?

## Not found in official sources

- Which file wins when `X.md` and `X.agent.md` both exist, on any product.
- `target` documented for the CLI, and why `target: vscode` stopped instructions loading on Local.
- Which `vscode/*` tools the Copilot harness provides, and any statement on `vscode/askQuestions` per harness.
- The editor hint text, and behaviour for an unknown `model` value.
- Whether camelCase and PascalCase can be mixed in one hook file, and how unknown hook events are handled.
- A date for removing Local, or any roadmap item for it.
- Official confirmation that the Copilot harness never uses the installed CLI.
- A statement that one agent file is meant to behave the same everywhere.

---

## Related issues

These are unofficial unless a maintainer is quoted above.

- [microsoft/vscode#313437](https://github.com/microsoft/vscode/issues/313437): harness tool sets and names differ. It predates the harness merge
- [microsoft/vscode#341937](https://github.com/microsoft/vscode/issues/341937): removal of Local
- [microsoft/vscode#339064](https://github.com/microsoft/vscode/issues/339064): `chat.editor.localAgent.enabled` is "the only setting that determines if it's hidden" (maintainer)
- [microsoft/vscode#304574](https://github.com/microsoft/vscode/issues/304574): VS Code-hosted CLI subagents lost declared tools, closed 2026-03-27
- [github/copilot-cli#2187](https://github.com/github/copilot-cli/issues/2187): any `.md` in `.github/agents` loads as an agent
- [github/copilot-cli#2133](https://github.com/github/copilot-cli/issues/2133): model value formats
- [github/copilot-cli#2475](https://github.com/github/copilot-cli/issues/2475): a `name` that differs from the filename can list an agent twice. Not verified
- [github/copilot-cli#2109](https://github.com/github/copilot-cli/issues/2109): request for an `ask_user`-style extension. Not read in detail

## Main sources

- [Custom agents configuration, GitHub](https://docs.github.com/en/copilot/reference/custom-agents-configuration)
- [Custom agents for the CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-custom-agents-for-cli)
- [Hooks configuration, GitHub](https://docs.github.com/en/copilot/reference/hooks-configuration) and [CLI hooks reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-hooks-reference)
- [Copilot CLI changelog](https://github.com/github/copilot-cli/blob/main/changelog.md)
- [Custom agent file reference, VS Code](https://code.visualstudio.com/docs/agents/reference/custom-agent-file-reference)
- [Agent Host](https://code.visualstudio.com/docs/agents/concepts/agent-host), [tools](https://code.visualstudio.com/docs/agent-customization/tools) and [subagents](https://code.visualstudio.com/docs/agents/subagents), VS Code
- [Skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills) and [instructions](https://code.visualstudio.com/docs/copilot/customization/custom-instructions), VS Code
- VS Code release notes: [1.129](https://code.visualstudio.com/updates/v1_129), [1.142](https://code.visualstudio.com/updates/v1_142), [1.143](https://code.visualstudio.com/updates/v1_143), [1.144](https://code.visualstudio.com/updates/v1_144)
