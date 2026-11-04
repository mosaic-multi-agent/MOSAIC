package tui

// The source screen tells the user that digest input is accepted.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func TestSourceScreen_Guidance_MentionsDigestInput(t *testing.T) {
	s := screens.NewSourceScreen(200, 24, plainStyles())

	view := strings.ToLower(s.View(200, 24))

	if !strings.Contains(view, "digest") {
		t.Errorf("source screen guidance should mention digest input; got:\n%s", view)
	}
}
