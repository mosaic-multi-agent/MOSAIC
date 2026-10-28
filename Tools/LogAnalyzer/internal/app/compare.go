package app

import (
	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// CompareRuns finds both named runs in r and compares them; ok is false when
// either id is not a named run of r.
func CompareRuns(r domain.Report, idA, idB string) (domain.RunComparison, bool) {
	a, okA := r.FindRun(idA)
	b, okB := r.FindRun(idB)
	if !okA || !okB {
		return domain.RunComparison{}, false
	}
	return analysis.CompareRuns(a, b), true
}
