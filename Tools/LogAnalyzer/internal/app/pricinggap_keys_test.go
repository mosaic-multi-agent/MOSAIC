package app_test

// Tests for the per-(provider, model) gap list and the gap flow's per-key
// decline rule.
//
// Rules under test:
//   - PricingGaps lists one gap per unpriced key, in the order of
//     Report.UnpricedModels, each with the number of actor reports that list it.
//   - ModelPricingStates reports priced/unpriced per key under the strict lookup.
//   - Declining one key's gap never stops the next key from being asked and
//     saved.

import (
	"fmt"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// keyField prints the key of a struct that identifies its entry either by a
// Key or a Model field.
func keyField(v any) string {
	rv := reflect.ValueOf(v)
	for _, name := range []string{"Key", "Model"} {
		if f := rv.FieldByName(name); f.IsValid() {
			return fmt.Sprint(f.Interface())
		}
	}
	return ""
}

func TestPricingGaps_SameModelUnderTwoProviders_OneGapPerKeyWithOccurrences(t *testing.T) {
	_, result, _ := providerGapAnalysis(t, &alwaysSkippingInteraction{}, []domain.ProviderID{"p1", "p2", "p1"})

	gaps := app.PricingGaps(result.Report)

	var labels []string
	var counts []int
	for _, g := range gaps {
		labels = append(labels, keyField(g))
		counts = append(counts, g.Occurrences)
	}
	if want := []string{"shared-model [p1]", "shared-model [p2]"}; !equalStrings(labels, want) {
		t.Fatalf("gap keys = %q, want %q", labels, want)
	}
	if !reflect.DeepEqual(counts, []int{2, 1}) {
		t.Errorf("occurrences = %v, want [2 1] (actor reports listing each key)", counts)
	}
}

func TestModelPricingStates_PricedUnderOneProvider_IsPricedOnlyForThatKey(t *testing.T) {
	_, result, _ := providerGapAnalysis(t, &alwaysSkippingInteraction{},
		[]domain.ProviderID{"p1", "p2", ""}, providerPricing("p1"))

	states := app.ModelPricingStates(result.Report, result.Session.Table())

	got := make(map[string]bool)
	for _, s := range states {
		got[keyField(s)] = s.Priced
	}
	want := map[string]bool{
		"shared-model":      false,
		"shared-model [p1]": true,
		"shared-model [p2]": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("priced state per key = %v, want %v", got, want)
	}
}

func TestResolvePricingGaps_DecliningFirstProvider_StillAsksAndSavesTheNext(t *testing.T) {
	ia := enteringInteraction(func(subject string) bool { return subjectOf(subject, "shared-model [p2]") })

	repriced, store, _, inTable := resolveGaps(t, []domain.ProviderID{"p1", "p2"}, ia)

	if got := putProviders(store); len(got) != 1 || got[0] != "p2" {
		t.Fatalf("saved entry providers = %q, want [p2]", got)
	}
	if !inTable("shared-model", "p2") || inTable("shared-model", "p1") {
		t.Error("only the p2 entry may be added to the session table")
	}
	if got, want := unpricedLabels(repriced), []string{"shared-model [p1]"}; !equalStrings(got, want) {
		t.Errorf("unpriced after declining p1 = %q, want %q", got, want)
	}
}
