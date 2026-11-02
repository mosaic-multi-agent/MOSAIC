"""mosaic_logger.py -- Dispatcher entrypoint for the mosaic-logger GHCP CLI hook adapter.

This module is the process entrypoint spawned by the GHCP CLI harness on every hook
firing. It reads the event name from sys.argv[1] and one JSON object from stdin,
dispatches to the appropriate handler, and exits 0 unconditionally. Nothing is
written to stdout.
"""

import json
import sys

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_handlers_session as _session
import mosaic_logger_handlers_invocation as _invocation
import mosaic_logger_handlers_tools as _tools
import mosaic_logger_handlers_runner as _runner
import mosaic_logger_runner_mode as runner_mode


# ---------------------------------------------------------------------------
# Handler registry -- populated at import time from handler modules
# ---------------------------------------------------------------------------

HANDLERS: dict = {}
for _mod in [_session, _invocation, _tools]:
    if hasattr(_mod, "HANDLERS"):
        HANDLERS.update(_mod.HANDLERS)


# ---------------------------------------------------------------------------
# Dispatcher functions
# ---------------------------------------------------------------------------

# Only a subagentStart prompt that carries an agent instance id (a MOSAIC
# dispatch) can bind a session to a run. User prompts, subagent replies and
# notification text never do, even when they quote a run id.
_DISPATCH_PROMPT_EVENT = "subagentStart"
_DISPATCH_PROMPT_FIELD = "agentPrompt"


def build_context(event: str, payload: dict) -> core.HookContext:
    """Construct the immutable per-firing context from an event name and parsed payload.
    The event name comes from sys.argv[1] (passed by the hook registration command).
    Resolves the workspace root and LogPaths tree. Does not create directories.
    In Runner mode every orchestrator transcript gets a session-scoped name."""
    workspace_root = core.resolve_workspace_root(payload)
    mode = runner_mode.read_runner_mode()
    paths = core.build_paths(workspace_root, mode is not None)
    timestamp = core.current_timestamp()
    ctx = core.HookContext(event, payload, workspace_root, paths, timestamp)
    ctx.runner_mode = mode
    if mode is not None and mode.agent_type and ctx.agent_id is None:
        # The primary session of a Runner subagent is the invocation itself.
        ctx.agent_type = ctx.agent_type or mode.agent_type
    return ctx


def resolve_run_identity(ctx: core.HookContext) -> None:
    """Set ctx.run_id using the following precedence.

    1. Runner mode: a valid MOSAIC_RUN_ID is the run identity; nothing else
       overrides it and nothing is cached.
    2. Dispatch prompt: on subagentStart, an agentPrompt carrying an agent
       instance id and a run id binds the session to that run (cached; a run
       differing from the current binding wins from then on). The preToolUse
       subagent-launch dispatch binds in the tool handler.
    3. Session binding: the run cached for ctx.session_id.
    4. Miss: ctx.run_id stays None and core.effective_run_id yields
       'unknown-run'.

    User prompts, subagent replies, notification text, tool-argument paths and
    prompts without an agent instance id never bind, quoted run ids included.

    Never raises. Reports unexpected failures through core.debug_log.
    """
    try:
        mode = ctx.runner_mode
        if mode is not None and mode.run_id:
            ctx.run_id = mode.run_id
            return

        dispatched = None
        if ctx.event == _DISPATCH_PROMPT_EVENT:
            prompt = ctx.field(_DISPATCH_PROMPT_FIELD)
            if runstate.extract_instance_id(prompt):
                dispatched = runstate.extract_run_id(prompt)

        if dispatched:
            ctx.run_id = dispatched
            if ctx.session_id:
                try:
                    runstate.put_session_run_id(
                        ctx.paths, ctx.session_id, dispatched,
                        created_at=ctx.timestamp)
                except Exception as wb_exc:
                    core.debug_log("resolve_run_identity write-back failed", wb_exc)
            return

        try:
            cached = runstate.get_session_run_id(ctx.paths, ctx.session_id)
            if cached:
                ctx.run_id = cached
        except Exception as cache_exc:
            core.debug_log("resolve_run_identity cache lookup failed", cache_exc)

    except Exception as exc:
        core.debug_log("resolve_run_identity failed", exc)


def dispatch(event: str, raw_input: str) -> None:
    """Testable core of main. Receives the event name and raw JSON from stdin.

    Steps:
      1. Parse raw_input as JSON; abort silently if not a dict.
      2. Build HookContext from event name and parsed payload.
      3. resolve_run_identity -- binds the run from a dispatch prompt or Runner env.
      4. Emit run_start on sessionStart.
      5. Invoke registered handler for ctx.event, if any.
      6. Emit run_end on sessionEnd -- always runs even if step 5 raised.
    """
    try:
        # Step 1: parse
        try:
            payload = json.loads(raw_input)
        except Exception:
            return
        if not isinstance(payload, dict):
            return

        # Step 2: context
        ctx = build_context(event, payload)

        # Step 3: resolve run identity via extraction
        try:
            resolve_run_identity(ctx)
        except Exception as exc:
            core.debug_log("resolve_run_identity failed", exc)

        # Step 4: emit run_start on sessionStart
        try:
            if ctx.event == "sessionStart" and ctx.runner_mode is None:
                _session.emit_run_start(ctx)
        except Exception as exc:
            core.debug_log("emit_run_start failed", exc)

        # Step 5: handler
        try:
            handler = _runner.handlers_for(ctx.runner_mode).get(ctx.event)
            handler = handler or HANDLERS.get(ctx.event)
            if handler is not None:
                handler(ctx)
        except Exception as exc:
            core.debug_log(f"handler for {ctx.event!r} raised", exc)

        # Step 6: emit run_end on sessionEnd -- always runs
        try:
            if ctx.event == "sessionEnd" and ctx.runner_mode is None:
                outcome = ctx.field("reason")
                _session.emit_run_end(ctx, outcome)
        except Exception as exc:
            core.debug_log("emit_run_end failed", exc)

    except Exception as exc:
        core.debug_log("dispatch outer exception", exc)


def main() -> int:
    """Process entrypoint. Reads event name from sys.argv[1] and one JSON object
    from stdin, dispatches via dispatch(event, raw_input), exits 0.

    ALWAYS exits 0. Writes NOTHING to stdout. Never raises.
    """
    try:
        event = sys.argv[1] if len(sys.argv) > 1 else ""
        raw = sys.stdin.read()
        dispatch(event, raw)
    except Exception as exc:
        core.debug_log("main() uncaught exception", exc)
    return 0


if __name__ == "__main__":
    sys.exit(main())
