package setup

// The orchestrator-file screen reads the script through the shared agent
// loader, so its guidance must not require a Markdown file.

import (
	"strings"
	"testing"
)

func TestOrchestratorFileView_Guidance_DoesNotRequireMarkdownFile(t *testing.T) {
	view := newOrchestratorScreen().View()

	// Neither the guidance nor the path placeholder may point at a Markdown file.
	for _, forbidden := range []string{".md", "markdown"} {
		if strings.Contains(strings.ToLower(view), forbidden) {
			t.Errorf("the view still points at a Markdown file (contains %q):\n%s", forbidden, view)
		}
	}
}

func TestOrchestratorFileView_Guidance_StillAsksForAnExistingAgentFile(t *testing.T) {
	view := strings.ToLower(newOrchestratorScreen().View())

	if !strings.Contains(view, "existing") || !strings.Contains(view, "agent file") {
		t.Errorf("guidance must still tell the user the path has to be an existing agent file:\n%s", view)
	}
}
