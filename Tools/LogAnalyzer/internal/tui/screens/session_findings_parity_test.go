package screens_test

// A run loaded from a digest shows the same findings and tools on the Session
// screen as the run it was written from, and a log without requested tools opens
// the screen normally with its tools marked "not provided".

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// parityPricing prices model-a with flat rates, so every call has a known cost.
func parityPricing(t *testing.T) domain.PricingTable {
	t.Helper()
	rate := func(s string) domain.Rate {
		r, err := domain.ParseRate(s)
		if err != nil {
			t.Fatalf("rate %q: %v", s, err)
		}
		return r
	}
	return domain.NewPricingTable([]domain.ModelPricing{{
		Model: "model-a",
		Standard: domain.RateSet{
			Input: rate("3.00"), CacheRead: rate("0.30"), CacheWrite5m: rate("3.75"), CacheWrite1h: rate("6.00"), Output: rate("15.00"),
		},
	}})
}

func parityTool(id, name string, startSec, endSec int, status domain.ToolCallStatus) domain.ToolCall {
	return domain.ToolCall{CallID: id, ToolName: name, Status: status, StartedAt: parityAt(startSec), EndedAt: parityAt(endSec), Pairing: domain.ToolCallPaired}
}

// findingsAggregate: an orchestrator and two coders with tool calls that failed, never
// ended or never started, and singular calls. withTools = false models an older log
// that does not carry the requested tools.
func findingsAggregate(withTools bool) domain.Aggregate {
	one := func(tool string) domain.RequestedTools {
		if !withTools {
			return domain.RequestedTools{}
		}
		return domain.NewRequestedTools(map[string]int{tool: 1})
	}
	none := one("")
	if withTools {
		none = domain.NewRequestedTools(nil)
	}
	coder1 := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	coder2 := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#2", Type: "coder"}
	orch := []domain.InvocationUsage{
		parityCall("o1", 10, 1000, 100, one("read_file")),
		parityCall("o2", 20, 1500, 50, one("read_file")),
		parityCall("o3", 40, 1700, 50, none),
	}
	usage1 := []domain.InvocationUsage{
		parityCall("a1", 12, 2000, 100, one("glob")),
		parityCall("a2", 22, 2301, 500, one("glob")),
		parityCall("a3", 32, 2400, 10, none),
		parityCall("a4", 42, 2600, 10, none),
	}
	usage2 := []domain.InvocationUsage{
		parityCall("b1", 14, 4000, 100, one("glob")),
		parityCall("b2", 24, 4500, 10, none),
	}
	orchTools := []domain.ToolCall{
		parityTool("ot1", "Bash", 11, 12, domain.ToolCallError),
		parityTool("ot2", "Bash", 21, 23, domain.ToolCallError),
		{CallID: "ot3", ToolName: "Read", StartedAt: parityAt(30), Pairing: domain.ToolCallStartOnly},
	}
	tools1 := []domain.ToolCall{
		parityTool("at1", "Read", 13, 16, ""),
		{CallID: "at2", EndedAt: parityAt(26), Pairing: domain.ToolCallEndOnly},
	}
	tools2 := []domain.ToolCall{parityTool("bt1", "Glob", 15, 17, "")}
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: orch},
		Agents: []domain.ActorAggregate{
			{Actor: coder1, Invocations: usage1}, {Actor: coder2, Invocations: usage2},
		},
		Turns: []domain.Turn{{Actor: domain.AgentInstance("coder#1"), Role: domain.TurnUser, At: parityAt(37)}},
		Activity: domain.RunActivity{
			OrchestratorToolCalls: orchTools,
			Invocations: []domain.Invocation{
				{Actor: coder1, State: domain.InvocationClosed, StartedAt: parityAt(0), EndedAt: parityAt(100), Usage: usage1, ToolCalls: tools1},
				{Actor: coder2, State: domain.InvocationClosed, StartedAt: parityAt(0), EndedAt: parityAt(100), Usage: usage2, ToolCalls: tools2},
			},
		},
	}}}
}

// pricedFlat analyses an aggregate with priced calls, the way the application does.
func pricedFlat(t *testing.T, agg domain.Aggregate) domain.RunReport {
	t.Helper()
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	report, _ := analysis.Price(agg, parityPricing(t), domain.DefaultAnalysisSettings(), src)
	return report.Runs[0]
}

