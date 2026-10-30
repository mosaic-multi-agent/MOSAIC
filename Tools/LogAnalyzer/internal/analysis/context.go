package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// buildRunContext derives the context figures of every actor of a run from its
// per-call usage records. Only records with a record id are calls; the legacy
// aggregate entries never count as a context size.
func buildRunContext(run domain.RunAggregate, table domain.PricingTable, settings domain.AnalysisSettings) domain.RunContext {
	runHarness := onlyHarness(run.Facts.Harnesses)
	rc := domain.RunContext{
		Orchestrator: buildActorContext(actorInput{
			actor:       domain.Orchestrator(),
			usage:       run.Orchestrator.Invocations,
			runHarness:  runHarness,
			compactions: run.Activity.Compactions,
		}, table, settings),
	}
	for _, inv := range run.Activity.Invocations {
		rc.Invocations = append(rc.Invocations, buildActorContext(actorInput{
			actor:       inv.Actor,
			usage:       inv.Usage,
			tools:       inv.ToolCalls,
			runHarness:  runHarness,
			compactions: run.Activity.Compactions,
			firstWrite:  true,
		}, table, settings))
	}
	sort.SliceStable(rc.Invocations, func(i, j int) bool {
		return rc.Invocations[i].Actor.Instance < rc.Invocations[j].Actor.Instance
	})
	return rc
}

// onlyHarness returns the harness when the run used exactly one, else "".
func onlyHarness(harnesses []domain.Harness) domain.Harness {
	if len(harnesses) == 1 {
		return harnesses[0]
	}
	return ""
}

// actorInput is everything the context figures of one actor derive from.
type actorInput struct {
	actor       domain.ActorRef
	usage       []domain.InvocationUsage
	tools       []domain.ToolCall
	runHarness  domain.Harness
	compactions []domain.Compaction
	firstWrite  bool // compute the context at first write
}

func buildActorContext(in actorInput, table domain.PricingTable, settings domain.AnalysisSettings) domain.ActorContext {
	out := domain.ActorContext{Actor: in.actor, Calls: contextCalls(in.usage, table, settings)}
	for _, c := range out.Calls {
		fig := callFigure(c)
		if c.Kind != domain.CallMain || !fig.Tokens.IsPresent() {
			continue
		}
		if !out.First.Tokens.IsPresent() {
			out.First = fig
		}
		if !out.Peak.Tokens.IsPresent() || fig.Tokens.ValueOr(0) > out.Peak.Tokens.ValueOr(0) {
			out.Peak = fig
		}
		out.Final = fig
	}
	if in.firstWrite {
		harness := actorHarness(in.usage, in.runHarness)
		applyFirstWrite(&out, in.tools, harness, settings)
	}
	out.Rewrites = detectRewrites(in.usage, table, settings)
	out.Compactions = loggedMarks(out.Calls, in.actor, in.compactions)
	out.Compactions = append(out.Compactions, detectedMarks(out.Calls, settings.CompactionDropPercent)...)
	orderMarks(out.Compactions)
	applyCompactionCalls(&out)
	return out
}

func callFigure(c domain.ContextCall) domain.ContextFigure {
	v, ok := c.Context.Value()
	if !ok {
		return domain.ContextFigure{}
	}
	return domain.ContextFigure{Tokens: c.Context, Partial: c.Partial, Band: domain.BandOf(v)}
}

// contextCalls lists the per-call records in time order; a call without a
// parseable time follows the timed calls.
func contextCalls(usage []domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) []domain.ContextCall {
	var calls []domain.ContextCall
	for _, u := range usage {
		if u.RecordID == "" || u.Usage.IsEmpty() {
			continue
		}
		priced := priceUsage(u, table, settings)
		calls = append(calls, domain.ContextCall{
			At:        u.At,
			RecordID:  u.RecordID,
			Model:     u.Model,
			Provider:  u.Provider,
			Harness:   u.Harness,
			Usage:     u.Usage,
			Context:   u.Usage.TotalContext(),
			Partial:   !u.Usage.Input.IsPresent() || !u.Usage.CacheRead.IsPresent() || !u.Usage.CacheCreation.IsPresent(),
			Cost:      priced.Money.Total,
			Kind:      callKindOrMain(u.Kind),
			Label:     u.Label,
			SessionID: u.SessionID,
		})
	}
	sort.SliceStable(calls, func(i, j int) bool {
		a, b := calls[i].At, calls[j].At
		if a.IsZero() != b.IsZero() {
			return b.IsZero()
		}
		return a.Before(b)
	})
	return calls
}

// actorHarness is the first non-empty harness of the actor's usage entries, else
// the run's only harness, else "" (the union of all harness lists applies).
func actorHarness(usage []domain.InvocationUsage, runHarness domain.Harness) domain.Harness {
	for _, u := range usage {
		if u.Harness != "" {
			return u.Harness
		}
	}
	return runHarness
}

// applyFirstWrite sets the heuristic context at the actor's first write-class
// tool call: that of the last call starting at or before the write.
func applyFirstWrite(out *domain.ActorContext, tools []domain.ToolCall, harness domain.Harness, settings domain.AnalysisSettings) {
	var start time.Time
	for _, t := range tools {
		if t.StartedAt.IsZero() || !settings.IsWriteTool(harness, t.ToolName) {
			continue
		}
		if start.IsZero() || t.StartedAt.Before(start) {
			start = t.StartedAt
		}
	}
	out.FirstWriteAt = start
	if start.IsZero() {
		return
	}
	source := -1
	for i, c := range out.Calls {
		if c.Kind == domain.CallMain && !c.At.IsZero() && !c.At.After(start) && c.Context.IsPresent() {
			source = i
		}
	}
	if source < 0 {
		return
	}
	out.Calls[source].FirstWrite = true
	out.FirstWrite = callFigure(out.Calls[source])
}

// loggedMarks places the actor's logged compactions among its calls.
func loggedMarks(calls []domain.ContextCall, actor domain.ActorRef, compactions []domain.Compaction) []domain.CompactionMark {
	var marks []domain.CompactionMark
	for _, c := range compactions {
		if c.Actor.Kind != actor.Kind || c.Actor.Instance != actor.Instance {
			continue
		}
		mark := domain.CompactionMark{Kind: domain.CompactionLogged, At: c.At, Before: -1, After: -1}
		if !c.At.IsZero() {
			spanEnd := c.At
			if !c.EndedAt.IsZero() {
				spanEnd = c.EndedAt
			}
			for i, call := range calls {
				if call.At.IsZero() {
					continue
				}
				if call.At.Before(c.At) {
					mark.Before = i
				}
				if mark.After < 0 && call.At.After(spanEnd) {
					mark.After = i
				}
			}
		}
		marks = append(marks, mark)
	}
	return marks
}
