# OTel ingest conformance fixtures

Language-neutral cases that define the behaviour of the Copilot OTel ingest. They are plain files (JSON, JSONL,
Markdown) with no Go-specific encoding. `go test` in `Tools/Common` iterates them; the Python implementation of the
ingest iterates the same folder with pytest. A new behaviour is added as a case here first.

## Case layout

```
<case>/
  case.json
  otel/<name>.jsonl            OTel file contents per step
  logs/                        initial run folder content (MOSAIC logs, optionally .otel-ingest/state.json)
  logs-<step>/                 optional overlay copied over the run folder before that step
  expected/<step>/
    streams/<relative path>    expected OTel usage_record lines per stream file
    state.json                 expected .otel-ingest/state.json; absent = no state file may exist
    result.json                status, records_written, finalized, pending, lines_read, lines_malformed,
                               tool_ends_written
    diagnostics.json           [{"kind", "counts", "ids"}] sorted by kind; absent = no diagnostics
```

`case.json` holds `description`, `run_id` and `steps`. A step holds `name`, `otel` (path of the OTel file contents,
`null` = the OTel file must not exist), `logs` (path of an overlay folder or `null`), `final`, `whole_file`,
`start_offset` (number or `null`) and `scope` (list of conversation IDs).

## Rules

- The runner builds `<tmp>/OrchestrationLogs/<run_id>/` from `logs/`. Before each step it copies the step's `otel`
  file to one fixed path per case and the step's `logs` overlay into the run folder, then runs one ingest with the
  step's parameters.
- Streams: the `usage_record` lines with `source` `otel` and every `tool_call_end` line of every stream file
  (`00_orchestrator_events.jsonl`, `00_unattributed_events.jsonl`, `<agent_instance_id>/03_events.jsonl`), in file
  order, are compared with `expected/<step>/streams/` as JSON data, ignoring key order; `timestamp` is compared.
  `tool_call_end` lines of the input `logs/` (hook-written ends) are therefore part of the expected files. A stream
  without an expected file must hold no such lines. Expected stream files are cumulative (the whole file after the
  step). Other lines (starts, other events, usage records of other sources) are ignored.
- `state.json`, `result.json` and `diagnostics.json` are compared as JSON data. Arrays compare in the order given.
- Fixed by the fixtures, so both implementations produce identical records: `schema_version` is `1.7.0` on every
  `usage_record` and on every ingest-written `tool_call_end` (hook-written ends copied from `logs/` keep their own
  version); `harness` is the envelope harness of the session the span is tied through (else of the invocation
  whose `spawn_call_id` matches); `session_id` is the tying session or omitted. Only diagnostic `message` is not
  compared.
- Placement keys come only from the MOSAIC logs in `logs/`: envelope `session_id` of the orchestrator stream and
  invocation streams, and `invocation_start.spawn_call_id`.
- The ingest reads tool names only from content attributes: the `tool_call` parts of `gen_ai.output.messages`.
  Inputs hold no other content attributes except in cases that test their exclusion (marked by the text
  `CONTENT-SENTINEL`, which must never appear in any expected file). In the requested-tools cases the sentinel
  surrounds the tool names (text, reasoning, arguments, call ids).
- The OTel files are hand-written in the shape of real captures.
- Lock-held and time-budget behaviour are tested in each language's own suite, not here.

## Reading rules (counts)

- Only complete lines (ending in `\n`) are consumed. A trailing partial line is not read, counted or consumed; the
  cursor stays before it.
- `lines_read` counts every complete line consumed from the start offset, including empty lines, non-object JSON
  lines, metric lines and lines without the marker.
- Marker: a line whose text does not contain the byte string `"gen_ai.operation.name"` (with quotes) cannot carry
  usage and is skipped without JSON parsing. It is not counted as malformed, whether or not it is valid JSON and
  whether or not it contains the text `usage` (plain text, empty lines and cut-off fragments fall here).
- A line that contains the marker is parsed as JSON. If parsing fails, or it parses to something other than a JSON
  object (for example an array), it is counted in `lines_malformed`.
