package cli_test

// Regression tests pinning the exact field set of `total --format json`
// (EncodeRunTotal), which external tools decode.
//
// Rules under test:
//   - A run whose figures include assumed-tier cache writes and a cache-write
//     tier split encodes with exactly the committed keys: no assumed-tier field,
//     no tier split, no extra token or money keys.
//   - The optional keys (unpriced_models, partial_money, unknown_run_*) appear
//     only under their existing conditions.

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// totalWithAssumedTier builds the RunTotal of a run that carries a tier split
// and assumed-tier cache-write tokens in its report figures.
func totalWithAssumedTier(t *testing.T, complete bool) app.RunTotal {
	t.Helper()
	totals := domain.Totals{
		Tokens: domain.TokenUsage{
			Input:           domain.Tokens(1_000),
			CacheRead:       domain.Tokens(2_000),
			CacheCreation:   domain.Tokens(150_000),
			CacheCreation5m: domain.Tokens(100_000),
			CacheCreation1h: domain.Tokens(30_000),
			Output:          domain.Tokens(500),
		},
		Money: domain.CategoryMoney{
			Total:    domain.KnownMoney(domain.Money(1_500_000_000)),
			Complete: complete,
		},
		AssumedTierCacheWrite: 20_000,
	}
	report := domain.Report{
		Runs: []domain.RunReport{{
			Run:    domain.NamedRun(cliTestRunID),
			Totals: totals,
		}},
	}
	if !complete {
		report.Runs[0].UnpricedModels = []domain.PricingKey{{Model: "unpriced-model"}}
	}
	total, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
	}
	return total
}

func encodedKeys(t *testing.T, total app.RunTotal) (map[string]any, []string) {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	return m, sortedKeys(m)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertKeySet(t *testing.T, name string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s keys = %v, want exactly %v", name, got, want)
	}
}

func TestEncodeRunTotal_RunWithAssumedTierAndSplit_KeepsExactTopLevelFieldSet(t *testing.T) {
	_, keys := encodedKeys(t, totalWithAssumedTier(t, true))

	assertKeySet(t, "top level", keys, []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
	})
}

func TestEncodeRunTotal_RunWithAssumedTierAndSplit_KeepsExactTokenAndMoneyFieldSets(t *testing.T) {
	m, _ := encodedKeys(t, totalWithAssumedTier(t, true))

	tokens, _ := m["tokens"].(map[string]any)
	money, _ := m["money"].(map[string]any)

	assertKeySet(t, "tokens", sortedKeys(tokens), []string{"input", "cache_read", "cache_creation", "output"})
	assertKeySet(t, "money", sortedKeys(money), []string{"state", "amount"})
	if got, _ := tokens["cache_creation"].(float64); got != 150_000 {
		t.Errorf("tokens.cache_creation = %v, want the cache-write total 150000", tokens["cache_creation"])
	}
}

func TestEncodeRunTotal_PartiallyPricedRunWithAssumedTier_AddsOnlyTheExistingOptionalFields(t *testing.T) {
	_, keys := encodedKeys(t, totalWithAssumedTier(t, false))

	assertKeySet(t, "top level", keys, []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
		"unpriced_models", "partial_money",
	})
}
