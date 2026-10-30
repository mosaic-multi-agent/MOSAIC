package analysis_test

// Tests that a report carries the effective currency of the pricing table and
// lists, for each unpriced model that a currency mismatch left unpriced, the
// reason. Models unpriced for any other reason, and excluded keys the report
// does not use, have no reason.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	curGlobalPath = "/cfg/global/pricing.yaml"
	curUsedA      = domain.ModelID("a-model")
	curUsedZ      = domain.ModelID("z-model")
	curUsedAcme   = domain.ModelID("m-acme")
	curPlain      = domain.ModelID("plain-unpriced")
	curPricedOK   = domain.ModelID("priced-model")
	curNotUsed    = domain.ModelID("never-used")
)

func curExcluded(provider domain.ProviderID, model domain.ModelID) domain.ExcludedPrice {
	return domain.ExcludedPrice{
		Key:      domain.PricingKey{Provider: provider, Model: model},
		Level:    domain.LevelGlobal,
		Path:     curGlobalPath,
		Currency: domain.CurrencyUSD,
	}
}

func curReason(projectCurrency string) string {
	return "priced only in the global file (" + curGlobalPath + ") in USD; the project file's currency is " + projectCurrency
}

// curAggregate holds one agent whose requests use every model of the tests.
func curAggregate() domain.Aggregate {
	use := func(model domain.ModelID, provider domain.ProviderID) domain.InvocationUsage {
		inv := testInvocationUsage(model, domain.TokenUsage{Input: domain.Tokens(1000)})
		inv.Provider = provider
		return inv
	}
	return singleAgentAggregate(testRunRef, "Agent#1",
		use(curPricedOK, ""),
		use(curUsedZ, ""),
		use(curUsedA, ""),
		use(curUsedAcme, "acme"),
		use(curPlain, ""),
	)
}

// curTable is a EUR table: one priced model, three used models excluded by a
// mismatch, and one excluded model that no request uses.
func curTable(t *testing.T) domain.PricingTable {
	return domain.NewPricingTableIn(domain.CurrencyEUR,
		[]domain.ModelPricing{flatModelPricing(t, curPricedOK)},
		[]domain.ExcludedPrice{
			curExcluded("", curUsedZ),
			curExcluded("", curUsedA),
			curExcluded("acme", curUsedAcme),
			curExcluded("", curNotUsed),
		})
}

func TestPrice_Currency_ReportCarriesTheTableCurrency(t *testing.T) {
	for _, cur := range []domain.CurrencyCode{domain.CurrencyEUR, domain.CurrencyUSD} {
		table := domain.NewPricingTableIn(cur, []domain.ModelPricing{flatModelPricing(t, curPricedOK)}, nil)
		agg := singleAgentAggregate(testRunRef, "Agent#1",
			testInvocationUsage(curPricedOK, domain.TokenUsage{Input: domain.Tokens(100)}))

		report, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), testSource())

		if report.Currency != cur {
			t.Errorf("Report.Currency = %q, want %q", report.Currency, cur)
		}
	}
}

func TestPrice_Currency_EmptyAggregateStillCarriesTheTableCurrency(t *testing.T) {
	table := domain.NewPricingTableIn(domain.CurrencyEUR, nil, nil)

	report, _ := analysis.Price(domain.Aggregate{}, table, domain.DefaultAnalysisSettings(), testSource())

	if report.Currency != domain.CurrencyEUR {
		t.Errorf("Report.Currency = %q, want EUR", report.Currency)
	}
}

func TestPrice_Currency_PlainTableReadsAsUSD(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, curPricedOK)})

	report, _ := analysis.Price(curAggregate(), table, domain.DefaultAnalysisSettings(), testSource())

	if report.Currency != domain.CurrencyUSD {
		t.Errorf("Report.Currency = %q, want USD", report.Currency)
	}
}