- A parsed object that is not a chat span with usage (metric, `invoke_agent`) is consumed without effect and not
  counted. An `execute_tool` object has an effect only when it is a failed span (`status.code` 2) with a matching
  start (see below); every other `execute_tool` object is consumed without effect and not counted.
  `lines_read` is unchanged in meaning; a written end does not change it, and `tool_ends_written` counts the ends
  the ingest wrote.
- One `lines_skipped` diagnostic carries `{"malformed": lines_malformed, "read": lines_read}` and is emitted only
  when `lines_malformed` is greater than 0.

## Decisions fixed by the fixtures

- Token names: when a span carries both the new generation (`cache_write`, `reasoning.output_tokens`) and the legacy
  names (`cache_creation`, `reasoning_tokens`), the new generation wins (`conv_vscode_token_names`, span `tn-2`).
- A negative input-minus-cache difference omits `input_tokens` from the record (`conv_vscode_variants`, `var-4`).
- Token attribute values that are strings are not usage and are ignored (`conv_vscode_variants`, `var-6`).

## Unsupported dialects

- Only the VS Code dialect (raw SDK span objects) is ingested. A line wrapped as `{"type":"span", ...}` (the former
  GHCP CLI shape) is consumed without effect: it writes no record, creates no link, placement or pending entry, and is
  not counted as malformed (`ignored_cli_shaped_lines`). `lines_read` still counts it.
  The wrapper alone decides: a wrapped line that carries VS Code attributes (including `copilot_chat.chat_session_id`)
  is ignored too.
- A state file written before the removal still loads (`state_version` 1). A pending entry without `chat_session_id`
  (the shape the old code wrote for a CLI span) is placed by the VS Code rules: tied to a logged session, it is placed
  on the orchestrator and written; untied, it is skipped and the next state write leaves it out. A pending entry with
  `chat_session_id` still resolves; VS Code ingest is unaffected
  (`legacy_pending_entries_load`).

## Incremental ingest, pending spans and finality

The cursor, pending, split, placement, call-time-while-pending, robustness and state cases below (and the
attribution-order cases `attr_children_before_parents` and `attr_parents_before_children`) use VS Code dialect input
(raw SDK span objects). A subagent chat span there is pending until the `execution_subagent` tool span whose call id
is its `chat_session_id` has been read; that tool span's own `chat_session_id` is the spawn call id of the invocation.

- Cursor cases (`cursor_*`): a start offset inside a line skips that partial line without counting it; a stored
  cursor beats `start_offset`; a file shorter than the cursor or with a changed head fingerprint is read from byte 0;
  `whole_file` reads from byte 0 and appends only records not yet placed (placement is final).
- Pending cases (`pending_*`, `placement_final_late_parent`): `state.json` `pending` entries hold the nested
  `record` (placement-independent usage_record fields without `source`) plus `parent_span_id`, `conversation_id`
  (and chat session ids when present). Only a `final` step moves pending spans to `00_unattributed_events.jsonl`
  (a non-final step ignores `scope`); `scope` limits a final step to the listed conversations.
- Split cases: `split_inside_parent_child_pair` (steps `part1`, `part2`) and `split_inside_parent_child_pair_full`
  (step `full`) must end in the same streams and the same state.
- `state_unknown_version_refused`: an unknown `state_version` leaves `state.json` byte-for-byte unchanged and
  nothing is read or written.
- `pending_final_resolved_by_same_ingest_lines`: a final ingest places pending spans using the lines it reads in the
  same ingest first; only the span whose parent is still missing is finalised.
- `pending_reread_not_duplicated`: a span that is already pending and is read again (shorter file, `whole_file`)
  stays pending exactly once.
- `pending_final_scoped_by_parent_chat_session`: VS Code dialect; a final scope also matches
  `parent_chat_session_id`, not only `conversation_id`.

## Call time and call kind

