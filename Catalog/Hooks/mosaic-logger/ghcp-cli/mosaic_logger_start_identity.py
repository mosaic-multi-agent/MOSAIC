"""mosaic_logger_start_identity.py - decides the invocation identity of a subagentStart.

Rules, first that applies wins: continue an existing invocation of the harness agent id;
the type-matched pending dispatch; an explicit agent_instance_id line in the task message;
the unique fallback id. Only the second rule consumes a queue entry.

The started agent's type is agentType, else agentName.
"""

import collections
import re

import mosaic_logger_core as core
import mosaic_logger_pending_dispatch as pending
import mosaic_logger_runstate as runstate

SOURCE_EXISTING = "existing"
SOURCE_DISPATCH = "dispatch"
SOURCE_TASK_MESSAGE = "task_message"
SOURCE_FALLBACK = "fallback"

# queue_run_id is the run bucket the dispatch queue (and its spawn call ids) live under.
StartIdentity = collections.namedtuple(
    "StartIdentity", ["agent_instance_id", "run_id", "prompt", "source", "queue_run_id"])

_PREFIX_UNSAFE_RE = re.compile(r"[#\s]")


def started_agent_type(ctx: "core.HookContext") -> "str | None":
    """The started agent's type (agentType, else agentName), or None when both are empty."""
    for value in (ctx.agent_type, ctx.field("agentName")):
        if isinstance(value, str) and value.strip():
            return value
    return None


def _task_message(ctx: "core.HookContext") -> "str | None":
    value = ctx.field("agentPrompt")
    return value if isinstance(value, str) and value else None


def _fallback_prefix(agent_type: "str | None") -> "str | None":
    if not agent_type:
        return None
    return _PREFIX_UNSAFE_RE.sub("_", core.sanitize_component(agent_type))


def _existing(ctx: "core.HookContext", queue_run_id: str) -> "StartIdentity | None":
    """The invocation already mapped for this agent id, looked up under the run the agent
    is associated with, else under the effective run."""
    runs = [runstate.resolve_run_for_agent(ctx.paths, ctx.agent_id), queue_run_id]
    for run_id in runs:
        if not run_id:
            continue
        mapping = runstate.get_agent_mapping(ctx.paths, run_id, ctx.agent_id)
        instance = mapping.get("agent_instance_id") if mapping else None
        if isinstance(instance, str) and instance:
            return StartIdentity(instance, run_id, None, SOURCE_EXISTING, queue_run_id)
    return None


def _resolve(ctx: "core.HookContext") -> "StartIdentity":
    queue_run_id = core.effective_run_id(ctx)
    existing = _existing(ctx, queue_run_id)
    if existing is not None:
        return existing
    agent_type = started_agent_type(ctx)
    dispatch = pending.take_pending_dispatch(
        ctx.paths, queue_run_id, ctx.session_id, agent_type)
    message = _task_message(ctx)
    if dispatch is not None:
        if dispatch.run_id and not ctx.run_id:
            ctx.run_id = dispatch.run_id
        prompt = dispatch.prompt if dispatch.prompt is not None else message
        return StartIdentity(dispatch.agent_instance_id, core.effective_run_id(ctx),
                             prompt, SOURCE_DISPATCH, queue_run_id)
    explicit = runstate.extract_instance_id(message)
    if explicit:
        return StartIdentity(explicit, core.effective_run_id(ctx), message,
                             SOURCE_TASK_MESSAGE, queue_run_id)
    return StartIdentity(runstate.fallback_instance_id(_fallback_prefix(agent_type)),
                         core.effective_run_id(ctx), message, SOURCE_FALLBACK, queue_run_id)


def resolve_start_identity(ctx: "core.HookContext") -> "StartIdentity":
    """Return the identity for this start. Never raises; a failure yields a fallback id."""
    try:
        return _resolve(ctx)
    except Exception as exc:
        core.debug_log("start-identity: resolution failed, using fallback id", exc)
        run_id = core.effective_run_id(ctx)
        return StartIdentity(runstate.fallback_instance_id(None), run_id, None,
                             SOURCE_FALLBACK, run_id)
