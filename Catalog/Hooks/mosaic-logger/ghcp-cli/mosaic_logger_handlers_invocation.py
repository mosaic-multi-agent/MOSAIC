"""mosaic_logger_handlers_invocation.py -- Invocation-scoped event handlers.

Handles: subagentStart, subagentStop. Writes invocation_start, invocation_end,
boundary turn events, artifact files, and transcript exports for each subagent.
"""

import os
import re

import mosaic_logger_core as core
import mosaic_logger_runstate as runstate
import mosaic_logger_spawn_key as spawn_key
import mosaic_logger_start_identity as start_identity
import mosaic_logger_export as export
import mosaic_logger_transcript as transcript
import mosaic_logger_artifacts as artifacts
import mosaic_logger_transcript_logging as transcript_logging


# ---------------------------------------------------------------------------
# Closed enum for status_code
# ---------------------------------------------------------------------------

_VALID_STATUS_CODES = frozenset({
    "SUCCESS",
    "COMPLETED_NEEDS_ACTION",
    "PARTIALLY_DONE",
    "NEEDS_CLARIFICATION",
    "CAPABILITY_EXCEEDED",
    "BLOCKED",
})

# Match a structured status_code key followed by a quoted enum value.
_STATUS_CODE_RE = re.compile(
    r'(?<![A-Za-z0-9_])status_code\s*["\']?\s*[:=]\s*["\']([A-Z_]+)["\']'
)


# ---------------------------------------------------------------------------
# Public helpers
# ---------------------------------------------------------------------------

def extract_status_code(message: "str | None") -> "str | None":
    """Extract MOSAIC status code from subagent's final response.
    Conservative: only structured key-value with quoted value from closed enum.
    Last occurrence wins. Returns None on ambiguity. Never raises."""
    try:
        if not message:
            return None

        matches = _STATUS_CODE_RE.findall(message)
        if not matches:
            return None

        last_value = matches[-1]
        if last_value in _VALID_STATUS_CODES:
            return last_value
        return None

    except Exception:
        return None


# ---------------------------------------------------------------------------
# Handlers
# ---------------------------------------------------------------------------

def _write_invocation_start(ctx: "core.HookContext", identity) -> None:
    """Write the mapping, agent-to-run association, invocation_start, the initial user
    turn and 01_input.md for a newly resolved invocation."""
    run_id = identity.run_id
    agent_instance_id = identity.agent_instance_id
    agent_prompt = identity.prompt
    agent_type = start_identity.started_agent_type(ctx)

    # Persist mapping FIRST -- before any event write
    runstate.put_agent_mapping(
        ctx.paths, run_id, ctx.agent_id, agent_instance_id, agent_type
    )
    runstate.put_agent_run(ctx.paths, ctx.agent_id, run_id)

    sink = ctx.paths.invocation_events(run_id, agent_instance_id)

    # invocation_start (spawn_call_id only when a payload or a remembered dispatch supplied one)
    spawn_call_id = spawn_key.spawn_call_id_from(ctx) or spawn_key.take_spawn_call(
        ctx.paths, identity.queue_run_id, ctx.session_id, agent_instance_id)
    event = core.build_event(
        "invocation_start", ctx,
        agent_instance_id=agent_instance_id,
        agent_type=agent_type,
        prompt=agent_prompt,
        spawn_call_id=spawn_call_id,
    )
    core.append_event(sink, event)

    # Initial user turn (only when agent_prompt is present)
    if agent_prompt is not None:
        turn = core.build_event("turn", ctx, role="user", content=agent_prompt)
        core.append_event(sink, turn)

    artifacts.write_artifact(
        ctx.paths.invocation_input(run_id, agent_instance_id),
        artifacts.render_input(ctx, agent_instance_id, agent_prompt))


def handle_subagent_start(ctx: "core.HookContext") -> None:
    """Handle subagentStart: resolve the invocation identity, register the agent mapping,
    emit invocation_start and input turn, write 01_input.md.

    The identity comes from start_identity.resolve_start_identity: an existing
    invocation of this agentId (a re-fired start writes nothing into its folder and
    keeps its mapping, agent-to-run association and spawn call id), else the
    type-matched pending dispatch, else an explicit agent_instance_id line in the task
    message, else a unique fallback id. Usage is logged for every start, into the
    stream the mapping names.
    Never raises."""
    if not ctx.agent_id:
        return

    identity = start_identity.resolve_start_identity(ctx)
    if identity.source == start_identity.SOURCE_EXISTING:
        core.debug_log(
            f"subagent-start: continued existing invocation for agent_id={ctx.agent_id!r} "
            f"agent_instance_id={identity.agent_instance_id!r}")
    else:
        _write_invocation_start(ctx, identity)

    # Refresh orchestrator raw transcript (best-effort)
    export.export_transcript(
        ctx.transcript_path,
        ctx.paths.orchestrator_raw(identity.run_id, ctx.session_id),
        "transcript_path",
        owner_session_id=ctx.session_id,
    )

    # Log the transcript's usage; the mapping places this agent's calls
    transcript_logging.log_transcript_usage(ctx)


