package domain

// ActivityObserver receives the live activity of harness invocations.
// Called from adapter goroutines; implementations are safe for concurrent
// use, never return an error, never panic and never block the caller.
type ActivityObserver interface {
	// InvocationStarted announces one invocation before its process starts
	// and returns the feed for that invocation's activity. Never returns nil.
	InvocationStarted(inv ActivityInvocation) ActivityFeed
}

// ActivityFeed is the activity channel of one invocation.
type ActivityFeed interface {
	// Activity reports one item. Items arrive in harness output order.
	// Calls after Ended are ignored.
	Activity(item ActivityItem)
	// Status reports the invocation's state after one stream line. Called once
	// per stdout line (after that line's items), including lines that yield no
	// item and lines that cannot be decoded, so each call is also the
	// heartbeat. Latest wins. Ignored after Ended. Never blocks, never panics.
	Status(st ActivityStatus)
	// Ended announces that the invocation finished (any outcome). Called
	// exactly once, after the adapter's Run returned.
	Ended()
}

// ActivityStatus is the latest-wins state of one invocation's stream after
// the lines decoded so far. Each value replaces the previous one; nothing in
// it is a delta.
type ActivityStatus struct {
	// Model is the model the harness reported for the agent's own (main-thread)
	// calls; "" until reported or when the harness never reports one.
	Model string

	// ContextTokens is input + cache-read + cache-write tokens of the newest
	// main-thread model call. Calls of sub-agents the agent spawned are
	// excluded. Meaningful only when HasContext.
	ContextTokens int
	HasContext    bool

	// RunningTool / RunningToolID name the most recently started main-thread
	// tool call that has not finished; both "" when none is open. When it
	// finishes and an older one is still open, that one is reported.
	// Always "" for a harness that reports tools only on completion.
	RunningTool   string
	RunningToolID string

	// CostUSD is the invocation's cost in USD as the harness reports it.
	// Meaningful only when HasCost; never set for GHCP CLI.
	CostUSD float64
	HasCost bool
}

// NopActivityObserver returns a feed that discards everything.
type NopActivityObserver struct{}

// InvocationStarted implements ActivityObserver.
func (NopActivityObserver) InvocationStarted(ActivityInvocation) ActivityFeed {
	return nopActivityFeed{}
}

type nopActivityFeed struct{}

func (nopActivityFeed) Activity(ActivityItem) {}
func (nopActivityFeed) Status(ActivityStatus) {}
func (nopActivityFeed) Ended()                {}

// ActivitySource tells which kind of invocation an activity belongs to.
type ActivitySource string

const (
	ActivitySourceDispatch     ActivitySource = "dispatch"     // HarnessAdapter.Invoke
	ActivitySourceConsultation ActivitySource = "consultation" // RawInvoker.InvokeRaw
)

// ActivityInvocation identifies one invocation.
type ActivityInvocation struct {
	Source  ActivitySource
	Agent   string // dispatch: AgentInstanceID; consultation: agent identifier
	Harness string // harness ID, e.g. "claude-code"
}

// ActivityKind classifies an ActivityItem.
type ActivityKind string

const (
	ActivityToolCall ActivityKind = "tool-call"
	ActivityText     ActivityKind = "text"
	ActivitySubagent ActivityKind = "subagent"
	// ActivityKindUsage is not named ActivityUsage: that names the payload type.
	ActivityKindUsage ActivityKind = "usage"
)

// ActivitySummaryMaxRunes bounds ActivityItem.Summary.
const ActivitySummaryMaxRunes = 200

// ActivityItem is one harness-neutral piece of live activity.
type ActivityItem struct {
	Kind    ActivityKind
	Name    string        // tool name (tool-call); subagent type/name (subagent); "" otherwise
	Summary string        // single line, <= ActivitySummaryMaxRunes runes; "" for usage
	Usage   ActivityUsage // Kind == ActivityKindUsage only
}

// ActivityUsage is the usage reported by one harness event (a delta).
type ActivityUsage struct {
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	HasCost      bool // CostUSD is meaningful
}
