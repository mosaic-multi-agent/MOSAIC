# mosaic-logger — Hook Adapter Bundle

A hook adapter bundle that captures hook events and writes structured logs under
`OrchestrationLogs/` in your project root. Implemented for `claude-code` (12 events),
`ghcp-cli` (14 events), `vscode-ghcp` (8 events), and `opencode` (4 hooks).

## Supported variants

| Variant | Status |
|---|---|
| `claude-code` | Implemented — Python adapter deployed to `.claude/hooks/` |
| `opencode` | Implemented — TypeScript plugin deployed to `.opencode/plugins/` |
| `vscode-ghcp` | Implemented — Python adapter deployed to `.github/hooks/` |
| `ghcp-cli` | Implemented — Python adapter deployed to `.github/hooks/` |

## Claude Code: subagent result delivery

On Claude Code 2.1.271+ in auto mode a subagent delivers its result through a
`SubagentHandback` tool call. The report is `tool_input.message`, and it reaches
the caller just before the hand-back's PostToolUse hook fires. The adapter
therefore writes `invocation_end` (`response`, `status_code`, timestamp), the
final `turn` and `02_output.md` from that PostToolUse. `model` and `token_usage`
are read from the agent transcript at hand-back time. `SubagentStop` fires later
and its `last_assistant_message` is only post-hand-back commentary; it does not
produce a second `invocation_end`.

Without a hand-back (older CLI, non-auto mode, or Haiku as the main model, which
silently falls back from auto to default) `SubagentStop` /
`last_assistant_message` remains the delivery point.

Known limitation: if the agent transcript has not been flushed at hand-back time,
`invocation_end` omits `model` and `token_usage`, and they are never back-filled.
`Tools/LogAnalyzer` reports the missing fields as an issue for that invocation.
Tokens used after the hand-back are still counted through `usage_record` events.

`tool_call_end` carries the tool's output in `tool_output`, taken from the
PostToolUse `tool_response`, without truncation.

## Compaction events (Claude Code)

`PreCompact` and `PostCompact` each write one `compaction` event (log format 1.5.0),
with `phase` `pre` and `post`. A payload carrying `agent_id` of a mapped subagent
routes the event to that subagent's stream; otherwise it goes to the orchestrator
stream. `tokens_before`/`tokens_after` are written only when the payload carries an
integer value.

Unverified pending a live run: whether Claude Code fires `PreCompact`/`PostCompact`
for a subagent's own compaction, and whether those payloads carry the subagent's
`agent_id`. Until verified, a subagent compaction without an identifying payload is
recorded in the orchestrator stream.

## Runner mode

The `claude-code`, `ghcp-cli` and `opencode` adapters also log sessions started by
the MOSAIC Runner (`Tools/Runner`). The Runner sets `MOSAIC_ROLE` (`orchestrator` or
`subagent`), `MOSAIC_RUN_ID` and, for the subagent role, `MOSAIC_AGENT_INSTANCE_ID`
in the harness process environment. Without `MOSAIC_ROLE` the adapter is a native
adapter. The deployed registration is the same in both modes.

In Runner mode the Runner writes `run_start` and `run_end` itself; the adapter writes
everything else. A subagent-role session is one invocation and is logged in the
invocation folder named by `MOSAIC_AGENT_INSTANCE_ID`; an orchestrator-role session
writes to the run's orchestrator stream and exports its transcript to a
session-scoped `00_orchestrator_session__{scope}.raw`. Fields Runner mode cannot
produce, the accepted differences from native logs, and the per-harness constraints
are listed in `Development/Designs/MosaicLogFormat.md` section 4.6 (Runner-hosted
mode). One difference worth knowing on Claude Code: in the subagent role
`invocation_end.token_usage` is summed over all assistant records of the session,
whereas native `invocation_end` uses the last assistant record only.

### Claude Code registration: synchronous Stop and SessionEnd

`Stop` and `SessionEnd` are registered synchronous (together with `SubagentStart`) in
both modes. Claude Code kills async hooks when a `-p` process exits, so the Runner
could otherwise find a session's final events missing. Native sessions pay the same
cost: handler wall time measured on 2026-10-04 (adapter 1.5.0, 1200-record, 1.24 MB
transcript, including about 30 ms interpreter start-up) is about 200 ms for `Stop` and
about 60 ms for `SessionEnd` natively, and about 215 ms for the Runner subagent-role
`SessionEnd`. Existing deployments keep the previous (asynchronous) registration and
file set until redeployed, so redeploy the bundle to pick up the new registration and
the Runner-mode modules.

### GHCP CLI: trusted folder required

GHCP CLI loads repository hooks (`.github/hooks/*.json`) in `-p` mode only from a
trusted folder, so without trust no MOSAIC logs are written and no warning is shown.
See `ghcp-cli/README.md`.

### OpenCode: `session.idle` is not awaited on exit

`opencode run` does not await the plugin's `session.idle` handler at exit. See
`opencode/LIMITATIONS.md`.

## Copilot token usage (GHCP CLI and VS Code Copilot Chat)

