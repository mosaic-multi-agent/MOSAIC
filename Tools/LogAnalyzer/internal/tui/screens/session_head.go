package screens

import (
	"strings"
	"time"

	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// The head of the Session screen: who the actor is, what it cost and the share of
// the run's cost, and the cost split into the four token categories.

// sessionInfo are the figures of the head that come from the run report rather
// than from the actor's context records.
type sessionInfo struct {
	status      string
	models      []string
	effort      domain.EffortLevels
	duration    time.Duration
	hasDuration bool
	totals      domain.Totals
}

// noSessionTotals is the cost split of an actor the report has no figures for.
func noSessionTotals() domain.Totals {
	n := domain.NoMoneyData()
	return domain.Totals{Money: domain.CategoryMoney{Input: n, CacheRead: n, CacheCreation: n, Output: n, Total: n}}
}

// sessionInfoOf collects the head figures of an actor: the orchestrator's from the
// run report, an invocation's from its dispatch or, failing that, its actor report.
func sessionInfoOf(run domain.RunReport, actor domain.ActorRef) sessionInfo {
	info := sessionInfo{status: AbsentMarker, totals: noSessionTotals()}
	if actor.Kind == domain.ActorOrchestrator {
		info.setReport(run.Orchestrator)
		if run.WallTime.HasSpan {
			info.duration, info.hasDuration = run.WallTime.Span.End.Sub(run.WallTime.Span.Start), true
		}
		return info
	}
	for _, g := range run.ByAgentType {
		for _, d := range g.Dispatches {
			if d.Invocation.Actor == actor {
				info.setReport(domain.ActorReport{Models: d.Models, Totals: d.Totals, Effort: d.Effort})
				info.status = statusText(d.Invocation)
				info.duration, info.hasDuration = d.Invocation.Duration()
				return info
			}
		}
	}
	for _, r := range run.Agents {
		if r.Actor == actor {
			info.setReport(r)
			break
		}
	}
	return info
}

// setReport takes the models, effort and cost of an actor report.
func (i *sessionInfo) setReport(r domain.ActorReport) {
	for _, m := range r.Models {
		i.models = append(i.models, string(m))
	}
	i.effort, i.totals = r.Effort, r.Totals
}

// sessionContextOf is the context records of an actor; an actor the context
// view does not list has none.
func sessionContextOf(run domain.RunReport, actor domain.ActorRef) domain.ActorContext {
	found := domain.ActorContext{}
	if actor.Kind == domain.ActorOrchestrator {
		found = run.Context.Orchestrator
	} else {
		for _, a := range run.Context.Invocations {
			if a.Actor == actor {
				found = a
			}
		}
	}
	found.Actor = actor
	return found
}

// sessionTitle names the actor: the orchestrator, or an instance with its type.
func sessionTitle(actor domain.ActorRef) string {
	if actor.Kind == domain.ActorOrchestrator {
		return orchestratorRowLabel
	}
	return actor.Label() + " (" + typeLabel(actor.Type) + ")"
}

// sessionHeadLines are the lines of the head: title, status, model, effort,
// duration, cost with its share of the run, and the cost split.
func sessionHeadLines(st Styles, width int, run domain.RunReport, actor domain.ActorRef, info sessionInfo, calls []domain.ContextCall) []string {
	models := info.models
	if len(models) == 0 {
		models = modelsOf(calls)
	}
	model := AbsentMarker
	if len(models) > 0 {
		model = strings.Join(models, ",")
	}
	effort := AbsentMarker
	if !info.effort.IsEmpty() {
		effort = info.effort.Label()
	}
	money := info.totals.Money
	cost := formatMoneyTotal(st.Currency, money)
	part, known := moneyNanos(money.Total)
	whole, _ := moneyNanos(run.Totals.Money.Total)
	share := sharePercentText(part, whole, known)

	lines := []string{
		clip(st.Subtitle, width, sessionTitle(actor)),
		clip(st.Body, width, "Status: "+info.status+"   Model: "+model),
		clip(st.Body, width, "Effort: "+effort+"   Duration: "+FormatDuration(info.duration, info.hasDuration)),
		clip(st.Body, width, "Cost: "+cost+"   Share of run cost: "+share),
	}
	return append(lines, costSplitLines(st, width, money)...)
}

// costSplitLines list the four token categories of a cost with amount and percent
// of the total.
func costSplitLines(st Styles, width int, m domain.CategoryMoney) []string {
	whole, wholeKnown := moneyNanos(m.Total)
	parts := []struct {
		name string
		v    domain.MoneyValue
	}{{"input", m.Input}, {"cache read", m.CacheRead}, {"cache write", m.CacheCreation}, {"output", m.Output}}
	lines := make([]string, len(parts))
	for i, p := range parts {
		nanos, ok := moneyNanos(p.v)
		pct := sharePercentText(nanos, whole, ok && wholeKnown)
		line := "  " + textwidth.Pad(p.name, 12, false) + " " + textwidth.Pad(FormatMoney(st.Currency, p.v), 12, true) + "  " + textwidth.Pad(pct, 5, true)
		lines[i] = clip(st.Body, width, line)
	}
	return lines
}
