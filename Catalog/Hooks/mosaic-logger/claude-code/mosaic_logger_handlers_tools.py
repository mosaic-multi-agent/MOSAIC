"""mosaic_logger_handlers_tools.py — Tool-use event handlers.

Handles: PreToolUse, PostToolUse, PostToolUseFailure. Routes tool events to
the orchestrator stream or the owning subagent's stream based on agent_id.

High-frequency path: performs no transcript reads and no directory scanning
for tool events. When tool_capture_enabled() allows it and the firing is in
orchestrator scope (no ctx.agent_id), a transcript read is deliberately
performed to emit usage_record events. Subagent-scoped firings suppress
usage emission entirely; the subagent-stop handler is the sole source of a
subagent's own usage records, reading the subagent's own transcript at stop.
"""

import json
import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_usage as usage
import mosaic_logger_handlers_invocation as invocation
import mosaic_logger_runner_mode as runner_mode

HANDBACK_TOOL_NAME = "SubagentHandback"
DISPATCH_TOOL_NAMES = ("Agent", "Task")


def resolve_destination(ctx: "core.HookContext"):
    """Route tool event to orchestrator or subagent stream based on agent_id presence."""
    run_id = core.effective_run_id(ctx)
    if not ctx.agent_id:
        mode = runner_mode.read_runner_mode()
        if mode is not None and mode.agent_instance_id:
            # Runner subagent role: the primary session is the invocation.
            return ctx.paths.invocation_events(run_id, mode.agent_instance_id)
        return ctx.paths.orchestrator_events(run_id)
    agent_instance_id, _mapped = runstate.resolve_invocation(
        ctx.paths, run_id, ctx.agent_id
    )
    return ctx.paths.invocation_events(run_id, agent_instance_id)


def _resolve_usage_scope(ctx: "core.HookContext"):
    """Return (agent_instance_id, source) for orchestrator-scoped firings,
    matching resolve_destination's routing decision.

    Only called for orchestrator-scoped firings (ctx.agent_id unset).
    Subagent-scoped firings are suppressed before this function is reached;
    _emit_tool_usage_records returns early when ctx.agent_id is set, so
    this function never runs in subagent scope."""
    if not ctx.agent_id:
        return None, "orchestrator_transcript"
    run_id = core.effective_run_id(ctx)
    agent_instance_id, _mapped = runstate.resolve_invocation(
        ctx.paths, run_id, ctx.agent_id
    )
    return agent_instance_id, "agent_transcript"


def _emit_tool_usage_records(ctx: "core.HookContext") -> None:
    """Emit usage_record events from ctx.transcript_path when in orchestrator
    scope and tool_capture_enabled() allows it. Never suppresses the
    surrounding tool event.

    When ctx.agent_id is set (subagent scope), returns immediately without
    emitting any usage records to any stream. The subagent-stop handler in
    mosaic_logger_handlers_invocation.py is the sole source of a subagent's
    own usage records; it reads the subagent's own transcript at stop time.

    When ctx.agent_id is unset (orchestrator scope) and tool_capture_enabled()
    allows it, reads ctx.transcript_path and emits one usage_record per
    assistant record to the orchestrator stream, unchanged from prior behaviour."""
    if ctx.agent_id:
        return
    if not usage.tool_capture_enabled():
        return
    mode = runner_mode.read_runner_mode()
    if mode is not None and mode.agent_instance_id:
        return  # Runner subagent role emits usage once, at SessionEnd.
    agent_instance_id, source = _resolve_usage_scope(ctx)
    usage.emit_usage_records(ctx, ctx.transcript_path, agent_instance_id, source)


def resolve_call_id(ctx: "core.HookContext") -> str:
    """Return harness-native tool_use_id when present, else a suffixed fallback."""
    native = ctx.field("tool_use_id")
    if native:
        return native
    return runstate.fallback_call_id(ctx.field("tool_name"))


