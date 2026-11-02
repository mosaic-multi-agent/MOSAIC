package gantt_test

import (
	"testing"
	"time"

	"mosaic-common/tui/kit/gantt"
)

func TestFormatCollapseLength_ShowsTheTwoLargestNonZeroUnitsAndDropsSeconds(t *testing.T) {
	const (
		min = time.Minute
		hr  = time.Hour
		day = 24 * time.Hour
	)
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"days and hours", 3*day + 2*hr + 5*min + 30*time.Second, "3d 2h"},
		{"whole days", 3 * day, "3d"},
		{"days and minutes skip the zero hours", 3*day + 5*min, "3d 5m"},
		{"hours and minutes", 4*hr + 5*min, "4h 5m"},
		{"whole hours", 2 * hr, "2h"},
		{"hours with only seconds left", hr + 45*time.Second, "1h"},
		{"minutes", 45 * min, "45m"},
		{"minutes drop seconds", 10*min + 59*time.Second, "10m"},
		{"hours roll into days", 26 * hr, "1d 2h"},
		{"three units show two", day + 23*hr + 59*min, "1d 23h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gantt.FormatCollapseLength(tc.d); got != tc.want {
				t.Errorf("FormatCollapseLength(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
