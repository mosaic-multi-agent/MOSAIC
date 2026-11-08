package activity_test

// Helpers for the status tests: decode a fixture line by line and read the
// cumulative status after every line.

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

// fixtureLines returns the lines of testdata/<name>.
func fixtureLines(t *testing.T, name string) [][]byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	var lines [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, []byte(sc.Text()))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	return lines
}

// statusAfterEachLine decodes every fixture line with dec and returns
// dec.Status() after each one.
func statusAfterEachLine(t *testing.T, dec activity.Decoder, name string) []domain.ActivityStatus {
	t.Helper()
	var out []domain.ActivityStatus
	for _, l := range fixtureLines(t, name) {
		dec.Decode(l)
		out = append(out, dec.Status())
	}
	return out
}

// statusAfter decodes the given lines and returns the final status.
func statusAfter(dec activity.Decoder, lines ...string) domain.ActivityStatus {
	for _, l := range lines {
		dec.Decode([]byte(l))
	}
	return dec.Status()
}

// assertStatuses compares got with want line by line; costs compare with a
// tolerance.
func assertStatuses(t *testing.T, got, want []domain.ActivityStatus) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("want %d statuses, got %d", len(want), len(got))
	}
	for i := range want {
		g, w := got[i], want[i]
		if !approxEqual(g.CostUSD, w.CostUSD) {
			t.Errorf("line %d: want cost %v, got %v", i+1, w.CostUSD, g.CostUSD)
		}
		g.CostUSD, w.CostUSD = 0, 0
		if g != w {
			t.Errorf("line %d: want status %+v, got %+v", i+1, w, g)
		}
	}
}
