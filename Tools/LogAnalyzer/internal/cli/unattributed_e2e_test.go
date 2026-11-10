package cli_test

// End-to-end tests of unattributed usage on the tool-local fixtures in
// testdata/unattributed, through the real scanner, reader and pricing, driven
// through cli.Execute.
//
// Fixture logs/ (model claude-sonnet-4, $3.00 / $15.00 per million tokens):
//
//	20261030T120000Z-d001  orchestrator in 1000 out 100 = 4,500,000 nanodollars
//	                       worker#1     in 2000 out 200 = 9,000,000
//	                       unattributed file: u-1 (500/50, then re-observed as
//	                       600/60) and u-2 (300/30) = in 900 out 90 = 4,050,000
//	                       and a decoy file inside .otel-ingest/ that must not count
//	20261030T140000Z-d003  orchestrator in 100 out 10, no unattributed file
//
// Fixture copilot-nousage/ is a Copilot run whose logs hold no usage at all.
//
// Rules under test:
//   - total --format json counts the unattributed usage in tokens and money and
//     keeps its committed field set.
//   - The report shows it as its own actor and as an overview share; the shares
//     cover the run total; no attribution finding is raised.
//   - A run without the file is unchanged.
//   - A Copilot run without usage reports usage as not provided, never zero.

import (
	"encoding/json"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
)

const (
	unattrLogs        = "../../testdata/unattributed/logs"
	unattrNoUsageLogs = "../../testdata/unattributed/copilot-nousage"
	unattrRunWith     = "20261030T120000Z-d001"
	unattrRunWithout  = "20261030T140000Z-d003"
	unattrRunNoUsage  = "20261030T130000Z-d002"
)

// unattrTotalJSON runs `total --format json` and decodes the output.
func unattrTotalJSON(t *testing.T, path, runID string) map[string]any {
	t.Helper()
	code, stdout, stderr := e2eRun(t, "total", "--format", "json", "--path", path, "--run", runID)
	if code != cli.ExitSuccess {
		t.Fatalf("total on %s/%s exited %d: %s", path, runID, code, stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("total output is not JSON: %v\n%s", err, stdout)
	}
	return m
}

func unattrRunObject(t *testing.T, path, runID string) map[string]any {
	t.Helper()
	run := e2eRunsOf(e2eReport(t, path))[runID]
	if run == nil {
		t.Fatalf("report has no run %s", runID)
	}
	return run
}

func TestE2E_Total_IncludesUnattributedUsageInTokensAndMoney(t *testing.T) {
	total := unattrTotalJSON(t, unattrLogs, unattrRunWith)

	tokens := total["tokens"].(map[string]any)
	if tokens["input"] != float64(3_900) || tokens["output"] != float64(390) {
		t.Errorf("tokens = %v, want input 3900 (1000+2000+900) and output 390 (100+200+90)", tokens)
	}
	money := total["money"].(map[string]any)
	if money["state"] != "known" || money["amount"] != "0.01755" {
		t.Errorf("money = %v, want known 0.01755 (17,550,000 nanodollars)", money)
	}
	if total["complete"] != true {
		t.Errorf("complete = %v, want true", total["complete"])
	}
}

func TestE2E_Total_KeepsTheCommittedFieldSet(t *testing.T) {
	total := unattrTotalJSON(t, unattrLogs, unattrRunWith)

	assertKeySet(t, "total", sortedKeys(total), []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
	})
	if total["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\"", total["schema_version"])
	}
}

func TestE2E_Total_RunWithoutUnattributedFile_IsUnchanged(t *testing.T) {
	total := unattrTotalJSON(t, unattrLogs, unattrRunWithout)

	tokens := total["tokens"].(map[string]any)
	if tokens["input"] != float64(100) || tokens["output"] != float64(10) {
		t.Errorf("tokens = %v, want input 100 and output 10 of its own orchestrator", tokens)
	}
}

func TestE2E_Report_ShowsUnattributedUsageAsItsOwnActor(t *testing.T) {
	run := unattrRunObject(t, unattrLogs, unattrRunWith)

	un, ok := run["unattributed"].(map[string]any)
	if !ok {
		t.Fatalf("run has no \"unattributed\" object; keys = %v", sortedKeys(run))
	}
	tokens := un["totals"].(map[string]any)["tokens"].(map[string]any)
	if un["actor"] != "unattributed" || tokens["input"] != float64(900) || tokens["output"] != float64(90) {
		t.Errorf("unattributed = actor %v tokens %v, want actor \"unattributed\" with input 900 output 90 (decoy in .otel-ingest/ excluded, u-1 superseded)", un["actor"], tokens)
	}
	orch := run["orchestrator"].(map[string]any)["totals"].(map[string]any)["tokens"].(map[string]any)
	if orch["input"] != float64(1_000) {
		t.Errorf("orchestrator input = %v, want 1000: never folded with unattributed usage", orch["input"])
	}
	if agents := run["agents"].([]any); len(agents) != 1 {
		t.Errorf("agents = %d, want 1 (worker#1 only)", len(agents))
	}
}

