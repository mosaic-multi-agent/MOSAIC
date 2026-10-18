package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/pathinput"
	"mosaic-deploy/internal/tui/widgets"
)

// defaultRowMarker labels the bottom row, which stands for the MOSAIC default catalog.
const defaultRowMarker = "(default)"

// StackScreen is the catalog-stack entry screen. The user builds an ordered stack of catalogs
// (add by path, remove, reorder) and keeps or leaves out the MOSAIC default catalog, which is
// always listed as the bottom row.
//
// Navigation contract (same style as WorkspaceScreen):
//   - Enter outside path entry -> Done() == true, Description() returns the stack. Refused
//     inline while the stack would have no layer at all.
//   - Esc outside path entry   -> Back() == true.
//   - Esc inside path entry    -> cancels the entry only.
//
// Keys: up/down move the cursor; a starts path entry; d removes the user layer under the cursor;
// space keeps or leaves out the default layer when the cursor is on its row; shift+up and
// shift+down move the user layer under the cursor one step in precedence.
type StackScreen struct {
	start         stackconfig.Description
	layers        []string
	includeDef    bool
	cursor        int
	entering      bool
	input         *widgets.TextInput
	identities    map[string]domain.CatalogIdentity
	defaultID     domain.CatalogIdentity
	done          bool
	back          bool
	inlineMessage string
	notice        string
	width         int
	height        int
	styles        Styles
}

// NewStackScreen builds the screen from a starting stack.
func NewStackScreen(start stackconfig.Description, width, height int, styles Styles) *StackScreen {
	s := &StackScreen{width: width, height: height, styles: styles}
	s.input = s.newPathInput()
	s.Reset(start)
	return s
}

func (s *StackScreen) newPathInput() *widgets.TextInput {
	in := widgets.NewTextInput("Catalogue folder path:", "/path/to/catalog", s.width, widgets.TextInputStyles{
		Label:  s.styles.Subtitle,
		Input:  s.styles.Body,
		ErrMsg: s.styles.Error,
	})
	in.SetValidate(validateCatalogFolderPath)
	return in
}

// SetIdentities supplies display identities for the rows: byRoot maps a user layer's root path to
// its identity and defaultLayer is the default catalog's identity. It replaces any earlier call.
func (s *StackScreen) SetIdentities(byRoot map[string]domain.CatalogIdentity, defaultLayer domain.CatalogIdentity) {
	s.identities = byRoot
	s.defaultID = defaultLayer
}

// SetNotice sets the migration notice shown above the layer rows; "" clears it. The notice stays
// until it is changed, also across Reset and edits of the stack.
func (s *StackScreen) SetNotice(msg string) { s.notice = msg }

// Reset restarts the screen from a new starting stack and clears the done and back flags.
// Identities supplied earlier are kept until the next SetIdentities call.
func (s *StackScreen) Reset(start stackconfig.Description) {
	s.start = stackconfig.Description{
		UserLayers:     append([]string(nil), start.UserLayers...),
		IncludeDefault: start.IncludeDefault,
	}
	s.layers = append([]string(nil), start.UserLayers...)
	s.includeDef = start.IncludeDefault
	s.cursor = 0
	s.entering = false
	s.done, s.back = false, false
	s.inlineMessage = ""
	s.input = s.newPathInput()
}

// Resize updates the screen dimensions.
func (s *StackScreen) Resize(width, height int) {
	s.width, s.height = width, height
	s.input.Resize(width)
}

// Done reports whether the user confirmed a stack.
func (s *StackScreen) Done() bool { return s.done }

// Back reports whether the user asked to go back without confirming.
func (s *StackScreen) Back() bool { return s.back }

// SetExternalError shows msg inline and keeps the screen active; it clears on the next Update.
func (s *StackScreen) SetExternalError(msg string) {
	s.inlineMessage = msg
	s.done, s.back = false, false
}

// Description returns the stack as currently edited; after Done() it is the confirmed stack.
func (s *StackScreen) Description() stackconfig.Description {
	return stackconfig.Description{
		UserLayers:     append([]string(nil), s.layers...),
		IncludeDefault: s.includeDef,
	}
}

// Changed reports whether Description differs from the starting stack.
func (s *StackScreen) Changed() bool {
	if s.includeDef != s.start.IncludeDefault || len(s.layers) != len(s.start.UserLayers) {
		return true
	}
	for i, l := range s.layers {
		if l != s.start.UserLayers[i] {
			return true
		}
	}
	return false
}

// InputInit returns the command that starts the path field's cursor blink.
func (s *StackScreen) InputInit() tea.Cmd { return s.input.Init() }

