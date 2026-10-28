package cli

import "mosaic-log-analyzer/internal/domain"

// This file encodes the further-analysis figures of the report JSON: outcome
// split, rework, cache and wall time, plus the peak-context figures of agent
// types and dispatches. Absent token counts are null, money uses the
// {"state": ...} shape, durations are integer milliseconds.

type jsonOutcomeSplit struct {
	Success     jsonGroupFigures `json:"success"`
	OtherStatus jsonGroupFigures `json:"other_status"`
	OpenOrNone  jsonGroupFigures `json:"open_or_no_status"`
}

func encodeOutcomeSplit(o domain.OutcomeSplit) jsonOutcomeSplit {
	return jsonOutcomeSplit{
		Success:     encodeGroupFigures(o.Success),
		OtherStatus: encodeGroupFigures(o.OtherStatus),
		OpenOrNone:  encodeGroupFigures(o.OpenOrNone),
	}
}

// jsonRework labels itself: repeats include planned repeats and likely rework
// is a heuristic.
type jsonRework struct {
	Repeats               jsonGroupFigures `json:"repeats"`
	LikelyRework          jsonGroupFigures `json:"likely_rework"`
	RepeatsIncludePlanned bool             `json:"repeats_include_planned"`
	LikelyReworkHeuristic bool             `json:"likely_rework_heuristic"`
}

func encodeRework(r domain.ReworkFigures) jsonRework {
	return jsonRework{
		Repeats:               encodeGroupFigures(r.Repeats),
		LikelyRework:          encodeGroupFigures(r.LikelyRework),
		RepeatsIncludePlanned: true,
		LikelyReworkHeuristic: true,
	}
}

type jsonCache struct {
	HitRatio          jsonRatio      `json:"hit_ratio"`
	WriteTokens       *int64         `json:"write_tokens"`
	WriteCost         jsonMoneyValue `json:"write_cost"`
	NetBenefit        jsonMoneyValue `json:"net_benefit"`
	UnreadWriteTokens *int64         `json:"unread_write_tokens"`
	UnreadWriteCost   jsonMoneyValue `json:"unread_write_cost"`
	Complete          bool           `json:"complete"`
}

func encodeCache(c domain.CacheFigures) jsonCache {
	return jsonCache{
		HitRatio:          encodeRatio(c.HitRatio),
		WriteTokens:       encodeTokenCount(c.WriteTokens),
		WriteCost:         encodeMoneyValue(c.WriteCost),
		NetBenefit:        encodeMoneyValue(c.NetBenefit),
		UnreadWriteTokens: encodeTokenCount(c.UnreadWriteTokens),
		UnreadWriteCost:   encodeMoneyValue(c.UnreadWriteCost),
		Complete:          c.Complete,
	}
}

type jsonPeakContext struct {
	Basis   int                `json:"basis"`
	Average *int64             `json:"average"`
	Max     *jsonContextFigure `json:"max"`
}

func encodePeakContext(p domain.PeakContextStats) jsonPeakContext {
	return jsonPeakContext{
		Basis:   p.Basis,
		Average: encodeTokenCount(p.Average),
		Max:     encodeContextFigure(p.Max),
	}
}

type jsonCountedTool struct {
	Tool    string `json:"tool"`
	Harness string `json:"harness"`
	Calls   int    `json:"calls"`
}

type jsonOpenWait struct {
	Actor string  `json:"actor"`
	At    *string `json:"at"`
	Kind  string  `json:"kind"`
	Tool  string  `json:"tool,omitempty"`
}

type jsonWallTime struct {
	SpanMS                  *int64            `json:"span_ms"`
	WaitingMS               int64             `json:"waiting_ms"`
	SubagentsMS             int64             `json:"subagents_ms"`
	OrchestratorOnlyMS      int64             `json:"orchestrator_only_ms"`
	BetweenSessionsMS       int64             `json:"between_sessions_ms"`
	CountedTools            []jsonCountedTool `json:"counted_tools"`
	OpenWaits               []jsonOpenWait    `json:"open_waits"`
	MissingContactToolsHint bool              `json:"missing_contact_tools_hint"`
	UnconfiguredTools       []string          `json:"unconfigured_tools"`
}

func openWaitKindKey(k domain.OpenWaitKind) string {
	switch k {
	case domain.OpenWaitPermission:
		return "permission_notification"
	case domain.OpenWaitIdlePrompt:
		return "idle_prompt"
	}
	return "tool_call"
}

func encodeWallTime(w domain.WallTimeSplit) jsonWallTime {
	var span *int64
	if w.HasSpan {
		span = durationMillis(w.Span.End.Sub(w.Span.Start))
	}
	tools := make([]jsonCountedTool, len(w.CountedTools))
	for i, t := range w.CountedTools {
		tools[i] = jsonCountedTool{Tool: t.Tool, Harness: string(t.Harness), Calls: t.Calls}
	}
	waits := make([]jsonOpenWait, len(w.OpenWaits))
	for i, o := range w.OpenWaits {
		waits[i] = jsonOpenWait{Actor: o.Actor.Label(), At: timeOrNull(o.At), Kind: openWaitKindKey(o.Kind), Tool: o.Tool}
	}
	unconfigured := append([]string{}, w.UnconfiguredTools...)
	return jsonWallTime{
		SpanMS:                  span,
		WaitingMS:               w.Waiting.Milliseconds(),
		SubagentsMS:             w.Subagents.Milliseconds(),
		OrchestratorOnlyMS:      w.OrchestratorOnly.Milliseconds(),
		BetweenSessionsMS:       w.BetweenSessions.Milliseconds(),
		CountedTools:            tools,
		OpenWaits:               waits,
		MissingContactToolsHint: w.MissingContactToolsHint,
		UnconfiguredTools:       unconfigured,
	}
}
