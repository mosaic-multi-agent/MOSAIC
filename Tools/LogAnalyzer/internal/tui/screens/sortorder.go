package screens

import "sort"

// sortedOrder returns the indexes 0..n-1 ordered by the sort order: by the figure
// of the order's key, in the order's direction, or by name when the key is the
// name. Items without a figure come last in either direction, and ties break on
// name ascending.
func sortedOrder(n int, o SortOrder, name func(i int) string, figure func(key SortKey, i int) (int64, bool)) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ia, ib := idx[a], idx[b]
		if o.Key == SortName {
			if o.Desc {
				return name(ia) > name(ib)
			}
			return name(ia) < name(ib)
		}
		va, okA := figure(o.Key, ia)
		vb, okB := figure(o.Key, ib)
		if okA != okB {
			return okA
		}
		if okA && va != vb {
			return (va > vb) == o.Desc
		}
		return name(ia) < name(ib)
	})
	return idx
}

// clampIndex keeps a cursor inside a list of n rows.
func clampIndex(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}
