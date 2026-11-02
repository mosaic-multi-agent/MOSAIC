package screens_test

// The head and the cost split of the Session screen: who the actor is, what it
// cost and which share of the run's cost that is, and the four token categories
// of its cost with amount and percent.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessHead is the part of an actor's Session screen above its first call row.
func sessHead(t *testing.T, id string) string {
	t.Helper()
	return sessAbove(sessView(t, id), sessFirstCall[id])
}

func money(cents int64) string { return screens.FormatMoney("", known(cents)) }

func TestSessionHead_InvocationShowsInstanceTypeStatusModelEffortDurationCostAndShare(t *testing.T) {
	head := sessHead(t, "builder#1")

	assertContains(t, head, "builder#1", "builder", "BLOCKED", "model-b", sessHighEffort.Label(),
		screens.FormatDuration(125*time.Second, true), money(1000), "10%")
}

func TestSessionHead_EachInvocationShowsItsOwnStatusModelAndShare(t *testing.T) {
	cases := []struct {
		id, status, model, share string
		cents                    int64
	}{
		{"coder#1", "SUCCESS", "model-c", "25%", 2500},
		{"reviewer#1", "PARTIALLY_DONE", "model-r", "20%", 2000},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			head := sessHead(t, c.id)

			assertContains(t, head, c.id, c.status, c.model, c.share, money(c.cents))
			assertNotContains(t, head, "BLOCKED", "model-b")
		})
	}
}

func TestSessionHead_OrchestratorShowsTheAbsentStatusAndTheRunSpanAsDuration(t *testing.T) {
	head := sessHead(t, "orchestrator")

	assertContains(t, head, "orchestrator", "model-o", money(4000), "40%", screens.FormatDuration(3725*time.Second, true))
	if lineWithAll(strings.ToLower(head), "status", screens.AbsentMarker) == "" {
		t.Errorf("the orchestrator has no status: the status line must show %q\n%s", screens.AbsentMarker, head)
	}
}

func TestSessionHead_OrchestratorWithoutARunSpanShowsAnAbsentDuration(t *testing.T) {
	run := sessionRun()
	run.WallTime.HasSpan = false

	view := stripStyling(sessOpenFrom(t, run, "orchestrator", screens.ScreenContext, tableW, tallH).View(tableW, tallH))
	head := sessAbove(view, 0)

	assertNotContains(t, head, screens.FormatDuration(3725*time.Second, true))
	if lineWithAll(strings.ToLower(head), "duration", screens.AbsentMarker) == "" {
		t.Errorf("a run without a span must show %q as the duration\n%s", screens.AbsentMarker, head)
	}
}

func TestSessionHead_InvocationWithoutStatusEffortOrTypeShowsTheAbsentLabels(t *testing.T) {
	head := sessHead(t, "tester#1")

	assertContains(t, head, "tester#1", screens.UnknownTypeLabel, screens.NoStatusLabel, "model-t")
}

func TestSessionHead_InvocationWithoutEffortShowsTheAbsentMarker(t *testing.T) {
	head := strings.ToLower(sessHead(t, "coder#1"))

	if lineWithAll(head, "effort", screens.AbsentMarker) == "" {
		t.Errorf("an invocation without effort must show %q for it\n%s", screens.AbsentMarker, head)
	}
}

// percentOnly matches the percent texts of a block of lines.
var percentOnly = regexp.MustCompile(`\d%`)

func TestSessionCostSplit_ShowsAmountAndPercentOfEveryCategory(t *testing.T) {
	cases := map[string][4]struct {
		cents int64
		pct   string
	}{
		"builder#1": {{150, "15%"}, {250, "25%"}, {400, "40%"}, {200, "20%"}},
		"coder#1":   {{500, "20%"}, {700, "28%"}, {900, "36%"}, {400, "16%"}},
	}
	names := [4]string{"input", "cache read", "cache write", "output"}
	for id, parts := range cases {
		head := strings.ToLower(sessHead(t, id))
		for i, p := range parts {
			if lineWithAll(head, names[i], strings.ToLower(money(p.cents)), p.pct) == "" {
				t.Errorf("%s: no line shows %s with %s and %s\n%s", id, names[i], money(p.cents), p.pct, head)
			}
		}
	}
}

