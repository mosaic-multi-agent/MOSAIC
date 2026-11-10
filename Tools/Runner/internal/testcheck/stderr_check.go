package testcheck

import (
	"fmt"
	"strings"
)

// stderrExcerptBytes bounds how much of the actual stderr is quoted in a
// stderr mismatch. The tail is kept because stop reasons are written last.
const stderrExcerptBytes = 300

// checkStderr verifies the expected stderr substring, when one is set.
// An empty StderrContains means stderr is not checked.
func checkStderr(actual CheckInput, expected *ExpectedOutcome) CheckResult {
	if expected.StderrContains == "" || strings.Contains(actual.Stderr, expected.StderrContains) {
		return CheckResult{Pass: true}
	}
	excerpt := actual.Stderr
	if len(excerpt) > stderrExcerptBytes {
		excerpt = excerpt[len(excerpt)-stderrExcerptBytes:]
	}
	return CheckResult{
		Pass: false,
		Mismatch: &Mismatch{
			Kind:          MismatchStderrContains,
			Index:         -1,
			Message:       fmt.Sprintf("stderr does not contain %q; stderr (tail): %q", expected.StderrContains, excerpt),
			ExpectedValue: expected.StderrContains,
			ActualValue:   excerpt,
		},
	}
}
