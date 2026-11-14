package tui

// The version the TUI is started with reaches the Runs screen the model shows,
// both when the screen is first created by a finished analysis and when a later
// analysis result arrives.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
)

var wiringANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func showRunsScreen(t *testing.T, version string) Model {
	t.Helper()
	m := newNavModel()
	m.opts.Version = version
	updated, _ := m.Update(analysisDoneMsg{
		result: app.Result{Outcome: app.OutcomeReport, Report: makeReport("20260101T120000Z-abcd")},
	})
	m2 := asModel(updated)
	if m2.screen != screenRuns {
		t.Fatalf("screen = %q, want %q", m2.screen, screenRuns)
	}
	return m2
}

func TestModel_RunsScreen_TitleCarriesTheConfiguredVersion(t *testing.T) {
	m := showRunsScreen(t, "7.8.9")

	view := wiringANSI.ReplaceAllString(m.View(), "")

	want := "Log Analyzer v7.8.9 \u2014 Runs"
	if !strings.Contains(view, want) {
		t.Errorf("runs screen view does not contain %q:\n%s", want, view)
	}
}

func TestModel_RunsScreen_NoConfiguredVersion_KeepsUnversionedTitle(t *testing.T) {
	m := showRunsScreen(t, "")

	view := wiringANSI.ReplaceAllString(m.View(), "")

	if !strings.Contains(view, "Log Analyzer \u2014 Runs") {
		t.Errorf("runs screen view does not contain the unversioned title:\n%s", view)
	}
}

func TestModel_RunsScreen_NewAnalysisResultKeepsTheVersionInTheTitle(t *testing.T) {
	m := showRunsScreen(t, "7.8.9")

	updated, _ := m.Update(analysisDoneMsg{
		result: app.Result{Outcome: app.OutcomeReport, Report: makeReport("20260102T120000Z-efgh")},
	})
	view := wiringANSI.ReplaceAllString(asModel(updated).View(), "")

	want := "Log Analyzer v7.8.9 \u2014 Runs"
	if !strings.Contains(view, want) {
		t.Errorf("runs screen view after a second analysis does not contain %q:\n%s", want, view)
	}
}
