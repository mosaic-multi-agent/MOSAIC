package app_test

// Tests for the interactive pricing-entry flow with the redesigned rate model:
// five standard rates, then an optional long-context block (threshold plus five
// rates). An entry made interactively is saved in the new shape and prices the
// model.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// standardAnswers and longContextAnswers give every field a distinct rate so a
// mis-mapped field is detected.
var standardAnswers = map[string]string{
	app.RateFieldInput: "4.00", app.RateFieldCacheRead: "0.40", app.RateFieldCacheWrite5m: "5.00",
	app.RateFieldCacheWrite1h: "8.00", app.RateFieldOutput: "20.00",
}

var longContextAnswers = map[string]string{
	app.RateFieldInput: "8.00", app.RateFieldCacheRead: "0.80", app.RateFieldCacheWrite5m: "10.00",
	app.RateFieldCacheWrite1h: "16.00", app.RateFieldOutput: "30.00",
}

func rateSetOf(answers map[string]string) domain.RateSet {
	return domain.RateSet{
		Input:        mustParseRate(answers[app.RateFieldInput]),
		CacheRead:    mustParseRate(answers[app.RateFieldCacheRead]),
		CacheWrite5m: mustParseRate(answers[app.RateFieldCacheWrite5m]),
		CacheWrite1h: mustParseRate(answers[app.RateFieldCacheWrite1h]),
		Output:       mustParseRate(answers[app.RateFieldOutput]),
	}
}

// entryScript answers model-x's questions: standard rates by field, the
// threshold answer as given, and long-context rates by field. A field absent
// from an answer map is skipped.
func entryScript(standard map[string]string, threshold string, longContext map[string]string) *scriptedInteraction {
	return &scriptedInteraction{
		onAskText: func(q interaction.TextQuestion) interaction.TextAnswer {
			prefix := "model-x:"
			answer := func(m map[string]string, field string) interaction.TextAnswer {
				if v, ok := m[field]; ok {
					return interaction.TextAnswer{Status: interaction.Answered, Text: v}
				}
				return interaction.TextAnswer{Status: interaction.SkippedOne}
			}
			switch q.ID {
			case app.QuestionPricingRate:
				field := strings.TrimPrefix(q.Subject, prefix)
				if lcField, isLC := strings.CutPrefix(field, app.LongContextFieldPrefix); isLC {
					return answer(longContext, lcField)
				}
				return answer(standard, field)
			case app.QuestionPricingThreshold:
				if threshold == "" {
					return interaction.TextAnswer{Status: interaction.SkippedOne}
				}
				return interaction.TextAnswer{Status: interaction.Answered, Text: threshold}
			}
			return interaction.TextAnswer{Status: interaction.SkippedOne}
		},
	}
}

// askedSubjects lists the Subjects of every AskText call in order, prefixed by
// the question id.
func askedSubjects(ia *scriptedInteraction) []string {
	var out []string
	for _, q := range ia.askTextCalls {
		out = append(out, string(q.ID)+"|"+q.Subject)
	}
	return out
}

func editModelX(t *testing.T, ia *scriptedInteraction) (app.PricingEditResult, *fakePricingStore, app.Result) {
	t.Helper()
	session, _, _ := buildEditorTestSession(t)
	store := newFakePricingStore()
	res, err := newEditService(ia, store).EditModelPricing(context.Background(), session.Session, session.Report, "model-x")
	if err != nil {
		t.Fatalf("EditModelPricing: %v", err)
	}
	return res, store, session
}

