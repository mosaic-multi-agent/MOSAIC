package main

// Report file names derived by the tool never carry a model. The default name
// is built from the suite and the time alone; in particular nothing from the
// model selection reaches it, so a file name can never claim a model that was
// not the one observed.

import (
	"io"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

var defaultNamePattern = regexp.MustCompile(`^report-[A-Za-z0-9._-]+-\d{8}T\d{6}\.json$`)

func TestDefaultReportPath_NameIsSuiteAndTimestampOnly(t *testing.T) {
	now := time.Date(2026, 8, 22, 11, 5, 41, 0, time.UTC)
	for _, suite := range []string{"checkout", "org/project/suite", "claude-opus-4-5-suite"} {
		base := filepath.Base(defaultReportPath(suite, now))

		if !defaultNamePattern.MatchString(base) {
			t.Errorf("defaultReportPath(%q) base = %q, want report-<suite>-<timestamp>.json with no further component", suite, base)
		}
	}
}

func TestCLIReportPathFor_NameIsSuiteAndTimestampOnly(t *testing.T) {
	opts := cliOptions(Deps{}, io.Discard, io.Discard)
	if opts.ReportPathFor == nil {
		t.Fatal("cliOptions leaves ReportPathFor unset")
	}

	base := filepath.Base(opts.ReportPathFor("suites/checkout.suite.yaml"))

	if !defaultNamePattern.MatchString(base) {
		t.Errorf("report path base = %q, want report-<suite>-<timestamp>.json with no model component", base)
	}
}
