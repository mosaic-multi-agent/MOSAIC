package runflow

// progress_help_test.go verifies the progress screen's help bar: every key that
// works in the normal state is listed, with and without standing guidance, and
// the guidance version differs from the plain one only by the guidance hint.

import (
	"strings"
	"testing"
)

const (
	wantHelpBarPlain    = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  a artifact-view  ctrl+c force-quit  l activity  x force-stop  up/down/pgup/pgdn scroll  home/end oldest/newest"
	wantHelpBarGuidance = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  e guidance  a artifact-view  ctrl+c force-quit  l activity  x force-stop  up/down/pgup/pgdn scroll  home/end oldest/newest"
)

// normalStateHints are the hints of every key that works with no stop pending
// and no prompt open, excluding the guidance key.
var normalStateHints = []string{
	"s graceful-stop",
	"m manual-dispatch",
	"h hitl-next",
	"g loop-guard",
	"a artifact-view",
	"ctrl+c force-quit",
	"l activity",
	"x force-stop",
}

func newHelpScreen(guidance bool) *ProgressScreen {
	s := NewProgressScreen(200, 30, progressStyles())
	s.SetGuidanceAvailable(guidance)
	return s
}

func TestProgressScreen_HelpBar_WithoutGuidance_IsTheFullPlainBar(t *testing.T) {
	s := newHelpScreen(false)

	if got := s.helpBar(); got != wantHelpBarPlain {
		t.Errorf("helpBar() = %q, want %q", got, wantHelpBarPlain)
	}
}

func TestProgressScreen_HelpBar_WithGuidance_IsThePlainBarWithGuidanceHintInserted(t *testing.T) {
	s := newHelpScreen(true)

	if got := s.helpBar(); got != wantHelpBarGuidance {
		t.Errorf("helpBar() = %q, want %q", got, wantHelpBarGuidance)
	}
}

func TestProgressScreen_HelpBar_ListsEveryNormalStateKeyInBothVersions(t *testing.T) {
	for _, guidance := range []bool{false, true} {
		name := "without guidance"
		if guidance {
			name = "with guidance"
		}
		t.Run(name, func(t *testing.T) {
			s := newHelpScreen(guidance)

			view := s.View()

			for _, hint := range normalStateHints {
				if !strings.Contains(view, hint) {
					t.Errorf("View() help bar lacks %q:\n%s", hint, view)
				}
			}
		})
	}
}

func TestProgressScreen_HelpBar_ListsTheScrollKeysInBothVersions(t *testing.T) {
	for _, guidance := range []bool{false, true} {
		bar := newHelpScreen(guidance).helpBar()

		for _, hint := range []string{"up/down/pgup/pgdn scroll", "home/end oldest/newest"} {
			if !strings.Contains(bar, hint) {
				t.Errorf("helpBar() (guidance=%v) lacks %q: %q", guidance, hint, bar)
			}
		}
	}
}

func TestProgressScreen_HelpBar_GuidanceHintOnlyWithGuidance(t *testing.T) {
	with := newHelpScreen(true).View()
	without := newHelpScreen(false).View()

	if !strings.Contains(with, "e guidance") {
		t.Errorf("View() with guidance lacks %q:\n%s", "e guidance", with)
	}
	if strings.Contains(without, "e guidance") {
		t.Errorf("View() without guidance shows %q:\n%s", "e guidance", without)
	}
}

func TestProgressScreen_HelpBar_VersionsDifferOnlyByTheGuidanceHint(t *testing.T) {
	plain := newHelpScreen(false).helpBar()
	guided := newHelpScreen(true).helpBar()

	if got := strings.Replace(guided, "e guidance  ", "", 1); got != plain {
		t.Errorf("guidance bar minus the guidance hint = %q, want the plain bar %q", got, plain)
	}
}
