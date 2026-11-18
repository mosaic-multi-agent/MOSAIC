package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// applyFirstWrite sets the heuristic context at the first write-class tool call
// of every session of the actor: per session, that of the last main call of the
// session starting at or before the session's earliest write. FirstWrite is the
// first session's figure and FirstWriteAt the actor-wide earliest write.
func applyFirstWrite(out *domain.ActorContext, tools []domain.ToolCall, harness domain.Harness, settings domain.AnalysisSettings) {
	var writes []time.Time
	for _, t := range tools {
		if !t.StartedAt.IsZero() && settings.IsWriteTool(harness, t.ToolName) {
			writes = append(writes, t.StartedAt)
		}
	}
	for _, w := range writes {
		if out.FirstWriteAt.IsZero() || w.Before(out.FirstWriteAt) {
			out.FirstWriteAt = w
		}
	}
	if len(writes) == 0 || len(out.Calls) == 0 {
		return
	}

	sessions := callSessions(out.Calls)
	ordinals := sessionOrdinals(sessions)
	earliest := make(map[string]time.Time)
	for _, w := range writes {
		id := sessions[writeSessionCall(out.Calls, w)]
		if cur, ok := earliest[id]; !ok || w.Before(cur) {
			earliest[id] = w
		}
	}

	for id, start := range earliest {
		source := -1
		for i, c := range out.Calls {
			if sessions[i] == id && c.Kind == domain.CallMain && !c.At.IsZero() && !c.At.After(start) && c.Context.IsPresent() {
				source = i
			}
		}
		if source < 0 {
			continue
		}
		out.Calls[source].FirstWrite = true
		out.FirstWrites = append(out.FirstWrites, domain.SessionFirstWrite{
			Session: ordinals[id], SessionID: id, At: start, Call: source,
			Figure: callFigure(out.Calls[source]),
		})
	}
	sort.Slice(out.FirstWrites, func(i, j int) bool { return out.FirstWrites[i].Session < out.FirstWrites[j].Session })
	if len(out.FirstWrites) > 0 {
		out.FirstWrite = out.FirstWrites[0].Figure
	}
}

// writeSessionCall is the index of the call whose session a write belongs to:
// the last timed call starting at or before the write, else the first call.
func writeSessionCall(calls []domain.ContextCall, write time.Time) int {
	idx := 0
	for i, c := range calls {
		if !c.At.IsZero() && !c.At.After(write) {
			idx = i
		}
	}
	return idx
}

// sessionOrdinals numbers the distinct effective sessions from 1 in order of
// first appearance.
func sessionOrdinals(sessions []string) map[string]int {
	ordinals := make(map[string]int)
	for _, s := range sessions {
		if _, ok := ordinals[s]; !ok {
			ordinals[s] = len(ordinals) + 1
		}
	}
	return ordinals
}
