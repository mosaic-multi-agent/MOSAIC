package domain

import "time"

// ContextBand classifies a context size. The bands are fixed for every model.
type ContextBand uint8

const (
	BandBelowOptimal ContextBand = iota // [0, 100000)
	BandOptimal                         // [100000, 250000)
	BandAboveOptimal                    // [250000, 350000)
	BandDegraded                        // [350000, inf)
)

// BandOf returns the band a context size falls into.
func BandOf(tokens int64) ContextBand {
	switch {
	case tokens >= bandDegradedFrom:
		return BandDegraded
	case tokens >= bandAboveOptimalFrom:
		return BandAboveOptimal
	case tokens >= bandOptimalFrom:
		return BandOptimal
	}
	return BandBelowOptimal
}

const (
	bandOptimalFrom      int64 = 100_000
	bandAboveOptimalFrom int64 = 250_000
	bandDegradedFrom     int64 = 350_000
)

// Label returns the band name as text.
func (b ContextBand) Label() string {
	switch b {
	case BandOptimal:
		return "optimal"
	case BandAboveOptimal:
		return "above optimal"
	case BandDegraded:
		return "degraded"
	}
	return "below optimal"
}

// Key returns the band name as a machine key.
func (b ContextBand) Key() string {
	switch b {
	case BandOptimal:
		return "optimal"
	case BandAboveOptimal:
		return "above_optimal"
	case BandDegraded:
		return "degraded"
	}
	return "below_optimal"
}

// ContextBandRange is one band with its token range; To == 0 means open-ended.
type ContextBandRange struct {
	Band     ContextBand
	From, To int64
}

// ContextBandRanges returns the four band ranges in order.
func ContextBandRanges() []ContextBandRange {
	return []ContextBandRange{
		{Band: BandBelowOptimal, From: 0, To: bandOptimalFrom},
		{Band: BandOptimal, From: bandOptimalFrom, To: bandAboveOptimalFrom},
		{Band: BandAboveOptimal, From: bandAboveOptimalFrom, To: bandDegradedFrom},
		{Band: BandDegraded, From: bandDegradedFrom, To: 0},
	}
}

// ContextFigure is one context size with its band.
type ContextFigure struct {
	Tokens  TokenCount  // absent = no figure
	Partial bool        // the source call lacked input, cache read or cache write
	Band    ContextBand // meaningful only when Tokens is present
}

// ContextCall is one per-call record of an actor.
type ContextCall struct {
	At         time.Time
	RecordID   string // never "" (only per-call records are included)
	Model      ModelID
	Provider   ProviderID
	Harness    Harness
	Usage      TokenUsage
	Context    TokenCount // Usage.TotalContext(); absent when all three categories are absent
	Partial    bool       // any of Input, CacheRead, CacheCreation absent
	Cost       MoneyValue // this call's priced total
	FirstWrite bool       // the call whose context is ActorContext.FirstWrite
}

// CompactionKind says whether a mark was logged or detected.
type CompactionKind uint8

const (
	CompactionLogged CompactionKind = iota
	CompactionDetected
)

// CompactionMark places one compaction among an actor's calls.
type CompactionMark struct {
	Kind   CompactionKind
	At     time.Time // logged: Compaction.At; detected: the after call's At
	Before int       // index into Calls of the last call before; -1 none
	After  int       // index into Calls of the first call after; -1 none
	// Detected only:
	BeforeTokens, AfterTokens int64
}

// ActorContext holds one actor's context figures and per-call rows.
type ActorContext struct {
	Actor        ActorRef
	Calls        []ContextCall // per-call records in time order
	First        ContextFigure // first call with a present Context
	Peak         ContextFigure // largest Context (earliest on ties)
	Final        ContextFigure // last call with a present Context; output tokens never added
	FirstWrite   ContextFigure // heuristic; always absent for the orchestrator
	FirstWriteAt time.Time     // start of the first write-class tool call; zero when none
	Compactions  []CompactionMark
}

// LoggedCount is the number of logged compaction marks.
func (a ActorContext) LoggedCount() int { return a.countMarks(CompactionLogged) }

// DetectedCount is the number of detected compaction marks.
func (a ActorContext) DetectedCount() int { return a.countMarks(CompactionDetected) }

func (a ActorContext) countMarks(kind CompactionKind) int {
	n := 0
	for _, m := range a.Compactions {
		if m.Kind == kind {
			n++
		}
	}
	return n
}

// RunContext holds the context figures of every actor of a run.
type RunContext struct {
	Orchestrator ActorContext
	Invocations  []ActorContext // one per Activity.Invocations entry, sorted by instance id
}

// Find returns the context figures of one invocation.
func (r RunContext) Find(id AgentInstanceID) (ActorContext, bool) {
	for _, a := range r.Invocations {
		if a.Actor.Instance == id {
			return a, true
		}
	}
	return ActorContext{}, false
}

// FinalContextDefinition is the text shown next to every final-context figure.
const FinalContextDefinition = "last call's context as sent (input + cache read + cache write), excluding its output tokens"

// PeakContextStats summarises the peak context of a group's members.
type PeakContextStats struct {
	Basis   int           // members with a present Peak
	Average TokenCount    // sum of present peaks / Basis, rounded half-up; absent when Basis == 0
	Max     ContextFigure // absent when Basis == 0
}
