package app_test

// Tests that a digest-loaded run is re-priced with the current pricing table, so
// its report carries the current effective currency and unpriced reasons exactly
// like a live run, and that loading a digest raises no currency finding.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const currencyGlobalFile = "/test/global/pricing.yaml"

func eurPricing() domain.PricingTable {
	return domain.NewPricingTableIn(domain.CurrencyEUR, []domain.ModelPricing{flatPricing(dgModel)}, nil)
}

// excludedByMismatch is a EUR table in which dgModel was priced only in a global USD file.
func excludedByMismatch() domain.PricingTable {
	return domain.NewPricingTableIn(domain.CurrencyEUR, nil, []domain.ExcludedPrice{{
		Key:      domain.PricingKey{Model: dgModel},
		Level:    domain.LevelGlobal,
		Path:     currencyGlobalFile,
		Currency: domain.CurrencyUSD,
	}})
}

func currencyFindingsOf(r domain.Report) []domain.Finding {
	var out []domain.Finding
	for _, f := range r.Quality.Findings {
		if f.Kind == domain.FindingPricingCurrencyMismatch ||
			strings.Contains(strings.ToLower(f.Detail), "currency") {
			out = append(out, f)
		}
	}
	return out
}

func TestAnalyze_DigestRun_ReportCarriesTheCurrentEffectiveCurrency(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = eurPricing()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	if res.Report.Currency != domain.CurrencyEUR {
		t.Errorf("Report.Currency = %q, want EUR", res.Report.Currency)
	}
}

func TestAnalyze_DigestRun_AmountsComeFromTheCurrentPricingTable(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = eurPricing()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	run, ok := res.Report.FindRun(dgRunA)
	want := domain.KnownMoney(domain.Money(6_000_000_000))
	if !ok || !run.Totals.Money.Complete || run.Totals.Money.Total != want {
		t.Errorf("digest run money = %+v (found=%v), want %+v complete", run.Totals.Money, ok, want)
	}
}

func TestAnalyze_DigestRun_CurrencyMatchesTheSameRunFromLogs(t *testing.T) {
	logEnv := newDigestEnv()
	logEnv.pricing.table = eurPricing()
	logEnv.addLogRun(dgRunA, 2_000_000)
	digEnv := newDigestEnv()
	digEnv.pricing.table = eurPricing()
	digEnv.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	fromLogs := analyzeDGReport(t, logEnv, logEnv.logsPath).Report
	fromDigest := analyzeDGReport(t, digEnv, dgFilePath).Report

	if fromLogs.Currency != domain.CurrencyEUR || fromDigest.Currency != fromLogs.Currency {
		t.Errorf("currency: logs %q, digest %q, want both EUR", fromLogs.Currency, fromDigest.Currency)
	}
}

func TestAnalyze_DigestRun_PricedWithPlainTableReadsAsUSD(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	if res.Report.Currency != domain.CurrencyUSD {
		t.Errorf("Report.Currency = %q, want USD", res.Report.Currency)
	}
}

func TestAnalyze_DigestRun_UnpricedReasonsComeFromTheCurrentTable(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = excludedByMismatch()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	want := "priced only in the global file (" + currencyGlobalFile + ") in USD; the project file's currency is EUR"
	got := res.Report.UnpricedReasons
	if len(got) != 1 || got[0].Key != (domain.PricingKey{Model: dgModel}) || got[0].Reason != want {
		t.Errorf("UnpricedReasons = %+v, want one reason %q for %s", got, want, dgModel)
	}
	if run, _ := res.Report.FindRun(dgRunA); run.Totals.Money.Complete {
		t.Error("the mismatch-excluded model must leave the digest run unpriced")
	}
}

func TestAnalyze_DigestRun_LoadingRaisesNoCurrencyFinding(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = eurPricing()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	if got := currencyFindingsOf(res.Report); len(got) != 0 {
		t.Errorf("currency findings = %+v, want none", got)
	}
	if got := dgFindings(res.Report, domain.FindingPricingCurrencyMismatch); len(got) != 0 {
		t.Errorf("mismatch findings = %+v, want none", got)
	}
}

func TestSessionReprice_DigestRun_KeepsTheCurrencyAndClearsTheReasonOfThePricedKey(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = excludedByMismatch()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))
	res := analyzeDGReport(t, e, dgFilePath)
	if len(res.Report.UnpricedReasons) != 1 {
		t.Fatalf("precondition: want one unpriced reason, got %+v", res.Report.UnpricedReasons)
	}

	repriced := res.Session.Reprice(flatPricing(dgModel))

	if repriced.Currency != domain.CurrencyEUR {
		t.Errorf("repriced Currency = %q, want EUR", repriced.Currency)
	}
	if len(repriced.UnpricedReasons) != 0 {
		t.Errorf("UnpricedReasons = %+v, want none once the model is priced", repriced.UnpricedReasons)
	}
}
