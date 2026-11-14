package main

// module_boundary_test.go runs the complete boundary checker over the real module, so a
// format-boundary violation in application code fails the normal test run and not only the
// separate import check task.

import (
	"strings"
	"testing"
)

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
