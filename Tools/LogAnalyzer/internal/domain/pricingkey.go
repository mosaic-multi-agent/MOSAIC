package domain

import "sort"

// PricingKey identifies one pricing gap or entry: a model under a provider.
// An empty Provider is the top-level models section / a record that logged no
// provider.
type PricingKey struct {
	Provider ProviderID
	Model    ModelID
}

// String prints "model [provider]", or the bare model id without a provider.
func (k PricingKey) String() string {
	if k.Provider == "" {
		return string(k.Model)
	}
	return string(k.Model) + " [" + string(k.Provider) + "]"
}

// Less orders keys by model id, then by provider with "" first.
func (k PricingKey) Less(o PricingKey) bool {
	if k.Model != o.Model {
		return k.Model < o.Model
	}
	return k.Provider < o.Provider
}

// SortedPricingKeys converts a set of keys to a deterministically sorted slice,
// nil when the set is empty.
func SortedPricingKeys(set map[PricingKey]bool) []PricingKey {
	if len(set) == 0 {
		return nil
	}
	keys := make([]PricingKey, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Less(keys[j]) })
	return keys
}

// DistinctModelIDs lists the distinct model ids of keys in ascending order,
// nil when keys is empty.
func DistinctModelIDs(keys []PricingKey) []ModelID {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[ModelID]bool, len(keys))
	out := make([]ModelID, 0, len(keys))
	for _, k := range keys {
		if !seen[k.Model] {
			seen[k.Model] = true
			out = append(out, k.Model)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
