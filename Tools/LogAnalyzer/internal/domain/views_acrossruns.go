package domain

// RunSummary is one named run's figures in the across-runs view (V6).
type RunSummary struct {
	Run            RunRef
	Provisional    bool
	Totals         Totals       // == RunReport.Totals (orchestrator + subagents)
	UnpricedModels []PricingKey // == RunReport.UnpricedModels
	Subagents      GroupFigures // == RunReport.Overview.Subagents (subagent dispatches only)
}

// AgentTypeAcrossRuns is one agent type's figures summed over the named runs
// that hold at least one invocation of it.
type AgentTypeAcrossRuns struct {
	AgentType         string       // "" = unknown type
	Runs              int          // named runs with >= 1 invocation of this type
	ProvisionalRuns   int          // of those, runs with Provisional == true
	Figures           GroupFigures // summed over those runs
	PerRunInvocations Ratio        // Figures.Invocations / Runs
	PerRunTokens      Ratio        // sum of present four-category tokens / Runs
	PerRunMoney       MoneyStats   // members = the type's per-run figures, one per counted run
}

// AcrossRuns is the across-runs view (V6). The unattributable bucket is never
// included.
type AcrossRuns struct {
	Runs        []RunSummary          // named runs only, Report.Runs order
	ByAgentType []AgentTypeAcrossRuns // ascending by agent type, "" last
}
