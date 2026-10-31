"""Attribution rules and placement of calls. A call is placed only by hard keys, never by time window.

This module mirrors Tools/Common/otelingest/ (attribute.go, place.go, pending.go inScope).
Standard library only; deployed flat next to the adapter modules.
"""

from mosaic_logger_otel_contract import Diagnostic
from mosaic_logger_otel_parse import KIND_EXECUTE_TOOL

DEST_ORCHESTRATOR = "orchestrator"
DEST_UNATTRIBUTED = "unattributed"
DEST_INVOCATION = "invocation/"
MAX_OWNER_DEPTH = 8


def sole_invocation(ids):
    """The invocation stream for a key carried by exactly one invocation; a shared key is ambiguous."""
    if len(ids) == 1:
        return DEST_INVOCATION + ids[0]
    return DEST_UNATTRIBUTED


class Resolver:
    """Places calls using the run's keys and the span links known so far."""

    def __init__(self, keys, links):
        self.keys = keys
        self.links = links
        self.by_call = {}
        for link in links.values():
            if link.get("kind") == KIND_EXECUTE_TOOL and link.get("tool_call_id"):
                self.by_call[link["tool_call_id"]] = link

    def place(self, call):
        """(destination, True) for a call tied to the run; (None, False) when the keys needed to
        decide have not arrived yet and the call stays pending."""
        keys = self.keys
        spawn_ids = keys.inv_by_spawn.get(call.chat_session_id) if call.chat_session_id else None
        if spawn_ids:
            return sole_invocation(spawn_ids), True
        conversation = keys.conversation(call)
        if conversation in keys.orch_sessions:
            return self._owner(call.chat_session_id, 0)
        session_ids = keys.inv_by_session.get(conversation)
        if session_ids:
            return sole_invocation(session_ids), True
        return DEST_UNATTRIBUTED, True

    def _owner(self, chat_session, depth):
        """The VS Code chat session a call ran in: the orchestrator session, a spawn_call_id, or the
        id of an execution_subagent tool call whose own session says who ran the tool."""
        keys = self.keys
        if chat_session == "" or chat_session in keys.orch_sessions:
            return DEST_ORCHESTRATOR, True
        spawn_ids = keys.inv_by_spawn.get(chat_session)
        if spawn_ids:
            return sole_invocation(spawn_ids), True
        tool = self.by_call.get(chat_session)
        if tool is None:
            return None, False
        if tool.get("tool_name") == "execution_subagent" and depth < MAX_OWNER_DEPTH:
            return self._owner(tool.get("chat_session_id", ""), depth + 1)
        return DEST_UNATTRIBUTED, True


def in_scope(scope, call):
    """Whether a final ingest with the given scope may finalise the call: an empty scope covers every
    conversation, otherwise the call's conversation or parent chat session must be listed."""
    if not scope:
        return True
    return any(s and (s == call.conversation_id or s == call.parent_chat_session_id) for s in scope)


def place_calls(final, scope, state, resolver, writer, calls, result):
    """Place every call it can, in file order, and keep the rest pending. A call whose record_id was
    already placed is not written again. A final ingest sends pending calls of its scope to the
    unattributed stream."""
    timed_out = 0
    pending = []
    keys = resolver.keys
    for call in calls:
        if call.record_id in state.placements:
            continue
        if not keys.tied(call.conversation_id, call.chat_session_id, call.parent_chat_session_id):
            continue
        dest, ok = resolver.place(call)
        finalized = False
        if not ok and final and in_scope(scope, call):
            dest, ok, finalized = DEST_UNATTRIBUTED, True, True
        if not ok:
            pending.append(call)
            continue
        if not writer.write(call, dest):
            # The append lock was not obtained in time: the call is not placed and is retried.
            pending.append(call)
            timed_out += 1
            continue
        state.placements[call.record_id] = dest
        result.records_written += 1
        if finalized:
            result.finalized += 1
    state.pending = pending
    result.pending = len(pending)
    result.diagnostics.extend(placement_diagnostics(result.finalized))
    if timed_out > 0:
        result.diagnostics.append(Diagnostic(
            "append_timed_out", "lines not written because the append lock wait limit was reached",
            counts={"lines": timed_out}))


def placement_diagnostics(finalized):
    """The pending calls a final ingest placed on the unattributed stream."""
    out = []
    if finalized > 0:
        out.append(Diagnostic("pending_finalized", "pending spans were placed on the unattributed stream",
                              counts={"records": finalized}))
    return out