func TestAnalyze_Currency_ReportCarriesTheTableCurrency(t *testing.T) {
	report := analysis.Analyze(analysis.Input{}, curTable(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	if report.Currency != domain.CurrencyEUR {
		t.Errorf("Report.Currency = %q, want EUR", report.Currency)
	}
}

func TestPrice_UnpricedReasons_OneEntryPerExcludedUnpricedModelSortedByKey(t *testing.T) {
	report, _ := analysis.Price(curAggregate(), curTable(t), domain.DefaultAnalysisSettings(), testSource())

	want := []domain.UnpricedReason{
		{Key: domain.PricingKey{Model: curUsedA}, Reason: curReason("EUR")},
		{Key: domain.PricingKey{Provider: "acme", Model: curUsedAcme}, Reason: curReason("EUR")},
		{Key: domain.PricingKey{Model: curUsedZ}, Reason: curReason("EUR")},
	}
	// The report lists reasons in the order of its sorted unpriced models.
	var inReportOrder []domain.UnpricedReason
	for _, k := range report.UnpricedModels {
		for _, r := range want {
			if r.Key == k {
				inReportOrder = append(inReportOrder, r)
			}
		}
	}
	if len(report.UnpricedReasons) != 3 {
		t.Fatalf("UnpricedReasons = %+v, want 3 entries", report.UnpricedReasons)
	}
	if !reflect.DeepEqual(report.UnpricedReasons, inReportOrder) {
		t.Errorf("UnpricedReasons = %+v, want %+v (sorted by key like UnpricedModels)", report.UnpricedReasons, inReportOrder)
	}
	if report.UnpricedReasons[0].Key.Model != curUsedA {
		t.Errorf("first reason is for %v, want a-model before z-model", report.UnpricedReasons[0].Key)
	}
}

func TestPrice_UnpricedReasons_ModelUnpricedForAnotherReasonHasNoEntry(t *testing.T) {
	report, _ := analysis.Price(curAggregate(), curTable(t), domain.DefaultAnalysisSettings(), testSource())

	plain := domain.PricingKey{Model: curPlain}
	if !containsKey(report.UnpricedModels, plain) {
		t.Fatalf("precondition: %v must be unpriced; unpriced = %v", plain, report.UnpricedModels)
	}
	for _, r := range report.UnpricedReasons {
		if r.Key == plain || r.Key.Model == curPricedOK {
			t.Errorf("unexpected reason %+v for a model not left unpriced by a mismatch", r)
		}
	}
	if len(report.UnpricedReasons) == 0 {
		t.Error("the mismatch-excluded models must still have reasons")
	}
}

func TestPrice_UnpricedReasons_ExcludedKeyTheReportDoesNotUseHasNoEntry(t *testing.T) {
	report, _ := analysis.Price(curAggregate(), curTable(t), domain.DefaultAnalysisSettings(), testSource())

	if len(report.UnpricedReasons) == 0 {
		t.Fatal("precondition: the used excluded models must have reasons")
	}
	for _, r := range report.UnpricedReasons {
		if r.Key.Model == curNotUsed {
			t.Errorf("reason %+v for a key the report does not use", r)
		}
	}
}

func TestPrice_UnpricedReasons_NoExclusionsMeansNoReasons(t *testing.T) {
	table := domain.NewPricingTableIn(domain.CurrencyEUR, []domain.ModelPricing{flatModelPricing(t, curPricedOK)}, nil)

	report, _ := analysis.Price(curAggregate(), table, domain.DefaultAnalysisSettings(), testSource())

	if len(report.UnpricedModels) == 0 {
		t.Fatal("precondition: the plain unpriced models must be listed")
	}
	if len(report.UnpricedReasons) != 0 {
		t.Errorf("UnpricedReasons = %+v, want none without a currency mismatch", report.UnpricedReasons)
	}
}

func TestPrice_UnpricedReasons_ReasonTextNamesGlobalCurrencyAndProjectCurrency(t *testing.T) {
	ex := curExcluded("", curUsedA)
	ex.Currency = domain.CurrencyEUR
	ex.Path = "/other/place/pricing.yaml"
	table := domain.NewPricingTableIn(domain.CurrencyUSD, nil, []domain.ExcludedPrice{ex})
	agg := singleAgentAggregate(testRunRef, "Agent#1",
		testInvocationUsage(curUsedA, domain.TokenUsage{Input: domain.Tokens(10)}))

	report, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), testSource())

	want := "priced only in the global file (/other/place/pricing.yaml) in EUR; the project file's currency is USD"
	if len(report.UnpricedReasons) != 1 || report.UnpricedReasons[0].Reason != want {
		t.Errorf("UnpricedReasons = %+v, want one reason %q", report.UnpricedReasons, want)
	}
}

func containsKey(keys []domain.PricingKey, k domain.PricingKey) bool {
	for _, c := range keys {
		if c == k {
			return true
		}
	}
	return false
}
