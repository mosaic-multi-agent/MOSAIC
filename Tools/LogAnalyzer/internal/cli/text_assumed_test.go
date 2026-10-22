package cli_test

// Tests for the assumed-tier disclosure in the text encoder (EncodeText).
//
// Rules under test:
//   - A totals block with assumed-tier cache writes shows a line that names the
//     assumed tier and the grouped token count.
//   - A report whose assumed-tier count is zero never mentions "assumed".
//   - A notice follows the unpriced-models section when named runs or the
//     unattributable bucket carry assumed-tier tokens.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func encodeTextString(t *testing.T, report domain.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeText(&buf, report); err != nil {
		t.Fatalf("EncodeText returned error: %v", err)
	}
	return buf.String()
}

func runWithAssumedTier(assumed int64) domain.RunReport {
	return domain.RunReport{
		Run: domain.NamedRun(cliTestRunID),
		Totals: domain.Totals{
			Tokens:                domain.TokenUsage{CacheCreation: domain.Tokens(900_000)},
			AssumedTierCacheWrite: assumed,
		},
	}
}

func TestEncodeText_RunWithAssumedTierTokens_ShowsLabelledGroupedCount(t *testing.T) {
	report := domain.Report{
		Currency: domain.Currency,
		Runs:     []domain.RunReport{runWithAssumedTier(12_345)},
	}

	out := encodeTextString(t, report)

	var found bool
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(line), "assumed") && strings.Contains(line, "12,345") {
			found = true
		}
	}
	if !found {
		t.Errorf("no line names the assumed tier together with 12,345; output:\n%s", out)
	}
}

func TestEncodeText_ActorWithAssumedTierTokens_ShowsCountForTheActor(t *testing.T) {
	run := runWithAssumedTier(0)
	run.Agents = []domain.ActorReport{{
		Actor:  domain.AgentInstance("Implementer#3"),
		Totals: domain.Totals{AssumedTierCacheWrite: 4_567},
	}}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}}

	out := encodeTextString(t, report)

	actorAt := strings.Index(out, "Agent: Implementer#3")
	if actorAt < 0 {
		t.Fatalf("actor block not found; output:\n%s", out)
	}
	if !lineWithAssumedAndCount(out[actorAt:], "4,567") {
		t.Errorf("actor assumed-tier count 4,567 not disclosed on one line inside the actor block; output:\n%s", out)
	}
}

func TestEncodeText_NoAssumedTierTokens_NeverMentionsAssumed(t *testing.T) {
	report := domain.Report{
		Currency:       domain.Currency,
		Runs:           []domain.RunReport{runWithAssumedTier(0)},
		UnpricedModels: []domain.ModelID{"mystery-model"},
	}

	out := encodeTextString(t, report)

	if strings.Contains(strings.ToLower(out), "assumed") {
		t.Errorf("output mentions an assumed tier although none was used; output:\n%s", out)
	}
}

func TestEncodeText_AssumedTierTokens_AddNoticeAfterUnpricedModelsSection(t *testing.T) {
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        domain.Totals{AssumedTierCacheWrite: 4_321},
		UnpricedModels: []domain.ModelID{"mystery-model"},
	}

	out := encodeTextString(t, report)

	unpricedAt := strings.Index(out, "mystery-model")
	assumedAt := strings.LastIndex(strings.ToLower(out), "assumed")
	if unpricedAt < 0 {
		t.Fatalf("unpriced model not listed; output:\n%s", out)
	}
	if assumedAt < unpricedAt {
		t.Errorf("no assumed-tier notice after the unpriced-models section; output:\n%s", out)
	}
}

func TestEncodeText_UnattributableAssumedTierTokens_AddNoticeAfterUnpricedModelsSection(t *testing.T) {
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 8_765},
	}
	report := domain.Report{
		Currency:       domain.Currency,
		Unattributable: &unattributable,
		UnpricedModels: []domain.ModelID{"mystery-model"},
	}

	out := encodeTextString(t, report)

	unpricedAt := strings.Index(out, "mystery-model")
	assumedAt := strings.LastIndex(strings.ToLower(out), "assumed")
	if unpricedAt < 0 || assumedAt < unpricedAt || !strings.Contains(out[assumedAt:], "8,765") {
		t.Errorf("no assumed-tier notice with 8,765 after the unpriced-models section; output:\n%s", out)
	}
}

func lineWithAssumedAndCount(out, count string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(line), "assumed") && strings.Contains(line, count) {
			return true
		}
	}
	return false
}

func TestEncodeText_AssumedTierInAllRunsAndUnattributable_NoticeShowsSummedCount(t *testing.T) {
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 8_765},
	}
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        domain.Totals{AssumedTierCacheWrite: 4_321},
		Unattributable: &unattributable,
		UnpricedModels: []domain.ModelID{"mystery-model"},
	}

	out := encodeTextString(t, report)

	unpricedAt := strings.Index(out, "mystery-model")
	if unpricedAt < 0 {
		t.Fatalf("unpriced model not listed; output:\n%s", out)
	}
	if !lineWithAssumedAndCount(out[unpricedAt:], "13,086") {
		t.Errorf("no assumed-tier notice with summed count 13,086 after the unpriced-models section; output:\n%s", out)
	}
}

func TestEncodeText_AssumedTierTokensWithoutUnpricedModels_StillDisclosesCount(t *testing.T) {
	report := domain.Report{
		Currency: domain.Currency,
		AllRuns:  domain.Totals{AssumedTierCacheWrite: 4_321},
	}

	out := encodeTextString(t, report)

	if !lineWithAssumedAndCount(out, "4,321") {
		t.Errorf("no line names the assumed tier together with 4,321; output:\n%s", out)
	}
}
