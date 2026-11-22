package runflow

import "strings"

// helpHint is one key hint of the progress screen help bar.
type helpHint struct {
	text         string
	needGuidance bool // listed only when the session holds standing guidance
}

// progressHelpHints lists, in display order, every key that works on the
// progress screen in the normal state. Both help bar versions are built from it.
var progressHelpHints = []helpHint{
	{text: "s graceful-stop"},
	{text: "m manual-dispatch"},
	{text: "h hitl-next"},
	{text: "g loop-guard"},
	{text: "e guidance", needGuidance: true},
	{text: "a artifact-view"},
	{text: "ctrl+c force-quit"},
	{text: "l activity"},
	{text: "x force-stop"},
	{text: "up/down/pgup/pgdn scroll"},
	{text: "home/end oldest/newest"},
}

// buildHelpBar joins the applicable hints with the screen's two-space separator.
func buildHelpBar(guidanceAvailable bool) string {
	texts := make([]string, 0, len(progressHelpHints))
	for _, h := range progressHelpHints {
		if h.needGuidance && !guidanceAvailable {
			continue
		}
		texts = append(texts, h.text)
	}
	return strings.Join(texts, "  ")
}

// helpBar returns the help bar text for the current state.
func (s *ProgressScreen) helpBar() string { return buildHelpBar(s.guidanceAvailable) }
