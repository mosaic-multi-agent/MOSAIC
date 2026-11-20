package screens

// promoteagents.go implements PromoteAgentScreen, the multi-select list of the promotable
// agents of a batch promote (the QPromoteAgents question).
//
// Navigation contract, as on the Deploy agents list:
//   - up/down or k/j move the cursor.
//   - Space toggles the focused agent.
//   - Enter confirms a non-empty selection; an empty one shows a validation message.
//   - Esc goes back without confirming.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/widgets"
)

// PromoteAgentScreen is the batch promote agent list.
type PromoteAgentScreen struct {
	opts     []domain.Option
	multisel *widgets.MultiSelect
	detail   *widgets.DetailPane

	validationMsg string
	done          bool
	back          bool
	width         int
	height        int
	styles        Styles
}

// NewPromoteAgentScreen builds the batch promote agent list from the QPromoteAgents options,
// in option order. Each row shows Option.Label, plus Option.Hint when non-empty; the detail
// pane shows Option.Description and Option.Hint for the focused row.
func NewPromoteAgentScreen(opts []domain.Option, width, height int, styles Styles) *PromoteAgentScreen {
	items := make([]widgets.ListItem, len(opts))
	for i, o := range opts {
		label := o.Label
		if label == "" {
			label = o.ID
		}
		if o.Hint != "" {
			label += "  (" + o.Hint + ")"
		}
		items[i] = widgets.ListItem{ID: o.ID, Label: label}
	}
	lw, dw := splitListWidths(width)
	lh := uaListHeight(height)
	s := &PromoteAgentScreen{
		opts: opts,
		multisel: widgets.NewMultiSelect(items, lh, lw, widgets.MultiSelectStyles{
			Normal:   styles.Body,
			Selected: styles.Selected,
			Checked:  styles.Checked,
			Disabled: styles.Muted,
			Cursor:   "▶",
			CheckOn:  "[x]",
			CheckOff: "[ ]",
		}),
		detail: widgets.NewDetailPane(lh, dw, widgets.DetailPaneStyles{
			Title:  styles.Subtitle,
			Body:   styles.Body,
			Border: styles.Border,
			Empty:  styles.Muted,
		}),
		width:  width,
		height: height,
		styles: styles,
	}
	s.updateDetail()
	return s
}

// Update handles a message: up/down and k/j move, space toggles, enter confirms a non-empty
// selection, esc goes back.
func (s *PromoteAgentScreen) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch keyMsg.Type {
	case tea.KeyEsc:
		s.back = true
		return nil
	case tea.KeyEnter:
		if len(s.SelectedIDs()) > 0 {
			s.done = true
		} else {
			s.validationMsg = MsgNoAgentsSelected
		}
		return nil
	}
	cmd := s.multisel.Update(keyMsg)
	if s.multisel.Done() || s.multisel.Back() {
		s.multisel.Reset()
	} else {
		s.validationMsg = ""
		s.updateDetail()
	}
	return cmd
}

// updateDetail shows the description and hint of the focused row in the detail pane.
func (s *PromoteAgentScreen) updateDetail() {
	idx := s.multisel.CursorIndex()
	if idx < 0 || idx >= len(s.opts) {
		s.detail.SetContent("", "")
		return
	}
	o := s.opts[idx]
	title := o.Label
	if title == "" {
		title = o.ID
	}
	body := o.Description
	if o.Hint != "" {
		if body != "" {
			body += "\n"
		}
		body += o.Hint
	}
	s.detail.SetContent(title, body)
}

// View renders the screen.
func (s *PromoteAgentScreen) View() string {
	title := s.styles.Title.Width(s.width).Render("Select the agents to promote")
	border := s.styles.Border.Width(s.width).Render(strings.Repeat("─", s.width))
	summary := s.renderSummary()
	validation := validationLine(s.styles.Error, s.width, s.validationMsg)
	help := s.styles.Help.Width(s.width).Render(
		"↑/k up  ↓/j down  space toggle  enter confirm  esc cancel  ctrl+c quit",
	)

	if len(s.opts) == 0 {
		empty := s.styles.Muted.Width(s.width).Render("No promotable agents were found in the folder.")
		return strings.Join([]string{title, border, empty, border, help}, "\n")
	}

	fitMultiSelect(s.multisel, s.detail, s.width, s.height, len(s.opts), title, border, border, summary, validation, help)
	lw, _ := splitListWidths(s.width)
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(lw).Render(s.multisel.View()),
		s.styles.Border.Render("│"),
		s.detail.View(),
	)
	parts := []string{title, border, content, border, summary}
	if validation != "" {
		parts = append(parts, validation)
	}
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

// renderSummary is the persistent selection summary line below the list.
func (s *PromoteAgentScreen) renderSummary() string {
	ids := s.SelectedIDs()
	if len(ids) == 0 {
		return oneLineSummary(s.styles.Muted, s.width, "Selected 0 agents.")
	}
	noun := "agent"
	if len(ids) > 1 {
		noun = "agents"
	}
	return oneLineSummary(s.styles.Success, s.width,
		fmt.Sprintf("Selected %d %s: %s", len(ids), noun, strings.Join(ids, ", ")))
}

// Done reports that enter confirmed a non-empty selection.
func (s *PromoteAgentScreen) Done() bool { return s.done }

// Back reports that esc was pressed.
func (s *PromoteAgentScreen) Back() bool { return s.back }

// SelectedIDs returns the Option.IDs of the selected rows, in option order.
func (s *PromoteAgentScreen) SelectedIDs() []string { return s.multisel.SelectedIDs() }

// Reset clears the done and back flags.
func (s *PromoteAgentScreen) Reset() {
	s.done = false
	s.back = false
}

// Resize updates the terminal size.
func (s *PromoteAgentScreen) Resize(width, height int) {
	s.width = width
	s.height = height
	lw, dw := splitListWidths(width)
	lh := uaListHeight(height)
	s.multisel.Resize(lh, lw)
	s.detail.Resize(lh, dw)
}
