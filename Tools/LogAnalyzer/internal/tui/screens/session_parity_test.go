package screens_test

// A run loaded from a digest shows the same Session screen as the run it was
// written from: the same tools and growth on every call row, the same head and
// cost split.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var parityBase = time.Date(2026, 10, 23, 19, 13, 32, 0, time.UTC)

func parityAt(sec int) time.Time { return parityBase.Add(time.Duration(sec) * time.Second) }

func parityLabel(sec int) string { return parityAt(sec).Format("15:04:05") }

// parityCall is a main call whose context is `context` tokens and output `out`.
func parityCall(id string, sec int, context, out int64, tools domain.RequestedTools) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: "model-a", Harness: domain.HarnessClaudeCode, RecordID: id, At: parityAt(sec), Kind: domain.CallMain,
		Usage: domain.TokenUsage{
			Input: domain.Tokens(context), CacheRead: domain.Tokens(0), CacheCreation: domain.Tokens(0), Output: domain.Tokens(out),
		},
		RequestedTools: tools,
	}
}

// parityAggregate has measured, negative, turn-broken, compaction-broken and
// not-provided calls on the orchestrator and on one invocation.
func parityAggregate() domain.Aggregate {
	one := func(tool string) domain.RequestedTools { return domain.NewRequestedTools(map[string]int{tool: 1}) }
	none := domain.NewRequestedTools(nil)
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	orch := []domain.InvocationUsage{
		parityCall("o1", 10, 1000, 100, one("read_file")),       // measured 400
		parityCall("o2", 20, 1500, 50, one("read_file")),        // compaction after it: not singular
		parityCall("o3", 40, 1700, 50, none),                    // provided, empty
		parityCall("o4", 50, 1900, 50, domain.RequestedTools{}), // not provided
	}
	usage := []domain.InvocationUsage{
		parityCall("a1", 12, 2000, 100, one("glob")),     // measured 201
		parityCall("a2", 22, 2301, 500, one("glob")),     // negative
		parityCall("a3", 32, 2400, 10, one("read_file")), // user turn between: not singular
		parityCall("a4", 42, 2600, 10, none),
	}
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: orch},
		Agents:       []domain.ActorAggregate{{Actor: agent, Invocations: usage}},
		Turns:        []domain.Turn{{Actor: domain.AgentInstance("coder#1"), Role: domain.TurnUser, At: parityAt(37)}},
		Activity: domain.RunActivity{
			Compactions: []domain.Compaction{{Actor: domain.Orchestrator(), At: parityAt(30), Shape: domain.CompactionSingle}},
			Invocations: []domain.Invocation{{Actor: agent, State: domain.InvocationClosed, StartedAt: parityAt(0), EndedAt: parityAt(100), Usage: usage}},
		},
	}}}
}

// runA is the run id of the parity aggregate.
const runA = "20261023T191332Z-aaaa"

// priced analyses an aggregate the way the application does.
func priced(agg domain.Aggregate) domain.RunReport {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	report, _ := analysis.Price(agg, domain.NewPricingTable(nil), domain.DefaultAnalysisSettings(), src)
	return report.Runs[0]
}

// viaDigest writes the aggregate to a digest and analyses what is read back.
func viaDigest(t *testing.T, agg domain.Aggregate) domain.RunReport {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.digest.json")
	if err := digest.NewStore().Write(context.Background(), path, domain.Digest{Aggregate: agg}, false); err != nil {
		t.Fatalf("write the digest: %v", err)
	}
	loaded, _, err := digest.NewStore().Read(context.Background(), path)
	if err != nil {
		t.Fatalf("read the digest: %v", err)
	}
	return priced(loaded.Aggregate)
}

// parityRow is the call row at the given second of the parity aggregate.
func parityRow(t *testing.T, view string, sec int) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if strings.Contains(l, parityLabel(sec)) {
			return l
		}
	}
	t.Fatalf("no call row at %s\n%s", parityLabel(sec), view)
	return ""
}

func parityView(t *testing.T, run domain.RunReport, ref domain.ActorRef) string {
	t.Helper()
	colourOff(t)
	return stripStyling(screens.NewSessionScreen(run, ref, screens.ScreenContext, tableW, tallH, paletteStyles()).View(tableW, tallH))
}

func TestSessionParity_DigestLoadedRunShowsTheSameScreenAsTheLiveRun(t *testing.T) {
	agg := parityAggregate()
	live, restored := priced(agg), viaDigest(t, agg)
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}

	for _, ref := range []domain.ActorRef{domain.Orchestrator(), agent} {
		want, got := parityView(t, live, ref), parityView(t, restored, ref)

		if got != want {
			t.Errorf("%s: the digest-loaded Session screen differs from the live one\nlive:\n%s\ndigest:\n%s", ref.Label(), want, got)
		}
	}
}

func TestSessionParity_TheComparedScreensShowToolsAndGrowth(t *testing.T) {
	// Guards the parity test above against comparing two screens that show nothing.
	live := priced(parityAggregate())
	orch := parityView(t, live, domain.Orchestrator())
	agent := parityView(t, live, domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"})

	assertContains(t, parityRow(t, orch, 10), "read_file", screens.FormatTokens(domain.Tokens(400)))
	assertContains(t, parityRow(t, orch, 40), "none")
	assertContains(t, parityRow(t, agent, 12), "glob", screens.FormatTokens(domain.Tokens(201)))
	assertContains(t, parityRow(t, agent, 22), "glob", "negative")
	assertNotContains(t, parityRow(t, agent, 32), "negative", "not measurable")
}
