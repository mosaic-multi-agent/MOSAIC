package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/widgets"
)

var modeItems = []widgets.ListItem{
	{
		ID:          string(domain.ModeDeployWorkspace),
		Label:       "Deploy workspace",
		Description: "Deploy agents, skills, and hooks to a workspace for the first time.",
		Detail: "Creates a fresh deployment in the workspace directory.\n" +
			"Use this when setting up a new project or a clean install.\n\n" +
			"The tool will ask you to select:\n" +
			"  • Workflows to include\n" +
			"  • Models for each agent tier\n" +
			"  • Any custom tools or hook bundles",
	},
	{
		ID:          string(domain.ModeUpdateWorkspace),
		Label:       "Update workspace",
		Description: "Update an existing workspace deployment to the latest source versions.",
		Detail: "Compares the deployed files against the current source versions.\n" +
			"Stale files are updated; locally-modified files prompt for a decision.\n\n" +
			"Infrastructure agent declarations in the orchestrator are refreshed when\n" +
			"their catalog definition changed; new or hand-added declarations are\n" +
			"never added or touched.\n\n" +
			"Hook bundles already deployed are refreshed when the catalog version is\n" +
			"newer (local edits lost); hooks not deployed are never added, and an\n" +
			"existing registration file is never modified (a TODO is reported).\n\n" +
			"Use this after updating the generic agents or when a harness\n" +
			"descriptor has changed.",
	},
	{
		ID:          string(domain.ModeUpdateWorkflows),
		Label:       "Update workflows",
		Description: "Replace the workflow set in an already-deployed orchestrator.",
		Detail: "Only the orchestrator file is rewritten — no agent, skill, or hook\n" +
			"artifact is touched, even if stale.\n\n" +
			"The selected workflow set fully replaces the currently deployed set;\n" +
			"workflows that are not reselected are removed from the orchestrator.\n\n" +
			"Use this to add, remove, or swap workflows without disturbing the rest\n" +
			"of the deployment.",
	},
	{
		ID:          string(domain.ModeDeployAgents),
		Label:       "Deploy agents",
		Description: "Deploy a chosen mix of subagents, utility agents, and standalone agents.",
		Detail: "Presents a single browsable list spanning all subagents (including\n" +
			"Infrastructure-category subagents), utility agents, and standalone agents.\n\n" +
			"Only the selected agents and the skills they require are deployed. No\n" +
			"workflow or hook work is performed.\n\n" +
			"If an orchestrator is already present, declarations for newly deployed\n" +
			"infrastructure agents are added to it. Existing declarations, including\n" +
			"hand-added ones, are kept. No orchestrator is created, and its workflows\n" +
			"are not changed.\n\n" +
			"Use this to add or update individual agents across any catalog category\n" +
			"without disturbing workflows or hooks.",
	},
	{
		ID:          string(domain.ModeDeployHooks),
		Label:       "Deploy hooks",
		Description: "Deploy hook bundles alone, without any agent, model, or workflow changes.",
		Detail: "Asks only which hook bundles to deploy.\n\n" +
			"No agent selection, model resolution, workflow configuration, or\n" +
			"orchestrator rewriting is performed — only the selected hook bundles\n" +
			"and their registration steps are executed.\n\n" +
			"Use this when you want to add or update hook bundles without touching\n" +
			"agents or the orchestrator.",
	},
	{
		ID:          string(domain.ModePromoteToGeneric),
		Label:       "Promote to generic",
		Description: "Generate a generic agent source file from an already-transformed harness-only agent.",
		Detail: "Reads a harness-only agent file (one with a harness_version or transform_version stamp and canonical\n" +
			"boundary tags) and generates a generic source file under the catalog root.\n\n" +
			"After promotion the new agent is immediately available to the Deploy/Update\n" +
			"flow. The original harness-only source file is left untouched.\n\n" +
			"Use this to turn a one-off harness-specific agent into a reusable generic\n" +
			"agent that can be deployed to any harness.",
	},
	{
		ID:          string(domain.ModeTransformHarness),
		Label:       "Transform harness",
		Description: "Move a workspace's whole deployment from one harness to another.",
		Detail: "Moves the whole deployment of a\n" +
			"workspace root (agents, skills,\n" +
			"hooks) to a target harness.\n\n" +
			"The plan is shown before anything is written, and you confirm it first.\n" +
			"Items that cannot be carried over are listed with the reason, and agents\n" +
			"left without a model are reported so you can set one.\n\n" +
			"Use this to migrate a deployment between harnesses without losing tool\n" +
			"mappings or injected content.",
	},
}

