package screens_test

// Behaviour of the sort-cycling helper offered to table views.

import (
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func offeredOrders() []screens.SortOrder {
	return []screens.SortOrder{
		{Key: screens.SortCost, Desc: true},
		{Key: screens.SortTokens, Desc: true},
		{Key: screens.SortInvocations, Desc: true},
		{Key: screens.SortDuration, Desc: true},
		{Key: screens.SortName, Desc: false},
	}
}

func TestSortCycle_StartsOnTheFirstOfferedOrder(t *testing.T) {
	orders := offeredOrders()

	c := screens.NewSortCycle(orders...)

	if got := c.Active(); got != orders[0] {
		t.Errorf("Active() = %+v, want the default %+v", got, orders[0])
	}
}

func TestSortCycle_NextVisitsEveryOrderInOfferedOrderThenWraps(t *testing.T) {
	orders := offeredOrders()
	c := screens.NewSortCycle(orders...)

	for round := 0; round < 2; round++ {
		for i, want := range orders {
			if got := c.Active(); got != want {
				t.Errorf("round %d step %d: Active() = %+v, want %+v", round, i, got, want)
			}
			c = c.Next()
		}
	}
}

func TestSortCycle_NextLeavesTheReceiverUnchanged(t *testing.T) {
	orders := offeredOrders()
	c := screens.NewSortCycle(orders...)

	next := c.Next()

	if c.Active() != orders[0] {
		t.Errorf("Next must return a new cycle; the receiver moved to %+v", c.Active())
	}
	if next.Active() != orders[1] {
		t.Errorf("Next().Active() = %+v, want %+v", next.Active(), orders[1])
	}
}

func TestSortCycle_SingleOrderStaysOnItself(t *testing.T) {
	only := screens.SortOrder{Key: screens.SortName}
	c := screens.NewSortCycle(only)

	if got := c.Next().Active(); got != only {
		t.Errorf("Active() after Next = %+v, want %+v", got, only)
	}
}

func TestSortCycle_WithoutOrdersPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewSortCycle with no order must panic")
		}
	}()

	screens.NewSortCycle()
}

func TestSortCycle_LabelNamesTheActiveSort(t *testing.T) {
	orders := offeredOrders()
	c := screens.NewSortCycle(orders...)

	for _, o := range orders {
		label := c.Label()

		assertContains(t, label, "sort:", string(o.Key))
		c = c.Next()
	}
}

func TestSortCycle_LabelShowsDirection(t *testing.T) {
	desc := screens.NewSortCycle(screens.SortOrder{Key: screens.SortCost, Desc: true}).Label()
	asc := screens.NewSortCycle(screens.SortOrder{Key: screens.SortCost, Desc: false}).Label()

	if desc == asc {
		t.Errorf("ascending and descending must be told apart, both read %q", desc)
	}
	assertContains(t, desc, "sort:", "cost")
	assertContains(t, asc, "sort:", "cost")
}

func TestSortCycle_LabelChangesWhenTheCycleAdvances(t *testing.T) {
	c := screens.NewSortCycle(offeredOrders()...)

	if c.Label() == c.Next().Label() {
		t.Errorf("label must follow the active sort, both read %q", c.Label())
	}
}
