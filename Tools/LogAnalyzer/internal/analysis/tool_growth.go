package analysis

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// applyToolGrowth sets the context growth of every singular call of an actor.
// A call is singular when it is a main call that requested exactly one tool
// once and the actor's next timed main call follows with no compaction (logged,
// detected, or a compaction call) and no user turn of the actor in between.
// Singular is decided from recorded figures; timestamps only order the calls.
func applyToolGrowth(a *domain.ActorContext, turns []domain.Turn) {
	userTurns := userTurnTimes(turns, a.Actor)
	for i := range a.Calls {
		c := &a.Calls[i]
		tool, ok := c.RequestedTools.Single()
		if !ok || c.Kind != domain.CallMain || c.At.IsZero() {
			continue
		}
		j := nextTimedMain(a.Calls, i)
		if j < 0 || compactionBetween(*a, i, j) || turnBetween(userTurns, c.At, a.Calls[j].At) {
			continue
		}
		c.Growth = measureGrowth(tool, *c, a.Calls[j])
	}
}

// userTurnTimes lists the parseable user-turn times of one actor.
func userTurnTimes(turns []domain.Turn, actor domain.ActorRef) []time.Time {
	var out []time.Time
	for _, t := range turns {
		if t.Role == domain.TurnUser && !t.At.IsZero() &&
			t.Actor.Kind == actor.Kind && t.Actor.Instance == actor.Instance {
			out = append(out, t.At)
		}
	}
	return out
}

// turnBetween reports a user turn within the closed interval [from, to].
func turnBetween(turns []time.Time, from, to time.Time) bool {
	for _, t := range turns {
		if !t.Before(from) && !t.After(to) {
			return true
		}
	}
	return false
}

// nextTimedMain is the index of the next main call with a timestamp, or -1.
func nextTimedMain(calls []domain.ContextCall, i int) int {
	for j := i + 1; j < len(calls); j++ {
		if calls[j].Kind == domain.CallMain && !calls[j].At.IsZero() {
			return j
		}
	}
	return -1
}

// compactionBetween reports a compaction call or a compaction mark lying
// between calls i and j. A mark counts only when calls on both sides of it
// exist and fall inside the pair.
func compactionBetween(a domain.ActorContext, i, j int) bool {
	for k := i + 1; k < j; k++ {
		if a.Calls[k].Kind == domain.CallCompaction {
			return true
		}
	}
	for _, m := range a.Compactions {
		if m.After >= 0 && m.Before >= i && m.After <= j {
			return true
		}
	}
	return false
}

// measureGrowth is next context - own context - own output, from exact figures.
func measureGrowth(tool string, own, next domain.ContextCall) domain.CallGrowth {
	g := domain.CallGrowth{Tool: tool}
	if !hasFullContext(own.Usage) || !hasFullContext(next.Usage) || !own.Usage.Output.IsPresent() {
		g.State = domain.GrowthNotMeasurable
		return g
	}
	g.Tokens = next.Context.ValueOr(0) - own.Context.ValueOr(0) - own.Usage.Output.ValueOr(0)
	g.State = domain.GrowthMeasured
	if g.Tokens < 0 {
		g.State = domain.GrowthNegative
	}
	return g
}

func hasFullContext(u domain.TokenUsage) bool {
	return u.Input.IsPresent() && u.CacheRead.IsPresent() && u.CacheCreation.IsPresent()
}

// growthAcc accumulates the growth of one tool.
type growthAcc struct {
	domain.ToolGrowth
	sum int64
}

// growthByTool pools the singular calls of the given actors per tool.
func growthByTool(actors []domain.ActorContext) map[string]*growthAcc {
	out := make(map[string]*growthAcc)
	for _, a := range actors {
		for _, c := range a.Calls {
			if c.Growth.State == domain.GrowthNotSingular {
				continue
			}
			acc := out[c.Growth.Tool]
			if acc == nil {
				acc = &growthAcc{}
				out[c.Growth.Tool] = acc
			}
			acc.add(c.Growth)
		}
	}
	for _, acc := range out {
		acc.finish()
	}
	return out
}

func (g *growthAcc) add(c domain.CallGrowth) {
	switch c.State {
	case domain.GrowthNegative:
		g.Negative++
	case domain.GrowthNotMeasurable:
		g.NotMeasurable++
	case domain.GrowthMeasured:
		v := domain.Tokens(c.Tokens)
		if g.Samples == 0 || c.Tokens < g.Min.ValueOr(0) {
			g.Min = v
		}
		if g.Samples == 0 || c.Tokens > g.Max.ValueOr(0) {
			g.Max = v
		}
		g.Samples++
		g.sum += c.Tokens
	}
}

// finish sets the average, rounded half-up.
func (g *growthAcc) finish() {
	if g.Samples > 0 {
		n := int64(g.Samples)
		g.Average = domain.Tokens((2*g.sum + n) / (2 * n))
	}
}