func TestSessionCostSplit_OrchestratorSplitUsesItsOwnTotals(t *testing.T) {
	head := strings.ToLower(sessHead(t, "orchestrator"))

	for i, want := range []struct {
		cents int64
		pct   string
	}{{1000, "25%"}, {1200, "30%"}, {1400, "35%"}, {400, "10%"}} {
		name := [4]string{"input", "cache read", "cache write", "output"}[i]
		if lineWithAll(head, name, strings.ToLower(money(want.cents)), want.pct) == "" {
			t.Errorf("orchestrator: no line shows %s with %s and %s\n%s", name, money(want.cents), want.pct, head)
		}
	}
}

func TestSessionCostSplit_SmallPartsRoundLikeEveryOtherShare(t *testing.T) {
	head := sessHead(t, "reviewer#1")

	// cache write is 8 of 2,000 (0.4%): below one percent, never 0%.
	assertContains(t, head, money(8), "<1%", money(12), money(1000), "50%", money(980), "49%")
	assertNotContains(t, head, " 0%")
}

func TestSessionCostSplit_UnpricedActorShowsTheAbsentMarkerInsteadOfPercents(t *testing.T) {
	head := sessHead(t, "tester#1")

	assertContains(t, head, screens.UnpricedMarker)
	if m := percentOnly.FindString(head); m != "" {
		t.Errorf("an unpriced actor has no cost to share: found %q\n%s", m, head)
	}
	if lineWithAll(strings.ToLower(head), "input", screens.AbsentMarker) == "" &&
		lineWithAll(strings.ToLower(head), "input", screens.UnpricedMarker) == "" {
		t.Errorf("the input category of an unpriced actor must show an absent or unpriced amount\n%s", head)
	}
}

func TestSessionHead_ActorWithoutAnyDataStillRendersWithinTheScreen(t *testing.T) {
	view := sessView(t, "ghost#1")

	assertContains(t, view, "ghost#1", "ghost", "No per-call records")
	assertNotContains(t, view, "NaN", "Inf")
	assertWithinBounds(t, view, tableW, tallH)
	if m := percentOnly.FindString(view); m != "" {
		t.Errorf("an actor with no cost data has no percent: found %q\n%s", m, view)
	}
}

func TestSessionHead_ActorMissingFromTheDispatchesStillRendersFromItsContext(t *testing.T) {
	run := sessionRun()
	run.Agents, run.ByAgentType = nil, nil

	view := stripStyling(sessOpenFrom(t, run, "builder#1", screens.ScreenContext, tableW, tallH).View(tableW, tallH))

	assertContains(t, view, "builder#1", ctxCallLabels(70)[0])
	assertWithinBounds(t, view, tableW, tallH)
}

func TestSessionHead_ShowsTheFiguresOfTheOtherCurrency(t *testing.T) {
	colourOff(t)
	st := paletteStyles()
	st.Currency = domain.CurrencyCode("EUR")
	s := screens.NewSessionScreen(sessionRun(), sessRef("builder#1"), screens.ScreenContext, tableW, tallH, st)

	view := stripStyling(s.View(tableW, tallH))
	head := sessAbove(view, sessFirstCall["builder#1"])

	assertContains(t, head, screens.FormatMoney("EUR", known(1000)), screens.FormatMoney("EUR", known(150)))
}

func TestSessionHead_ShowsTheTypeOfTheInvocation(t *testing.T) {
	run := sessionRun()
	ref := ctxAgent("builder#1", "wizard")
	run.Context.Invocations[0].Actor = ref
	for i := range run.Agents {
		if run.Agents[i].Actor.Instance == "builder#1" {
			run.Agents[i].Actor = ref
		}
	}
	for i := range run.ByAgentType {
		if run.ByAgentType[i].Dispatches[0].Invocation.Actor.Instance == "builder#1" {
			run.ByAgentType[i].AgentType = "wizard"
			run.ByAgentType[i].Dispatches[0].Invocation.Actor = ref
		}
	}
	colourOff(t)

	view := stripStyling(screens.NewSessionScreen(run, ref, screens.ScreenContext, tableW, tallH, paletteStyles()).View(tableW, tallH))

	assertContains(t, sessAbove(view, sessFirstCall["builder#1"]), "wizard")
}
