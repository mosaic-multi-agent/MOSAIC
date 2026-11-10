"""mosaic_logger_transcript_logging.py - Log GHCP CLI usage and compactions from the session transcript.

Read points call log_transcript_usage(ctx). It reads the whole session transcript, places each
usage call and compaction on the stream of the actor that made it, and appends what is not yet
logged. Placement uses hard keys only:
  - native mode: no agentId = the orchestrator; an agentId = the invocation the agent mapping
    (written at subagentStart) names. An item whose agentId has no mapping yet waits for a later read.
  - Runner subagent role: every item belongs to the invocation. Runner orchestrator role: every
    item belongs to the orchestrator.
Nothing falls back to a default invocation or to timestamps. A run-less session logs nothing yet.

Exactly once: the whole read runs under the session's state lock (see mosaic_logger_transcript_state),
and an item is marked logged only after every event it needs was appended.
"""

import json
from typing import Optional

import mosaic_logger_core as core
import mosaic_logger_diagnostics as diagnostics
import mosaic_logger_locked_append as locked_append
import mosaic_logger_runner_mode as runner_mode
import mosaic_logger_runstate as runstate
import mosaic_logger_transcript_events as events
import mosaic_logger_transcript_state as state_store
import mosaic_logger_transcript_usage as reader

ALARM_KIND = "transcript_format_changed"
FIELD_KIND = "transcript_field_unreadable"


def log_transcript_usage(ctx: "core.HookContext") -> None:
    """Log every not-yet-logged usage call and compaction of the session transcript. Never raises."""
    try:
        _log(ctx)
    except Exception as exc:
        core.debug_log("transcript logging failed", exc)


def _log(ctx) -> None:
    path = ctx.transcript_path if isinstance(ctx.transcript_path, str) else None
    state_file = state_store.state_path(ctx.paths, ctx.session_id, path)
    if state_file is None:
        return
    with locked_append.holding_lock(state_file, core.debug_log) as held:
        if not held:
            return
        state = state_store.LoggedState(state_file).load()
        try:
            path = path or state.transcript
            if path is None:
                return
            state.remember_transcript(path)
            _log_read(ctx, state, reader.read_transcript_usage(path))
        finally:
            state.close()


def _log_read(ctx, state, read) -> None:
    if not read.readable:
        return
    places = {}

    def place(agent_id):
        if agent_id not in places:
            places[agent_id] = _place(ctx, agent_id)
        return places[agent_id]

    for call in read.calls:
        _log_call(ctx, state, call, place(call.agent_id))
    for compaction in read.compactions:
        _log_compaction(ctx, state, compaction, place(compaction.agent_id))
    _report_alarm(ctx, state, read)
    _report_problems(ctx, state, read)


# ---------------------------------------------------------------------------
# Placement
# ---------------------------------------------------------------------------

def _place(ctx, agent_id: Optional[str]) -> "events.Placement | None":
    mode = ctx.runner_mode
    if mode is not None and mode.role == runner_mode.ROLE_SUBAGENT and mode.agent_instance_id:
        if not ctx.run_id:
            return None
        return events.Placement(ctx.run_id, ctx.paths.invocation_events(ctx.run_id, mode.agent_instance_id),
                                mode.agent_instance_id, events.SOURCE_AGENT)
    if agent_id is None or mode is not None:
        if not ctx.run_id:
            return None
        return events.Placement(ctx.run_id, ctx.paths.orchestrator_events(ctx.run_id), None,
                                events.SOURCE_ORCHESTRATOR)
    return _place_agent(ctx, agent_id)


def _place_agent(ctx, agent_id: str) -> "events.Placement | None":
    run_id = runstate.resolve_run_for_agent(ctx.paths, agent_id) or ctx.run_id
    if not run_id:
        return None
    mapping = runstate.get_agent_mapping(ctx.paths, run_id, agent_id)
    instance_id = mapping.get("agent_instance_id") if mapping else None
    if not instance_id:
        return None
    return events.Placement(run_id, ctx.paths.invocation_events(run_id, instance_id), instance_id,
                            events.SOURCE_ORCHESTRATOR)


# ---------------------------------------------------------------------------
# Writing
# ---------------------------------------------------------------------------

def _append(stream, event: dict) -> bool:
    """Append one event; True only when it was written."""
    try:
        line = (json.dumps(event, ensure_ascii=False) + "\n").encode("utf-8")
        return locked_append.locked_append(stream, line, _dropped(stream)) is True
    except Exception as exc:
        core.debug_log(f"transcript append failed for {stream}", exc)
        return False


def _dropped(stream):
    def report(message, exc=None):
        core.debug_log(f"transcript append dropped for {stream}: {message}", exc)
    return report


def _log_call(ctx, state, call, placement) -> None:
    key = "u:" + call.record_id
    if placement is None or call.token_usage is None or state.has(key):
        return  # a record without any readable token figure carries no usage to count
    if _append(placement.stream, events.usage_record_event(ctx, placement, call)):
        state.mark(key)


def _has_cost(compaction) -> bool:
    return compaction.usage is not None and compaction.usage.token_usage is not None


def _log_compaction(ctx, state, compaction, placement) -> None:
    if placement is None:
        return
    cost_key, event_key = "u:" + compaction.record_id, "e:" + compaction.record_id
    if _has_cost(compaction) and not state.has(cost_key):
        if not _append(placement.stream, events.compaction_cost_event(ctx, placement, compaction)):
            return  # the compaction event is never written without its cost
        state.mark(cost_key)
    if compaction.applied and not state.has(event_key):
        if _append(placement.stream, events.compaction_event(ctx, placement, compaction)):
            state.mark(event_key)


# ---------------------------------------------------------------------------
# Diagnostics
# ---------------------------------------------------------------------------

def _report_alarm(ctx, state, read) -> None:
    """Assistant turns but no readable usage: one entry per session and run, once the run is known."""
    if not read.alarm or not ctx.run_id:
        return
    key = "a:" + ctx.run_id
    if state.has(key):
        return
    written = diagnostics.report(
        ctx.paths.root, ALARM_KIND,
        "The session transcript has assistant turns but no readable usage records; "
        "its format may have changed and the run's cost is incomplete.",
        harness=core.HARNESS, run_id=ctx.run_id, session_id=ctx.session_id,
        counts={"assistant_turns": read.assistant_turns, "usage_records": 0})
    if written:
        state.mark(key)


def _report_problems(ctx, state, read) -> None:
    """One entry per session and unreadable transcript field; the first record that showed it is named."""
    for problem in read.problems:
        key = "f:" + problem.field
        if state.has(key):
            continue
        ids = [problem.field] + ([problem.record_id] if problem.record_id else [])
        written = diagnostics.report(
            ctx.paths.root, FIELD_KIND,
            "A transcript field could not be read; the record was logged without it.",
            harness=core.HARNESS, run_id=ctx.run_id, session_id=ctx.session_id, ids=ids)
        if written:
            state.mark(key)
