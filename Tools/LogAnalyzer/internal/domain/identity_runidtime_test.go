package domain_test

// The start moment encoded in a run ID.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestRunIDTime_ReadsTheEncodedMomentInUTC(t *testing.T) {
	got, ok := domain.RunIDTime("20261112T175027Z-2d24")

	want := time.Date(2026, 11, 12, 17, 50, 27, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("RunIDTime = %v, %v; want %v, true", got, ok, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("the moment must be in UTC, got %v", got.Location())
	}
}

func TestRunIDTime_RefusesWhatIsNotAValidRunIDTimestamp(t *testing.T) {
	cases := []struct{ name, id string }{
		{"empty", ""},
		{"a plain name", "my-named-run"},
		{"no suffix", "20261112T175027Z"},
		{"month 13", "20261313T175027Z-2d24"},
		{"day 32", "20261132T175027Z-2d24"},
		{"hour 25", "20261112T255027Z-2d24"},
		{"minute 61", "20261112T176127Z-2d24"},
		{"29 February in a common year", "20270229T120000Z-2d24"},
		{"trailing text", "20261112T175027Z-2d24x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := domain.RunIDTime(c.id)

			if ok || !got.IsZero() {
				t.Errorf("RunIDTime(%q) = %v, %v; want the zero time and false", c.id, got, ok)
			}
		})
	}
}

func TestRunIDTime_AcceptsALeapDay(t *testing.T) {
	got, ok := domain.RunIDTime("20280229T000001Z-abcd")

	want := time.Date(2028, 2, 29, 0, 0, 1, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("RunIDTime = %v, %v; want %v, true", got, ok, want)
	}
}
