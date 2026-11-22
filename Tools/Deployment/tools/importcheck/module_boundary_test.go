package main

// module_boundary_test.go runs the complete boundary checker over the real module, so a
// format-boundary violation in application code fails the normal test run and not only the
// separate import check task.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleBoundaries_CommonRulesScanRealFiles asserts that every rule applied to the
// mosaic-common module points at an existing directory holding at least one non-test Go
// file. The checker skips a missing directory silently, so without this test a moved
// package would leave its rule checking nothing while the real-tree test stays green.
func TestModuleBoundaries_CommonRulesScanRealFiles(t *testing.T) {
	commonRoot := filepath.Join("..", "..", "..", "Common")
	checked := 0
	for _, r := range rules {
		if !r.inCommon {
			continue
		}
		checked++
		dir := filepath.Join(commonRoot, filepath.FromSlash(r.dir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("rule directory %s is not readable: %v", r.location(), err)
			continue
		}
		found := false
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule directory %s holds no non-test Go file; the rule would check nothing", r.location())
		}
	}
	if checked == 0 {
		t.Fatal("no rule is marked inCommon; the mosaic-common rules are missing")
	}
}

// TestModuleBoundaries_RealTree asserts that every rule in runChecks holds for the module
// containing this package. The module root is two directories above tools/importcheck, the
// package directory in which tests run.
func TestModuleBoundaries_RealTree(t *testing.T) {
	violations, errs := runChecks("../..")

	if len(errs) > 0 {
		t.Errorf("boundary checker reported %d tool error(s):\n%s", len(errs), strings.Join(errs, "\n"))
	}
	if len(violations) > 0 {
		t.Errorf("boundary checker reported %d violation(s):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
