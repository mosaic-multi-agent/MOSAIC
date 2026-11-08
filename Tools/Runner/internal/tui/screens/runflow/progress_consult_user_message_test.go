package runflow

// The consultation row of the progress screen shows that a routing
// consultation carries a user message, with and without a deviation kind.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestProgressScreen_View_ConsultationRowShowsUserMessage(t *testing.T) {
	cases := []struct {
		name     string
		activity domain.ConsultationActivity
		want     string
	}{
		{"routing without kind",
			domain.ConsultationActivity{Context: domain.ConsultContextRouting, HasUserMessage: true},
			"consultation: routing (user message)"},
		{"routing with kind",
			domain.ConsultationActivity{Context: domain.ConsultContextRouting, Kind: domain.DeviationHarnessError, HasUserMessage: true},
			"consultation: routing (harness-error, user message)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewProgressScreen(120, 24, progressStyles())

			s.StartConsultation(c.activity)

			if view := s.View(); !strings.Contains(view, "\u25b6 "+c.want) {
				t.Errorf("View() lacks the running consultation row %q:\n%s", "\u25b6 "+c.want, view)
			}
		})
	}
}

func TestProgressScreen_View_ConsultationRowWithoutUserMessage_NeverMentionsIt(t *testing.T) {
	for _, a := range []domain.ConsultationActivity{routingNoKind, routingHarnessKd, preConsultation} {
		s := NewProgressScreen(120, 24, progressStyles())

		s.StartConsultation(a)

		if view := s.View(); strings.Contains(view, "user message") {
			t.Errorf("View() mentions a user message for %+v:\n%s", a, view)
		}
	}
}
