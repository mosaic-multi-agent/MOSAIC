package domain_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestIsFindingsFileName(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"findings.json", true},
		{"findings-20261031T073445Z-0ada.json", true},
		{"out/dir/findings.json", true},
		{"FINDINGS.JSON", true},
		{"my.report.json", true},
		{"a.json", true},
		{"findings.digest.json", false},
		{"runs.DIGEST.JSON", false},
		{"out/x.digest.json", false},
		{".json", false},
		{"findings.txt", false},
		{"findings", false},
		{"findings.json.bak", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := domain.IsFindingsFileName(c.path); got != c.want {
				t.Errorf("IsFindingsFileName(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}