func TestE2E_Report_OverviewSharesCoverTheRunTotal(t *testing.T) {
	ov := unattrRunObject(t, unattrLogs, unattrRunWith)["overview"].(map[string]any)

	vwAssertRatio(t, "overview", ov, "orchestrator_share", 4_500_000, 17_550_000)
	vwAssertRatio(t, "overview", ov, "subagent_share", 9_000_000, 17_550_000)
	vwAssertRatio(t, "overview", ov, "unattributed_share", 4_050_000, 17_550_000)
}

func TestE2E_Report_UnattributedRecordsRaiseNoFindings(t *testing.T) {
	report := e2eReport(t, unattrLogs)
	if unattrRunObject(t, unattrLogs, unattrRunWith)["unattributed"] == nil {
		t.Fatal("the unattributed stream was not read, so absence of findings would prove nothing")
	}

	for _, f := range e2eFindingsOf(report) {
		path, _ := f["path"].(string)
		if strings.Contains(path, unattrRunWith) {
			t.Errorf("finding on the run with unattributed usage (fixtures use a supported schema version): %v", f)
		}
	}
}

func TestE2E_Report_RunWithoutUnattributedFile_HasNoUnattributedKeys(t *testing.T) {
	run := unattrRunObject(t, unattrLogs, unattrRunWithout)

	if _, present := run["unattributed"]; present {
		t.Errorf("run has an \"unattributed\" key; keys = %v", sortedKeys(run))
	}
	if _, present := run["overview"].(map[string]any)["unattributed_share"]; present {
		t.Error("overview has an \"unattributed_share\" key for a run without unattributed usage")
	}
}

func TestE2E_TextReport_NamesUnattributedUsageOnlyForTheRunThatHasIt(t *testing.T) {
	code, stdout, stderr := e2eRun(t, "report", "--format", "text", "--path", unattrLogs)
	if code != cli.ExitSuccess {
		t.Fatalf("report exited %d: %s", code, stderr)
	}

	i := strings.Index(stdout, "Run: "+unattrRunWithout)
	j := strings.Index(stdout, "Run: "+unattrRunWith)
	if i < 0 || j < 0 {
		t.Fatalf("text report lacks one of the runs:\n%s", stdout)
	}
	if !strings.Contains(strings.ToLower(sectionOf(stdout, j)), "unattributed") {
		t.Errorf("section of %s does not name the unattributed usage:\n%s", unattrRunWith, sectionOf(stdout, j))
	}
	if strings.Contains(strings.ToLower(sectionOf(stdout, i)), "unattributed") {
		t.Errorf("section of %s names unattributed usage:\n%s", unattrRunWithout, sectionOf(stdout, i))
	}
}

// sectionOf returns the text from start up to the next "Run: " header.
func sectionOf(out string, start int) string {
	rest := out[start+1:]
	if next := strings.Index(rest, "\nRun: "); next >= 0 {
		return out[start : start+1+next]
	}
	return out[start:]
}

func TestE2E_CopilotRunWithoutUsage_TotalShowsUsageAsNotProvided(t *testing.T) {
	total := unattrTotalJSON(t, unattrNoUsageLogs, unattrRunNoUsage)

	tokens := total["tokens"].(map[string]any)
	for _, category := range []string{"input", "cache_read", "cache_creation", "output"} {
		if v := tokens[category]; v != nil {
			t.Errorf("tokens.%s = %v, want null (not provided), never zero or estimated", category, v)
		}
	}
	money := total["money"].(map[string]any)
	if money["state"] != "no_data" {
		t.Errorf("money = %v, want state no_data", money)
	}
	if _, present := money["amount"]; present {
		t.Errorf("money has an amount %v for a run without usage", money["amount"])
	}
}

func TestE2E_CopilotRunWithoutUsage_ReportShowsUsageAsNotProvided(t *testing.T) {
	run := unattrRunObject(t, unattrNoUsageLogs, unattrRunNoUsage)

	totals := run["totals"].(map[string]any)
	tokens := totals["tokens"].(map[string]any)
	for _, category := range []string{"input", "cache_read", "cache_creation", "output", "reasoning"} {
		if v := tokens[category]; v != nil {
			t.Errorf("totals.tokens.%s = %v, want null (not provided)", category, v)
		}
	}
	if state := totals["money"].(map[string]any)["total"].(map[string]any)["state"]; state != "no_data" {
		t.Errorf("money total state = %v, want no_data", state)
	}
	if _, present := run["unattributed"]; present {
		t.Error("a run without any usage must not invent an unattributed actor")
	}
}
