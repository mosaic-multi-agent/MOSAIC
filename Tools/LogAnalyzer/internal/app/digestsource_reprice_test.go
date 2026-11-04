package app_test

// Tests that digest runs re-price interactively like log runs and that data-loss
// findings carried in a digest make the report incomplete.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func unpricedDigestEnv() *digestEnv {
	e := newDigestEnv()
	e.pricing.table = domain.EmptyPricingTable()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))
	return e
}

func TestSessionReprice_DigestRun_ChangesMoneyFromUnpricedToKnown(t *testing.T) {
	e := unpricedDigestEnv()
	res := analyzeDGReport(t, e, dgFilePath)
	before, ok := res.Report.FindRun(dgRunA)
	if !ok || before.Totals.Money.Complete {
		t.Fatalf("precondition: digest run must be present and unpriced; found=%v", ok)
	}
	readsBefore := len(e.digests.readCalls)

	repriced := res.Session.Reprice(flatPricing(dgModel))

	after, _ := repriced.FindRun(dgRunA)
	want := domain.KnownMoney(domain.Money(6_000_000_000))
	if !after.Totals.Money.Complete || !reflect.DeepEqual(after.Totals.Money.Total, want) {
		t.Errorf("repriced money = %+v complete=%v, want %+v complete", after.Totals.Money.Total, after.Totals.Money.Complete, want)
	}
	if len(e.digests.readCalls) != readsBefore {
		t.Error("Reprice must not re-read digest files")
	}
}

func TestSessionReprice_DigestRun_MatchesRepriceOfSameRunFromLogs(t *testing.T) {
	logEnv := newDigestEnv()
	logEnv.pricing.table = domain.EmptyPricingTable()
	logEnv.addLogRun(dgRunA, 2_000_000)
	logRes := analyzeDGReport(t, logEnv, logEnv.logsPath)
	digRes := analyzeDGReport(t, unpricedDigestEnv(), dgFilePath)

	fromLogs, _ := logRes.Session.Reprice(flatPricing(dgModel)).FindRun(dgRunA)
	fromDigest, _ := digRes.Session.Reprice(flatPricing(dgModel)).FindRun(dgRunA)

	if !reflect.DeepEqual(fromLogs.Totals, fromDigest.Totals) {
		t.Errorf("repriced totals differ:\nlogs:   %+v\ndigest: %+v", fromLogs.Totals, fromDigest.Totals)
	}
}

func TestResolvePricingGaps_DigestRun_EnteredPricesPriceTheRun(t *testing.T) {
	e := unpricedDigestEnv()
	res := analyzeDGReport(t, e, dgFilePath)
	ia := &scriptedInteraction{
		onSelectOne: func(q interaction.ChoiceQuestion) interaction.ChoiceAnswer {
			if q.ID == app.QuestionPricingAction {
				return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: app.PricingActionEnter}
			}
			return interaction.ChoiceAnswer{Status: interaction.SkippedOne}
		},
		onAskText: func(q interaction.TextQuestion) interaction.TextAnswer {
			if q.ID == app.QuestionPricingRate {
				return interaction.TextAnswer{Status: interaction.Answered, Text: "3.00"}
			}
			return interaction.TextAnswer{Status: interaction.SkippedOne}
		},
	}

	updated, err := e.withInteraction(ia).service().ResolvePricingGaps(context.Background(), res.Session, res.Report)

	if err != nil {
		t.Fatalf("ResolvePricingGaps returned unexpected error: %v", err)
	}
	if len(e.pricing.putCalls) == 0 {
		t.Error("entered prices must be persisted through the pricing store")
	}
	if run, _ := updated.FindRun(dgRunA); !run.Totals.Money.Complete {
		t.Errorf("digest run still unpriced after entering prices: %+v", run.Totals.Money)
	}
}

func TestAnalyze_DigestCarryingDataLossFinding_ReportIsIncomplete(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)),
		dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA)))

	res := analyzeDG(t, e, dgFilePath)

	if !res.Report.Quality.Incomplete() {
		t.Error("a carried data-loss finding must make the report incomplete")
	}
	got := dgFindings(res.Report, domain.FindingMalformedLine)
	if len(got) != 1 || got[0].Run != domain.NamedRun(dgRunA) {
		t.Errorf("carried finding = %+v, want one malformed_line scoped to run %s", got, dgRunA)
	}
}

func TestAnalyze_DigestWithoutFindings_ReportIsComplete(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDGReport(t, e, dgFilePath)

	if res.Report.Quality.Incomplete() {
		t.Error("a clean digest must not make the report incomplete")
	}
}
