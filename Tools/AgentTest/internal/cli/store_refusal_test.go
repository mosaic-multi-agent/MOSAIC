package cli_test

// Tests that the store command reports a report refused for an unverified
// subject model: per report with its reason, and in the summary line.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/resultstore"
)

func refusedResult() resultstore.StoreResult {
	return resultstore.StoreResult{
		Stored:                 1,
		SkippedUnverifiedModel: 1,
		Reports: []resultstore.StoredReport{
			{SourcePath: "good.json", TargetPath: "/T/good.json"},
			{
				SourcePath: "bad.json",
				Skipped:    true,
				SkipReason: resultstore.SkipUnverifiedModel,
				Message:    "no observed model",
			},
		},
	}
}

func runStoreWith(t *testing.T, result resultstore.StoreResult) (int, string) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts := storeBaseOpts(stdout, stderr, captureStoreFunc(nil, result, nil), nil)
	code := cli.Execute(context.Background(), []string{"store", "good.json", "bad.json"}, opts)
	return code, stdout.String()
}

func TestStoreCommand_RefusedReport_SummaryLineCountsRefusal(t *testing.T) {
	code, out := runStoreWith(t, refusedResult())

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess (refusal is reported, not an error)", code)
	}
	if !strings.Contains(out, "Stored 1 reports") || !strings.Contains(out, "1 refused") {
		t.Errorf("stdout lacks stored count and \"1 refused\":\n%s", out)
	}
	if strings.Contains(out, "$") {
		t.Errorf("stdout shows cost:\n%s", out)
	}
}

func TestStoreCommand_RefusedReport_IsListedAsRefusedWithReason(t *testing.T) {
	_, out := runStoreWith(t, refusedResult())

	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "bad.json") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("stdout does not list the refused report:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(line), "refused") || !strings.Contains(line, "no observed model") {
		t.Errorf("refused report line %q must say refused and show the reason", line)
	}
}

func TestStoreCommand_StoredReport_IsNotListedAsRefused(t *testing.T) {
	_, out := runStoreWith(t, refusedResult())

	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "good.json") && strings.Contains(strings.ToLower(l), "refused") {
			t.Errorf("stored report listed as refused: %q", l)
		}
	}
}