// ModeScreen presents the available run modes.
//
// Navigation contract:
//   - Enter -> Done() == true, SelectedMode() returns the chosen RunMode.
//   - Esc   -> Back() == true (caller returns to the harness selection screen, preserving
//              the previously selected harness so prior input is not lost).
type ModeScreen struct {
	list        *widgets.List
	detail      *widgets.DetailPane
	width       int
	height      int
	styles      Styles
	toolVersion string
}

// NewModeScreen creates the mode selection screen.
func NewModeScreen(width, height int, styles Styles) *ModeScreen {
	listStyles := widgets.ListStyles{
		Normal:   styles.Body,
		Selected: styles.Selected,
		Disabled: styles.Muted,
		Cursor:   "▶",
	}
	listHeight := height - 8
	if listHeight < 2 {
		listHeight = 2
	}
	listWidth := width * 2 / 3
	if listWidth < 30 {
		listWidth = width
	}
	detailWidth := width - listWidth - 1
	if detailWidth < 10 {
		detailWidth = 10
	}
	detailStyles := widgets.DetailPaneStyles{
		Title:  styles.Subtitle,
		Body:   styles.Body,
		Border: styles.Border,
		Empty:  styles.Muted,
	}

	list := widgets.NewList(modeItems, listHeight, listWidth, listStyles)
	detail := widgets.NewDetailPane(listHeight, detailWidth, detailStyles)

	s := &ModeScreen{
		list:   list,
		detail: detail,
		width:  width,
		height: height,
		styles: styles,
	}
	s.updateDetail()
	return s
}

// Update processes a key message and delegates to the list widget.
func (s *ModeScreen) Update(msg tea.Msg) tea.Cmd {
	cmd := s.list.Update(msg)
	if !s.list.Done() && !s.list.Back() {
		s.updateDetail()
	}
	return cmd
}

func (s *ModeScreen) updateDetail() {
	item, ok := s.list.SelectedItem()
	if !ok {
		s.detail.SetContent("", "")
		return
	}
	s.detail.SetContent(item.Label, item.Detail)
}

// SetToolVersion sets the tool version string displayed in the screen title.
// Call before the first View() for the version to appear on entry.
func (s *ModeScreen) SetToolVersion(v string) {
	s.toolVersion = v
}

// View renders the mode selection screen.
func (s *ModeScreen) View() string {
	titleText := "Select Mode"
	if s.toolVersion != "" {
		titleText = "MOSAIC Deploy v" + s.toolVersion + " -- Select Mode"
	}
	title := s.styles.Title.Width(s.width).Render(titleText)
	subtitle := s.styles.Subtitle.Width(s.width).Render("What do you want to do?")
	border := s.styles.Border.Width(s.width).Render(strings.Repeat("─", s.width))

	listWidth := s.width * 2 / 3
	if listWidth < 30 {
		listWidth = s.width
	}
	sep := s.styles.Border.Render("│")
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(listWidth).Render(s.list.View()),
		sep,
		s.detail.View(),
	)

	help := s.styles.Help.Width(s.width).Render("↑/k up  ↓/j down  enter select  esc back  ctrl+c quit")

	return strings.Join([]string{title, subtitle, border, content, border, help}, "\n")
}

// Done reports whether the user selected a mode.
func (s *ModeScreen) Done() bool { return s.list.Done() }

// Back reports whether the user pressed back.
func (s *ModeScreen) Back() bool { return s.list.Back() }

// SelectedMode returns the chosen RunMode. Only valid when Done() is true.
func (s *ModeScreen) SelectedMode() domain.RunMode {
	return domain.RunMode(s.list.SelectedID())
}

// Resize updates the screen dimensions.
func (s *ModeScreen) Resize(width, height int) {
	s.width = width
	s.height = height
	listHeight := height - 8
	if listHeight < 2 {
		listHeight = 2
	}
	listWidth := width * 2 / 3
	if listWidth < 30 {
		listWidth = width
	}
	detailWidth := width - listWidth - 1
	if detailWidth < 10 {
		detailWidth = 10
	}
	s.list.Resize(listHeight, listWidth)
	s.detail.Resize(listHeight, detailWidth)
}

// Reset clears the done and back flags.
func (s *ModeScreen) Reset() { s.list.Reset() }
