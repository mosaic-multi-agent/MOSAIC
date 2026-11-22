"""mosaic_logger_start_identity.py - decides the invocation identity of a SubagentStart.

Rules, first that applies wins: the type-matched pending dispatch; the unique fallback id.
Only the first rule consumes a queue entry. (The VS Code SubagentStart payload carries no
task message, so there is no explicit-line source at start; the dispatch capture already
applies the explicit-line rule.)

The started agent's type is agent_type, else agent_name.
"""

import collections
import re

import mosaic_logger_core as core
import mosaic_logger_pending_dispatch as pending
import mosaic_logger_runstate as runstate

SOURCE_DISPATCH = "dispatch"
SOURCE_FALLBACK = "fallback"

StartIdentity = collections.namedtuple(
    "StartIdentity", ["agent_instance_id", "run_id", "prompt", "source"])

_PREFIX_UNSAFE_RE = re.compile(r"[#\s]")


def started_agent_type(ctx: "core.HookContext") -> "str | None":
    """The started agent's type (agent_type, else agent_name), or None when both are empty."""
    for value in (ctx.agent_type, ctx.field("agent_name")):
        if isinstance(value, str) and value.strip():
            return value
    return None


def _fallback_prefix(agent_type: "str | None") -> "str | None":
    if not agent_type:
        return None
    return _PREFIX_UNSAFE_RE.sub("_", core.sanitize_component(agent_type))


def _resolve(ctx: "core.HookContext") -> "StartIdentity":
    agent_type = started_agent_type(ctx)
    dispatch = pending.take_pending_dispatch(
        ctx.paths, core.effective_run_id(ctx), ctx.session_id, agent_type)
    if dispatch is not None:
        if dispatch.run_id and not ctx.run_id:
            ctx.run_id = dispatch.run_id
        return StartIdentity(dispatch.agent_instance_id, core.effective_run_id(ctx),
                             dispatch.prompt, SOURCE_DISPATCH)
    return StartIdentity(runstate.fallback_instance_id(_fallback_prefix(agent_type)),
                         core.effective_run_id(ctx), None, SOURCE_FALLBACK)


def resolve_start_identity(ctx: "core.HookContext") -> "StartIdentity":
    """Return the identity for this start. Never raises; a failure yields a fallback id."""
    try:
        return _resolve(ctx)
    except Exception as exc:
        core.debug_log("start-identity: resolution failed, using fallback id", exc)
        return StartIdentity(runstate.fallback_instance_id(None),
                             core.effective_run_id(ctx), None, SOURCE_FALLBACK)
