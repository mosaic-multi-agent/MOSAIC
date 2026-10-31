package cli

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// This file encodes the run-view breakdowns (overview, by agent type, by
// status, tools) carried by every run object of the report JSON. Lists are
// always arrays, never null; ratios are {numerator, denominator} with a
// denominator of 0 meaning undefined; durations are integer milliseconds or
// null; times are RFC3339 strings or null.

type jsonRatio struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

func encodeRatio(r domain.Ratio) jsonRatio {
	return jsonRatio{Numerator: r.Numerator, Denominator: r.Denominator}
}

func encodeModelIDs(ids []domain.ModelID) []string {
	out := make([]string, len(ids))
	for i, m := range ids {
		out[i] = string(m)
	}
	return out
}

// encodeUnpricedModels encodes unpriced gaps for the wire as a list of
// model-id strings. Wire contract (decoded by AgentTest as []string): each
// model id is listed ONCE, even when it is unpriced under several providers,
// and entries never carry the provider. The provider-level detail is shown by
// the text and TUI frontends only.
func encodeUnpricedModels(keys []domain.PricingKey) []string {
	return encodeModelIDs(domain.DistinctModelIDs(keys))
}

func durationMillis(d time.Duration) *int64 {
	ms := d.Milliseconds()
	return &ms
}

func timeOrNull(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339Nano)
	return &s
}

// jsonGroupFigures summarises a set of subagent invocations.
type jsonGroupFigures struct {
	Invocations    int        `json:"invocations"`
	Totals         jsonTotals `json:"totals"`
	UnpricedModels []string   `json:"unpriced_models"`
	MissingUsage   int        `json:"missing_usage"`
	Open           int        `json:"open"`
}

func encodeGroupFigures(f domain.GroupFigures) jsonGroupFigures {
	return jsonGroupFigures{
		Invocations:    f.Invocations,
		Totals:         encodeTotals(f.Totals),
		UnpricedModels: encodeUnpricedModels(f.UnpricedModels),
		MissingUsage:   f.MissingUsage,
		Open:           f.Open,
	}
}

type jsonModelFigures struct {
	Model    string     `json:"model"`
	Provider string     `json:"provider,omitempty"`
	Priced   bool       `json:"priced"`
	Requests int        `json:"requests"`
	Totals   jsonTotals `json:"totals"`
}

type jsonOverview struct {
	Totals            jsonTotals         `json:"totals"`
	Orchestrator      jsonTotals         `json:"orchestrator"`
	Subagents         jsonGroupFigures   `json:"subagents"`
	OrchestratorShare jsonRatio          `json:"orchestrator_share"`
	SubagentShare     jsonRatio          `json:"subagent_share"`
	ByModel           []jsonModelFigures `json:"by_model"`
	CacheHitRatio     jsonRatio          `json:"cache_hit_ratio"`

	// Both are absent for a run without unattributed usage.
	Unattributed      *jsonTotals `json:"unattributed,omitempty"`
	UnattributedShare *jsonRatio  `json:"unattributed_share,omitempty"`
}

func encodeOverview(o domain.RunOverview) jsonOverview {
	models := make([]jsonModelFigures, len(o.ByModel))
	for i, m := range o.ByModel {
		models[i] = jsonModelFigures{
			Model: string(m.Model), Provider: string(m.Provider), Priced: m.Priced, Requests: m.Requests, Totals: encodeTotals(m.Totals),
		}
	}
	ov := jsonOverview{
		Totals:            encodeTotals(o.Totals),
		Orchestrator:      encodeTotals(o.Orchestrator),
		Subagents:         encodeGroupFigures(o.Subagents),
		OrchestratorShare: encodeRatio(o.OrchestratorShare),
		SubagentShare:     encodeRatio(o.SubagentShare),
		ByModel:           models,
		CacheHitRatio:     encodeRatio(o.CacheHitRatio),
	}
	if o.Unattributed != nil {
		u, share := encodeTotals(*o.Unattributed), encodeRatio(o.UnattributedShare)
		ov.Unattributed, ov.UnattributedShare = &u, &share
	}
	return ov
}

// jsonMoneyStats marks partial statistics with complete=false.
type jsonMoneyStats struct {
	Basis    int            `json:"basis"`
	Min      jsonMoneyValue `json:"min"`
	Max      jsonMoneyValue `json:"max"`
	Average  jsonMoneyValue `json:"average"`
	Complete bool           `json:"complete"`
}

