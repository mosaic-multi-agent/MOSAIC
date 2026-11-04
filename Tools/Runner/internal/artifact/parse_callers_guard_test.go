package artifact_test

// Guard: a refusal only names its file when the caller reads it through
// ParseAt. Any direct artifact.Parse call outside this package drops the path.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var directParseCall = regexp.MustCompile(`\bartifact\.Parse\(`)

func TestNoProductionCallerUsesParseWithoutAPath(t *testing.T) {
	root := filepath.Join("..", "..") // the Runner tool tree
	var offenders []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(path)
		if d.IsDir() {
			if strings.HasSuffix(slash, "internal/artifact") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if directParseCall.MatchString(line) {
				offenders = append(offenders, slash+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("walking the Runner tree: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("call artifact.ParseAt(path, data) instead of artifact.Parse at: %s", strings.Join(offenders, ", "))
	}
}
