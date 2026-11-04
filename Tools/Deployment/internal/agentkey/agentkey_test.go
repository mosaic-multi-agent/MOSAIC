package agentkey_test

import (
	"testing"

	"mosaic-deploy/internal/agentkey"
)

func TestFromFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"agent md suffix is removed whole", "x.agent.md", "x"},
		{"plain md suffix is removed", "x.md", "x"},
		{"neither suffix leaves the name unchanged", "x", "x"},
		{"bare agent without md is not stripped", "x.agent", "x.agent"},
		{"other extension is not stripped", "x.txt", "x.txt"},
		{"agent in the middle is kept", "my.agent.notes.md", "my.agent.notes"},
		{"only one agent md suffix is removed", "x.agent.agent.md", "x.agent"},
		{"dotted key with md suffix", "a.b.md", "a.b"},
		{"hyphenated key with agent md", "orchestrator-script.agent.md", "orchestrator-script"},
		{"matching is case sensitive for md", "x.MD", "x.MD"},
		{"matching is case sensitive for agent md", "x.Agent.md", "x.Agent"},
		{"empty name stays empty", "", ""},
		{"md only stays as the key is empty", ".md", ""},
		{"agent md only gives empty key", ".agent.md", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := agentkey.FromFileName(tc.in)
			if got != tc.want {
				t.Errorf("FromFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
