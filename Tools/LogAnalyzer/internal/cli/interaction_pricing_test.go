package cli_test

// Tests that the non-blocking CLI Interaction drives the pricing-entry flow with
// the redesigned rate model: answers keyed "{QuestionID}:{Subject}" for the five
// standard rates and for an optional long-context block.

import (
	"bytes"
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// pricingAnswerKey builds the Answers key of one rate question for model-x.
func pricingAnswerKey(field string) string {
	return string(app.QuestionPricingRate) + ":model-x:" + field
}

// analyzeWithAnswers runs Analyze with pricing-gap resolution over a log with
// one unpriced model, answered by the CLI Interaction built from answers.
func analyzeWithAnswers(t *testing.T, answers cli.Answers) (app.Result, *fakePricingStore) {
	t.Helper()
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"}
	reader := newFakeEventReader()
	reader.addEvents(cliTestOrchFile,
		cliTestTurnEvent("model-x", domain.TokenUsage{Input: domain.Tokens(1_000_000)}),
		cliTestRunEndEvent(),
	)
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			return domain.Inventory{Source: s, Runs: []domain.RunEntry{{
				Run: cliTestRunRef, Dir: "/logs/" + cliTestRunID, OrchestratorFile: cliTestOrchFile,
			}}}, nil
		},
	}
	store := newFakePricingStore()
	svc := app.New(app.Deps{
		Source:      logSource,
		Reader:      reader,
		Pricing:     store,
		Clock:       newFakeClock(),
		Interaction: cli.NewInteraction(&bytes.Buffer{}, answers),
		WorkDir:     "/workdir",
	})

	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/logs", ResolvePricingGaps: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return res, store
}

func standardCLIAnswers() cli.Answers {
	return cli.Answers{
		string(app.QuestionPricingAction):           app.PricingActionEnter,
		pricingAnswerKey(app.RateFieldInput):        "4.00",
		pricingAnswerKey(app.RateFieldCacheRead):    "0.40",
		pricingAnswerKey(app.RateFieldCacheWrite5m): "5.00",
		pricingAnswerKey(app.RateFieldCacheWrite1h): "8.00",
		pricingAnswerKey(app.RateFieldOutput):       "20.00",
	}
}

func TestCLIInteraction_PricingEntryWithoutLongContext_SavesAndPricesTheModel(t *testing.T) {
	res, store := analyzeWithAnswers(t, standardCLIAnswers())

	got, found := store.table.Lookup("model-x")
	if !found {
		t.Fatal("the answered entry must be saved")
	}
	want := domain.ModelPricing{Model: "model-x", Standard: domain.RateSet{
		Input: mustParseRate("4.00"), CacheRead: mustParseRate("0.40"), CacheWrite5m: mustParseRate("5.00"),
		CacheWrite1h: mustParseRate("8.00"), Output: mustParseRate("20.00"),
	}}
	if got != want {
		t.Errorf("saved entry = %+v, want %+v", got, want)
	}
	if res.Report.HasUnpricedModels() {
		t.Errorf("model-x must be priced after the entry; unpriced: %v", res.Report.UnpricedModels)
	}
}

func TestCLIInteraction_PricingEntryWithLongContext_SavesTheBlock(t *testing.T) {
	answers := standardCLIAnswers()
	answers[string(app.QuestionPricingThreshold)+":model-x"] = "5000000"
	lc := app.LongContextFieldPrefix
	answers[pricingAnswerKey(lc+app.RateFieldInput)] = "8.00"
	answers[pricingAnswerKey(lc+app.RateFieldCacheRead)] = "0.80"
	answers[pricingAnswerKey(lc+app.RateFieldCacheWrite5m)] = "10.00"
	answers[pricingAnswerKey(lc+app.RateFieldCacheWrite1h)] = "16.00"
	answers[pricingAnswerKey(lc+app.RateFieldOutput)] = "30.00"

	res, store := analyzeWithAnswers(t, answers)

	got, found := store.table.Lookup("model-x")
	if !found {
		t.Fatal("the answered entry must be saved")
	}
	if !got.HasLongContext || got.LongContext.Threshold != 5_000_000 {
		t.Fatalf("saved entry must carry the long-context block; got %+v", got)
	}
	if got.LongContext.Rates.CacheWrite1h != mustParseRate("16.00") || got.LongContext.Rates.Output != mustParseRate("30.00") {
		t.Errorf("long-context rates mis-mapped: %+v", got.LongContext.Rates)
	}
	if res.Report.HasUnpricedModels() {
		t.Errorf("model-x must be priced after the entry; unpriced: %v", res.Report.UnpricedModels)
	}
}

func TestCLIInteraction_PricingEntryMissingOneStandardRate_SavesNothing(t *testing.T) {
	answers := standardCLIAnswers()
	delete(answers, pricingAnswerKey(app.RateFieldCacheWrite1h))

	res, store := analyzeWithAnswers(t, answers)

	if _, found := store.table.Lookup("model-x"); found {
		t.Error("an entry without cache_write_1h must not be saved")
	}
	if !res.Report.HasUnpricedModels() {
		t.Error("model-x must stay unpriced")
	}
}
