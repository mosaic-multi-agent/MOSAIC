package otelingest_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The last part of the split case and the one-go case must hold the same expected streams and state; a difference
// means the two sets of expected files drifted apart, not that the state is allowed to differ.
func TestSplitCaseEndsLikeFullCase(t *testing.T) {
	split := filepath.Join("testdata", "conformance", "split_inside_parent_child_pair", "expected", "part2")
	full := filepath.Join("testdata", "conformance", "split_inside_parent_child_pair_full", "expected", "full")

	checked := 0
	err := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "result.json" {
			return err
		}
		rel, _ := filepath.Rel(full, path)
		want, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		got, readErr := os.ReadFile(filepath.Join(split, rel))
		if readErr != nil {
			t.Errorf("%s: missing from the split case: %v", rel, readErr)
			return nil
		}
		checked++
		if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
			t.Errorf("%s differs between the split and the full case", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatalf("no expected files compared under %s", full)
	}
}
