package screens

// transform_summary_view.go renders the transform summary: the plan listing of every
// outcome with its reason, the counts, the agents left without a model, the run-level
// notes and the TODO path.

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/app"
)

// View renders the transform report. Each line is rendered individually in the style of
// its status. The plan listing is the same for a real run and a dry run; only the header
// says that nothing was written.
func (s *TransformSummaryScreen) View() string {
	lines := s.headerLines()
	for _, f := range s.result.Files {
		lines = append(lines, transformFileLines(f)...)
	}
	lines = append(lines, transformChangeLines("remove", s.result.Removals)...)
	lines = append(lines, transformChangeLines("replace", s.result.Replacements)...)
	lines = append(lines, s.trailerLines()...)
	lines = append(lines, transformLine{text: "press q or enter to exit", style: "help"})

	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(s.renderTransformLine(line))
	}
	return sb.String()
}

// headerLines are the title, harnesses, workspace, agents folder and counts, styled by the
// overall outcome.
func (s *TransformSummaryScreen) headerLines() []transformLine {
	r := s.result
	style := s.overallOutcomeStyle()
	title := "transform complete"
	if r.DryRun {
		title = "dry run: transform plan (nothing is written)"
	}
	lines := []transformLine{
		{text: title, style: style},
		{text: fmt.Sprintf("source: %s → %s", r.SourceHarnessID, r.TargetHarnessID), style: style},
		{text: fmt.Sprintf("workspace: %s", r.Workspace), style: style},
	}
	if r.SourceAgentsDir != "" {
		lines = append(lines, transformLine{text: fmt.Sprintf("agents folder: %s", r.SourceAgentsDir), style: style})
	}
	return append(lines, transformLine{
		text:  fmt.Sprintf("transformed: %d  skipped: %d  failed: %d", r.Transformed, r.Skipped, r.Failed),
		style: style,
	})
}

// trailerLines are the unresolved-model agents, the run-level notes and the TODO path.
func (s *TransformSummaryScreen) trailerLines() []transformLine {
	r := s.result
	var lines []transformLine
	if len(r.UnresolvedModels) > 0 {
		lines = append(lines, transformLine{
			text: fmt.Sprintf("agents written without a model (set one in the file): %s",
				strings.Join(r.UnresolvedModels, ", ")),
			style: "warning",
		})
	}
	for _, n := range r.Notes {
		lines = append(lines, transformLine{text: "note: " + n.Message, style: "warning"})
		for _, p := range n.Paths {
			lines = append(lines, transformLine{text: "  " + p, style: "muted"})
		}
	}
	if r.TodoPath != "" {
		lines = append(lines, transformLine{text: "todo: " + r.TodoPath, style: "muted"})
	}
	return lines
}

// transformFileLines is the plan entry of one item: status, planned action, source and
// destination, and the reason for a skipped or failed item, followed by its tool details.
func transformFileLines(f app.TransformFileOutcome) []transformLine {
	style := "warning"
	switch f.Status {
	case app.StatusTransformed:
		style = "success"
	case app.StatusFailed:
		style = "error"
	}
	lines := []transformLine{{text: transformEntryText(f), style: style}}
	if f.Status != app.StatusTransformed {
		return lines
	}
	if len(f.Tools) > 0 {
		lines = append(lines, transformLine{text: "    tools: " + strings.Join(f.Tools, ", "), style: "muted"})
	}
	if len(f.CustomTools) > 0 {
		lines = append(lines, transformLine{text: "    custom tools: " + strings.Join(f.CustomTools, ", "), style: "muted"})
	}
	if len(f.UnmappedTools) > 0 {
		lines = append(lines, transformLine{
			text:  "    tools without a target equivalent: " + strings.Join(f.UnmappedTools, ", "),
			style: "warning",
		})
	}
	return lines
}

// transformChangeLines lists the files a run removes or replaces, each with its reason and,
// when a backup was taken, the backup path.
func transformChangeLines(verb string, changes []app.TransformFileChange) []transformLine {
	var lines []transformLine
	for _, c := range changes {
		text := fmt.Sprintf("  %s %s:%s %s", verb, c.Kind, c.Key, c.Path)
		if c.Key == "" {
			text = fmt.Sprintf("  %s %s", verb, c.Path)
		}
		if c.Reason != "" {
			text += " — " + c.Reason
		}
		lines = append(lines, transformLine{text: text, style: "warning"})
		if c.BackupPath != "" {
			lines = append(lines, transformLine{text: "    backup: " + c.BackupPath, style: "muted"})
		}
	}
	return lines
}

func transformEntryText(f app.TransformFileOutcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s [%s] ", f.Status, f.Action)
	switch {
	case f.SourcePath != "" && f.DestinationPath != "":
		fmt.Fprintf(&b, "%s → %s", f.SourcePath, f.DestinationPath)
	case f.SourcePath != "":
		b.WriteString(f.SourcePath)
	case f.DestinationPath != "":
		b.WriteString(f.DestinationPath)
	default:
		b.WriteString(f.Key)
	}
	if f.Reason != "" {
		fmt.Fprintf(&b, " — %s", f.Reason)
	}
	return b.String()
}
