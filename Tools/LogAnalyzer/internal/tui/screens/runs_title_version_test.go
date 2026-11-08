package screens_test

// The runs page title carries the tool version, in the same form as the source
// page, with the em dash separator the runs title always used. A screen that was
// given no version keeps the unversioned title.

import (
	"strings"
	"testing"
)

const runsTitleSeparator = "—"

func versionedRunsTitle(version string) string {
	return "Log Analyzer v" + version + " " + runsTitleSeparator + " Runs"
}

func TestRunsPage_VersionIsPartOfTheTitle(t *testing.T) {
	colourOff(t)
	s := newRuns(140, tallH)
	s.SetVersion("1.2.3")

	lines := viewLines(s.View(140, tallH))

	assertContains(t, headOf(lines), versionedRunsTitle("1.2.3"))
}

func TestRunsPage_WithoutVersionKeepsTheUnversionedTitle(t *testing.T) {
	colourOff(t)

	lines := viewLines(newRuns(140, tallH).View(140, tallH))

	head := headOf(lines)
	assertContains(t, head, "Log Analyzer "+runsTitleSeparator+" Runs")
	if strings.Contains(head, "Log Analyzer v") {
		t.Errorf("an unversioned runs page must not show a version, head: %q", head)
	}
}

func TestRunsPage_VersionSurvivesSetReportAndResize(t *testing.T) {
	colourOff(t)
	s := newRuns(140, tallH)
	s.SetVersion("4.5.6")

	s.SetReport(runsReport())
	lines := viewLines(s.View(120, 40))

	assertContains(t, headOf(lines), versionedRunsTitle("4.5.6"))
}

func TestRunsPage_LaterSetVersionReplacesTheVersion(t *testing.T) {
	colourOff(t)
	s := newRuns(140, tallH)
	s.SetVersion("1.0.0")
	s.SetVersion("2.0.0")

	head := headOf(viewLines(s.View(140, tallH)))

	assertContains(t, head, versionedRunsTitle("2.0.0"))
	if strings.Contains(head, "v1.0.0") {
		t.Errorf("the replaced version must not remain, head: %q", head)
	}
}
