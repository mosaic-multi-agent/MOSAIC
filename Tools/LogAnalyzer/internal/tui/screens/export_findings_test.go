package screens_test

// The findings export prompt: the digest path entry with its own default name,
// validation (any .json name that is not a digest name), heading, and the same
// navigation and overwrite confirmation as the digest export.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

const findingsRun = "20261031T073445Z-0ada"

func newFindingsExport(label string) *screens.ExportScreen {
	return screens.NewFindingsExportScreen(label, 120, paletteStyles())
}

// typeReplacing clears the n pre-filled characters and types text.
func typeReplacing(s *screens.ExportScreen, n int, text string) {
	clearInput(s, n)
	for _, r := range text {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// findingsConfirming returns a findings export screen asking whether to overwrite.
func findingsConfirming(t *testing.T, label string) *screens.ExportScreen {
	t.Helper()
	s := newFindingsExport(label)
	press(s, "enter")
	if s.Action() != screens.ExportSubmit {
		t.Fatalf("setup: action = %v, want submit", s.Action())
	}
	s.AskOverwrite()
	return s
}

func TestFindingsFileName(t *testing.T) {
	if got := screens.FindingsFileName(""); got != "findings.json" {
		t.Errorf("FindingsFileName(\"\") = %q, want findings.json", got)
	}
	if got, want := screens.FindingsFileName(findingsRun), "findings-"+findingsRun+".json"; got != want {
		t.Errorf("FindingsFileName(run) = %q, want %q", got, want)
	}
}

func TestFindingsExportScreen_RunsListIsPrefilledAndHeadedForAllFindings(t *testing.T) {
	colourOff(t)

	view := stripStyling(newFindingsExport("").View(120, 24))

	assertContains(t, view, "Export findings", "findings.json")
	assertNotContains(t, view, "Export all runs", "Export run", "findings of run", ".digest.json")
}

func TestFindingsExportScreen_InARunIsPrefilledWithTheRunNameAndHeadedWithIt(t *testing.T) {
	colourOff(t)

	view := stripStyling(newFindingsExport(findingsRun).View(120, 24))

	assertContains(t, view, "Export findings of run "+findingsRun, "findings-"+findingsRun+".json")
	assertNotContains(t, view, ".digest.json", "Export all runs")
}

func TestFindingsExportScreen_EnterSubmitsThePrefilledName(t *testing.T) {
	for label, want := range map[string]string{"": "findings.json", findingsRun: "findings-" + findingsRun + ".json"} {
		t.Run("label="+label, func(t *testing.T) {
			s := newFindingsExport(label)

			press(s, "enter")

			if s.Action() != screens.ExportSubmit {
				t.Fatalf("action = %v, want submit for a valid pre-filled name", s.Action())
			}
			if s.Path() != want {
				t.Errorf("path = %q, want %q", s.Path(), want)
			}
		})
	}
}

func TestFindingsExportScreen_AcceptsAnyJSONNameWithoutTheDigestSuffix(t *testing.T) {
	for _, name := range []string{"notes.json", "out/dir/all.json", "findings.report.json"} {
		t.Run(name, func(t *testing.T) {
			s := newFindingsExport("")
			typeReplacing(s, len("findings.json"), name)

			press(s, "enter")

			if s.Action() != screens.ExportSubmit || s.Path() != name {
				t.Errorf("action = %v path = %q, want submit of %q", s.Action(), s.Path(), name)
			}
		})
	}
}

func TestFindingsExportScreen_RejectsNamesThatAreNotFindingsFiles(t *testing.T) {
	cases := map[string]string{"empty": "", "no suffix": "findings", "text file": "findings.txt", "digest name": "runs.digest.json"}
	for name, typed := range cases {
		t.Run(name, func(t *testing.T) {
			colourOff(t)
			s := newFindingsExport("")
			typeReplacing(s, len("findings.json"), typed)

			press(s, "enter")

			if s.Action() == screens.ExportSubmit {
				t.Fatalf("%q must be rejected, got a submit of %q", typed, s.Path())
			}
			assertContains(t, stripStyling(s.View(120, 24)), ".json")
		})
	}
}

func TestFindingsExportScreen_ValidationMessageNamesTheJSONRuleNotTheDigestRule(t *testing.T) {
	colourOff(t)
	s := newFindingsExport("")
	typeReplacing(s, len("findings.json"), "findings.txt")

	press(s, "enter")

	view := stripStyling(s.View(120, 24))
	assertContains(t, view, "path must end in .json")
	assertNotContains(t, view, "path must end in .digest.json")
}

func TestFindingsExportScreen_DigestNameIsExplainedAsNotAllowed(t *testing.T) {
	colourOff(t)
	s := newFindingsExport("")
	typeReplacing(s, len("findings.json"), "runs.digest.json")

	press(s, "enter")

	assertContains(t, stripStyling(s.View(120, 24)), ".digest.json")
}

func TestFindingsExportScreen_EscOnPathEntryCancels(t *testing.T) {
	s := newFindingsExport("")

	press(s, "esc")

	if s.Action() != screens.ExportCancel {
		t.Errorf("action = %v, want cancel", s.Action())
	}
}

func TestFindingsExportScreen_OverwriteQuestionNamesThePathAndAnswers(t *testing.T) {
	colourOff(t)
	s := findingsConfirming(t, findingsRun)

	view := stripStyling(s.View(120, 24))

	assertContains(t, view, "findings-"+findingsRun+".json", "Overwrite")
}

func TestFindingsExportScreen_OverwriteAnswers(t *testing.T) {
	cases := []struct {
		key  string
		want screens.ExportAction
	}{
		{"y", screens.ExportOverwrite},
		{"Y", screens.ExportOverwrite},
		{"n", screens.ExportDecline},
		{"N", screens.ExportDecline},
		{"esc", screens.ExportDecline},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			s := findingsConfirming(t, "")

			press(s, c.key)

			if s.Action() != c.want {
				t.Errorf("after %q: action = %v, want %v", c.key, s.Action(), c.want)
			}
		})
	}
}

func TestFindingsExportScreen_OverwriteQuestionIgnoresOtherKeys(t *testing.T) {
	s := findingsConfirming(t, "")

	press(s, "x", "enter", "j")

	if s.Action() != screens.ExportNone {
		t.Errorf("action = %v, want none until y, n or esc", s.Action())
	}
	if !strings.Contains(stripStyling(s.View(120, 24)), "Overwrite") {
		t.Error("the overwrite question must stay on screen")
	}
}
