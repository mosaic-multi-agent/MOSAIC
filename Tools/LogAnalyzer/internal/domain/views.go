package domain

import "time"

// Ratio is an exact ratio; undefined when Denominator <= 0.
type Ratio struct {
	Numerator   int64
	Denominator int64
}

// GroupFigures summarises a set of subagent invocations. It never includes the
// orchestrator; run-level totals are carried separately.
type GroupFigures struct {
	Invocations    int
	Totals         Totals       // sum of the members' Dispatch.Totals
	UnpricedModels []PricingKey // distinct, sorted, across members
	MissingUsage   int          // members with Dispatch.MissingUsage
	Open           int          // members with State == InvocationOpen
}

// MoneyStats summarises per-member money totals.
type MoneyStats struct {
	Basis   int        // members whose Totals.Money.Total is MoneyKnown
	Min     MoneyValue // NoData when Basis == 0
	Max     MoneyValue
	Average MoneyValue // sum of Known totals / Basis, rounded half-up to the nanodollar
	// Complete is true only when Basis equals the number of members and every
	// member's Totals.Money.Complete is true. False marks Min/Max/Average partial.
	Complete bool
}

// DurationStats summarises usable durations.
type DurationStats struct {
	Count   int           // calls/invocations with a usable Duration()
	Total   time.Duration // 0 when Count == 0
	Min     time.Duration
	Max     time.Duration
	Average time.Duration // Total / Count (integer division); 0 when Count == 0
}

// ModelFigures is one model's priced requests within a run.
type ModelFigures struct {
	Model    ModelID    // "" = requests that logged no model
	Provider ProviderID // "" = requests that logged no provider; rows are keyed by (Provider, Model)
	Priced   bool       // a pricing entry exists for Model
	Requests int        // number of InvocationUsage entries
	Totals   Totals     // priced sum of those requests
}

// RunOverview is the per-run overview view (V1).
type RunOverview struct {
	Totals            Totals       // == RunReport.Totals
	Orchestrator      Totals       // == RunReport.Orchestrator.Totals
	Subagents         GroupFigures // over all Dispatches of the run
	OrchestratorShare Ratio        // orchestrator Money.Total nanos / run Money.Total nanos
	SubagentShare     Ratio        // sum of agents' Money.Total nanos / run Money.Total nanos
	ByModel           []ModelFigures
	CacheHitRatio     Ratio // CacheRead / TotalContext() of the run tokens
	Outcomes          OutcomeSplit

	// Unattributed is the run-level unattributed usage; nil when the run has none.
	Unattributed *Totals
	// UnattributedShare is unattributed Money.Total nanos / run Money.Total nanos;
	// the zero Ratio when Unattributed is nil.
	UnattributedShare Ratio
}

// Dispatch is one subagent invocation with its priced usage.
type Dispatch struct {
	Invocation     Invocation
	Models         []ModelID // distinct, sorted, from Invocation.Usage
	UnpricedModels []PricingKey
	Totals         Totals        // priced sum of Invocation.Usage; all NoData when none
	MissingUsage   bool          // no entry in Invocation.Usage carries any token count
	Effort         EffortLevels  // effort levels of Invocation.Usage
	PeakContext    ContextFigure // equals the matching ActorContext.Peak
	Cache          CacheFigures
	Repeat         bool
	LikelyRework   bool
}

// AgentTypeGroup is one agent type's dispatches (V2).
type AgentTypeGroup struct {
	AgentType     string // "" = unknown type
	Figures       GroupFigures
	PerInvocation MoneyStats // over the members' Totals.Money.Total
	Dispatches    []Dispatch // sorted by instance id
	PeakContext   PeakContextStats
	Cache         CacheFigures
	Rework        ReworkFigures
}

// StatusKind separates reported status codes from the two groups without one.
type StatusKind uint8

const (
	StatusReported StatusKind = iota // closed with a status_code (enum or verbatim other value)
	StatusNone                       // closed, no status_code
	StatusOpen                       // no invocation_end
)

// StatusGroup is the invocations sharing one status outcome.
type StatusGroup struct {
	Kind    StatusKind
	Code    string // set only for StatusReported
	Figures GroupFigures
}

// AgentTypeStatus is one agent type's status groups.
type AgentTypeStatus struct {
	AgentType string
	Groups    []StatusGroup
}

// StatusBreakdown is the by-status view (V3).
type StatusBreakdown struct {
	Run         []StatusGroup
	ByAgentType []AgentTypeStatus
}

// ToolFigures are the exact figures of one tool; no token or money attribution.
type ToolFigures struct {
	Tool      string        // "" = name not logged
	Calls     int           // ToolCall entries
	Errors    int           // Status == ToolCallError
	Unpaired  int           // Pairing != ToolCallPaired
	Durations DurationStats // over calls whose Duration() is ok
	// Harnesses are the distinct non-empty harnesses (actor attribution) of the
	// calls in this row, sorted.
	Harnesses []Harness
	// Growth is the context growth of the singular calls that requested only this tool.
	Growth ToolGrowth
}

// AgentTypeTools is one agent type's tool figures.
type AgentTypeTools struct {
	AgentType string
	Tools     []ToolFigures
}

// ToolsBreakdown is the tools view (V4).
type ToolsBreakdown struct {
	Run          []ToolFigures // orchestrator + all invocations
	Orchestrator []ToolFigures
	ByAgentType  []AgentTypeTools
}