def capture_dispatch(ctx: "core.HookContext") -> None:
    """Record a subagent dispatch from a PreToolUse payload.

    When tool_name is a dispatch tool and tool_input.prompt carries an agent
    instance id, adopts the prompt's run id (if any) and queues one pending
    dispatch for ctx.session_id. Writes no event and reads no transcript.
    Both 'Agent' and 'Task' are accepted because Claude Code may dispatch
    subagents under either name depending on the harness version.
    Never raises.
    """
    try:
        if ctx.field("tool_name") not in DISPATCH_TOOL_NAMES:
            return
        tool_input = ctx.field("tool_input")
        if not isinstance(tool_input, dict):
            return
        prompt = tool_input.get("prompt") or None
        if not prompt:
            return
        agent_instance_id = runstate.extract_instance_id(prompt)
        if not agent_instance_id:
            return
        dispatch_type = tool_input.get("subagent_type")
        if not (isinstance(dispatch_type, str) and dispatch_type):
            dispatch_type = None
        extracted_run_id = runstate.extract_run_id(prompt)
        try:
            runstate.adopt_run_id(ctx, extracted_run_id)
        except Exception as exc:
            # A failed binding write must not prevent the queue write.
            core.debug_log("capture_dispatch: run adoption failed", exc)
        runstate.put_pending_dispatch(
            ctx.paths,
            ctx.session_id,
            agent_instance_id,
            extracted_run_id,
            prompt=prompt,
            agent_type=dispatch_type,
        )
    except Exception as exc:
        core.debug_log("capture_dispatch failed", exc)


def handle_pre_tool_use(ctx: "core.HookContext") -> None:
    """Emit tool_call_start, then usage records, for every tool (Agent and
    Task included). Does not adopt a run id and does not queue a dispatch:
    the synchronous capture-only invocation (capture_dispatch) owns that.
    """
    event = core.build_event(
        "tool_call_start", ctx,
        call_id=resolve_call_id(ctx),
        tool_name=ctx.field("tool_name"),
        tool_input=ctx.field("tool_input"),
    )
    core.append_event(resolve_destination(ctx), event)

    # Deliberate reversal of the "no transcript reads" property on this
    # high-frequency path, gated by tool_capture_enabled() (design decision,
    # not incidental). Runs after tool_call_start so an emission failure
    # can never suppress it.
    _emit_tool_usage_records(ctx)


def _output_text(ctx: "core.HookContext"):
    """Return the tool's output from the payload's tool_response, or None.

    Strings are kept as-is; structured values (objects, content-block lists)
    are serialised to JSON text so nested empty members survive the event
    builder's pruning unchanged.
    """
    value = ctx.field("tool_response")
    if value is None or isinstance(value, str):
        return value
    try:
        return json.dumps(value, ensure_ascii=False)
    except (TypeError, ValueError):
        return str(value)


def handle_post_tool_use(ctx: "core.HookContext") -> None:
    """Emit tool_call_end with status 'success'."""
    event = core.build_event(
        "tool_call_end", ctx,
        call_id=resolve_call_id(ctx),
        status="success",
        tool_name=ctx.field("tool_name"),
        tool_output=_output_text(ctx),
    )
    core.append_event(resolve_destination(ctx), event)
    _emit_tool_usage_records(ctx)
    if ctx.field("tool_name") == HANDBACK_TOOL_NAME:
        invocation.complete_from_handback(ctx)


def handle_post_tool_use_failure(ctx: "core.HookContext") -> None:
    """Emit tool_call_end with status 'error'."""
    tool_output = _output_text(ctx)
    error = ctx.field("error") or (str(tool_output) if tool_output is not None else None)
    event = core.build_event(
        "tool_call_end", ctx,
        call_id=resolve_call_id(ctx),
        status="error",
        tool_name=ctx.field("tool_name"),
        tool_output=tool_output,
        error=error,
    )
    core.append_event(resolve_destination(ctx), event)
    _emit_tool_usage_records(ctx)


HANDLERS = {
    "PreToolUse": handle_pre_tool_use,
    "PostToolUse": handle_post_tool_use,
    "PostToolUseFailure": handle_post_tool_use_failure,
}
