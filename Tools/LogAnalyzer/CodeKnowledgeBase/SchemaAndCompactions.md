# Schema versions and compaction counting

## Schema version rule

`domain.IsSupportedSchemaVersion` accepts `MAJOR.MINOR.PATCH` (digits only, any size, leading zeros allowed) whose major is 1. There is no exact-version allow-list. `logread` emits one `FindingUnknownSchema` per file, on the first line whose version is unsupported (another major, empty or unparseable); the events are still decoded. Unknown fields and event types are ignored without a finding.

## Compaction counting

The run model (`RunActivity.Compactions`, `Timeline.Compactions`) holds one `domain.Compaction` per compaction, not per logged event. Pairing is `domain.PairCompactionEvents`, applied per stream (a stream is one log file: kind, instance hint, path) by `analysis/compactions.go`. Halves of different streams never pair.

- **Phased events (schema 1.5.0).** A `post` pairs with the nearest preceding unpaired `pre` of the same stream (`CompactionPaired`). An unpaired `pre` is `CompactionPreOnly`, an unpaired `post` is `CompactionPostOnly`. A phase-less event at 1.5.0 or later is `CompactionSingle`.
- **Legacy rule (phase-less events below 1.5.0, for example the Claude Code pre/post pair).** A legacy event P with no `tokens_after` pairs with the next compaction event Q of the same stream when Q is also a legacy phase-less event and carries neither `trigger` nor `tokens_before`. No time limit. Pairing is greedy left to right: a consumed Q cannot be the first half of another pair (three field-less events give one pair plus one single). Legacy events never pair with phased events. An event whose version is empty, unparseable or of another major is never legacy.
- **Paired fields.** `At`, `Trigger`, `TokensBefore` come from the first half; `EndedAt` and `TokensAfter` from the second.

## Detected compactions and the "near" rule

`analysis/compaction_detect.go` detects a compaction between two consecutive context-present calls of an actor when neither call is partial, the earlier context is above 0, and the drop is at least `AnalysisSettings.CompactionDropPercent` (default 30, integer arithmetic). Detected marks are always labelled detected and sit beside the logged marks in `ActorContext.Compactions`.

`analysis/compaction_match.go` compares logged and detected compactions per actor and raises `FindingCompactionMismatch` findings (returned by `analysis.Price`):

- A logged compaction L is **near** a detected drop D of the same actor when L's span intersects `[t0 - 2 min, t1 + 2 min]`, where t0 and t1 are the times of the calls before and after D. The span is `[At, EndedAt]` for a paired compaction, else `[At, At]`.
- Matching is one-to-one. Logged compactions go in `At` order and each takes the earliest unmatched near drop.
- An orchestrator-stream compaction left unmatched is tried against unmatched subagent drops (earliest drop first, by t0). A match raises one finding for the subagent, in the orchestrator-stream wording; that drop raises no second finding.
- Not checked (no finding): a zero `At`, or an actor with no context-present call strictly before `At` or none strictly after the span end.
- Remaining unmatched drops raise "no logged compaction near it"; remaining unmatched logged compactions raise "no context drop near it".

## Digests

Digest version 1.2.0 stores `ended_at` and `shape` per compaction and restores them as written; an unknown shape makes the digest invalid. Digests below 1.2.0 hold one entry per logged event; on load the legacy rule is applied per actor over the stored order (a digest records no stream, so out-of-order timestamps or several streams per actor may pair differently from the logs path; accepted).

## Field inventory: schema 1.4.0 and 1.5.0 fields

| Field | Decision | Reason |
|---|---|---|
| `compaction.phase` (1.5.0) | read | Drives pairing; unknown values decode as no phase. |
| Compaction written to the stream of the compacted actor (1.5.0) | relied on | Per-stream pairing and actor attribution use the stream the event came from. |
| `usage_record.source` value `otel` (1.4.0) | read | `source` is already carried verbatim as a forensic string; the new value needs no handling. |
| Unattributed stream `00_unattributed_events.jsonl` (1.4.0) | read | Read as `StreamUnattributed`; its usage records form the unattributable bucket. |
| `invocation_start.spawn_call_id` (1.4.0) | ignored | No current analysis links a dispatch to its spawning tool call; add it to the decoder when one does. |