- `timestamp` is the chat span's `startTime` (`[seconds, nanoseconds]`, two integers) in UTC, truncated (not
  rounded) to milliseconds, `YYYY-MM-DDTHH:MM:SS.mmmZ`. A span whose `startTime` is missing or is not such an array gets
  the ingest moment, which the runners fix at `2026-10-28T09:00:00.000Z`. Many older cases have spans without
  `startTime` and therefore expect that fixed value. A pending span keeps its start time in `state.json`
  (`pending[].record.timestamp`) and a later ingest writes it unchanged (`call_time_kept_while_pending`).
  `call_time_distinct_starts` pins the truncation rule, the fallback and that calls ingested together carry their own
  times.
- VS Code dialect: `call_label` is the chat span's `gen_ai.agent.name` verbatim when non-empty; `call_kind` follows
  from it (`call_kind_vscode_labels`): after removing every leading `retry-<letters>-` prefix, `panel/editAgent` and
  `tool/runSubagent` / `tool/runSubagent-*` are `main`; names starting with `summarizeConversationHistory` are
  `compaction`; `backgroundTodoAgent`, `title`, `progressMessages` and `copilotLanguageModelWrapper/*` are
  `auxiliary`; any other name leaves `call_kind` out and still sets `call_label`. The comparison is
  case-sensitive. A compaction span inside a subagent's session is placed like any other span of that session
  (`call_kind_vscode_subagent_session`). A subagent-session span kept pending carries its label and kind to the
  later ingest (`call_label_kept_while_pending`). Pending records carry `timestamp`, `call_kind` and `call_label` (the last
  two omitted when absent).

## Ends of failed tool calls (`tool_end_*`)

- A failed `execute_tool` span (`status.code` 2, message in `status.message`) whose call has a `tool_call_start` and
  no `tool_call_end` in the run's logs gets one `tool_call_end` from the ingest, written into the stream that holds
  the start. For VS Code that can be the orchestrator stream although the span belongs to a subagent's session
  (`tool_end_orchestrator_stream_for_subagent_span`); a start in an invocation stream gets its end there
  (`tool_end_invocation_stream`).
- The end carries the start's `call_id` verbatim, `tool_name` from the span (else the start), `status` `error`,
  `error` = `status.message` (omitted when empty), `timestamp` = the span's `endTime` converted like `startTime`
  (fallback: the ingest moment), `harness` and `session_id` of the start (`session_id` omitted when the start has
  none) and `schema_version` `1.7.0` (`tool_end_fallbacks`). Content attributes of the span are never copied.
- Matching: a start `call_id` equals the span call id after removing one trailing `__vscode-<digits>`. Spans with
  an empty or missing call id never match (`tool_end_empty_call_id`). A start found in two or more streams writes
  nothing (`tool_end_start_in_two_streams`).
- Nothing is written for a successful span (`tool_end_successful_span`) or when the stream already holds an end for
  the call, hook-written or ingest-written (`tool_end_hook_end_exists`, `tool_end_not_written_twice`).
- A failed span whose start is not found is not kept; a whole-file ingest after the start appears matches it
  (`tool_end_start_found_by_later_whole_file_ingest`).

## Requested tools (`requested_tools_*`)

- `requested_tools` of a VS Code `usage_record` is filled from the `tool_call` parts of the chat span's
  `gen_ai.output.messages` (a JSON string holding an array of messages with `parts`): the object maps each tool
  `name` to the number of `tool_call` parts carrying it, across all messages (`requested_tools_vscode`). Text,
  reasoning, arguments and call ids are never read into a record, the state file or a diagnostic.
- Output messages without any `tool_call` part (or an empty array) give `{}`; a span without the attribute (OTel
  content capture off) leaves the field out.
- A value that cannot be read (cut-off JSON, JSON that is not an array of messages) leaves the field out and never
  drops the usage record (`requested_tools_malformed_output_messages`).
- A pending span keeps its requested tools (`{}` included) in `state.json` `pending[].record.requested_tools` until a
  later ingest places it; `state_version` stays 1 (`requested_tools_kept_while_pending`).