type jsonDispatch struct {
	Actor          string       `json:"actor"`
	AgentType      string       `json:"agent_type"`
	State          string       `json:"state"`
	StatusCode     *string      `json:"status_code"`
	Model          *string      `json:"model"`
	StartedAt      *string      `json:"started_at"`
	EndedAt        *string      `json:"ended_at"`
	DurationMS     *int64       `json:"duration_ms"`
	EndEvents      int          `json:"end_events"`
	ToolCalls      int          `json:"tool_calls"`
	MissingUsage   bool         `json:"missing_usage"`
	Models         []string     `json:"models"`
	UnpricedModels []string     `json:"unpriced_models"`
	Totals         jsonTotals   `json:"totals"`
	Effort         []jsonEffort `json:"effort"`

	PeakContext  *jsonContextFigure `json:"peak_context"`
	Cache        jsonCache          `json:"cache"`
	Repeat       bool               `json:"repeat"`
	LikelyRework bool               `json:"likely_rework"`
}

func stringOrNull(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func encodeDispatch(d domain.Dispatch) jsonDispatch {
	inv := d.Invocation
	state := "closed"
	if inv.State == domain.InvocationOpen {
		state = "open"
	}
	var duration *int64
	if dur, ok := inv.Duration(); ok {
		duration = durationMillis(dur)
	}
	return jsonDispatch{
		Actor:          string(inv.Actor.Instance),
		AgentType:      inv.Actor.Type,
		State:          state,
		StatusCode:     stringOrNull(inv.StatusCode),
		Model:          stringOrNull(string(inv.Model)),
		StartedAt:      timeOrNull(inv.StartedAt),
		EndedAt:        timeOrNull(inv.EndedAt),
		DurationMS:     duration,
		EndEvents:      inv.EndEvents,
		ToolCalls:      len(inv.ToolCalls),
		MissingUsage:   d.MissingUsage,
		Models:         encodeModelIDs(d.Models),
		UnpricedModels: encodeUnpricedModels(d.UnpricedModels),
		Totals:         encodeTotals(d.Totals),
		Effort:         encodeEffort(d.Effort),
		PeakContext:    encodeContextFigure(d.PeakContext),
		Cache:          encodeCache(d.Cache),
		Repeat:         d.Repeat,
		LikelyRework:   d.LikelyRework,
	}
}

type jsonAgentTypeGroup struct {
	AgentType     string           `json:"agent_type"`
	Figures       jsonGroupFigures `json:"figures"`
	PerInvocation jsonMoneyStats   `json:"per_invocation"`
	Dispatches    []jsonDispatch   `json:"dispatches"`
	PeakContext   jsonPeakContext  `json:"peak_context"`
	Rework        jsonRework       `json:"rework"`
	Cache         jsonCache        `json:"cache"`
}

func encodeAgentTypeGroups(groups []domain.AgentTypeGroup) []jsonAgentTypeGroup {
	out := make([]jsonAgentTypeGroup, len(groups))
	for i, g := range groups {
		dispatches := make([]jsonDispatch, len(g.Dispatches))
		for j, d := range g.Dispatches {
			dispatches[j] = encodeDispatch(d)
		}
		out[i] = jsonAgentTypeGroup{
			AgentType: g.AgentType,
			Figures:   encodeGroupFigures(g.Figures),
			PerInvocation: jsonMoneyStats{
				Basis:    g.PerInvocation.Basis,
				Min:      encodeMoneyValue(g.PerInvocation.Min),
				Max:      encodeMoneyValue(g.PerInvocation.Max),
				Average:  encodeMoneyValue(g.PerInvocation.Average),
				Complete: g.PerInvocation.Complete,
			},
			Dispatches:  dispatches,
			PeakContext: encodePeakContext(g.PeakContext),
			Rework:      encodeRework(g.Rework),
			Cache:       encodeCache(g.Cache),
		}
	}
	return out
}

// jsonStatusGroup carries a code only for a reported status; the no-status and
// open groups are identified by their kind alone.
type jsonStatusGroup struct {
	Kind    string           `json:"kind"`
	Code    string           `json:"code,omitempty"`
	Figures jsonGroupFigures `json:"figures"`
}

type jsonAgentTypeStatus struct {
	AgentType string            `json:"agent_type"`
	Groups    []jsonStatusGroup `json:"groups"`
}

type jsonStatusBreakdown struct {
	Run         []jsonStatusGroup     `json:"run"`
	ByAgentType []jsonAgentTypeStatus `json:"by_agent_type"`
}

func encodeStatusGroups(groups []domain.StatusGroup) []jsonStatusGroup {
	out := make([]jsonStatusGroup, len(groups))
	for i, g := range groups {
		kind := "reported"
		switch g.Kind {
		case domain.StatusNone:
			kind = "none"
		case domain.StatusOpen:
			kind = "open"
		}
		out[i] = jsonStatusGroup{Kind: kind, Code: g.Code, Figures: encodeGroupFigures(g.Figures)}
	}
	return out
}

func encodeStatusBreakdown(b domain.StatusBreakdown) jsonStatusBreakdown {
	byType := make([]jsonAgentTypeStatus, len(b.ByAgentType))
	for i, a := range b.ByAgentType {
		byType[i] = jsonAgentTypeStatus{AgentType: a.AgentType, Groups: encodeStatusGroups(a.Groups)}
	}
	return jsonStatusBreakdown{Run: encodeStatusGroups(b.Run), ByAgentType: byType}
}

// jsonDurations holds millisecond figures, null when no call has a usable duration.
type jsonDurations struct {
	Count     int    `json:"count"`
	TotalMS   *int64 `json:"total_ms"`
	MinMS     *int64 `json:"min_ms"`
	MaxMS     *int64 `json:"max_ms"`
	AverageMS *int64 `json:"average_ms"`
}

// jsonToolFigures are exact figures only: no token or money attribution.
type jsonToolFigures struct {
	Tool      string         `json:"tool"`
	Calls     int            `json:"calls"`
	Errors    int            `json:"errors"`
	Unpaired  int            `json:"unpaired"`
	Durations jsonDurations  `json:"durations"`
	Growth    jsonToolGrowth `json:"context_growth"`
}

type jsonAgentTypeTools struct {
	AgentType string            `json:"agent_type"`
	Tools     []jsonToolFigures `json:"tools"`
}

type jsonToolsBreakdown struct {
	Run          []jsonToolFigures    `json:"run"`
	Orchestrator []jsonToolFigures    `json:"orchestrator"`
	ByAgentType  []jsonAgentTypeTools `json:"by_agent_type"`
}

func encodeToolFigures(tools []domain.ToolFigures) []jsonToolFigures {
	out := make([]jsonToolFigures, len(tools))
	for i, f := range tools {
		d := jsonDurations{Count: f.Durations.Count}
		if f.Durations.Count > 0 {
			d.TotalMS = durationMillis(f.Durations.Total)
			d.MinMS = durationMillis(f.Durations.Min)
			d.MaxMS = durationMillis(f.Durations.Max)
			d.AverageMS = durationMillis(f.Durations.Average)
		}
		out[i] = jsonToolFigures{Tool: f.Tool, Calls: f.Calls, Errors: f.Errors, Unpaired: f.Unpaired, Durations: d, Growth: encodeToolGrowth(f.Growth)}
	}
	return out
}

func encodeToolsBreakdown(b domain.ToolsBreakdown) jsonToolsBreakdown {
	byType := make([]jsonAgentTypeTools, len(b.ByAgentType))
	for i, a := range b.ByAgentType {
		byType[i] = jsonAgentTypeTools{AgentType: a.AgentType, Tools: encodeToolFigures(a.Tools)}
	}
	return jsonToolsBreakdown{
		Run:          encodeToolFigures(b.Run),
		Orchestrator: encodeToolFigures(b.Orchestrator),
		ByAgentType:  byType,
	}
}

// jsonEffort is one effort level with the number of requests observed at it.
type jsonEffort struct {
	Level   string `json:"level"`
	Records int    `json:"records"`
}

// encodeEffort returns nil (JSON null) when no request reported an effort level.
func encodeEffort(l domain.EffortLevels) []jsonEffort {
	if l.IsEmpty() {
		return nil
	}
	out := make([]jsonEffort, len(l))
	for i, c := range l {
		out[i] = jsonEffort{Level: c.Level, Records: c.Records}
	}
	return out
}
