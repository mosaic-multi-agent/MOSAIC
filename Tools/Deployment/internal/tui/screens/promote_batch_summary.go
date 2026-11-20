package screens

// promote_batch_summary.go implements PromoteBatchSummaryScreen, the final screen of a batch
// promote. It lists every selected agent as promoted, skipped, failed or not processed. The
// scroll keys (up, down, k, j) scroll a summary taller than the window; every other key
// dismisses the screen.

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/app"
)

const batchSummaryHelp = "↑/k up  ↓/j down  other keys: back to start  ctrl+c: quit"

// PromoteBatchSummaryScreen lists every selected agent of a batch promote with its outcome.
type PromoteBatchSummaryScreen struct {
	result       app.PromoteBatchResult
	lines        []summaryLine
	scrollOffset int
	done         bool
	width        int
	height       int
	styles       Styles
}

// NewPromoteBatchSummaryScreen builds the screen from the result of a batch promote.
func NewPromoteBatchSummaryScreen(result app.PromoteBatchResult, width, height int, styles Styles) *PromoteBatchSummaryScreen {
	return &PromoteBatchSummaryScreen{
		result: result,
		lines:  buildBatchSummaryLines(result),
		width:  width,
		height: height,
		styles: styles,
	}
}

// buildBatchSummaryLines constructs the scrollable content: the target layer, the counts line
// and one entry per selected agent.
func buildBatchSummaryLines(r app.PromoteBatchResult) []summaryLine {
	var lines []summaryLine
	add := func(text, style string) { lines = append(lines, summaryLine{text: text, style: style}) }

	layer := r.TargetLayer
	if layer == "" {
		layer = "not chosen"
	}
	add("Target layer: "+layer, "body")

	counts := map[app.PromoteBatchStatus]int{}
	for _, a := range r.Agents {
		counts[a.Status]++
	}
	add(fmt.Sprintf("%d promoted, %d skipped, %d failed, %d not processed",
		counts[app.PromoteBatchPromoted], counts[app.PromoteBatchSkipped],
		counts[app.PromoteBatchFailed], counts[app.PromoteBatchNotProcessed]), "body")
	add("", "body")

	for _, a := range r.Agents {
		appendBatchAgentLines(add, a)
	}
	return lines
}

// appendBatchAgentLines adds the entry of one agent: a header line with key, file and status,
// then the details of the outcome.
func appendBatchAgentLines(add func(text, style string), a app.PromoteBatchAgentResult) {
	file := filepath.Base(a.SourcePath)
	switch a.Status {
	case app.PromoteBatchPromoted:
		add(fmt.Sprintf("%s (%s): promoted", a.Key, file), "success")
		add(fmt.Sprintf("    destination: %s (id %s)", a.Result.DestinationPath, a.Result.NumericID), "body")
		if a.ReplacedLayer != "" {
			add("    replaced the agent in "+a.ReplacedLayer, "body")
		}
	case app.PromoteBatchSkipped:
		add(fmt.Sprintf("%s (%s): skipped", a.Key, file), "warning")
		add("    "+a.Reason, "body")
	case app.PromoteBatchFailed:
		add(fmt.Sprintf("%s (%s): failed", a.Key, file), "error")
		reason := a.Reason
		if a.Err != nil {
			reason = a.Err.Error()
		}
		add("    "+reason, "body")
	default:
		add(fmt.Sprintf("%s (%s): not processed", a.Key, file), "muted")
	}
}

// renderLine applies the named style to one summary line and wraps it to the width.
func (s *PromoteBatchSummaryScreen) renderLine(line summaryLine) []string {
	style := s.styles.Body
	switch line.style {
	case "success":
		style = s.styles.Success
	case "warning":
		style = s.styles.Warning
	case "error":
		style = s.styles.Error
	case "muted":
		style = s.styles.Muted
	}
	return strings.Split(style.Width(s.width).Render(line.text), "\n")
}

// wrapped returns the content lines as rendered at the current width.
func (s *PromoteBatchSummaryScreen) wrapped() []string {
	var out []string
	for _, l := range s.lines {
		out = append(out, s.renderLine(l)...)
	}
	return out
}

// frame renders the fixed parts of the screen.
func (s *PromoteBatchSummaryScreen) frame() (title, border, help string) {
	text := "Promote complete"
	if s.result.Aborted {
		text = "Promote aborted"
	}
	title = s.styles.Title.Width(s.width).Render(text)
	border = s.styles.Border.Width(s.width).Render(strings.Repeat("─", s.width))
	help = s.styles.Help.Width(s.width).Render(batchSummaryHelp)
	return title, border, help
}

// visibleLines is the height of the scrollable window: the terminal height without the frame
// and, when the content is taller than the window, the position indicator line.
func (s *PromoteBatchSummaryScreen) visibleLines(total int) int {
	title, border, help := s.frame()
	avail := s.height - frameLines(title, border, help)
	if avail < 1 {
		avail = 1
	}
	if total > avail {
		avail--
		if avail < 1 {
			avail = 1
		}
	}
	return avail
}

// maxOffset is the largest scroll offset.
func (s *PromoteBatchSummaryScreen) maxOffset() int {
	total := len(s.wrapped())
	return max(0, total-s.visibleLines(total))
}

// Update processes a message. It reports true (and Done becomes true) when the user dismisses
// the screen: on every key except the scroll keys up, k, down and j.
func (s *PromoteBatchSummaryScreen) Update(msg tea.Msg) bool {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch keyMsg.String() {
	case "up", "k":
		if s.scrollOffset > 0 {
			s.scrollOffset--
		}
	case "down", "j":
		if s.scrollOffset < s.maxOffset() {
			s.scrollOffset++
		}
	default:
		s.done = true
		return true
	}
	return false
}

// Done reports that the user dismissed the screen.
func (s *PromoteBatchSummaryScreen) Done() bool { return s.done }

// View renders the screen.
func (s *PromoteBatchSummaryScreen) View() string {
	title, border, help := s.frame()
	lines := s.wrapped()
	visible := s.visibleLines(len(lines))

	start := min(s.scrollOffset, max(0, len(lines)-visible))
	end := min(start+visible, len(lines))

	parts := []string{title, border}
	parts = append(parts, lines[start:end]...)
	if len(lines) > visible {
		parts = append(parts, s.styles.Muted.Width(s.width).Render(
			fmt.Sprintf("── %d–%d of %d lines ──", start+1, end, len(lines))))
	}
	parts = append(parts, help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// Resize updates the terminal size.
func (s *PromoteBatchSummaryScreen) Resize(width, height int) {
	s.width = width
	s.height = height
	if m := s.maxOffset(); s.scrollOffset > m {
		s.scrollOffset = m
	}
}
