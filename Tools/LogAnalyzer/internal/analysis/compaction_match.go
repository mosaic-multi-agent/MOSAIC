package analysis

import (
	"fmt"
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// compactionNearWindow widens a detected drop on both sides when deciding
// whether a logged compaction is near it.
const compactionNearWindow = 2 * time.Minute

// dropRef is one detected drop of an actor, with the calls' times it lies between.
type dropRef struct {
	actor   domain.ActorRef
	mark    *domain.CompactionMark
	t0, t1  time.Time
	matched bool
}

// match records that a logged compaction explains the drop; a matched drop is
// shown with the logged compaction, never as a harness compaction.
func (d *dropRef) match() {
	d.matched = true
	d.mark.MatchedLogged = true
	d.mark.Harness = false
}

// loggedRef is a checked logged compaction with its time span.
type loggedRef struct {
	at, end time.Time
}

// near reports whether the logged span intersects [t0-2min, t1+2min].
func (d *dropRef) near(l loggedRef) bool {
	return !l.at.After(d.t1.Add(compactionNearWindow)) && !l.end.Before(d.t0.Add(-compactionNearWindow))
}

// compactionFindings compares each actor's logged and detected compactions and
// reports every disagreement. Matching is one-to-one: logged compactions go in
// time order and take the earliest unmatched near drop; an orchestrator-stream
// compaction left over then tries the unmatched drops of subagents.
func compactionFindings(run domain.RunRef, rc domain.RunContext, compactions []domain.Compaction) []domain.Finding {
	actors := append([]domain.ActorContext{rc.Orchestrator}, rc.Invocations...)
	drops := make([][]*dropRef, len(actors))
	var findings []domain.Finding
	var orchLeft []loggedRef
	for i, a := range actors {
		drops[i] = dropsOf(a)
		var left []loggedRef
		for _, l := range checkedCompactions(a, compactions) {
			if d := earliestNear(drops[i], l); d != nil {
				d.match()
			} else {
				left = append(left, l)
			}
		}
		if a.Actor.Kind == domain.ActorOrchestrator {
			orchLeft = left
			continue
		}
		for _, l := range left {
			findings = append(findings, noDropFinding(run, a.Actor, l))
		}
	}
	for _, l := range orchLeft {
		if d := earliestNear(subagentDrops(drops[1:]), l); d != nil {
			d.match()
			findings = append(findings, domain.Finding{
				Kind: domain.FindingCompactionMismatch, Severity: domain.SeverityWarning,
				Run: run, Actor: d.actor,
				Detail: fmt.Sprintf("compaction logged on the orchestrator stream at %s matches a context drop of %s",
					l.at.Format(time.RFC3339), d.actor.Instance),
			})
			continue
		}
		findings = append(findings, noDropFinding(run, rc.Orchestrator.Actor, l))
	}
	for _, ds := range drops {
		for _, d := range ds {
			if !d.matched && !d.mark.Harness {
				findings = append(findings, noLoggedFinding(run, d))
			}
		}
	}
	return findings
}

// dropsOf lists the actor's detected drops that can be placed in time.
func dropsOf(a domain.ActorContext) []*dropRef {
	var out []*dropRef
	for i := range a.Compactions {
		m := &a.Compactions[i]
		if m.Kind != domain.CompactionDetected {
			continue
		}
		t0, t1 := a.Calls[m.Before].At, a.Calls[m.After].At
		if t0.IsZero() || t1.IsZero() {
			continue
		}
		out = append(out, &dropRef{actor: a.Actor, mark: m, t0: t0, t1: t1})
	}
	return out
}

// subagentDrops flattens the subagents' drops in time order (earliest first).
func subagentDrops(perActor [][]*dropRef) []*dropRef {
	var out []*dropRef
	for _, ds := range perActor {
		out = append(out, ds...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.t0.Equal(b.t0) {
			return a.t0.Before(b.t0)
		}
		return a.t1.Before(b.t1)
	})
	return out
}

// earliestNear returns the first unmatched drop (in the order given) near l.
func earliestNear(drops []*dropRef, l loggedRef) *dropRef {
	for _, d := range drops {
		if !d.matched && d.near(l) {
			return d
		}
	}
	return nil
}

// checkedCompactions lists the actor's logged compactions that the comparison
// covers: those with a time, a context-present call strictly before it and one
// strictly after the end of its span. They keep their time order.
func checkedCompactions(a domain.ActorContext, compactions []domain.Compaction) []loggedRef {
	var out []loggedRef
	for _, c := range compactions {
		if c.Actor.Kind != a.Actor.Kind || c.Actor.Instance != a.Actor.Instance || c.At.IsZero() {
			continue
		}
		end := c.At
		if !c.EndedAt.IsZero() {
			end = c.EndedAt
		}
		if hasContextCall(a.Calls, func(t time.Time) bool { return t.Before(c.At) }) &&
			hasContextCall(a.Calls, func(t time.Time) bool { return t.After(end) }) {
			out = append(out, loggedRef{at: c.At, end: end})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out
}

func hasContextCall(calls []domain.ContextCall, timeOK func(time.Time) bool) bool {
	for _, c := range calls {
		if !c.At.IsZero() && c.Context.IsPresent() && timeOK(c.At) {
			return true
		}
	}
	return false
}

func noDropFinding(run domain.RunRef, actor domain.ActorRef, l loggedRef) domain.Finding {
	return domain.Finding{
		Kind: domain.FindingCompactionMismatch, Severity: domain.SeverityWarning,
		Run: run, Actor: actor,
		Detail: fmt.Sprintf("logged compaction at %s with no context drop near it", l.at.Format(time.RFC3339)),
	}
}

func noLoggedFinding(run domain.RunRef, d *dropRef) domain.Finding {
	percent := (d.mark.BeforeTokens - d.mark.AfterTokens) * 100 / d.mark.BeforeTokens
	return domain.Finding{
		Kind: domain.FindingCompactionMismatch, Severity: domain.SeverityWarning,
		Run: run, Actor: d.actor,
		Detail: fmt.Sprintf("context drop of %d%% between %s and %s with no logged compaction near it",
			percent, d.t0.Format(time.RFC3339), d.t1.Format(time.RFC3339)),
	}
}
