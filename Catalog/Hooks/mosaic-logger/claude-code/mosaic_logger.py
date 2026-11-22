"""mosaic_logger.py — Dispatcher entrypoint for the mosaic-logger Claude Code hook adapter.

This module is the process entrypoint spawned by the Claude Code harness on every hook
firing. It reads one JSON object from stdin, dispatches it to the appropriate handler,
and exits 0 unconditionally. Nothing is written to stdout.
"""

import json
import os
import sys

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_handlers_session as _session
import mosaic_logger_handlers_invocation as _invocation
import mosaic_logger_handlers_tools as _tools
import mosaic_logger_handlers_runner as _runner
import mosaic_logger_runner_mode as runner_mode


# ---------------------------------------------------------------------------
# Handler registry — populated at import time from handler modules
# ---------------------------------------------------------------------------

CAPTURE_DISPATCH_ARG = "capture-dispatch"

HANDLERS: dict = {}
for _mod in [_session, _invocation, _tools]:
    if hasattr(_mod, "HANDLERS"):
        HANDLERS.update(_mod.HANDLERS)


# ---------------------------------------------------------------------------
# Dispatcher functions
# ---------------------------------------------------------------------------

# The only event whose payload text can bind a session to a run: a subagent
# launch prompt, and only when it is a dispatch (carries an agent instance id).
# User prompts, assistant replies and notification text never bind.
_DISPATCH_PROMPT_EVENT = "SubagentStart"
_DISPATCH_PROMPT_FIELD = "agent_prompt"


def build_context(payload: dict) -> core.HookContext:
    """Construct the immutable per-firing context from a parsed hook payload.
    Resolves the workspace root and LogPaths tree. Does not create directories.
    In Runner mode every orchestrator transcript gets a session-scoped name.
    """
    workspace_root = core.resolve_workspace_root(payload)
    mode = runner_mode.read_runner_mode()
    paths = core.build_paths(workspace_root, mode is not None)
    timestamp = core.current_timestamp()
    ctx = core.HookContext(payload, workspace_root, paths, timestamp)
    if mode is not None and mode.agent_type and ctx.agent_id is None:
        # The primary session of a Runner subagent is the invocation itself.
        ctx.agent_type = ctx.agent_type or mode.agent_type
    return ctx


def resolve_run_identity(ctx: core.HookContext) -> None:
    """Populate ctx.run_id before any handler runs.

    Resolution order:
      1. Dispatch prompt. On SubagentStart, when agent_prompt carries an
         agent instance id (a MOSAIC dispatch) and extract_run_id yields a
         value, that value is adopted via runstate.adopt_run_id -- it takes
         effect for this firing AND extends the session timeline. A run
         differing from the currently bound one therefore wins from then on.
         User prompts, assistant replies, notification text and prompts
         without an instance id never bind, quoted run ids included.
      2. Binding lookup. Otherwise the run active at ctx.timestamp is read
         from the session timeline (set earlier by a dispatch, e.g. the
         Agent/Task PreToolUse adoption) and assigned to ctx.run_id.
      3. Native MOSAIC_RUN_ID environment fallback.
      4. Unresolved. ctx.run_id stays None, core.effective_run_id returns
         'unknown-run', and a 'run-identity: unresolved' diagnostic is
         emitted.

    In Runner mode a valid MOSAIC_RUN_ID is the run identity and is adopted
    before any of the above; prompt text never overrides it.

    Never raises.
    """
    try:
        mode = runner_mode.read_runner_mode()
        if mode is not None and mode.run_id:
            runstate.adopt_run_id(ctx, mode.run_id)
            return

        extracted = None
        if ctx.event == _DISPATCH_PROMPT_EVENT:
            prompt = ctx.field(_DISPATCH_PROMPT_FIELD)
            if runstate.extract_instance_id(prompt):
                extracted = runstate.extract_run_id(prompt)

        if extracted:
            runstate.adopt_run_id(ctx, extracted)
            return

        bound = runstate.get_session_run_binding(ctx.paths, ctx.session_id, at=ctx.timestamp)
        if bound:
            ctx.run_id = bound
            return

        env_run_id = os.environ.get("MOSAIC_RUN_ID")
        if env_run_id and runstate.is_valid_run_id(env_run_id):
            runstate.adopt_run_id(ctx, env_run_id)
            return

        core.debug_log(
            f"run-identity: unresolved for session_id={ctx.session_id!r} "
            f"event={ctx.event!r}"
        )
    except Exception as exc:
        core.debug_log("resolve_run_identity failed", exc)


