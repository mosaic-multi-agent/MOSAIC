package resultstore

import (
	"fmt"
	"strings"
)

// observedModel returns the subject model a run is filed and labelled under:
// the first non-blank observed model, or "" when the run has none.
func observedModel(run RunReportWire) string {
	for _, m := range run.ObservedSubjectModels {
		if strings.TrimSpace(m) != "" {
			return m
		}
	}
	return ""
}

// unverifiedModelReason checks every run of every test and returns a
// human-readable reason when a run has no observed subject model or has one
// that differs from the run's selected subject model. It returns "" when all
// runs are verified.
func unverifiedModelReason(wire ReportWire) string {
	for _, test := range wire.Tests {
		for i, run := range test.Runs {
			where := fmt.Sprintf("test %q run %d", test.TestName, i+1)
			if len(run.ObservedSubjectModels) == 0 {
				return where + ": no observed subject model recorded"
			}
			for _, m := range run.ObservedSubjectModels {
				if strings.TrimSpace(m) == "" {
					return where + ": no observed subject model recorded"
				}
				if m != run.SubjectModel {
					return fmt.Sprintf("%s: observed subject model %q differs from selected model %q",
						where, m, run.SubjectModel)
				}
			}
		}
	}
	return ""
}
