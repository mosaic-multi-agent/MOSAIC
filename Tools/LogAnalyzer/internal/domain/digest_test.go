package domain_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestIsDigestFileName_RecognisesBaseNameSuffixCaseInsensitively(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"", false},
		{".digest.json", true},
		{"x.digest.json", true},
		{"X.DIGEST.JSON", true},
		{"x.Digest.Json", true},
		{"x.digest.json.bak", false},
		{"x.json", false},
		{"digest.json", false},
		{"dir.digest.json/file.txt", false},
		{"dir/sub/x.digest.json", true},
		{"dir.digest.json/x.digest.json", true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("IsDigestFileName(%q) panicked: %v", c.path, r)
				}
			}()
			if got := domain.IsDigestFileName(c.path); got != c.want {
				t.Errorf("IsDigestFileName(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}
