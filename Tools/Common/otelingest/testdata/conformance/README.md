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
    result.json                status, records_written, finalized, pending, lines_read, lines_malformed
    diagnostics.json           [{"kind", "counts", "ids"}] sorted by kind; absent = no diagnostics
```

`case.json` holds `description`, `run_id` and `steps`. A step holds `name`, `otel` (path of the OTel file contents,
`null` = the OTel file must not exist), `logs` (path of an overlay folder or `null`), `final`, `whole_file`,
`start_offset` (number or `null`) and `scope` (list of conversation IDs).

## Rules

- The runner builds `<tmp>/OrchestrationLogs/<run_id>/` from `logs/`. Before each step it copies the step's `otel`
  file to one fixed path per case and the step's `logs` overlay into the run folder, then runs one ingest with the
  step's parameters.
- Streams: the `usage_record` lines with `source` `otel` of every stream file (`00_orchestrator_events.jsonl`,
  `00_unattributed_events.jsonl`, `<agent_instance_id>/03_events.jsonl`), in file order, are compared with
  `expected/<step>/streams/` as JSON data, ignoring `timestamp` and key order. A stream without an expected file
  must hold no such lines. Expected stream files are cumulative (the whole file after the step). Lines of other
  sources are ignored.
- `state.json`, `result.json` and `diagnostics.json` are compared as JSON data. Arrays compare in the order given.
- Fixed by the fixtures, so both implementations produce identical records: `schema_version` is `1.4.0` on every
  `usage_record`; `harness` is the envelope harness of the session the span is tied through (else of the invocation
  whose `spawn_call_id` matches); `session_id` is the tying session or omitted. Only `timestamp` and diagnostic
  `message` are not compared.
- Placement keys come only from the MOSAIC logs in `logs/`: envelope `session_id` of the orchestrator stream and
  invocation streams, and `invocation_start.spawn_call_id`.
- Inputs hold no content attributes except in cases that test their exclusion (marked by the text
  `CONTENT-SENTINEL`, which must never appear in any expected file).
- Cases `conv_cli_real_excerpt` and `attr_cli_native_real_excerpt` hold verbatim lines cut from real GHCP CLI
  captures. The other OTel files are hand-written in the shape of real captures.
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
- A parsed object that is not a chat span with usage (metric, `execute_tool`, `invoke_agent`) is consumed without
  effect and not counted.
- One `lines_skipped` diagnostic carries `{"malformed": lines_malformed, "read": lines_read}` and is emitted only
  when `lines_malformed` is greater than 0.

## Decisions fixed by the fixtures

- Token names: when a span carries both the new generation (`cache_write`, `reasoning.output_tokens`) and the legacy
  names (`cache_creation`, `reasoning_tokens`), the new generation wins (`conv_cli_basic`, span `cli-3`).
- A negative input-minus-cache difference omits `input_tokens` from the record (`conv_vscode_variants`, `var-4`).
- Token attribute values that are strings are not usage and are ignored (`conv_vscode_variants`, `var-6`).

## Incremental ingest, pending spans and finality

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
