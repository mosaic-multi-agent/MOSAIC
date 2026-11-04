"""mosaic_logger_transcript_events.py - Log events built from transcript usage calls and compactions.

Each event carries its item's own transcript timestamp and the run of its placement, never the
hook firing's. Pure builders: nothing is written here.
"""

import types
from dataclasses import dataclass
from typing import Optional

import mosaic_logger_core as core

SOURCE_ORCHESTRATOR = "orchestrator_transcript"
SOURCE_AGENT = "agent_transcript"


@dataclass
class Placement:
    """Where an item belongs: the run, the stream and, on invocation streams, the invocation."""
    run_id: str
    stream: object
    instance_id: Optional[str]
    source: str


def _stamp(ctx, placement: Placement, timestamp: str):
    return types.SimpleNamespace(session_id=ctx.session_id, run_id=placement.run_id, timestamp=timestamp)


def _usage_fields(placement: Placement, record_id: str, record_index: int, item) -> dict:
    return {
        "record_id": record_id,
        "record_index": record_index,
        "source": placement.source,
        "agent_instance_id": placement.instance_id,
        "model": item.model,
        "provider": item.provider,
        "token_usage": item.token_usage,
    }


def usage_record_event(ctx, placement: Placement, call) -> dict:
    """The usage_record of one transcript call."""
    fields = _usage_fields(placement, call.record_id, call.record_index, call)
    fields["effort"] = call.effort
    fields["requested_tools"] = call.requested_tools
    event = core.build_event("usage_record", _stamp(ctx, placement, call.timestamp), **fields)
    if call.requested_tools == {}:
        event["requested_tools"] = {}  # "requested no tools" is a value; the builder prunes empty objects
    return event


def compaction_cost_event(ctx, placement: Placement, compaction) -> dict:
    """The usage_record for a compaction's own model call."""
    cost = compaction.usage
    fields = _usage_fields(placement, compaction.record_id, compaction.record_index, cost)
    fields["call_kind"] = "compaction"
    return core.build_event("usage_record", _stamp(ctx, placement, cost.timestamp), **fields)


def compaction_event(ctx, placement: Placement, compaction) -> dict:
    """The compaction event of an applied compaction."""
    return core.build_event(
        "compaction", _stamp(ctx, placement, compaction.completed_at),
        trigger=compaction.trigger,
        tokens_before=compaction.tokens_before,
        tokens_after=compaction.tokens_after,
    )
