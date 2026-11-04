package domain_test

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestPricingKey_String_NamesProviderOnlyWhenPresent(t *testing.T) {
	withProvider := domain.PricingKey{Provider: "p1", Model: "m"}
	without := domain.PricingKey{Model: "m"}

	if got := withProvider.String(); got != "m [p1]" {
		t.Errorf("String() = %q, want %q", got, "m [p1]")
	}
	if got := without.String(); got != "m" {
		t.Errorf("String() without provider = %q, want %q", got, "m")
	}
}

func TestSortedPricingKeys_OrdersByModelThenProviderWithEmptyFirst(t *testing.T) {
	set := map[domain.PricingKey]bool{
		{Provider: "p2", Model: "b"}: true,
		{Provider: "p1", Model: "b"}: true,
		{Model: "b"}:                 true,
		{Provider: "p9", Model: "a"}: true,
	}

	got := domain.SortedPricingKeys(set)

	want := []domain.PricingKey{
		{Provider: "p9", Model: "a"},
		{Model: "b"},
		{Provider: "p1", Model: "b"},
		{Provider: "p2", Model: "b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedPricingKeys = %+v, want %+v", got, want)
	}
}

func TestSortedPricingKeys_EmptySetIsNil(t *testing.T) {
	if got := domain.SortedPricingKeys(nil); got != nil {
		t.Errorf("SortedPricingKeys(nil) = %+v, want nil", got)
	}
}

func TestDistinctModelIDs_ListsEachModelOnceInOrder(t *testing.T) {
	keys := []domain.PricingKey{
		{Provider: "p2", Model: "b"},
		{Provider: "p1", Model: "b"},
		{Model: "a"},
	}

	got := domain.DistinctModelIDs(keys)

	want := []domain.ModelID{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DistinctModelIDs = %v, want %v", got, want)
	}
}
