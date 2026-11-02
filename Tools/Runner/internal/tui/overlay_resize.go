package tui

// resizeOverlays lays the open question overlay out for the current terminal
// size, keeping the state the user has entered so far.
func (m *rootModel) resizeOverlays() {
	if m.selectOverlay != nil {
		m.selectOverlay.resize(m.width, m.height)
	}
	if m.multiOverlay != nil {
		m.multiOverlay.resize(m.width, m.height)
	}
	if m.textOverlay != nil {
		m.textOverlay.resize(m.width, m.height)
	}
	if m.confirmOverlay != nil {
		m.confirmOverlay.resize(m.width)
	}
}

func (s *inlineSelectOne) resize(width, height int) {
	s.width = width
	s.height = height
}

func (c *inlineConfirm) resize(width int) {
	c.width = width
}

func (t *inlineText) resize(width, height int) {
	t.width = width
	t.height = height
	if t.area != nil {
		t.area.Resize(multilineSize(t.q, width, height))
		return
	}
	t.field.SetWidth(width)
}
