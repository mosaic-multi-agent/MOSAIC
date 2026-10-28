package cli_test

// Tests for the "usage not fully ingested" notice in the report and in
// `total --format json`, built from report and total values.
//
// Rules under test:
//   - Text report: a run with pending usage says "usage not fully ingested",
//     how many records are pending and the exact ingest-otel command for the run.
//   - An unreadable ingest state says so without pointing at ingest-otel (that
//     command would skip such a state).
//   - Runs without anything pending show no notice (and no JSON keys).
//   - JSON report: the run object gains "usage_ingest"; nothing else changes.
//   - total: the committed keys stay; two keys are added only when they apply.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func vwRunWithUsageIngest(state domain.UsageIngestState) domain.RunReport {
	run := vwRun()
	run.UsageIngest = state
	return run
}

func TestEncodeText_PendingUsage_SaysNotFullyIngestedWithCountAndCommand(t *testing.T) {
	run := vwRunWithUsageIngest(domain.UsageIngestState{PendingRecords: 3})

	out := encodeTextReport(t, run)

	if !strings.Contains(out, "usage not fully ingested") {
		t.Fatalf("text report lacks the notice:\n%s", out)
	}
	if !strings.Contains(out, "3 records pending") {
		t.Errorf("notice lacks the pending count:\n%s", out)
	}
	if want := "mosaic-log-analyzer ingest-otel --run " + run.Run.ID; !strings.Contains(out, want) {
		t.Errorf("notice lacks the command %q:\n%s", want, out)
	}
}

func TestEncodeText_UnreadableIngestState_SaysSoWithoutPointingAtIngestOtel(t *testing.T) {
	out := encodeTextReport(t, vwRunWithUsageIngest(domain.UsageIngestState{StateUnreadable: true}))

	if !strings.Contains(out, "usage not fully ingested: ingest state unreadable by this version") {
		t.Errorf("text report lacks the unreadable-state notice:\n%s", out)
	}
	if strings.Contains(out, "ingest-otel") {
		t.Errorf("unreadable-state notice points at ingest-otel, which would skip such a state:\n%s", out)
	}
}

func TestEncodeText_NothingPending_ShowsNoNotice(t *testing.T) {
	out := encodeTextReport(t, vwRunWithUsageIngest(domain.UsageIngestState{}))

	if strings.Contains(out, "not fully ingested") || strings.Contains(out, "ingest-otel") {
		t.Errorf("text report shows an ingest notice for a run with nothing pending:\n%s", out)
	}
}

func TestEncodeText_PendingNoticeBelongsToTheRunThatHasIt(t *testing.T) {
	pending := vwRunWithUsageIngest(domain.UsageIngestState{PendingRecords: 2})
	pending.Run = domain.NamedRun("20261031T110000Z-c001")
	clean := vwRun()
	clean.Run = domain.NamedRun("20261031T110000Z-c003")

	out := encodeTextReport(t, clean, pending)

	i := strings.Index(out, "Run: 20261031T110000Z-c003")
	j := strings.Index(out, "Run: 20261031T110000Z-c001")
	if i < 0 || j < 0 {
		t.Fatalf("text report lacks one of the runs:\n%s", out)
	}
	if strings.Contains(sectionOf(out, i), "not fully ingested") {
		t.Errorf("notice appears under the run with nothing pending:\n%s", sectionOf(out, i))
	}
	if !strings.Contains(sectionOf(out, j), "not fully ingested") {
		t.Errorf("notice missing under the run with pending usage:\n%s", sectionOf(out, j))
	}
}

func TestEncodeReport_PendingUsage_RunObjectCarriesUsageIngest(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRunWithUsageIngest(domain.UsageIngestState{PendingRecords: 3}))))

	ing := objectAt(t, run, "usage_ingest")

	if ing["pending_records"] != float64(3) {
		t.Errorf("usage_ingest = %v, want pending_records 3", ing)
	}
	if ing["state_unreadable"] == true {
		t.Errorf("usage_ingest = %v, want state_unreadable not true", ing)
	}
}

func TestEncodeReport_UnreadableIngestState_RunObjectFlagsIt(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRunWithUsageIngest(domain.UsageIngestState{StateUnreadable: true}))))

	ing := objectAt(t, run, "usage_ingest")

	if ing["state_unreadable"] != true {
		t.Errorf("usage_ingest = %v, want state_unreadable true", ing)
	}
}

func TestEncodeReport_NothingPending_RunObjectHasNoUsageIngestKey(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	if _, present := run["usage_ingest"]; present {
		t.Errorf("run object has a \"usage_ingest\" key; keys = %v", sortedKeys(run))
	}
}

// committedTotalKeys is the field set of `total --format json` for a fully
// priced run, fixed before the pending-usage keys were added.
var committedTotalKeys = []string{
	"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
}

func pricedTotal(t *testing.T) app.RunTotal {
	t.Helper()
	return totalWithAssumedTier(t, true)
}

func TestEncodeRunTotal_PendingUsage_AddsTheTwoKeysAndKeepsTheCommittedOnes(t *testing.T) {
	total := pricedTotal(t)
	total.PendingUsageRecords = 3

	m, keys := encodedKeys(t, total)

	assertKeySet(t, "total", keys, append(append([]string{}, committedTotalKeys...),
		"usage_not_fully_ingested", "pending_usage_records"))
	if m["usage_not_fully_ingested"] != true || m["pending_usage_records"] != float64(3) {
		t.Errorf("usage_not_fully_ingested = %v, pending_usage_records = %v, want true and 3",
			m["usage_not_fully_ingested"], m["pending_usage_records"])
	}
}

func TestEncodeRunTotal_UnreadableIngestState_FlagsItWithoutACount(t *testing.T) {
	total := pricedTotal(t)
	total.UsageStateUnreadable = true

	m, keys := encodedKeys(t, total)

	assertKeySet(t, "total", keys, append(append([]string{}, committedTotalKeys...), "usage_not_fully_ingested"))
	if m["usage_not_fully_ingested"] != true {
		t.Errorf("usage_not_fully_ingested = %v, want true", m["usage_not_fully_ingested"])
	}
}

func TestEncodeRunTotal_NothingPending_KeepsExactlyTheCommittedKeys(t *testing.T) {
	_, keys := encodedKeys(t, pricedTotal(t))

	assertKeySet(t, "total", keys, committedTotalKeys)
}

func TestEncodeRunTotal_PendingUsage_DoesNotChangeTheCommittedValues(t *testing.T) {
	plain := pricedTotal(t)
	pending := pricedTotal(t)
	pending.PendingUsageRecords = 3

	var a, b bytes.Buffer
	if err := cli.EncodeRunTotal(&a, plain); err != nil {
		t.Fatal(err)
	}
	if err := cli.EncodeRunTotal(&b, pending); err != nil {
		t.Fatal(err)
	}
	var ma, mb map[string]any
	if err := json.Unmarshal(a.Bytes(), &ma); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b.Bytes(), &mb); err != nil {
		t.Fatal(err)
	}
	if _, added := mb["pending_usage_records"]; !added {
		t.Fatal("pending_usage_records missing; the comparison below would prove nothing")
	}
	delete(mb, "usage_not_fully_ingested")
	delete(mb, "pending_usage_records")
	ja, _ := json.Marshal(ma)
	jb, _ := json.Marshal(mb)
	if string(ja) != string(jb) {
		t.Errorf("committed values changed:\n plain   %s\n pending %s", ja, jb)
	}
}
