"""mosaic_logger_start_identity.py - decides the invocation identity of a SubagentStart.

Rules, first that applies wins: continue an existing invocation of the harness agent id;
the type-matched pending dispatch; an explicit agent_instance_id line in the task message;
the unique fallback id. Only the second rule consumes a queue entry.
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

StartIdentity = collections.namedtuple(
    "StartIdentity", ["agent_instance_id", "run_id", "prompt", "source"])

_PREFIX_UNSAFE_RE = re.compile(r"[#\s]")


def started_agent_type(ctx: "core.HookContext") -> "str | None":
    """The started agent's type, or None when empty or absent."""
    try:
        value = ctx.agent_type
        if isinstance(value, str) and value.strip():
            return value
    except Exception:
        pass
    return None


def _task_message(ctx: "core.HookContext") -> "str | None":
    value = ctx.field("agent_prompt")
    return value if isinstance(value, str) and value else None


def _fallback_prefix(agent_type: "str | None") -> "str | None":
    if not agent_type:
        return None
    return _PREFIX_UNSAFE_RE.sub("_", core.sanitize_component(agent_type))


def _existing(ctx: "core.HookContext") -> "StartIdentity | None":
    run_id = core.effective_run_id(ctx)
    mapping = runstate.get_agent_mapping(ctx.paths, run_id, ctx.agent_id)
    instance = mapping.get("agent_instance_id") if mapping else None
    if isinstance(instance, str) and instance:
        return StartIdentity(instance, run_id, None, SOURCE_EXISTING)
    return None


def _resolve(ctx: "core.HookContext") -> "StartIdentity":
    existing = _existing(ctx)
    if existing is not None:
        return existing
    agent_type = started_agent_type(ctx)
    dispatch = pending.take_pending_dispatch(ctx.paths, ctx.session_id, agent_type)
    message = _task_message(ctx)
    if dispatch is not None:
        if dispatch.run_id and not ctx.run_id:
            runstate.adopt_run_id(ctx, dispatch.run_id)
        prompt = dispatch.prompt if dispatch.prompt is not None else message
        return StartIdentity(dispatch.agent_instance_id, core.effective_run_id(ctx),
                             prompt, SOURCE_DISPATCH)
    explicit = runstate.extract_instance_id(message)
    if explicit:
        return StartIdentity(explicit, core.effective_run_id(ctx), message, SOURCE_TASK_MESSAGE)
    return StartIdentity(runstate.fallback_instance_id(_fallback_prefix(agent_type)),
                         core.effective_run_id(ctx), message, SOURCE_FALLBACK)


def resolve_start_identity(ctx: "core.HookContext") -> "StartIdentity":
    """Return the identity for this start. Never raises; a failure yields a fallback id."""
    try:
        return _resolve(ctx)
    except Exception as exc:
        core.debug_log("start-identity: resolution failed, using fallback id", exc)
        return StartIdentity(runstate.fallback_instance_id(None),
                             core.effective_run_id(ctx), None, SOURCE_FALLBACK)
