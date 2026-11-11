package domain_test

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestAddReason_KeepsEveryReasonInOrder(t *testing.T) {
	var item domain.PlanItem

	item.AddReason("first cause")
	item.AddReason("second cause")

	want := []string{"first cause", "second cause"}
	if !reflect.DeepEqual(item.Reasons, want) {
		t.Errorf("Reasons = %q, want %q", item.Reasons, want)
	}
}

func TestAddReason_IgnoresEmptyText(t *testing.T) {
	var item domain.PlanItem

	item.AddReason("")
	item.AddReason("cause")
	item.AddReason("")

	if want := []string{"cause"}; !reflect.DeepEqual(item.Reasons, want) {
		t.Errorf("Reasons = %q, want %q", item.Reasons, want)
	}
}

func TestAddReason_IgnoresTextAlreadyPresent(t *testing.T) {
	item := domain.PlanItem{Reasons: []string{"cause"}}

	item.AddReason("cause")

	if want := []string{"cause"}; !reflect.DeepEqual(item.Reasons, want) {
		t.Errorf("Reasons = %q, want %q", item.Reasons, want)
	}
}

func TestAddReason_AppendsAfterExistingReasons(t *testing.T) {
	item := domain.PlanItem{Reasons: []string{"earlier"}}

	item.AddReason("later")

	if want := []string{"earlier", "later"}; !reflect.DeepEqual(item.Reasons, want) {
		t.Errorf("Reasons = %q, want %q", item.Reasons, want)
	}
}

func TestAddWarning_AppendsNonEmptyDistinctTextOnly(t *testing.T) {
	var item domain.PlanItem

	item.AddWarning("")
	item.AddWarning("note")
	item.AddWarning("note")
	item.AddWarning("other note")

	if want := []string{"note", "other note"}; !reflect.DeepEqual(item.Warnings, want) {
		t.Errorf("Warnings = %q, want %q", item.Warnings, want)
	}
	if len(item.Reasons) != 0 {
		t.Errorf("Reasons = %q, want none: a warning is not a reason", item.Reasons)
	}
}
