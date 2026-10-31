package screens

// transform_summary.go declares TransformSummaryScreen, the completion screen shown after
// a transform run. It mirrors PromoteSummaryScreen's method set so rootModel drives both
// the same way.

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
)

// transformLine is one rendered line of the transform summary together with the
// style role it must be drawn in. Mirrors summaryLine in summary.go.
type transformLine struct {
	text  string
	style string // "success" | "warning" | "error" | "muted" | "help" | "" (body)
}

// TransformSummaryScreen renders the outcome of a transform run. It mirrors
// PromoteSummaryScreen's method set so rootModel drives both the same way, but reads a
// TransformHarnessResult rather than a PromoteResult.
type TransformSummaryScreen struct {
	result app.TransformHarnessResult
	done   bool
	width  int
	height int
	styles Styles
}

// NewTransformSummaryScreen builds the screen from a completed transform result.
func NewTransformSummaryScreen(result app.TransformHarnessResult, width, height int, styles Styles) *TransformSummaryScreen {
	return &TransformSummaryScreen{
		result: result,
		width:  width,
		height: height,
		styles: styles,
	}
}

// Update handles key input and reports whether the user dismissed the screen.
func (s *TransformSummaryScreen) Update(msg tea.Msg) bool {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "enter", "esc":
			s.done = true
		}
	}
	return s.done
}

// Done reports that the user dismissed the screen.
func (s *TransformSummaryScreen) Done() bool { return s.done }

// overallOutcomeStyle returns the style tag for the overall run outcome: "error" when any
// file failed, "warning" when files were skipped but none failed, "success" otherwise.
func (s *TransformSummaryScreen) overallOutcomeStyle() string {
	r := s.result
	if r.Failed > 0 {
		return "error"
	}
	if r.Skipped > 0 {
		return "warning"
	}
	return "success"
}

// renderTransformLine applies the named style role to one transform summary line,
// following the sibling renderSummaryLine pattern from summary.go.
func (s *TransformSummaryScreen) renderTransformLine(line transformLine) string {
	switch line.style {
	case "success":
		return s.styles.Success.Width(s.width).Render(line.text)
	case "warning":
		return s.styles.Warning.Width(s.width).Render(line.text)
	case "error":
		return s.styles.Error.Width(s.width).Render(line.text)
	case "muted":
		return s.styles.Muted.Width(s.width).Render(line.text)
	case "help":
		return s.styles.Help.Width(s.width).Render(line.text)
	default:
		return s.styles.Body.Width(s.width).Render(line.text)
	}
}

// Resize adjusts the screen to new terminal dimensions.
func (s *TransformSummaryScreen) Resize(width, height int) {
	s.width = width
	s.height = height
}
