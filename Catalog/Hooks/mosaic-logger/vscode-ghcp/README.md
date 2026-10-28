# vscode-ghcp variant

A stdlib-only Python 3 hook adapter for the VS Code Chat Extension (GitHub Copilot
Chat). Deployed to `.github/hooks/` alongside the registration file, it captures
8 hook events and writes structured logs under `OrchestrationLogs/` in your project
root.

## File set

The adapter is a flat set of 9 Python modules, all deployed to `.github/hooks/`:

| Module | Role |
|---|---|
| `mosaic_logger.py` | Dispatcher entrypoint — reads stdin, routes by event name, exits 0 |
| `mosaic_logger_core.py` | Constants, `HookContext`, event envelope, path layout, write primitives |
| `mosaic_logger_runstate.py` | ID extraction, pending-dispatch queue, agent-name correlation map |
| `mosaic_logger_transcript.py` | Best-effort model/token extraction from the session transcript |
| `mosaic_logger_export.py` | Byte-for-byte transcript copy and sidecar metadata |
| `mosaic_logger_artifacts.py` | `01_input.md` / `02_output.md` rendering and writing |
| `mosaic_logger_diagnostics.py` | Default-visible diagnostics file (`OrchestrationLogs/mosaic-logger-diagnostics.jsonl`) |
| `mosaic_logger_subagent_state.py` | Subagent spawn index, stopped markers and completion claims |
| `mosaic_logger_handlers_session.py` | `SessionStart`, `UserPromptSubmit`, `PreCompact`, `Stop` |
| `mosaic_logger_handlers_invocation.py` | `SubagentStart`, `SubagentStop` |
| `mosaic_logger_handlers_tools.py` | `PreToolUse`, `PostToolUse` |

A copy of `hook.yaml` (the bundle manifest) is also deployed alongside the modules so
the adapter can stamp `adapter_version` on `run_start` events at runtime.

## Deployment target

All files deploy to `.github/hooks/`. The registration file
`.github/hooks/mosaic-logger.json` must exist in the same directory for VS Code to
discover it, and the bare `mosaic_logger.py` script name in each command entry resolves
because `cwd` is set to `.github/hooks` in the fragment.

## Interpreter requirement

Each command entry uses `python3` on POSIX and `py` on Windows (via the `windows`
override). The `windows` key carries a complete command line (`py mosaic_logger.py`)
rather than just an interpreter name, because `python3` is frequently absent from PATH
on Windows installs.

## Supported events

The adapter registers exactly 8 events:

`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PreCompact`,
`SubagentStart`, `SubagentStop`, `Stop`

Events that are unconfirmed for the VS Code harness (`SessionEnd`, `Notification`,
`PostToolUseFailure`, `PostCompact`) are not registered; the adapter silently ignores
any unrecognised event it receives.

## Compaction events

`PreCompact` is the only registered compaction hook, so it writes one self-contained
`compaction` event (log format 1.5.0) without `phase`. `tokens_before` is written only
when the payload carries an integer value; `custom_instructions` is never written. The
event goes to a subagent's stream when the payload's `agent_id` is in the spawn index,
or when the payload's `session_id` is mapped to a subagent by the spawn index and is
not an orchestrator session; otherwise to the orchestrator stream.

Unverified pending a live run: whether VS Code fires `PreCompact` for a subagent's own
compaction and what identifies the subagent in that payload, and whether `PostCompact`
exists (it is not registered).

## Enablement

VS Code hooks are enabled by default. No user-level setting needs to be flipped.

If hooks do not fire after deploying the registration file, the enterprise `ChatHooks`
policy (surfacing as `chat.useHooks`) may have been disabled by an administrator. This
is an organization-wide policy that per-project deployment tooling cannot override;
contact your administrator if hooks are expected to run but do not.
