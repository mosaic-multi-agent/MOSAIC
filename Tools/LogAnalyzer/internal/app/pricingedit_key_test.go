package app_test

// EditModelPricing takes the (provider, model) key of the record to edit and
// saves the entry under that key's provider, or without provider when the key
// carries none.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func editByKey(t *testing.T, key domain.PricingKey) (app.PricingEditResult, *fakePricingStore) {
	t.Helper()
	ia := choosingInteraction("")
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{"p1", "p2", ""})
	res, err := svc.EditModelPricing(context.Background(), result.Session, result.Report, key)
	if err != nil {
		t.Fatalf("EditModelPricing: %v", err)
	}
	return res, store
}

func TestEditModelPricing_ProviderKey_SavedUnderThatProvider(t *testing.T) {
	// The selection question is never needed: the key already names the entry.
	ia := &scriptedInteraction{onAskText: choosingInteraction("").onAskText}
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{"p1", "p2"})

	res, err := svc.EditModelPricing(context.Background(), result.Session, result.Report,
		domain.PricingKey{Provider: "p2", Model: providerGapModel})

	if err != nil {
		t.Fatalf("EditModelPricing: %v", err)
	}
	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if got := putProviders(store); len(got) != 1 || got[0] != "p2" {
		t.Errorf("saved entry providers = %q, want [p2]", got)
	}
	if want := (domain.PricingKey{Provider: "p2", Model: providerGapModel}); res.Key != want {
		t.Errorf("result Key = %+v, want %+v", res.Key, want)
	}
	if _, ok := result.Session.Table().LookupFor("p1", providerGapModel); ok {
		t.Error("editing p2 must not price p1")
	}
}

func TestEditModelPricing_KeyWithoutProvider_SavedWithoutProvider(t *testing.T) {
	ia := &scriptedInteraction{onAskText: choosingInteraction("").onAskText}
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{"p1", ""})

	res, err := svc.EditModelPricing(context.Background(), result.Session, result.Report,
		domain.PricingKey{Model: providerGapModel})

	if err != nil {
		t.Fatalf("EditModelPricing: %v", err)
	}
	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if got := putProviders(store); len(got) != 1 || got[0] != "" {
		t.Errorf("saved entry providers = %q, want one entry without provider", got)
	}
}

func TestEditModelPricing_EmptyModelWithProvider_CancelsWithoutPersisting(t *testing.T) {
	res, store := editByKey(t, domain.PricingKey{Provider: "p1"})

	if res.Key != (domain.PricingKey{}) {
		t.Errorf("cancelled result Key = %+v, want zero", res.Key)
	}
	if res.Outcome != app.PricingEditCancelled || len(store.putCalls) != 0 {
		t.Errorf("empty model must cancel and persist nothing; Outcome=%v puts=%d", res.Outcome, len(store.putCalls))
	}
}
