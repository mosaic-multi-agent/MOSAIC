package cli_test

// Tests for unattributed usage in the report JSON and text output, built from
// report values (the analysis is tested elsewhere).
//
// Rules under test:
//   - A run with unattributed usage encodes it as its own "unattributed" actor
//     object and in the overview as totals plus a share; it is never listed
//     among the agents.
//   - A run without it encodes none of these keys.
//   - The text report names the unattributed usage and shows its figures only
//     for runs that have it.

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// unattrTotals are distinctive figures (tokens 7,777; 330 nanodollars).
func unattrTotals() domain.Totals {
	return domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(7_777), Output: domain.Tokens(77)},
		Money:  domain.CategoryMoney{Total: vwKnown(330), Complete: true},
	}
}

// vwRunWithUnattributed is vwRun with the unattributed figures of the contract:
// orchestrator 200 + subagents 300 + unattributed 330 over a run total of 830.
func vwRunWithUnattributed() domain.RunReport {
	run := vwRun()
	run.Totals = vwTotals(830, false)
	un := domain.ActorReport{
		Actor:  domain.Unattributed(),
		Models: []domain.ModelID{"m-a"},
		Totals: unattrTotals(),
	}
	run.Unattributed = &un
	ut := unattrTotals()
	run.Overview.Totals = vwTotals(830, false)
	run.Overview.Unattributed = &ut
	run.Overview.OrchestratorShare = domain.Ratio{Numerator: 200, Denominator: 830}
	run.Overview.SubagentShare = domain.Ratio{Numerator: 300, Denominator: 830}
	run.Overview.UnattributedShare = domain.Ratio{Numerator: 330, Denominator: 830}
	return run
}

func TestEncodeReport_UnattributedUsage_IsItsOwnActorObjectOnTheRun(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRunWithUnattributed())))

	un := objectAt(t, run, "unattributed")

	if un["actor"] != "unattributed" {
		t.Errorf("actor = %v, want %q", un["actor"], "unattributed")
	}
	tokens := objectAt(t, objectAt(t, un, "totals"), "tokens")
	if tokens["input"] != float64(7_777) || tokens["output"] != float64(77) {
		t.Errorf("tokens = %v, want input 7777 and output 77", tokens)
	}
	if models := vwList(t, un, "models"); len(models) != 1 || models[0] != "m-a" {
		t.Errorf("models = %v, want [m-a]", models)
	}
}

func TestEncodeReport_UnattributedUsage_IsNotListedAmongAgentsOrFoldedIntoTheOrchestrator(t *testing.T) {
	baseline := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	run := firstRun(t, decodeReportJSON(t, vwReport(vwRunWithUnattributed())))

	if got, want := len(vwList(t, run, "agents")), len(vwList(t, baseline, "agents")); got != want {
		t.Errorf("agents = %d, want %d", got, want)
	}
	orch := objectAt(t, objectAt(t, run, "orchestrator"), "totals")
	if tokens := objectAt(t, orch, "tokens"); tokens["input"] != float64(100) {
		t.Errorf("orchestrator tokens = %v, want its own input 100 unchanged", tokens)
	}
}

func TestEncodeReport_UnattributedUsage_OverviewCarriesTotalsAndShare(t *testing.T) {
	ov := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRunWithUnattributed()))), "overview")

	un := objectAt(t, ov, "unattributed")

	if got := objectAt(t, un, "tokens")["input"]; got != float64(7_777) {
		t.Errorf("overview.unattributed.tokens.input = %v, want 7777", got)
	}
	vwAssertRatio(t, "overview", ov, "unattributed_share", 330, 830)
	vwAssertRatio(t, "overview", ov, "orchestrator_share", 200, 830)
	vwAssertRatio(t, "overview", ov, "subagent_share", 300, 830)
}

func TestEncodeReport_RunWithoutUnattributedUsage_HasNoUnattributedKeys(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	if _, present := run["unattributed"]; present {
		t.Errorf("run object has an \"unattributed\" key; keys = %v", sortedKeys(run))
	}
	ov := objectAt(t, run, "overview")
	for _, key := range []string{"unattributed", "unattributed_share"} {
		if _, present := ov[key]; present {
			t.Errorf("overview has a %q key; keys = %v", key, sortedKeys(ov))
		}
	}
}

func encodeTextReport(t *testing.T, runs ...domain.RunReport) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeText(&buf, domain.Report{Runs: runs, Currency: domain.Currency}); err != nil {
		t.Fatalf("EncodeText: %v", err)
	}
	return buf.String()
}

func TestEncodeText_UnattributedUsage_IsNamedWithItsOwnFigures(t *testing.T) {
	out := encodeTextReport(t, vwRunWithUnattributed())

	lower := strings.ToLower(out)
	idx := strings.Index(lower, "unattributed")
	if idx < 0 {
		t.Fatalf("text report never names the unattributed usage:\n%s", out)
	}
	if !strings.Contains(out[idx:], "7,777") {
		t.Errorf("the unattributed section lacks its input figure 7,777:\n%s", out[idx:])
	}
}

func TestEncodeText_UnattributedUsage_AppearsAsAShareAndAsARow(t *testing.T) {
	out := encodeTextReport(t, vwRunWithUnattributed())

	// Share 330/830 is 39.8% (a rendering rounded to 40% is also accepted).
	sharePattern := regexp.MustCompile(`(?i)unattributed.*(39\.8|40(\.0)?)\s*%`)
	if !sharePattern.MatchString(out) {
		t.Errorf("no overview share line for the unattributed usage (about 39.8%%):\n%s", out)
	}
	// The by-agent row: a line naming the actor without the share, followed by its figures.
	rowAt := -1
	offset := 0
	for _, line := range strings.SplitAfter(out, "\n") {
		if strings.Contains(strings.ToLower(line), "unattributed") && !strings.Contains(line, "%") {
			rowAt = offset
			break
		}
		offset += len(line)
	}
	if rowAt < 0 {
		t.Fatalf("no by-agent row naming the unattributed usage apart from its share:\n%s", out)
	}
	if !strings.Contains(out[rowAt:], "7,777") {
		t.Errorf("the unattributed row lacks its input figure 7,777:\n%s", out[rowAt:])
	}
}

func TestEncodeText_RunWithoutUnattributedUsage_NamesNoUnattributedUsage(t *testing.T) {
	out := encodeTextReport(t, vwRun())

	if strings.Contains(strings.ToLower(out), "unattributed") {
		t.Errorf("text report of a run without unattributed usage mentions it:\n%s", out)
	}
}
