package cli

import "mosaic-log-analyzer/internal/domain"

// This file encodes the run timeline carried by every run object of the report
// JSON. Lists are arrays, never null; absent times, durations and token counts
// are null.

type jsonTimelineEntry struct {
	Actor      string   `json:"actor"`
	AgentType  string   `json:"agent_type"`
	State      string   `json:"state"`
	StatusCode string   `json:"status_code"`
	StartedAt  *string  `json:"started_at"`
	EndedAt    *string  `json:"ended_at"`
	DurationMS *int64   `json:"duration_ms"`
	Overlaps   []string `json:"overlaps"`
}

type jsonTimelineCompaction struct {
	Actor        string  `json:"actor"`
	At           *string `json:"at"`
	Trigger      string  `json:"trigger"`
	TokensBefore *int64  `json:"tokens_before"`
	TokensAfter  *int64  `json:"tokens_after"`
}

type jsonTimeline struct {
	Entries     []jsonTimelineEntry      `json:"entries"`
	Compactions []jsonTimelineCompaction `json:"compactions"`
}

func encodeTimeline(t domain.Timeline) jsonTimeline {
	out := jsonTimeline{
		Entries:     make([]jsonTimelineEntry, 0, len(t.Entries)),
		Compactions: make([]jsonTimelineCompaction, 0, len(t.Compactions)),
	}
	for _, e := range t.Entries {
		inv := e.Invocation
		state := "closed"
		if inv.State == domain.InvocationOpen {
			state = "open"
		}
		var duration *int64
		if e.HasDuration {
			duration = durationMillis(e.Duration)
		}
		overlaps := make([]string, len(e.Overlaps))
		for i, id := range e.Overlaps {
			overlaps[i] = string(id)
		}
		out.Entries = append(out.Entries, jsonTimelineEntry{
			Actor:      string(inv.Actor.Instance),
			AgentType:  inv.Actor.Type,
			State:      state,
			StatusCode: inv.StatusCode,
			StartedAt:  timeOrNull(inv.StartedAt),
			EndedAt:    timeOrNull(inv.EndedAt),
			DurationMS: duration,
			Overlaps:   overlaps,
		})
	}
	for _, c := range t.Compactions {
		out.Compactions = append(out.Compactions, jsonTimelineCompaction{
			Actor:        c.Actor.Label(),
			At:           timeOrNull(c.At),
			Trigger:      c.Trigger,
			TokensBefore: encodeTokenCount(c.TokensBefore),
			TokensAfter:  encodeTokenCount(c.TokensAfter),
		})
	}
	return out
}