func TestPricingEntry_WithoutLongContext_AsksFiveRatesThenThreshold(t *testing.T) {
	ia := entryScript(standardAnswers, "", nil)

	editModelX(t, ia)

	want := []string{
		"pricing-rate|model-x:input", "pricing-rate|model-x:cache_read",
		"pricing-rate|model-x:cache_write_5m", "pricing-rate|model-x:cache_write_1h",
		"pricing-rate|model-x:output", "pricing-threshold|model-x",
	}
	got := askedSubjects(ia)
	if len(got) != len(want) {
		t.Fatalf("asked %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("question %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPricingEntry_WithoutLongContext_SavesFiveStandardRatesOnly(t *testing.T) {
	res, store, _ := editModelX(t, entryScript(standardAnswers, "", nil))

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("want exactly 1 Put; got %d", len(store.putCalls))
	}
	want := domain.ModelPricing{Model: "model-x", Standard: rateSetOf(standardAnswers)}
	if store.putCalls[0] != want {
		t.Errorf("saved entry = %+v, want %+v", store.putCalls[0], want)
	}
}

func TestPricingEntry_WithoutLongContext_PricesTheModel(t *testing.T) {
	res, _, session := editModelX(t, entryScript(standardAnswers, "", nil))

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if res.Report.HasUnpricedModels() {
		t.Errorf("model-x must be priced after the entry; unpriced: %v", res.Report.UnpricedModels)
	}
	// model-a: 1M input at 3.00 = 3e9 nanos; model-x: 2M input at 4.00 = 8e9 nanos.
	want := domain.KnownMoney(domain.Money(11_000_000_000))
	if got := res.Report.AllRuns.Money.Input; got != want {
		t.Errorf("run input money = %+v, want %+v", got, want)
	}
	if _, found := session.Session.Table().Lookup("model-x"); !found {
		t.Error("the session table must hold the new entry")
	}
}

func TestPricingEntry_WithLongContext_AsksFiveMoreRatesAfterTheThreshold(t *testing.T) {
	ia := entryScript(standardAnswers, "5000000", longContextAnswers)

	editModelX(t, ia)

	got := askedSubjects(ia)
	want := []string{
		"pricing-threshold|model-x",
		"pricing-rate|model-x:long_context.input", "pricing-rate|model-x:long_context.cache_read",
		"pricing-rate|model-x:long_context.cache_write_5m", "pricing-rate|model-x:long_context.cache_write_1h",
		"pricing-rate|model-x:long_context.output",
	}
	if len(got) != 6+len(want)-1 {
		t.Fatalf("asked %d questions %v; want the five standard ones, then %v", len(got), got, want)
	}
	for i, w := range want {
		if got[5+i] != w {
			t.Errorf("question %d = %q, want %q", 5+i, got[5+i], w)
		}
	}
}

func TestPricingEntry_WithLongContext_SavesThresholdAndSecondRateSet(t *testing.T) {
	res, store, _ := editModelX(t, entryScript(standardAnswers, "5000000", longContextAnswers))

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("want exactly 1 Put; got %d", len(store.putCalls))
	}
	want := domain.ModelPricing{
		Model:          "model-x",
		Standard:       rateSetOf(standardAnswers),
		HasLongContext: true,
		LongContext:    domain.LongContextPricing{Threshold: 5_000_000, Rates: rateSetOf(longContextAnswers)},
	}
	if store.putCalls[0] != want {
		t.Errorf("saved entry = %+v, want %+v", store.putCalls[0], want)
	}
}

func TestPricingEntry_WithLongContext_ThresholdAboveRequest_PricesAtStandardRates(t *testing.T) {
	// Regression coverage only: the threshold lies above the model's 2M-token
	// request, so standard rates apply. Not evidence that the long-context block
	// is saved; the save tests above cover that.
	res, _, _ := editModelX(t, entryScript(standardAnswers, "5000000", longContextAnswers))

	if res.Report.HasUnpricedModels() {
		t.Errorf("model-x must be priced after the entry; unpriced: %v", res.Report.UnpricedModels)
	}
	want := domain.KnownMoney(domain.Money(11_000_000_000))
	if got := res.Report.AllRuns.Money.Input; got != want {
		t.Errorf("run input money = %+v, want %+v", got, want)
	}
}

func TestPricingEntry_ThresholdWithIncompleteLongContextRates_SavesNothing(t *testing.T) {
	partial := map[string]string{app.RateFieldInput: "8.00", app.RateFieldCacheRead: "0.80"}

	res, store, _ := editModelX(t, entryScript(standardAnswers, "5000000", partial))

	if len(store.putCalls) != 0 {
		t.Errorf("an incomplete long-context rate set must save nothing; got %d Put calls", len(store.putCalls))
	}
	if res.Outcome == app.PricingEditSaved {
		t.Error("Outcome must not be PricingEditSaved when nothing was saved")
	}
}

func TestPricingEntry_NoValidThreshold_AsksNoLongContextQuestions(t *testing.T) {
	for _, threshold := range []string{"", "abc", "0", "-5", "1.5"} {
		t.Run("threshold="+threshold, func(t *testing.T) {
			ia := entryScript(standardAnswers, threshold, longContextAnswers)

			res, store, _ := editModelX(t, ia)

			for _, s := range askedSubjects(ia) {
				if strings.Contains(s, app.LongContextFieldPrefix) {
					t.Errorf("no long-context question may be asked; got %q", s)
				}
			}
			if res.Outcome != app.PricingEditSaved || len(store.putCalls) != 1 {
				t.Fatalf("the standard entry must still be saved; outcome=%v puts=%d", res.Outcome, len(store.putCalls))
			}
			if store.putCalls[0].HasLongContext {
				t.Error("an unusable threshold must yield an entry without a long-context tier")
			}
		})
	}
}

func TestPricingEntry_MissingStandardRate_SavesNothing(t *testing.T) {
	incomplete := map[string]string{}
	for k, v := range standardAnswers {
		incomplete[k] = v
	}
	delete(incomplete, app.RateFieldCacheWrite1h)

	res, store, _ := editModelX(t, entryScript(incomplete, "", nil))

	if len(store.putCalls) != 0 {
		t.Errorf("an entry missing cache_write_1h must not be saved; got %d Put calls", len(store.putCalls))
	}
	if res.Outcome != app.PricingEditNoChange {
		t.Errorf("Outcome = %v, want PricingEditNoChange", res.Outcome)
	}
}

func TestPricingEntry_RateQuestions_ValidateWithParseRate(t *testing.T) {
	ia := entryScript(standardAnswers, "5000000", longContextAnswers)

	editModelX(t, ia)

	rateQuestions := 0
	for _, q := range ia.askTextCalls {
		if q.ID != app.QuestionPricingRate {
			continue
		}
		rateQuestions++
		if q.Validate == nil {
			t.Fatalf("rate question %q must carry a validator", q.Subject)
		}
		if err := q.Validate("abc"); err == nil {
			t.Errorf("%q must reject a non-numeric rate", q.Subject)
		}
		if err := q.Validate("3.00"); err != nil {
			t.Errorf("%q must accept 3.00: %v", q.Subject, err)
		}
	}
	if rateQuestions != 10 {
		t.Errorf("want 10 rate questions with a long-context block; got %d", rateQuestions)
	}
}
