# Known Limitations: OpenCode V2 Adapter

This record lists the V1 (`../opencode/`) features that the V2 variant of
`mosaic-logger` cannot reproduce because V2 offers no signal for them, and the
payload shapes that were not observed. It is written so that future investigators
do not rediscover them. The transcript gaps (`04_session.raw`, the orchestrator
session transcript) are listed in the sections starting with "Transcripts".

---

## Resumed root sessions are not re-bound to their run

V1 recovered a resumed root's run from the Task inputs in its message history.
V2 has no verified history accessor, so a resumed root stays unbound (records go
to `unknown-run`) until its next MOSAIC dispatch binds it. Under the Runner the
run comes from `MOSAIC_RUN_ID` and is not affected.

## A child whose every parent signal is missed is logged as a root

A child is recognised by `session.created` (with `parentID`), by the launch
result of the dispatch, by the tool-progress event of the dispatch, or by the
synthetic wake-up it sends its parent. A child whose signals are all missed
(events delivered while the subscription was re-established are lost) is logged
as a root: its records go to the orchestrator stream, not to an invocation.

## The dispatching `subagent` call ends at launch

V1 closed the dispatching `task` call from its own signals when the subagent
finished. V2 fires the `subagent` `execute.after` hook at launch, with the child
still running, so `tool_call_end` of a dispatch reports the launch. The child's
completion is the `invocation_end` of its own stream. There is no V2 signal to
close the dispatch call at completion.

## Permission prompts: only asks are logged, shapes unobserved

`permission.evaluate` fires for `allow` and `ask` and never for an explicitly
configured `deny`, so a denied permission is invisible and nothing is logged for
it. An `allow` is not logged either: it needs no human. An `ask` writes one
`permission_prompt` notification. The resolution notification depends on the
ephemeral `permission.asked` and `permission.replied` events, whose payload
shapes (`id`, `source.id`, `requestID`, `reply`) come from source reading and
were not observed in the spike. When they differ, an ask stays a single
notification without a resolution. The edit permission's `filepath` and diff
metadata are not logged separately; the `edit` tool call carries the same
information.

## Compaction: shapes from source reading, never triggered in the spike

The `compaction` event and its `usage_record` (`call_kind: compaction`) are
written from `session.compaction.ended` (`reason`, `tokens`, `model`). That event
was not triggered in the spike, so the shape is unverified. A compaction that
reports no tokens writes only the `compaction` event; one whose event has no id
of its own writes no usage record. `tokens_before`, `tokens_after` and `phase`
are never written: V2 gives no context sizes. Started, delta and failed
compactions write nothing, and the `session.compaction` hook is only an ownership
signal.

## No cost

The log format has no cost field. V2's `cost` values are never read, recorded or
estimated; consumers price from tokens, model and provider.

## Token usage

- One `usage_record` per step (assistant message) from `session.step.ended`, or
  from `session.step.failed` when it carries tokens. The cumulative
  `session.usage.updated` event is ignored: it carries no location and would
  double count the steps.
- `reasoning` is added into `output_tokens` as the format requires and is also
  reported as `reasoning_tokens`.
- `call_kind` is written only for compactions. V2 gives no signal that tells a
  title or helper call apart from a main step, so `auxiliary` is never written.
- The record timestamp is the step's `started` time; the observation time is used
  when the step start was not seen.

## Reasoning text is not logged

`session.reasoning.ended` is not subscribed. Assistant turns carry the visible
text only.

## Runner mode

- The process's first root session is the Runner's invocation (subagent role) or
  consultation (orchestrator role). Any other root session of the process is not
  logged.
- The end records (`session_end` or `invocation_end`) are written once per
  process, at the first end point: a `stop` step, else a terminal execution event
  of the primary, held back while a child invocation is open.
- Every record is appended synchronously while the event or hook is processed,
  because the standalone private server exits when its stdin lease closes and
  pending asynchronous work is lost. This is verified with a Node child process
  that exits one macrotask after the final step; it has not been verified against
  a live standalone server.

## Unverified type shapes

Every V2 payload shape was taken from the spike probe logs or from source-reading
research, and nothing was type-checked against an installed `@opencode-ai` package.
All reads are defensive: a changed field name degrades to absent data, never to a
failure of the host.

## Events delivered while the subscription is down are lost

When the event iterator ends or throws, the plugin subscribes again no sooner than
one second later. Events delivered in between are not replayed.

## Transcripts, Runner mode

No transcript is written in either Runner role: there is no `04_session.raw` and
no session-scoped orchestrator transcript. V1 writes them in Runner mode.

## Transcripts, mechanism

V2 has no verified plugin-side accessor for a session's full history, so each
export is a separate `opencode session export <id> --standalone` process. It starts
its own private server, so a refresh takes seconds. It needs `opencode` on the PATH
of the process hosting the plugin, and that binary can be a different version from
the host. That private server may load the project's plugins, this logger
included; that instance owns no session and writes nothing. On Windows the command
can be an npm `.cmd` shim that only a shell starts, so an export runs there only
for a session id made of ASCII letters, digits, `_` and `-`.

## Transcripts, freshness

An export holds only settled messages, so a transcript can lag the event streams.
The orchestrator transcript's last refresh is at its latest root terminal event.

## Transcripts, loss

Exports that are waiting or running when the plugin is unloaded are dropped. This
happens when the host stops, when an idle location is evicted, and when a native
`opencode run --standalone` ends. A child whose completion wake-up never arrives has
no `04_session.raw`. A deleted session cannot be exported. A failed, timed-out
(120 seconds) or unexpected export writes nothing and is not retried; the next
trigger refreshes it.

## Transcripts, unverified

It is not verified that `session export --standalone` reads the shared database
without errors while the shared service writes it. The spike showed parallel
`--standalone` runs sharing it without errors.