Both Copilot harnesses report token usage through the harness's OpenTelemetry file
export, not through hook payloads. The `ghcp-cli` and `vscode-ghcp` hooks read that
file and add the usage to the run's logs as `usage_record` events.

### Enable it

Set one environment variable, for both harnesses, to a file path of your choice:

```
COPILOT_OTEL_FILE_EXPORTER_PATH=<absolute path of a .jsonl file>
```

- Setting only this variable switches on the harness's file export **and** MOSAIC's
  ingest. Do not use the VS Code user setting for the export instead: the hooks cannot
  read it, so MOSAIC would not ingest the file.
- Set it at user level (Windows: user environment variables; macOS/Linux: your login
  environment), not in one shell. Restart VS Code afterwards: VS Code started from the
  desktop sees only user-level variables.
- Content capture (prompt and response text in the file) is optional and only makes
  the file larger. MOSAIC never reads content and never modifies the OTel file.
- Copilot usage is priced from `github-copilot` entries you maintain in
  `pricing.yaml`; the shipped file has no priced entry, so without one costs stay
  unpriced.

### Recover missing usage

Usage can be missing when the variable was unset, the file is missing, a session
crashed, or a run's hooks did not see the variable. Recover the run with:

```
mosaic-log-analyzer ingest-otel [otel-file] --run <run-id>
```

The file is the argument, or the value of `COPILOT_OTEL_FILE_EXPORTER_PATH` when
omitted. It ingests the whole file as a final ingest. "Usage not fully ingested" in
LogAnalyzer means usage spans of the run are still pending (not yet tied to an actor,
or the ingest was cut short); a recovery ingest resolves them. In LogAnalyzer's views,
unattributed usage is usage tied to the run but not to an agent.

### Locations

The VS Code Copilot Chat hooks are deployed to `.github/hooks/` (the same place as
GHCP CLI) and log to the workspace root's `OrchestrationLogs/`. Existing deployments
keep their old file set until redeployed.

## Interpreter requirement

The `claude-code` variant requires **`python3`** to be available in the environment
where the Claude Code harness runs hook commands. The registration fragment uses:

```json
{
  "type": "command",
  "command": "python3",
  "args": ["${CLAUDE_PROJECT_DIR}/.claude/hooks/mosaic_logger.py"]
}
```

### Windows: using `python` or `py` instead of `python3`

On Windows installs where `python` or `py` is available but `python3` is not, update
the `command` field in the registration fragment pasted into `.claude/settings.json`:

```json
"command": "python"
```

or

```json
"command": "py"
```

The deployment tool emits the fragment for manual paste when `.claude/settings.json`
already exists. Make the one-line tweak before pasting.

## Why `placeholder` is absent

The schema's `placeholder` flag is a single bundle-level boolean with no per-variant
form. All four variants — `claude-code`, `vscode-ghcp`, `ghcp-cli`, and `opencode` —
are fully authored and deployable; marking the bundle as a placeholder would be
inaccurate. The schema provides no per-variant mechanism to distinguish authoring
states, so the whole-bundle key is omitted.

## Why `content_hash` is absent

The `claude-code` variant lists `../hook.yaml` in its file set so the deployed adapter
can read its own version at runtime (the `run_start.adapter_version` field). Including
`hook.yaml` in the hash computation makes the stored hash unstable by construction: any
change to the version field changes the hash, which must then be updated in the file,
which changes the hash again. `content_hash` is left unset; the deployment tool skips
hash validation when the field is absent.

## Maintainer: Copilot OTel ingest cost

Measured with `copilot-shared/tools/measure_ingest.py <otel-file>` (not deployed, not a
test). It runs the shared ingest module in scratch run folders: a first ingest from the
mid-file offset without a time limit, ten incremental ingests of 20 appended lines each
(a typical boundary), a whole-file ingest under the default 5 s budget and under a 50 ms
budget, and the start-up plus import of the module in a fresh interpreter. The scratch
folders have no run keys, so spans stay pending: reading, parsing and state saving are
measured, writing records to stream files is not.

Machine: AMD64 Family 25 Model 97 (Ryzen-class), Windows 11, Python 3.14.6, warm file cache,
2026-10-28. Samples from the 27.10 captures.

| Scenario | VS Code, 50.5 MB, content capture on (3199 lines) | GHCP CLI, 2.6 MB (2019 lines) |
|---|---|---|
| First ingest from mid-file offset | 150 ms | 8.9 ms |
| Incremental, 20 new lines (median / max) | 3.2 ms / 4.9 ms | 1.4 ms / 1.7 ms |
| Whole file, default budget | 322 ms | 18 ms |
| Whole file, 50 ms budget | 51 ms, cut off after 273 lines, `budget_exhausted` | 18 ms, not cut off |
| Interpreter start + module import | 57 ms | 57 ms |

A typical boundary ingest costs a few milliseconds on top of the interpreter start-up,
which dominates; the GHCP CLI runs every hook synchronously (copilot-cli#3063), so that
start-up is what it pays per event. A first ingest of a large content-capture file stays
under the 5 s default budget by a wide margin on this machine, and the time-budget
cut-off stops at a line boundary and leaves the rest for the next ingest. Figures depend
on machine speed; re-run the script to compare.
