package notationscan_test

// Guard: no Go test file, testdata file or TestCatalog file under Tools/Runner
// uses WorkflowFormat 1.x routing-table notation, apart from the files that
// hold it on purpose as the input of a test asserting its refusal.
//
// The 1.x notation is the numbered block reference of a phase, the numbered
// fill-in of an artifact path and the heading of the 1.x group table. The
// tokens are assembled from parts below so that this file does not contain
// them itself.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runnerRoot is Tools/Runner, relative to this package's directory.
const runnerRoot = "../.."

// retiredTokens are the 1.x routing-table tokens.
var retiredTokens = []string{
	"[Stage" + "Number]",
	"{Stage" + "Number}",
	"**Execution " + "Groups:**",
}

// refusalInputs are the files, as slash-separated paths relative to
// Tools/Runner, that hold the tokens as inputs of refusal tests.
var refusalInputs = map[string]bool{
	"internal/workflow/strict_1x_notation_test.go":      true,
	"internal/workflow/strict_region_refusals_test.go":  true,
	"internal/session/session_retired_notation_test.go": true,
}

// skippedDirs are directories the walk never enters.
var skippedDirs = map[string]bool{
	".git":          true,
	".pytest_cache": true,
	"node_modules":  true,
}

// isScanned reports whether the file at rel (slash-separated, relative to
// Tools/Runner) is a Go test file, a testdata file or a TestCatalog file.
func isScanned(rel string) bool {
	if strings.HasSuffix(rel, "_test.go") {
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "testdata" || seg == "TestCatalog" {
			return true
		}
	}
	return false
}

// scan walks Tools/Runner and returns the number of files it examined and,
// for every file that holds a retired token, the tokens it holds.
func scan(t *testing.T) (examined int, hits map[string][]string) {
	t.Helper()
	hits = map[string][]string{}
	err := filepath.WalkDir(runnerRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(runnerRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !isScanned(rel) || refusalInputs[rel] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		examined++
		for _, token := range retiredTokens {
			if strings.Contains(string(data), token) {
				hits[rel] = append(hits[rel], token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", runnerRoot, err)
	}
	return examined, hits
}

func TestRunnerTests_UseNoRetiredRoutingTableNotation(t *testing.T) {
	examined, hits := scan(t)

	if examined == 0 {
		t.Fatal("the scan examined no file: the walk root or the file selection is wrong")
	}
	var files []string
	for rel := range hits {
		files = append(files, rel)
	}
	sort.Strings(files)
	for _, rel := range files {
		t.Errorf("%s uses 1.x routing-table notation %q; port it to WorkflowFormat 2.16 or, when it is the input of a refusal test, list it in refusalInputs",
			rel, hits[rel])
	}
}

func TestRetiredNotationScan_EveryListedRefusalInputExists(t *testing.T) {
	for rel := range refusalInputs {
		if _, err := os.Stat(filepath.Join(runnerRoot, filepath.FromSlash(rel))); err != nil {
			t.Errorf("refusalInputs lists %s, which cannot be read: %v", rel, err)
		}
	}
}
