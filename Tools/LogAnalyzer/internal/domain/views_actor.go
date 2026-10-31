package domain

// ActorSession returns the figures of one actor.
func (r RunReport) ActorSession(a ActorRef) (ActorSession, bool) {
	for _, s := range r.ActorSessions {
		if s.Actor.Kind == a.Kind && s.Actor.Instance == a.Instance {
			return s, true
		}
	}
	return ActorSession{}, false
}

// ActorSession is the Session screen figures of one actor.
type ActorSession struct {
	Actor    ActorRef
	Findings []ActorFinding // in the order listed under ActorFindingKind
	Tools    []ToolFigures  // the actor's tool calls and growth, same rules and order as ToolsBreakdown
}

// ActorFindingKind says what a finding states.
type ActorFindingKind uint8

const (
	ActorFindingCostliestCall     ActorFindingKind = iota // up to 3, costliest first
	ActorFindingRewrites                                  // at most one
	ActorFindingCompactions                               // at most one
	ActorFindingFailedTool                                // one per tool with errors, most errors first, then name
	ActorFindingPeakContextVsType                         // at most one
	ActorFindingCostVsType                                // at most one
)

// ActorFinding is one measured fact about an actor.
type ActorFinding struct {
	Kind        ActorFindingKind
	Call        int        // CostliestCall: index into the actor's ActorContext.Calls
	Cost        MoneyValue // CostliestCall: call cost; Rewrites: summed ExtraCost; Compactions: CompactionCalls.Cost; CostVsType: actor total
	Share       Ratio      // CostliestCall: call cost / actor total
	Count       int        // Rewrites: rewrites; Compactions: logged + harness + detected-only; FailedTool: errors
	Unapplied   int        // Compactions: CompactionCalls.UnappliedCount
	Tool        string     // FailedTool
	Tokens      TokenCount // PeakContextVsType: actor peak context
	TypeAverage TokenCount // PeakContextVsType: PeakContextStats.Average of the actor's type
	TypeCost    MoneyValue // CostVsType: MoneyStats.Average of the actor's type
	TypeMembers int        // PeakContextVsType / CostVsType: basis of the type average
}
