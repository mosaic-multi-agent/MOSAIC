package screens

import (
	"mosaic-common/tui/pathutil"
	"mosaic-common/tui/widgets"

	"mosaic-log-analyzer/internal/domain"
)

// findingsExportTitle is the title of the findings export page.
const findingsExportTitle = "Log Analyzer -- Export findings"

// FindingsFileName returns the pre-filled name for a scope label:
// "" -> "findings.json"; otherwise "findings-<label>.json".
func FindingsFileName(scopeLabel string) string {
	if scopeLabel == "" {
		return "findings" + domain.FindingsFileSuffix
	}
	return "findings-" + scopeLabel + domain.FindingsFileSuffix
}

// NewFindingsExportScreen returns an ExportScreen for a findings export with the
// same navigation contract as the digest one (Enter on a valid path ->
// ExportSubmit; Esc -> ExportCancel; after AskOverwrite: y -> ExportOverwrite,
// n/Esc -> ExportDecline). It is pre-filled with FindingsFileName(scopeLabel) and
// validated by domain.IsFindingsFileName. Its heading is "Export findings" ("" label)
// or "Export findings of run <label>".
func NewFindingsExportScreen(scopeLabel string, width int, styles Styles) *ExportScreen {
	input := widgets.NewTextInput("Findings file path:", "path"+domain.FindingsFileSuffix, width, textInputStyles(styles))
	input.SetValidate(validateFindingsPath)
	input.SetValue(FindingsFileName(scopeLabel))
	return &ExportScreen{runID: scopeLabel, input: input, styles: styles, findings: true}
}

func validateFindingsPath(s string) error {
	p := pathutil.NormalizeInput(s)
	if p == "" {
		return errEmptyPath
	}
	if !domain.IsFindingsFileName(p) {
		return errString("path must end in " + domain.FindingsFileSuffix + " (" + domain.DigestFileSuffix + " is not allowed)")
	}
	return nil
}