// Update processes a message.
func (s *StackScreen) Update(msg tea.Msg) tea.Cmd {
	s.inlineMessage = ""
	if s.entering {
		return s.updateEntry(msg)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "up":
		s.cursor = max(s.cursor-1, 0)
	case "down":
		s.cursor = min(s.cursor+1, len(s.layers))
	case "shift+up":
		s.move(-1)
	case "shift+down":
		s.move(1)
	case "a":
		s.entering = true
		s.input = s.newPathInput()
		return s.input.Init()
	case "d":
		s.remove()
	case " ":
		if s.cursor == len(s.layers) {
			s.includeDef = !s.includeDef
		}
	case "enter":
		s.confirm()
	case "esc":
		s.back = true
	}
	return nil
}

func (s *StackScreen) updateEntry(msg tea.Msg) tea.Cmd {
	cmd := s.input.Update(msg)
	switch {
	case s.input.Back():
		s.entering = false
	case s.input.Done():
		s.addTyped()
	}
	return cmd
}

// addTyped adds the folder typed into the path field below the existing user layers.
func (s *StackScreen) addTyped() {
	abs, err := filepath.Abs(pathinput.Unquote(strings.TrimSpace(s.input.Value())))
	s.entering = false
	if err != nil {
		s.inlineMessage = "path is not valid: " + err.Error()
		return
	}
	for _, l := range s.layers {
		if l == abs {
			s.inlineMessage = "catalogue is already in the stack: " + abs
			return
		}
	}
	s.layers = append(s.layers, abs)
	s.cursor = len(s.layers) - 1
}

func (s *StackScreen) remove() {
	if s.cursor >= len(s.layers) {
		return
	}
	s.layers = append(s.layers[:s.cursor], s.layers[s.cursor+1:]...)
}

// move shifts the user layer under the cursor by delta rows; it never leaves the user rows.
func (s *StackScreen) move(delta int) {
	to := s.cursor + delta
	if s.cursor >= len(s.layers) || to < 0 || to >= len(s.layers) {
		return
	}
	s.layers[s.cursor], s.layers[to] = s.layers[to], s.layers[s.cursor]
	s.cursor = to
}

func (s *StackScreen) confirm() {
	if len(s.layers) == 0 && !s.includeDef {
		s.inlineMessage = "the stack needs at least one catalogue: add a folder or keep the default"
		return
	}
	s.done = true
}

// View renders the screen.
func (s *StackScreen) View() string {
	st := s.styles
	border := st.Border.Width(s.width).Render(strings.Repeat("─", s.width))
	parts := []string{
		st.Title.Width(s.width).Render("Catalogue Stack"),
		st.Subtitle.Width(s.width).Render("Build the stack of catalogues this session deploys from."),
		border,
		st.Muted.Width(s.width).Render("The top row has the highest precedence: when two layers supply the same item, the higher row wins.\n" +
			"The default catalog is always listed as the bottom row; leave it out to use only your own catalogues."),
	}
	if s.notice != "" {
		parts = append(parts, st.Body.Width(s.width).Render(s.notice))
	}
	parts = append(parts, s.rows()...)
	if s.entering {
		parts = append(parts, s.input.View())
	}
	if s.inlineMessage != "" {
		parts = append(parts, st.Error.Width(s.width).Render(s.inlineMessage))
	}
	help := "up/down move  a add  d remove  shift+up/down reorder  space keep/leave out default  enter confirm  esc back"
	if s.entering {
		help = "enter add folder  esc cancel"
	}
	parts = append(parts, border, st.Help.Width(s.width).Render(help))
	return strings.Join(parts, "\n")
}

func (s *StackScreen) rows() []string {
	var out []string
	for i, root := range s.layers {
		text := fmt.Sprintf("%d. %s", i+1, identityText(s.identities[root])+root)
		out = append(out, s.renderRow(i, text))
	}
	state := "[x] kept"
	if !s.includeDef {
		state = "[ ] left out"
	}
	text := fmt.Sprintf("%s %s %s", defaultRowMarker, identityText(s.defaultID), state)
	return append(out, s.renderRow(len(s.layers), strings.Join(strings.Fields(text), " ")))
}

func (s *StackScreen) renderRow(index int, text string) string {
	if index == s.cursor && !s.entering {
		return s.styles.Selected.Render("> " + text)
	}
	return s.styles.Body.Render("  " + text)
}

// identityText formats an identity as "id name version " (trailing space), or "" when the
// identity has no id.
func identityText(id domain.CatalogIdentity) string {
	if id.ID == "" {
		return ""
	}
	return strings.Join(strings.Fields(id.ID+" "+id.Name+" "+id.Version), " ") + "  "
}