COMPLETION_DIRNAME = ".completion"


def _claim_completion(ctx: "core.HookContext", run_id: str) -> bool:
    """Atomically create the completion marker of this agent id; True when this stop
    owns the invocation's end. When the marker cannot be attempted the stop proceeds
    (a possible duplicate is preferred to a lost end)."""
    try:
        path = (ctx.paths.run_root(run_id) / COMPLETION_DIRNAME
                / f"{core.sanitize_component(ctx.agent_id)}.json")
        path.parent.mkdir(parents=True, exist_ok=True)
        os.close(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY))
        return True
    except FileExistsError:
        return False
    except Exception as exc:
        core.debug_log("subagent-stop: completion claim failed", exc)
        return True


def _refresh_exports(ctx: "core.HookContext", run_id: str, agent_instance_id: str,
                     subagent_transcript_path: "str | None") -> None:
    export.export_transcript(
        subagent_transcript_path,
        ctx.paths.invocation_raw(run_id, agent_instance_id),
        "transcriptPath",
    )
    export.export_transcript(
        ctx.transcript_path,
        ctx.paths.orchestrator_raw(run_id, ctx.session_id),
        "transcript_path",
        owner_session_id=ctx.session_id,
    )


def handle_subagent_stop(ctx: "core.HookContext") -> None:
    """Handle subagentStop: emit invocation_end with status_code extraction,
    write 02_output.md, export agent transcript.
    Reads response (camelCase), agentId (camelCase), transcriptPath from payload.
    Guards against unmapped SubagentStop (no-op for unmapped_* invocation IDs).
    Never raises."""
    if not ctx.agent_id:
        return

    run_id = core.effective_run_id(ctx)

    # 1. Resolve agent_instance_id via mapping
    agent_instance_id = runstate.resolve_invocation_id(
        ctx.paths, run_id, ctx.agent_id
    )

    # Guard: skip unmapped subagentStop firings
    if agent_instance_id.startswith("unmapped_"):
        return

    sink = ctx.paths.invocation_events(run_id, agent_instance_id)

    # Log the transcript's usage first so the stream's usage precedes invocation_end
    transcript_logging.log_transcript_usage(ctx)

    subagent_transcript_path = ctx.field("transcriptPath")

    # A later stop of an invocation that already ended writes no second end, final turn
    # or output; its usage and transcript export still run.
    if not _claim_completion(ctx, run_id):
        core.debug_log(
            f"subagent-stop: invocation already ended for agent_id={ctx.agent_id!r}; "
            f"skipping invocation_end, final turn and 02_output.md")
        _refresh_exports(ctx, run_id, agent_instance_id, subagent_transcript_path)
        return

    # 2. Read subagent transcript facts
    facts = transcript.read_last_assistant_facts(subagent_transcript_path)

    # GHCP CLI uses 'response' field (not 'last_assistant_message')
    response = ctx.field("response")
    status_code = extract_status_code(response)

    # 3. Emit invocation_end
    event = core.build_event(
        "invocation_end", ctx,
        agent_instance_id=agent_instance_id,
        status_code=status_code,
        response=response,
        model=facts.model,
        token_usage=facts.token_usage,
    )
    core.append_event(sink, event)

    # 4. Emit final assistant turn (only when response is present)
    if response is not None:
        turn = core.build_event(
            "turn", ctx,
            role="assistant",
            content=response,
            model=facts.model,
            token_usage=facts.token_usage,
        )
        core.append_event(sink, turn)

    # 5. Write 02_output.md
    output_text = artifacts.render_output(ctx, agent_instance_id, response, status_code, facts)
    artifacts.write_artifact(ctx.paths.invocation_output(run_id, agent_instance_id), output_text)

    # 6-7. Export subagent transcript and refresh the orchestrator raw transcript
    _refresh_exports(ctx, run_id, agent_instance_id, subagent_transcript_path)


HANDLERS: dict = {
    "subagentStart": handle_subagent_start,
    "subagentStop": handle_subagent_stop,
}