// pricedViaDigest writes the aggregate to a digest and analyses what is read back.
func pricedViaDigest(t *testing.T, agg domain.Aggregate) domain.RunReport {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.digest.json")
	if err := digest.NewStore().Write(context.Background(), path, domain.Digest{Aggregate: agg}, false); err != nil {
		t.Fatalf("write the digest: %v", err)
	}
	loaded, _, err := digest.NewStore().Read(context.Background(), path)
	if err != nil {
		t.Fatalf("read the digest: %v", err)
	}
	return pricedFlat(t, loaded.Aggregate)
}

var findingsActors = []domain.ActorRef{
	domain.Orchestrator(),
	{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"},
	{Kind: domain.ActorAgentInstance, Instance: "coder#2", Type: "coder"},
}

func TestSessionFindingsParity_DigestLoadedRunHoldsTheSameActorSessionsAsTheLiveRun(t *testing.T) {
	agg := findingsAggregate(true)

	live, restored := pricedFlat(t, agg), pricedViaDigest(t, agg)

	if len(live.ActorSessions) != 3 {
		t.Fatalf("the live run holds %d actor sessions, want the orchestrator and two coders", len(live.ActorSessions))
	}
	if !reflect.DeepEqual(live.ActorSessions, restored.ActorSessions) {
		t.Errorf("actor sessions differ\nlive:   %+v\ndigest: %+v", live.ActorSessions, restored.ActorSessions)
	}
}

func TestSessionFindingsParity_DigestLoadedRunShowsTheSameScreenAsTheLiveRun(t *testing.T) {
	agg := findingsAggregate(true)
	live, restored := pricedFlat(t, agg), pricedViaDigest(t, agg)

	for _, ref := range findingsActors {
		want, got := findingsParityView(t, live, ref), findingsParityView(t, restored, ref)

		if got != want {
			t.Errorf("%s: the digest-loaded Session screen differs from the live one\nlive:\n%s\ndigest:\n%s", ref.Label(), want, got)
		}
		if len(findingLines(got)) == 0 {
			t.Errorf("%s: the digest-loaded screen shows no finding line\n%s", ref.Label(), got)
		}
		if tools, ok := parseHeader(got, "tool"); !ok || len(tools.rows) == 0 {
			t.Errorf("%s: the digest-loaded screen shows no tools table with rows\n%s", ref.Label(), got)
		}
	}
}

func TestSessionFindingsParity_TheComparedScreensShowFindingsAndTools(t *testing.T) {
	// Guards the parity tests above against comparing screens that show nothing.
	live := pricedFlat(t, findingsAggregate(true))
	orch := findingsParityView(t, live, findingsActors[0])
	coder := findingsParityView(t, live, findingsActors[1])

	assertContains(t, orch, "costliest call #", "failed tool Bash: 2 errors")
	assertContains(t, coder, "costliest call #", "peak context ", " vs coder average ", "cost ")
	sessToolCells(t, orch, "Read", "calls", "1", "unpaired", "1 "+screens.UnpairedLabel, "average duration", screens.AbsentMarker)
	sessToolCells(t, coder, screens.StartMissingLabel, "calls", "1", "unpaired", "1 "+screens.UnpairedLabel)
	sessToolCells(t, coder, "Read", "calls", "1", "average duration", screens.FormatDuration(3*time.Second, true))
}

func TestSessionFindingsParity_OlderLogWithoutRequestedToolsOpensAndMarksTheToolsNotProvided(t *testing.T) {
	agg := findingsAggregate(false)
	live, restored := pricedFlat(t, agg), pricedViaDigest(t, agg)

	for _, ref := range findingsActors {
		want, got := findingsParityView(t, live, ref), findingsParityView(t, restored, ref)

		if got != want {
			t.Errorf("%s: the digest-loaded screen of the older log differs from the live one", ref.Label())
		}
	}
	coder := findingsParityView(t, live, findingsActors[1])
	assertContains(t, coder, "costliest call #")
	sessToolCells(t, coder, "Read", "calls", "1", "average growth", notProvided, "growth samples", "0")
}

// findingsParityView is the styling-free Session screen of an actor on a terminal
// tall enough to show the findings and the tools.
func findingsParityView(t *testing.T, run domain.RunReport, ref domain.ActorRef) string {
	t.Helper()
	colourOff(t)
	return stripStyling(screens.NewSessionScreen(run, ref, screens.ScreenContext, headerW, legendOverlayH, paletteStyles()).View(headerW, legendOverlayH))
}
