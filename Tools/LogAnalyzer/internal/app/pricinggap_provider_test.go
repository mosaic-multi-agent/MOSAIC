package app_test

// Tests for the provider-aware pricing-gap flow.
//
// Rules under test:
//   - One gap per unpriced (provider, model); the same model unpriced under two
//     providers is two gaps, each asked about separately.
//   - The entry saved for a gap carries that gap's provider; a record without
//     provider is saved without one (top-level models).
//   - After saving, re-pricing prices exactly the records with that key.
//   - Question subjects, prompts and the saved notice name the key
//     ("model [provider]"); keys without provider stay byte-identical to the
//     model-only vocabulary.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/domain"
)

func resolveGaps(t *testing.T, providers []domain.ProviderID, ia *scriptedInteraction, preloaded ...domain.ModelPricing) (domain.Report, *fakePricingStore, *scriptedInteraction, func(string, domain.ProviderID) bool) {
	t.Helper()
	svc, result, store := providerGapAnalysis(t, ia, providers, preloaded...)
	repriced, err := svc.ResolvePricingGaps(context.Background(), result.Session, result.Report)
	if err != nil {
		t.Fatalf("ResolvePricingGaps: %v", err)
	}
	inTable := func(model string, p domain.ProviderID) bool {
		_, ok := result.Session.Table().LookupFor(p, domain.ModelID(model))
		return ok
	}
	return repriced, store, ia, inTable
}

func TestResolvePricingGaps_SameModelUnderTwoProviders_AsksAndSavesEachSeparately(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	repriced, store, _, inTable := resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia)

	if got := putProviders(store); len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("saved entry providers = %q, want [p1 p2]", got)
	}
	if !inTable("shared-model", "p1") || !inTable("shared-model", "p2") {
		t.Error("session pricing table must hold one entry per provider")
	}
	if inTable("shared-model", "") {
		t.Error("provider records must not create a top-level models entry")
	}
	if got := unpricedLabels(repriced); len(got) != 0 {
		t.Errorf("unpriced after saving both gaps = %q, want none", got)
	}
}

func TestResolvePricingGaps_RecordWithoutProvider_SavedWithoutProvider(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	repriced, store, _, inTable := resolveGaps(t, []domain.ProviderID{""}, ia)

	if got := putProviders(store); len(got) != 1 || got[0] != "" {
		t.Fatalf("saved entry providers = %q, want one entry without provider", got)
	}
	if !inTable("shared-model", "") {
		t.Error("session pricing table must hold the top-level entry")
	}
	if got := unpricedLabels(repriced); len(got) != 0 {
		t.Errorf("unpriced after saving = %q, want none", got)
	}
}

func TestResolvePricingGaps_SavingOneProvider_PricesExactlyThatProvidersRecords(t *testing.T) {
	ia := enteringInteraction(func(subject string) bool { return subjectOf(subject, "shared-model [p1]") })

	repriced, store, _, inTable := resolveGaps(t, []domain.ProviderID{"p1", "p2", ""}, ia)

	if got := putProviders(store); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("saved entry providers = %q, want [p1]", got)
	}
	if !inTable("shared-model", "p1") || inTable("shared-model", "p2") || inTable("shared-model", "") {
		t.Error("only the p1 entry may be added to the session table")
	}
	want := []string{"shared-model", "shared-model [p2]"}
	if got := unpricedLabels(repriced); !equalStrings(got, want) {
		t.Errorf("unpriced after saving p1 = %q, want %q", got, want)
	}
}

func TestResolvePricingGaps_PricedUnderOtherProvider_OnlyUnpricedKeyIsAsked(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	_, store, _, _ := resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia, providerPricing("p1"))

	if got := putProviders(store); len(got) != 1 || got[0] != "p2" {
		t.Fatalf("saved entry providers = %q, want only the unpriced provider [p2]", got)
	}
	for _, q := range ia.askTextCalls {
		if strings.Contains(q.Subject, "[p1]") {
			t.Errorf("priced key must not be asked about; got subject %q", q.Subject)
		}
	}
}

func TestResolvePricingGaps_QuestionSubjects_NameTheKey(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia)

	subjects := make(map[string]bool)
	for _, q := range ia.askTextCalls {
		subjects[string(q.ID)+"|"+q.Subject] = true
	}
	for _, want := range []string{
		"pricing-rate|shared-model [p1]:input",
		"pricing-rate|shared-model [p2]:input",
		"pricing-rate|shared-model [p1]:output",
		"pricing-threshold|shared-model [p1]",
		"pricing-threshold|shared-model [p2]",
	} {
		if !subjects[want] {
			t.Errorf("missing question %q; asked: %v", want, subjects)
		}
	}
}

func TestResolvePricingGaps_Prompts_IdentifyTheProvider(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia)

	for _, q := range ia.askTextCalls {
		if q.ID != "pricing-rate" {
			continue
		}
		provider := "[p1]"
		if strings.HasPrefix(q.Subject, "shared-model [p2]") {
			provider = "[p2]"
		}
		if !strings.Contains(q.Prompt, "shared-model "+provider) {
			t.Errorf("rate prompt %q must name \"shared-model %s\"", q.Prompt, provider)
		}
	}
}

func TestResolvePricingGaps_SavedNotice_NamesTheKey(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia)

	for _, key := range []string{"shared-model [p1]", "shared-model [p2]"} {
		found := false
		for _, n := range ia.notifyCalls {
			if n.Title == "Saved" && strings.Contains(n.Message, key) {
				found = true
			}
		}
		if !found {
			t.Errorf("no saved notice names %q; notices: %v", key, ia.notifyCalls)
		}
	}
}

func TestResolvePricingGaps_KeyWithoutProvider_KeepsModelOnlyVocabulary(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	resolveGaps(t, []domain.ProviderID{""}, ia)

	var rate, threshold *interaction.TextQuestion
	for i := range ia.askTextCalls {
		q := &ia.askTextCalls[i]
		switch {
		case q.ID == "pricing-rate" && q.Subject == "shared-model:input":
			rate = q
		case q.ID == "pricing-threshold" && q.Subject == "shared-model":
			threshold = q
		}
	}
	if rate == nil || threshold == nil {
		t.Fatalf("model-only subjects missing; asked: %v", ia.askTextCalls)
	}
	if rate.Prompt != "USD per million tokens for shared-model input:" {
		t.Errorf("rate prompt = %q, want the model-only wording", rate.Prompt)
	}
}
