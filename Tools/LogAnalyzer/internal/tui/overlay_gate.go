package tui

// An open help or findings overlay takes every key, so the app-level hooks of
// the runs list and the digest export must not see them.

// runsOverlayOpen reports whether the help or the findings overlay of the runs
// list is showing.
func (m Model) runsOverlayOpen() bool {
	return m.runsScreen.HelpOpen() || m.runsScreen.FindingsOpen()
}

// findingsOpen reports whether the view has a findings overlay and it is showing.
func findingsOpen(view any) bool {
	f, ok := view.(interface{ FindingsOpen() bool })
	return ok && f.FindingsOpen()
}
