"""mosaic_logger_handlers_invocation.py — Invocation-scoped event handlers.

Handles: SubagentStart, SubagentStop. Writes invocation_start, invocation_end,
boundary turn events, artifact files, and transcript exports for each subagent.

Key vscode-ghcp behaviour:
- Subagent correlation is keyed on the spawning tool-call ID: SubagentStart and
  SubagentStop carry it as agent_id (the `toolu_...` value, possibly with a
  `__vscode-<ms>` suffix, which is stripped). agent_name is a fallback key only.
- SubagentStop carries no response text and no run ID. It only marks the subagent as
  stopped; the invocation_end is written once, by whichever of the orchestrator's
  PostToolUse for the spawning tool or the orchestrator's Stop sweep claims it first.
- resolve_agent_key is the single seam for the key.
"""

import copy
import json
import re

import mosaic_logger_core as core
import mosaic_logger_diagnostics as diagnostics
import mosaic_logger_subagent_state as subagent_state
import mosaic_logger_runstate as runstate
import mosaic_logger_export as export
import mosaic_logger_transcript as transcript
import mosaic_logger_artifacts as artifacts


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

# Match a structured status_code key followed by an enum value (quoted or unquoted).
# Matches both JSON style ("status_code": "SUCCESS") and YAML style (status_code: SUCCESS).
# Free prose mentioning a status name without the key prefix still won't match.
_STATUS_CODE_RE = re.compile(
    r'(?<![A-Za-z0-9_])status_code\s*["\']?\s*[:=]\s*["\']?([A-Z_]+)["\']?'
)


# ---------------------------------------------------------------------------
# Public helpers
# ---------------------------------------------------------------------------

def extract_status_code(message: "str | None") -> "str | None":
    """Extract the MOSAIC status code from a subagent's final response.

    Matching is conservative: only a structured key-value occurrence (status_code
    key followed by a quoted value from the closed enum) is accepted. Free prose
    mentioning a status name is not a match. When multiple structured occurrences
    exist, the LAST one wins. Returns None on any ambiguity, any value outside the
    closed enum, or any missing/empty/None input. Never raises.
    """
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


def resolve_agent_key(ctx: "core.HookContext") -> "str | None":
    """Return the correlation key for this firing.

    The normalised agent_id (the spawning tool-call ID without any `__vscode-<ms>`
    suffix); agent_name only when agent_id is absent; None when both are absent.
    SubagentStart and SubagentStop both route through this function so start and
    stop cannot disagree about which key was used.
    """
    from_id = subagent_state.normalize_call_id(ctx.agent_id)
    if from_id is not None:
        return from_id
    return ctx.field("agent_name")


def resolve_agent_type(ctx: "core.HookContext") -> "str | None":
    """Return the agent type, falling back to agent_name when agent_type is absent.

    The schema defines agent_type as the bare agent name; VS Code's agent_name
    carries exactly that value. Without this fallback, invocation_start would
    systematically omit a field that invocation_end populates for the same
    invocation. This is a semantic mapping between equivalent fields, not a
    fabricated value.
    """
    return ctx.agent_type or ctx.field("agent_name")


# ---------------------------------------------------------------------------
# Handlers
# ---------------------------------------------------------------------------

def handle_subagent_start(ctx: "core.HookContext") -> None:
    """Handle SubagentStart: persist mapping, emit invocation_start and user turn,
    write 01_input.md, and refresh 00_orchestrator_session.raw.

    The correlation key is the spawning tool-call ID (agent_id); the same value,
    suffix stripped, is written on invocation_start as spawn_call_id. The agent map
    entry and the workspace-level spawn index entry are persisted BEFORE any event
    write to minimize the correlation window for concurrent firings.

    Never raises.
    """
    try:
        # Step 1: resolve correlation key; return early only when truly uncorrelatable.
        agent_key = resolve_agent_key(ctx)
        if agent_key is None:
            return

        # Step 2: pop pending dispatch.
        dispatch = runstate.pop_pending_dispatch(
            ctx.paths, core.effective_run_id(ctx), ctx.session_id
        )

        # Step 3: optionally set ctx.run_id from the dispatch when not yet set.
        if dispatch and dispatch.get("run_id") and not ctx.run_id:
            ctx.run_id = dispatch["run_id"]

        # Step 4: compute effective run_id for all downstream routing.
        run_id = core.effective_run_id(ctx)

        # Step 5: resolve agent_prompt and agent_instance_id.
        dispatch_prompt = dispatch.get("prompt") if dispatch else None
        agent_prompt = dispatch_prompt

        if dispatch:
            agent_instance_id = dispatch["agent_instance_id"]
        else:
            extracted = runstate.extract_instance_id(agent_prompt)
            agent_instance_id = extracted if extracted else "unknown-agent"

        # Step 6: persist the agent map entry FIRST — before any event write.
        runstate.put_agent_mapping(
            ctx.paths, run_id, agent_key, agent_instance_id, resolve_agent_type(ctx)
        )
        subagent_state.put_spawn_entry(
            ctx.paths, agent_key, run_id, agent_instance_id, ctx.session_id,
            resolve_agent_type(ctx),
        )

        sink = ctx.paths.invocation_events(run_id, agent_instance_id)

        # Step 7: emit invocation_start.
        event = core.build_event(
            "invocation_start", ctx,
            agent_instance_id=agent_instance_id,
            agent_type=resolve_agent_type(ctx),
            prompt=agent_prompt,
            spawn_call_id=subagent_state.normalize_call_id(ctx.agent_id),
        )
        core.append_event(sink, event)

        # Step 8: emit initial user turn when prompt is available.
        if agent_prompt is not None:
            turn = core.build_event("turn", ctx, role="user", content=agent_prompt)
            core.append_event(sink, turn)

        # Step 9: write 01_input.md.
        input_text = artifacts.render_input(ctx, agent_instance_id, agent_prompt)
        artifacts.write_artifact(
            ctx.paths.invocation_input(run_id, agent_instance_id), input_text
        )

        # Step 10: refresh 00_orchestrator_session.raw (best-effort).
        export.export_transcript(
            ctx.transcript_path,
            ctx.paths.orchestrator_raw(run_id, ctx.session_id),
            "transcript_path",
        )

    except Exception as exc:
        core.debug_log("handle_subagent_start failed", exc)


