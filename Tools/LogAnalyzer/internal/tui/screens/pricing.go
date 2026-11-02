package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// PricingScreen lists every model in the current report with its priced /
// unpriced state, and lets the user select one to set or overwrite its price.
// It also still serves as the background context while pricing questions
// arrive through the Interaction port.
//
// Navigation contract:
//   - ↑/↓ (and k/j) move the cursor.
//   - Enter -> Done() == true; SelectedModel() returns the chosen model.
//   - Esc/q -> Back() == true.
type PricingScreen struct {
	models     []app.ModelPricingStatus
	configPath string
	statusMsg  string
	cursor     int
	offset     int // scroll offset, clamped inside View
	width      int
	height     int
	styles     Styles
	done       bool
	back       bool
	pageChrome
}

// NewPricingScreen creates the pricing screen over the given per-model pricing
// states. An empty or nil slice is valid and renders without panic.
func NewPricingScreen(models []app.ModelPricingStatus, configPath string, width, height int, styles Styles) *PricingScreen {
	return &PricingScreen{
		models:     models,
		configPath: configPath,
		width:      width,
		height:     height,
		styles:     styles,
	}
}

// SetModels replaces the model list (used after a reprice without re-navigating).
// The cursor is clamped into range.
func (s *PricingScreen) SetModels(models []app.ModelPricingStatus) {
	s.models = models
	if len(models) == 0 {
		s.cursor = 0
	} else if s.cursor >= len(models) {
		s.cursor = len(models) - 1
	}
}

// ID implements Screen.
func (s *PricingScreen) ID() ScreenID { return ScreenPricing }

// Init implements Screen.
func (s *PricingScreen) Init() tea.Cmd { return nil }

// SetStatus updates the status line shown below the model list. Used to
// confirm that a price was written.
func (s *PricingScreen) SetStatus(msg string) { s.statusMsg = msg }

// Done reports whether the user chose a model with Enter.
func (s *PricingScreen) Done() bool { return s.done }

// SelectedModel returns the highlighted model. Only meaningful when Done() is
// true; returns "" when the list is empty or the cursor is out of range.
func (s *PricingScreen) SelectedModel() domain.ModelID {
	if len(s.models) == 0 || s.cursor < 0 || s.cursor >= len(s.models) {
		return ""
	}
	return s.models[s.cursor].Model
}

// SelectedKey returns the highlighted (provider, model). Only meaningful when
// Done() is true; the zero key when the list is empty or the cursor is out of
// range.
func (s *PricingScreen) SelectedKey() domain.PricingKey {
	if len(s.models) == 0 || s.cursor < 0 || s.cursor >= len(s.models) {
		return domain.PricingKey{}
	}
	return statusKey(s.models[s.cursor])
}

// statusKey is the row's (provider, model); rows built without a Key identify
// their model only.
func statusKey(m app.ModelPricingStatus) domain.PricingKey {
	if m.Key.Model == "" {
		return domain.PricingKey{Model: m.Model}
	}
	return m.Key
}

// Back reports whether the user pressed Esc.
func (s *PricingScreen) Back() bool { return s.back }

// Reset clears the done and back flags.
func (s *PricingScreen) Reset() {
	s.done = false
	s.back = false
}

// Update processes key messages.
func (s *PricingScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	switch k {
	case "esc", "q":
		s.back = true
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.models)-1 {
			s.cursor++
		}
	case "enter":
		if len(s.models) > 0 {
			s.done = true
		}
	}
	return s, nil
}

// View renders the pricing screen: one row per (provider, model) with its
// priced or unpriced state, then the config file and the last status message.
func (s *PricingScreen) View(width, height int) string {
	s.width = width
	s.height = height
	st := s.styles

	head := []string{st.Subtitle.Render(fmt.Sprintf("%d model(s) in report", len(s.models))) +
		st.Body.Render("  currency "+string(CurrencyOrDefault(st.Currency)))}
	var tail []string
	if s.configPath != "" {
		tail = append(tail, st.Muted.Render("Config file: "+s.configPath))
	}
	if s.statusMsg != "" {
		tail = append(tail, st.Theme.Style(tuicommon.RoleSignalGood).Render(s.statusMsg))
	}

	rows := make([][]table.Cell, len(s.models))
	for i, m := range s.models {
		rows[i] = pricingCells(m)
	}
	budget := PageBodyHeight(height) - len(head) - len(tail) - 1
	t := table.Table{Columns: pricingColumns(), Rows: rows, Cursor: s.cursor, SortColumn: table.NoSort}
	w := ScrollWindow(s.offset, s.cursor, len(rows), budget-extraHeaderLines(t, width))
	s.offset = w.Offset
	t.Offset = w.Offset
	body := append(append(head, renderTableLines(t, width, w.Count, st)...), tail...)

	page := Page{Title: pricingTitle, Keys: pricingKeys, Help: true, Hint: FormatScrollHint(w, len(rows))}
	return s.render(page, withLegend(pricingOverlayGroups(), pricingLegend()...), body, width, height, st)
}

// Help returns the key hints for this screen.
func (s *PricingScreen) Help() string { return leadHelp("", pricingKeys) }