def dispatch(raw_input: str) -> None:
    """Testable core of main. Reads raw JSON, dispatches, swallows all exceptions.

    Steps:
      1. Parse raw_input as JSON; abort silently if not a dict.
      2. Build HookContext.
      3. resolve_run_identity — extracts run_id from prompt content when present.
      4. Emit run_start on SessionStart (event-based, not mint-outcome-based).
      5. Invoke registered handler for ctx.event, if any.
      6. Emit run_end on SessionEnd (event-based, after the handler) — always runs.
    Step 6 runs even when step 5 raised, so a handler failure on SessionEnd
    cannot swallow run_end.
    Any exception at any step is caught and swallowed.
    """
    try:
        # Step 1: parse
        try:
            payload = json.loads(raw_input)
        except Exception as exc:
            core.debug_log(
                f"dispatch: hook payload is not valid JSON "
                f"(length={len(raw_input) if isinstance(raw_input, str) else 'n/a'}); "
                f"nothing logged", exc
            )
            return
        if not isinstance(payload, dict):
            core.debug_log(
                f"dispatch: hook payload is JSON but not an object "
                f"(type={type(payload).__name__}); nothing logged"
            )
            return

        # Step 2: context
        ctx = build_context(payload)

        # Step 3: resolve run identity via extraction
        try:
            resolve_run_identity(ctx)
        except Exception as exc:
            core.debug_log("resolve_run_identity failed", exc)

        # Runner mode: the Runner owns run_start/run_end; the role selects
        # handler overrides.
        mode = runner_mode.read_runner_mode()

        # Step 4: emit run_start on SessionStart
        try:
            if ctx.event == "SessionStart" and mode is None:
                _session.emit_run_start(ctx)
        except Exception as exc:
            core.debug_log("emit_run_start failed", exc)

        # Step 5: handler
        try:
            handler = _runner.handlers_for(mode).get(ctx.event)
            handler = handler or HANDLERS.get(ctx.event)
            if handler is not None:
                handler(ctx)
        except Exception as exc:
            core.debug_log(f"handler for {ctx.event!r} raised", exc)

        # Step 6: emit run_end on SessionEnd — always runs
        try:
            if ctx.event == "SessionEnd" and mode is None:
                outcome = ctx.field("reason")
                _session.emit_run_end(ctx, outcome)
        except Exception as exc:
            core.debug_log("emit_run_end failed", exc)

    except Exception as exc:
        core.debug_log("dispatch outer exception", exc)


def dispatch_capture(raw_input: str) -> None:
    """Capture-only entry for the synchronous Agent|Task PreToolUse registration.

    Accepts only PreToolUse payloads and hands them to capture_dispatch, which
    adopts the prompt's run id and queues the pending dispatch. Runs no
    handler, appends no event, emits no usage and reads no transcript.
    Never raises.
    """
    try:
        try:
            payload = json.loads(raw_input)
        except Exception as exc:
            core.debug_log("dispatch_capture: hook payload is not valid JSON", exc)
            return
        if not isinstance(payload, dict):
            return
        if payload.get("hook_event_name") != "PreToolUse":
            return
        _tools.capture_dispatch(build_context(payload))
    except Exception as exc:
        core.debug_log("dispatch_capture outer exception", exc)


def main(argv: "list[str] | None" = None) -> int:
    """Process entrypoint. Reads one JSON object from stdin, dispatches, exits 0.

    argv[1] == CAPTURE_DISPATCH_ARG selects dispatch_capture; anything else
    selects the full dispatch.

    ALWAYS exits 0. Writes NOTHING to stdout. Never raises.
    """
    try:
        args = sys.argv if argv is None else argv
        raw = sys.stdin.read()
        if len(args) > 1 and args[1] == CAPTURE_DISPATCH_ARG:
            dispatch_capture(raw)
        else:
            dispatch(raw)
    except Exception as exc:
        core.debug_log("main() uncaught exception", exc)
    return 0


if __name__ == "__main__":
    sys.exit(main())