def _report_unmapped(ctx: "core.HookContext", kind: str, message: str,
                     key: "str | None") -> None:
    diagnostics.report(
        ctx.paths.root, kind, message, harness=core.HARNESS,
        session_id=ctx.session_id, ids=[key] if key else None,
    )


def response_text(value) -> "str | None":
    """The text form of a tool_response: a non-empty string verbatim, a non-empty
    dict/list as JSON text, otherwise None."""
    if isinstance(value, str):
        return value or None
    if isinstance(value, (dict, list)) and value:
        return json.dumps(value, ensure_ascii=False)
    return None


def complete_invocation(ctx: "core.HookContext", entry: dict,
                        response: "str | None") -> None:
    """Write the one invocation_end and 02_output.md of the indexed subagent.

    The caller has already won the completion claim. No model, provider or token
    usage is written: the payload's transcript belongs to the orchestrator session.
    Never raises.
    """
    try:
        run_id = entry.get("run_id") or "unknown-run"
        instance_id = entry["agent_instance_id"]
        end_ctx = copy.copy(ctx)
        end_ctx.run_id = None if run_id == "unknown-run" else run_id
        end_ctx.agent_type = entry.get("agent_type")
        status_code = extract_status_code(response)

        event = core.build_event(
            "invocation_end", end_ctx,
            agent_instance_id=instance_id,
            status_code=status_code,
            response=response,
        )
        core.append_event(ctx.paths.invocation_events(run_id, instance_id), event)

        output_text = artifacts.render_output(
            end_ctx, instance_id, response, status_code, None
        )
        artifacts.write_artifact(ctx.paths.invocation_output(run_id, instance_id), output_text)
    except Exception as exc:
        core.debug_log("complete_invocation failed", exc)


def complete_from_dispatch_result(ctx: "core.HookContext", tool_use_id,
                                  tool_response) -> None:
    """Complete the subagent spawned by the tool call tool_use_id (PostToolUse path).

    A call ID absent from the spawn index is reported as a diagnostic and writes no
    invocation event; a lost completion claim writes nothing. Never raises.
    """
    try:
        key = subagent_state.normalize_call_id(tool_use_id)
        entry = subagent_state.get_spawn_entry(ctx.paths, key) if key else None
        if entry is None:
            _report_unmapped(
                ctx, "unmapped_subagent_completion",
                "Spawning tool call finished but no started subagent matches its call ID",
                key,
            )
            return
        if subagent_state.claim_completion(ctx.paths, key):
            complete_invocation(ctx, entry, response_text(tool_response))
    except Exception as exc:
        core.debug_log("complete_from_dispatch_result failed", exc)


def sweep_stopped_subagents(ctx: "core.HookContext") -> None:
    """At the orchestrator's Stop, end every subagent of this session that stopped
    but never had its spawning PostToolUse completed. Never raises."""
    try:
        if not ctx.session_id:
            return
        for entry in subagent_state.stopped_unclaimed(ctx.paths, ctx.session_id):
            if subagent_state.claim_completion(ctx.paths, entry["key"]):
                complete_invocation(ctx, entry, None)
    except Exception as exc:
        core.debug_log("sweep_stopped_subagents failed", exc)


def handle_subagent_stop(ctx: "core.HookContext") -> None:
    """Handle SubagentStop: mark the subagent as stopped.

    VS Code's SubagentStop carries no response text and no run ID, so it writes no
    event. A firing whose key is absent or not in the spawn index is reported as an
    unmapped_subagent_stop diagnostic (default-visible) and writes nothing else.

    Never raises.
    """
    try:
        key = resolve_agent_key(ctx)
        if key is None or subagent_state.get_spawn_entry(ctx.paths, key) is None:
            _report_unmapped(
                ctx, "unmapped_subagent_stop",
                "SubagentStop did not match any started subagent", key,
            )
            return
        subagent_state.mark_stopped(ctx.paths, key)
    except Exception as exc:
        core.debug_log("handle_subagent_stop failed", exc)


# ---------------------------------------------------------------------------
# Handler registry
# ---------------------------------------------------------------------------

HANDLERS = {
    "SubagentStart": handle_subagent_start,
    "SubagentStop":  handle_subagent_stop,
}
