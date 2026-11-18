package screens_test

// The legends of the three views that show the cache hit figure describe its
// pairwise definition: cache reads over the previous call's context, for calls
// whose previous call is in the same session. The name of the figure is unchanged.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// legendEntryText is the flattened overlay text from the key of an entry up to the
// next marker (or to a bounded length when the marker is empty).
func legendEntryText(t *testing.T, overlay, key, until string) string {
	t.Helper()
	flat := flatText(overlay)
	i := strings.Index(flat, " "+key+" ")
	if i < 0 {
		t.Fatalf("the legend has no entry %q\n%s", key, flat)
	}
	rest := flat[i+len(key)+2:]
	if j := strings.Index(rest, until); until != "" && j >= 0 {
		return rest[:j]
	}
	if len(rest) > 700 {
		rest = rest[:700]
	}
	return rest
}

// assertNewCacheHitDefinition fails unless the text states the pairwise definition
// and none of the old share-of-input meaning.
func assertNewCacheHitDefinition(t *testing.T, desc string) {
	t.Helper()
	assertContains(t, desc, "previous call", "same session", "100%")
	assertNotContains(t, strings.ToLower(desc), "served from the prompt cache", "share of the")
}

func TestCacheHitLegend_DispatchesEntryDescribesThePairwiseDefinition(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles())
	overlay := openOverlay(s, headerW, legendOverlayH)

	assertNewCacheHitDefinition(t, legendEntryText(t, overlay, "cache hit", " rewrites "))
}

func TestCacheHitLegend_ByTypeEntryDescribesThePairwiseDefinition(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles())
	overlay := openOverlay(s, headerW, legendOverlayH)

	assertNewCacheHitDefinition(t, legendEntryText(t, overlay, "cache hit", " repeat spend "))
}

func TestCacheHitLegend_OverviewLegendHasACacheHitRatioEntry(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles())
	body := s.View(headerW, legendOverlayH)
	overlay := openOverlay(s, headerW, legendOverlayH)

	assertContains(t, body, "Cache hit ratio")
	assertContains(t, flatText(overlay), "Cache figures")
	assertNewCacheHitDefinition(t, legendEntryText(t, overlay, "Cache hit ratio", "Columns: models"))
}

func TestCacheHitLegend_OverviewCacheGroupFollowsTheTokensGroup(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles())
	flat := flatText(openOverlay(s, headerW, legendOverlayH))

	tokens := strings.Index(flat, "Columns: tokens")
	cache := strings.Index(flat, "Cache figures")
	models := strings.Index(flat, "Columns: models")

	if tokens < 0 || cache < 0 || models < 0 || !(tokens < cache && cache < models) {
		t.Errorf("want the groups tokens < Cache figures < models, got offsets %d, %d, %d", tokens, cache, models)
	}
}

func TestCacheHitLegend_NoLegendDescribesTheOldShareOfInputMeaning(t *testing.T) {
	colourOff(t)
	views := map[string]screens.Screen{
		"dispatches": screens.NewRunDetailScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles()),
		"by type":    screens.NewAgentTypesScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles()),
		"overview":   screens.NewOverviewScreen(sampleRun(t), headerW, legendOverlayH, paletteStyles()),
	}
	for name, s := range views {
		t.Run(name, func(t *testing.T) {
			overlay := strings.ToLower(flatText(openOverlay(s, headerW, legendOverlayH)))

			assertNotContains(t, overlay, "served from the prompt cache")
		})
	}
}
