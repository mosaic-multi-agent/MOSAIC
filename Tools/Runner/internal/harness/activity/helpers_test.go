package activity_test

// Helpers shared by the decoder tests: reading a fixture transcript, decoding
// it line by line, and comparing the resulting items.

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

// decodeTranscript decodes every line of testdata/<name> with a fresh decoder
// and returns all items in order.
func decodeTranscript(t *testing.T, dec activity.Decoder, name string) []domain.ActivityItem {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	var all []domain.ActivityItem
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		all = append(all, dec.Decode([]byte(sc.Text()))...)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	return all
}

// withoutUsage drops usage items.
func withoutUsage(items []domain.ActivityItem) []domain.ActivityItem {
	var out []domain.ActivityItem
	for _, it := range items {
		if it.Kind != domain.ActivityKindUsage {
			out = append(out, it)
		}
	}
	return out
}

// sumUsage adds up every usage item.
func sumUsage(items []domain.ActivityItem) domain.ActivityUsage {
	var sum domain.ActivityUsage
	for _, it := range items {
		if it.Kind != domain.ActivityKindUsage {
			continue
		}
		sum.InputTokens += it.Usage.InputTokens
		sum.OutputTokens += it.Usage.OutputTokens
		sum.CostUSD += it.Usage.CostUSD
		sum.HasCost = sum.HasCost || it.Usage.HasCost
	}
	return sum
}

// wantItem describes the expected kind and name of one item and a fragment its
// summary must contain.
type wantItem struct {
	kind     domain.ActivityKind
	name     string
	contains string
}

func assertItems(t *testing.T, got []domain.ActivityItem, want []wantItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("want %d items, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.kind || g.Name != w.name || !strings.Contains(g.Summary, w.contains) {
			t.Errorf("item %d: want kind=%q name=%q summary containing %q, got %+v", i, w.kind, w.name, w.contains, g)
		}
		if g.Kind == domain.ActivityToolCall && strings.Contains(g.Summary, "{") {
			t.Errorf("item %d: tool-call summary must be a short form, not raw JSON: %q", i, g.Summary)
		}
	}
}

func approxEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// assertSingleLine checks the short-form rules for a summary.
func assertSingleLine(t *testing.T, summary string) {
	t.Helper()
	if strings.ContainsAny(summary, "\r\n") {
		t.Errorf("summary spans several lines: %q", summary)
	}
	if n := len([]rune(summary)); n > domain.ActivitySummaryMaxRunes {
		t.Errorf("summary has %d runes, limit is %d", n, domain.ActivitySummaryMaxRunes)
	}
}
