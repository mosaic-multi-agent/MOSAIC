package digest_test

// A run loaded from a digest shows the same tool context growth as the run
// read from logs: per-call growth and the Tools breakdown figures, including
// the cases that depend on turns and compactions the digest restores.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func growthCall(id string, sec int, ctx, out int64, tools domain.RequestedTools) domain.InvocationUsage {
	u := toolsCall(id, sec, ctx, tools)
	u.Usage.Output = domain.Tokens(out)
	return u
}

// growthRunAggregate holds measured, negative, turn-broken and
// compaction-broken singular calls on the orchestrator and on one invocation.
func growthRunAggregate() domain.Aggregate {
	one := func(tool string) domain.RequestedTools { return domain.NewRequestedTools(map[string]int{tool: 1}) }
	none := domain.NewRequestedTools(nil)
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	orch := []domain.InvocationUsage{
		growthCall("o1", 10, 1000, 100, one("read_file")), // measured 400
		growthCall("o2", 20, 1500, 50, one("read_file")),  // compaction between: not singular
		growthCall("o3", 40, 1700, 50, none),
	}
	usage := []domain.InvocationUsage{
		growthCall("a1", 12, 2000, 100, one("glob")),     // measured 201
		growthCall("a2", 22, 2301, 500, one("glob")),     // negative
		growthCall("a3", 32, 2400, 10, one("read_file")), // user turn between: not singular
		growthCall("a4", 42, 2600, 10, none),
	}
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: orch},
		Agents:       []domain.ActorAggregate{{Actor: agent, Invocations: usage}},
		Turns: []domain.Turn{
			{Actor: domain.AgentInstance("coder#1"), Role: domain.TurnUser, At: at(37)},
		},
		Activity: domain.RunActivity{
			Compactions: []domain.Compaction{{Actor: domain.Orchestrator(), At: at(30), Shape: domain.CompactionSingle}},
			Invocations: []domain.Invocation{{
				Actor: agent, State: domain.InvocationClosed, StartedAt: at(0), EndedAt: at(100), Usage: usage,
			}},
		},
	}}}
}

func TestRoundTrip_ToolGrowthOfADigestLoadedRun_EqualsThatOfTheOriginalRun(t *testing.T) {
	agg := growthRunAggregate()
	loaded, findings := roundTrip(t, domain.Digest{Aggregate: agg})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	table := domain.NewPricingTable(nil)

	original, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), src)
	restored, _ := analysis.Price(loaded.Aggregate, table, domain.DefaultAnalysisSettings(), src)

	want, got := original.Runs[0], restored.Runs[0]
	if !reflect.DeepEqual(got.Tools, want.Tools) {
		t.Errorf("digest-loaded tools differ:\n got %+v\nwant %+v", got.Tools, want.Tools)
	}
	if !reflect.DeepEqual(got.Context, want.Context) {
		t.Errorf("digest-loaded per-call growth differs:\n got %+v\nwant %+v", got.Context, want.Context)
	}
	// The comparison is vacuous unless the original run measured something.
	var glob, read domain.ToolFigures
	for _, f := range want.Tools.Run {
		switch f.Tool {
		case "glob":
			glob = f
		case "read_file":
			read = f
		}
	}
	if glob.Growth.Samples != 1 || glob.Growth.Negative != 1 {
		t.Errorf("original glob growth = %+v, want 1 sample and 1 negative", glob.Growth)
	}
	if read.Growth.Samples != 1 {
		t.Errorf("original read_file growth = %+v, want 1 sample (the other two calls are broken by a turn and a compaction)", read.Growth)
	}
}

func TestRead_OlderDigestWithoutRequestedTools_ShowsNoToolGrowth(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "requestedtools", "older.digest.json")
	loaded, _ := readDigest(t, path)

	report, _ := analysis.Price(loaded.Aggregate, domain.NewPricingTable(nil), domain.DefaultAnalysisSettings(),
		domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"})

	for _, run := range report.Runs {
		for _, f := range run.Tools.Run {
			if f.Growth != (domain.ToolGrowth{}) {
				t.Errorf("tool %q growth = %+v, want not provided", f.Tool, f.Growth)
			}
		}
		for _, c := range run.Context.Orchestrator.Calls {
			if c.Growth.State != domain.GrowthNotSingular {
				t.Errorf("call %s growth = %+v, want not singular", c.RecordID, c.Growth)
			}
		}
	}
}
