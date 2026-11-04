package app_test

// Tests for the pricing-edit flow against the provider-keyed pricing table.
//
// Rules under test:
//   - The model-selection options are one per (provider, model) of the report,
//     with id "model [provider]" (bare model id without provider), ordered by
//     model then provider with the provider-less key first.
//   - The priced/unpriced label follows the strict lookup: a top-level entry
//     does not price a provider record and a provider entry does not price a
//     record without provider.
//   - An edit is saved under the selected record's provider, or without
//     provider for a record that logged none, replacing only that entry.
//   - An answer that is not one of the offered ids is a cancel.

import (
	"context"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// choosingInteraction selects the given option in the model question and
// answers every rate question with "5.00".
func choosingInteraction(optionID string) *scriptedInteraction {
	return &scriptedInteraction{
		onSelectOne: func(q interaction.ChoiceQuestion) interaction.ChoiceAnswer {
			if q.ID == app.QuestionPricingModel {
				return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: optionID}
			}
			return interaction.ChoiceAnswer{Status: interaction.SkippedOne}
		},
		onAskText: func(q interaction.TextQuestion) interaction.TextAnswer {
			if q.ID == app.QuestionPricingRate {
				return interaction.TextAnswer{Status: interaction.Answered, Text: "5.00"}
			}
			return interaction.TextAnswer{Status: interaction.SkippedOne}
		},
	}
}

func editPricing(t *testing.T, ia *scriptedInteraction, providers []domain.ProviderID, preloaded ...domain.ModelPricing) (app.PricingEditResult, app.Result, *fakePricingStore) {
	t.Helper()
	svc, result, store := providerGapAnalysis(t, ia, providers, preloaded...)
	res, err := svc.EditPricing(context.Background(), result.Session, result.Report)
	if err != nil {
		t.Fatalf("EditPricing: %v", err)
	}
	return res, result, store
}

func optionLabels(t *testing.T, ia *scriptedInteraction) map[string]string {
	t.Helper()
	if len(ia.selectOneCalls) == 0 {
		t.Fatal("EditPricing asked no selection question")
	}
	labels := make(map[string]string)
	var order []string
	for _, o := range ia.selectOneCalls[0].Options {
		labels[o.ID] = o.Label
		order = append(order, o.ID)
	}
	wantOrder := []string{"shared-model", "shared-model [p1]", "shared-model [p2]"}
	if !equalStrings(order, wantOrder) {
		t.Fatalf("option ids = %q, want %q", order, wantOrder)
	}
	return labels
}

func TestEditPricing_Options_AreOnePerProviderAndModel(t *testing.T) {
	ia := choosingInteraction("")

	editPricing(t, ia, []domain.ProviderID{"p2", "p1", ""})

	labels := optionLabels(t, ia)
	for id, label := range labels {
		if label != id+" (unpriced)" {
			t.Errorf("label for %q = %q, want %q", id, label, id+" (unpriced)")
		}
	}
}

func TestEditPricing_PricedLabels_FollowTheStrictLookup(t *testing.T) {
	ia := choosingInteraction("")

	editPricing(t, ia, []domain.ProviderID{"p2", "p1", ""}, providerPricing("p1"))

	labels := optionLabels(t, ia)
	want := map[string]string{
		"shared-model":      "shared-model (unpriced)",
		"shared-model [p1]": "shared-model [p1] (priced)",
		"shared-model [p2]": "shared-model [p2] (unpriced)",
	}
	for id, w := range want {
		if labels[id] != w {
			t.Errorf("label for %q = %q, want %q", id, labels[id], w)
		}
	}
}

func TestEditPricing_TopLevelEntry_DoesNotMarkProviderRecordsPriced(t *testing.T) {
	ia := choosingInteraction("")

	editPricing(t, ia, []domain.ProviderID{"p2", "p1", ""}, providerPricing(""))

	labels := optionLabels(t, ia)
	want := map[string]string{
		"shared-model":      "shared-model (priced)",
		"shared-model [p1]": "shared-model [p1] (unpriced)",
		"shared-model [p2]": "shared-model [p2] (unpriced)",
	}
	for id, w := range want {
		if labels[id] != w {
			t.Errorf("label for %q = %q, want %q", id, labels[id], w)
		}
	}
}

