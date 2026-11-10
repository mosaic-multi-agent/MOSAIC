package report

import (
	"strconv"
	"strings"

	"mosaic-agent-test/internal/domain"
)

// ModelLabel is the run's model label: its observed subject model(s) when
// ModelValidity == ModelVerified, otherwise "". The selection is never a label.
func (r RunReport) ModelLabel() string {
	if r.ModelValidity != domain.ModelVerified {
		return ""
	}
	return strings.Join(r.ObservedSubjectModels, ", ")
}

// modelSummary is the suite-level model verdict derived from every run: the
// verdict, the label shared by all runs (empty unless verified and common)
// and the number of runs that are not verified.
func modelSummary(tests []TestReport) (domain.ModelVerdict, string, int) {
	runs, unverified := 0, 0
	label, common := "", true
	for _, t := range tests {
		for _, run := range t.Runs {
			runs++
			if run.ModelValidity != domain.ModelVerified {
				unverified++
				continue
			}
			l := run.ModelLabel()
			switch {
			case l == "":
				common = false
			case label == "":
				label = l
			case label != l:
				common = false
			}
		}
	}
	if runs == 0 || unverified > 0 {
		return domain.ModelVerdictUnverified, "", unverified
	}
	if !common {
		label = ""
	}
	return domain.ModelVerdictVerified, label, 0
}

// modelVerification is the wire name of a run's validity; the empty value is
// "not_assessed".
func modelVerification(v domain.ModelValidity) string {
	if v == "" {
		return "not_assessed"
	}
	return string(v)
}

// unverifiedVerdictLine is the prominent invalid-suite statement. It names no
// model: it is not a model result.
func totalRuns(r Result) int {
	n := 0
	for _, t := range r.Tests {
		n += len(t.Runs)
	}
	return n
}

// modelStatusLine is the one-line model statement for a result or a session:
// "Model: <label>" for a verified one with a common label, the prominent
// invalid verdict for an unverified one, "" for a verified one without a
// single common label.
func modelStatusLine(verdict domain.ModelVerdict, label string, runs, unverified int) string {
	if verdict == domain.ModelVerdictVerified {
		if label == "" {
			return ""
		}
		return "Model: " + label
	}
	return verdictLine(runs, unverified)
}

func verdictLine(runs, unverified int) string {
	const suffix = "; this is not a model result"
	if runs == 0 {
		return "MODEL UNVERIFIED: no runs were evaluated" + suffix
	}
	return "MODEL UNVERIFIED: " + strconv.Itoa(unverified) + " run(s) have no verified subject model" + suffix
}

// ModelStatus is the session's model statement, derived from the session
// verdict: the observed model label when every suite is verified under one
// label, the invalid verdict otherwise.
func (s Session) ModelStatus() string {
	runs := 0
	for _, suite := range s.Suites {
		runs += totalRuns(suite)
	}
	return modelStatusLine(s.ModelVerdict, s.ModelLabel, runs, s.UnverifiedRuns)
}

// sessionModelSummary combines the suites' verdicts: verified only when at
// least one suite exists and every suite is verified; the label is the one
// the verified suites share, empty when they differ or when unverified.
func sessionModelSummary(suites []Result) (domain.ModelVerdict, string, int) {
	unverified, label, common := 0, "", true
	verdict := domain.ModelVerdictVerified
	for _, s := range suites {
		unverified += s.UnverifiedRuns
		if s.ModelVerdict != domain.ModelVerdictVerified {
			verdict = domain.ModelVerdictUnverified
			continue
		}
		switch {
		case s.ModelLabel == "":
			common = false
		case label == "":
			label = s.ModelLabel
		case label != s.ModelLabel:
			common = false
		}
	}
	if len(suites) == 0 {
		verdict = domain.ModelVerdictUnverified
	}
	if verdict != domain.ModelVerdictVerified || !common {
		label = ""
	}
	return verdict, label, unverified
}

func unverifiedVerdictLine(r Result) string {
	return verdictLine(totalRuns(r), r.UnverifiedRuns)
}
