package screens

import (
	"fmt"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// KeyFindings toggles the findings overlay on the runs list and in every per-run view.
const KeyFindings = "f"

// findingsKeyEntry is the footer entry every screen with a findings overlay lists.
var findingsKeyEntry = tuicommon.HelpEntry{Key: KeyFindings, Desc: "findings"}

// findingsOverlayKeys is the short help while the findings overlay is open.
var findingsOverlayKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: KeyExportRun, Desc: "export"}, {Key: KeyFindings + "/esc", Desc: "close"}}

// findingsHelpEntry is the help overlay entry of the findings key.
var findingsHelpEntry = tuicommon.HelpEntry{Key: KeyFindings, Desc: "show what kinds of findings exist, how many and what they do to the figures"}

// FindingsScope is what the findings overlay shows and what the export writes:
// both derive from the same slice, so the counts are equal.
type FindingsScope struct {
	Label    string           // "" on the runs list; the run label inside a run
	Findings []domain.Finding // exactly the findings the overlay counts
}

// RunFindingsOf returns the findings of q whose Run equals run, in QualitySummary
// order; nil when there are none.
func RunFindingsOf(q domain.QualitySummary, run domain.RunRef) []domain.Finding {
	var own []domain.Finding
	for _, f := range q.Findings {
		if f.Run == run {
			own = append(own, f)
		}
	}
	return own
}

// findingsOverlay is the state of the f overlay of one screen. While it is open
// it takes every key; its scroll position is independent of the body's.
type findingsOverlay struct {
	open   bool
	offset int
}

// handle consumes a key meant for the overlay and reports whether it did: the
// findings key opens it, and while open every key is consumed so nothing leaks
// to the screen underneath.
func (o *findingsOverlay) handle(key string) bool {
	if !o.open {
		if key != KeyFindings {
			return false
		}
		o.open, o.offset = true, 0
		return true
	}
	switch key {
	case KeyFindings, "esc":
		o.open = false
	case "up", "k":
		o.offset = max(o.offset-1, 0)
	case "down", "j":
		o.offset++
	}
	return true
}

// lines renders the scope as one heading line and one row per finding kind within
// bodyHeight lines, clamping the scroll offset to the rows. withRuns adds how
// many runs each kind occurs in.
func (o *findingsOverlay) lines(scope FindingsScope, withRuns bool, width, bodyHeight int, st Styles) []string {
	if bodyHeight <= 0 {
		return nil
	}
	rows := domain.SummarizeFindingKinds(scope.Findings)
	if len(rows) == 0 {
		return []string{st.Muted.Render("no findings")}
	}
	heading := st.Subtitle.Render("Findings") + st.Body.Render(fmt.Sprintf("  %s in %s", countLabel(len(scope.Findings), "finding"), countLabel(len(rows), "kind")))
	visible := bodyHeight - 1
	o.offset = min(o.offset, max(len(rows)-visible, 0))
	end := min(o.offset+visible, len(rows))
	return append([]string{heading}, findingRowLines(rows, withRuns, st)[o.offset:end]...)
}

// findingRowLines renders every row with the severity, count, kind, run count and
// explanation in aligned columns.
func findingRowLines(rows []domain.FindingKindSummary, withRuns bool, st Styles) []string {
	countW, kindW, runsW := 0, 0, 0
	for _, r := range rows {
		countW = max(countW, len(fmt.Sprint(r.Count)))
		kindW = max(kindW, textwidth.Of(string(r.Kind)))
		runsW = max(runsW, textwidth.Of(inRunsText(r.Runs)))
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		sevStyle := st.Muted
		if r.Severity == domain.SeverityWarning {
			sevStyle = st.Warning
		}
		line := sevStyle.Render(textwidth.Pad(r.Severity.Label(), len("warning"), false)) +
			st.Body.Render(fmt.Sprintf("  %*d  ", countW, r.Count)) +
			st.Subtitle.Render(textwidth.Pad(string(r.Kind), kindW, false))
		if withRuns {
			line += st.Body.Render("  " + textwidth.Pad(inRunsText(r.Runs), runsW, false))
		}
		if text := r.Kind.Explanation(); text != string(r.Kind) {
			line += st.Body.Render("  " + text)
		}
		out[i] = strings.TrimRight(line, " ")
	}
	return out
}

// inRunsText is "in N runs" for the number of runs a kind occurs in, "" for none.
func inRunsText(n int) string {
	if n == 0 {
		return ""
	}
	return "in " + countLabel(n, "run")
}

// FindingsOpen reports whether the findings overlay of the runs list is showing.
func (s *RunsScreen) FindingsOpen() bool { return s.findings.open }

// FindingsScope returns the findings the runs-list overlay covers: every finding
// of the report.
func (s *RunsScreen) FindingsScope() FindingsScope {
	return FindingsScope{Findings: s.report.Quality.Findings}
}

// FindingsOpen reports whether the findings overlay of a per-run view is showing.
func (c *runChrome) FindingsOpen() bool { return c.findings.open }

// FindingsScope returns the findings the overlay of a per-run view covers, under
// the label of their run.
func (c *runChrome) FindingsScope() FindingsScope {
	label := c.runLabel
	if len(c.runFindings) > 0 {
		label = c.runFindings[0].Run.Label()
	}
	return FindingsScope{Label: label, Findings: c.runFindings}
}

// SetRunFindings sets the findings the overlay of a per-run view covers.
func (c *runChrome) SetRunFindings(findings []domain.Finding) { c.runFindings = findings }

// handleOverlays gives a key to the open overlay first: the findings overlay
// opens only while the help overlay is closed, and the help key does nothing
// inside the findings overlay.
func (c *runChrome) handleOverlays(key string) bool {
	if !c.help.open && c.findings.handle(key) {
		return true
	}
	return c.help.handle(key)
}
