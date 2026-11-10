package cli_test

// Wire-shape guard for every `unpriced_models` array of the report JSON (actor,
// group, dispatch, across-runs) and for the text "Unpriced models" list, for a
// run whose gaps are tracked per (provider, model).
//
// Rules under test:
//   - Each JSON array is a distinct, sorted list of model-id strings; it never
//     carries the provider and never repeats a model that is unpriced under
//     several providers.
//   - The text list names the provider of each gap ("model [provider]").

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

// collectUnpriced gathers every "unpriced_models" value in a decoded JSON
// document, keyed by its path.
func collectUnpriced(v any, path string, out map[string]any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			p := path + "." + k
			if k == "unpriced_models" {
				out[p] = child
				continue
			}
			collectUnpriced(child, p, out)
		}
	case []any:
		for i, child := range x {
			collectUnpriced(child, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

func TestEncodeReport_ProviderGaps_EveryUnpricedArrayIsDistinctSortedModelIDs(t *testing.T) {
	var buf bytes.Buffer
	if err := cli.EncodeReport(&buf, providerGapReport(domain.EmptyPricingTable())); err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}

	found := make(map[string]any)
	collectUnpriced(decodeObject(t, buf.Bytes()), "", found)

	for _, suffix := range []string{
		".agents[0].unpriced_models",                            // actor
		".by_agent_type[0].figures.unpriced_models",             // group
		".dispatches[0].unpriced_models",                        // dispatch
		".across_runs.runs[0].unpriced_models",                  // across-runs run
		".across_runs.by_agent_type[0].figures.unpriced_models", // across-runs group
	} {
		matched := false
		for path := range found {
			matched = matched || strings.HasSuffix(path, suffix)
		}
		if !matched {
			t.Errorf("no unpriced_models array at a path ending %q; found: %v", suffix, sortedKeys(found))
		}
	}
	for path, raw := range found {
		got := stringList(t, raw)
		if !sort.StringsAreSorted(got) {
			t.Errorf("%s = %q, want sorted", path, got)
		}
		for i := 1; i < len(got); i++ {
			if got[i] == got[i-1] {
				t.Errorf("%s = %q, want distinct model ids", path, got)
			}
		}
		for _, id := range got {
			if strings.ContainsAny(id, "[ ") {
				t.Errorf("%s entry %q must be a bare model id", path, id)
			}
		}
	}
}

func TestEncodeText_ProviderGaps_UnpricedListNamesTheProvider(t *testing.T) {
	var buf bytes.Buffer
	if err := cli.EncodeText(&buf, providerGapReport(domain.EmptyPricingTable())); err != nil {
		t.Fatalf("EncodeText: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"beta [p1]", "beta [p2]", "alpha [p1]"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}