func TestEditPricing_ProviderRecordSelected_SavedUnderThatProvider(t *testing.T) {
	ia := choosingInteraction("shared-model [p2]")

	res, result, store := editPricing(t, ia, []domain.ProviderID{"p1", "p2"})

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if got := putProviders(store); len(got) != 1 || got[0] != "p2" {
		t.Fatalf("saved entry providers = %q, want [p2]", got)
	}
	want := []string{"shared-model [p1]"}
	if got := unpricedLabels(res.Report); !equalStrings(got, want) {
		t.Errorf("unpriced after edit = %q, want %q", got, want)
	}
	if _, ok := result.Session.Table().LookupFor("p1", providerGapModel); ok {
		t.Error("editing p2 must not price p1")
	}
}

func TestEditPricing_RecordWithoutProviderSelected_SavedWithoutProvider(t *testing.T) {
	ia := choosingInteraction("shared-model")

	res, result, store := editPricing(t, ia, []domain.ProviderID{"p1", ""})

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if got := putProviders(store); len(got) != 1 || got[0] != "" {
		t.Fatalf("saved entry providers = %q, want one entry without provider", got)
	}
	if _, ok := result.Session.Table().LookupFor("", providerGapModel); !ok {
		t.Error("the top-level entry must be in the session table")
	}
	want := []string{"shared-model [p1]"}
	if got := unpricedLabels(res.Report); !equalStrings(got, want) {
		t.Errorf("unpriced after edit = %q, want %q", got, want)
	}
}

func TestEditPricing_EditingPricedProviderEntry_ReplacesOnlyThatEntry(t *testing.T) {
	ia := choosingInteraction("shared-model [p1]")

	res, result, store := editPricing(t, ia, []domain.ProviderID{"p1", "p2"},
		providerPricing("p1"), providerPricing("p2"))

	if res.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
	}
	if got := putProviders(store); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("saved entry providers = %q, want [p1]", got)
	}
	table := result.Session.Table()
	edited, _ := table.LookupFor("p1", providerGapModel)
	untouched, _ := table.LookupFor("p2", providerGapModel)
	if edited.Standard.Input == untouched.Standard.Input {
		t.Error("p1 must carry the newly entered rate while p2 keeps its previous rate")
	}
}

func TestEditPricing_BareModelIdWithoutProviderRecords_IsNotOffered(t *testing.T) {
	ia := choosingInteraction("shared-model")

	res, _, store := editPricing(t, ia, []domain.ProviderID{"p1"})

	if len(ia.selectOneCalls) == 0 {
		t.Fatal("EditPricing asked no selection question")
	}
	for _, o := range ia.selectOneCalls[0].Options {
		if o.ID == "shared-model" {
			t.Errorf("no record lacks a provider, yet option %q is offered", o.ID)
		}
	}
	if res.Outcome != app.PricingEditCancelled || len(store.putCalls) != 0 {
		t.Errorf("an id that was not offered must cancel and persist nothing; Outcome=%v puts=%d",
			res.Outcome, len(store.putCalls))
	}
}

func TestEditModelPricing_NilSession_StaysCancelledForProviderReport(t *testing.T) {
	ia := choosingInteraction("shared-model [p1]")
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{"p1"})

	res, err := svc.EditPricing(context.Background(), nil, result.Report)
	if err != nil {
		t.Fatalf("EditPricing: %v", err)
	}
	if res.Outcome != app.PricingEditCancelled || len(store.putCalls) != 0 {
		t.Errorf("nil session must cancel without persisting; Outcome=%v puts=%d", res.Outcome, len(store.putCalls))
	}
}
