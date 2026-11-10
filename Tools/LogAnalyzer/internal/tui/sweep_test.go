package tui

// Screens compose kit widgets: no file under internal/tui builds a row with a
// hand-set width or padding, or picks a colour inline. Widths come from the kit
// (table, textwidth) and colours from theme roles.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// inlineLayoutPatterns are the constructs the screens must no longer contain.
var inlineLayoutPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"inline colour", regexp.MustCompile(`lipgloss\.(Adaptive|Complete)?Color\(`)},
	{"style width set by hand", regexp.MustCompile(`\.Width\(`)},
	{"padded format verb", regexp.MustCompile(`%-?[0-9*]+[sv]`)},
	{"row padded with spaces", regexp.MustCompile(`strings\.Repeat\(" "`)},
}

func TestSweep_NoScreenOrOverlayBuildsFixedWidthRowsOrPicksColoursInline(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, p := range inlineLayoutPatterns {
				if p.re.MatchString(line) {
					t.Errorf("%s:%d: %s: %s", path, i+1, p.name, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/tui: %v", err)
	}
}
